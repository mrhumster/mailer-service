# mailer-service

Delivers GoCast's transactional email — currently only **address verification** — from the asynq
queue `emails`. An **asynq** worker scaled by **KEDA**: it idles at 0 replicas and wakes up when an
email task lands in the queue. Real delivery uses SMTP; a log-only fallback keeps the whole flow
exercisable without a mail server.

Built on the shared worker skeleton used by transcoder/thumbnail:

- `github.com/mrhumster/go-shared` (`worker`, `config`, `metrics`) via `replace => ../shared`
- asynq queue `emails` (Redis **DB 2**, KEDA `ScaledObject` on `asynq:{emails}:pending/active`)
- Prometheus `/metrics` on `METRICS_ADDR` (`:9090` in K8s), also used as the liveness probe

## How it works

1. identity-service creates a one-time verification token (Redis, TTL from `VERIFY_TOKEN_TTL`) on
   **register** and on **resend**, then enqueues a task into the asynq `emails` queue (Redis DB 2).
2. KEDA's `ScaledObject` (`mailer-keda`) watches the queue length and scales the Deployment from 0.
3. The worker decodes the payload, renders the verification email, and hands it to the selected
   sender (`SMTPSender` or `LogSender`).
4. KEDA scales back to 0 when the queue drains (normal, not a fault).

The producer enqueues **best-effort**: a Redis/asynq failure only logs a warning, the register/resend
request still succeeds, and the token is written to the identity log as a fallback. The mailer
itself **never** stores tokens or talks to the database — it only renders and sends.

## Message types

| Task | Queue | Payload |
| --- | --- | --- |
| `email:verification` | `emails` (priority 6) | `{user_id, email, token}` |
| `email:download_ready` | `emails` (priority 6) | `{user_id, email, stream_id, stream_url, size}` |

`EmailVerificationPayload` (`internal/queue/payload.go`): `user_id uuid.UUID`, `email string`,
`token string`. The rendered link is `${FRONTEND_URL}/verify?token=<token>`; trailing slashes on
`FRONTEND_URL` are trimmed.

`EmailDownloadReadyPayload`: `user_id`, `email`, `stream_id`, `stream_url`, `size int64`.
The producer is the **export worker** (`services/transcoder-service/cmd/exporter`), which enqueues
after the MP4 is in storage and stream-service has been told the export succeeded. It builds
`stream_url` itself (`${FRONTEND_URL}/streams/<id>`) because it owns `FRONTEND_URL`; this service
only renders. `size` is optional and formatted for humans (`1.3 GiB`), omitted when `<= 0`.

## Modes

The transport is chosen in `cmd/worker/main.go` from one variable:

- **`SMTP_ADDR` set → real delivery** via `mailer.NewSMTPSender` (`SMTP_USER`/`SMTP_PASS`/`SMTP_FROM`).
- **`SMTP_ADDR` empty → log mode**: `mailer.LogSender` prints the full rendered message (to, subject,
  body) via `slog` and returns success. This is the dev default.

### SMTP details

`SMTPSender` runs a full client session (`MAIL`/`RCPT`/`DATA`/`QUIT`) rather than `smtp.SendMail`:

- **Port 465 → implicit TLS** (`tls.Dial` + `smtp.NewClient`, TLS 1.2 min, SNI = host).
- **Any other port → plain dial, then STARTTLS** only if the server advertises the extension
  (587 / 25). Falls back to a plaintext session if STARTTLS is not offered.
- **Auth only when `SMTP_USER` is non-empty** (`smtp.PlainAuth`); system CA roots are used.
- The `From` header/envelope and the recipient must be non-empty or `Send` errors.
- A `QUIT` failure after a successful `DATA` is **not** treated as a delivery failure (the server
  often closes the connection late); the message counts as sent.

`SMTP_FROM` should be the authorized mailbox — Gmail/Yandex rewrite `From` otherwise. Gmail requires
an **App Password** (the mailbox account password is rejected by `net/smtp`); generate one in
Google Account → Security → App passwords.

## Retry classification

The queue handler decides retryability per stage (`internal/queue/handler.go`):

- JSON decode failure → `asynq.SkipRetry` (payload is permanently malformed).
- Email-construction failure → `asynq.SkipRetry` (empty `email`/`token`).
- Sender failure → **retryable** (wrapped, not skipped) — transient SMTP/network errors are retried.

The producer enqueues with `asynq.MaxRetry(3)` and `asynq.Timeout(30*time.Second)`, so a single email
gets at most 4 attempts. A send error increments `mailer_sent_total{status="error"}`; only a
successful send increments `success` and observes `mailer_duration_seconds`.

## Metrics

Exposed via `METRICS_ADDR` (empty = off) through `promhttp`:

- `mailer_sent_total{status="success|error"}` — counter, emails handled by outcome
- `mailer_duration_seconds` — histogram, delivery duration (observed **on success only**)

Plus the shared asynq metrics from `go-shared/metrics`: `asynq_task_processed_total`,
`asynq_task_duration_seconds`, `asynq_task_inflight`. The Deployment carries
`prometheus.io/{scrape,port,path}` annotations and an HTTP liveness probe on `/metrics` (not the old
`ps aux` exec). Because KEDA holds the pod at 0 replicas when idle, the asynq and business series
disappear from Prometheus between bursts — empty panels are expected, not a broken pipeline.

## Producer contract

`internal/notifier/notifier.go` enqueues the task; the flow lives in the verification service and the
register/resend handlers:

- Both register and resend build the token, then enqueue best-effort (failure = warning, request
  still succeeds, raw token written to the identity log as fallback).
- The token is stored in Redis (`verify:<token>` → user id, TTL from `VERIFY_TOKEN_TTL`); the mailer
  never sees storage, only the token string in the payload.

The email copy says the link is valid for **2 hours**; that TTL is owned by identity-service, not this
worker. Keep them in sync if `VERIFY_TOKEN_TTL` is ever changed.

### transcoder-service (export worker)

`HandleEmailDownloadReady` renders the "your video is ready" mail. The link points at the **stream
page**, not at the file: downloads are owner-only and go through stream-service, so a direct object
URL would be both unusable for a guest and trivially shareable. The exporter enqueues *after* the
gRPC callback and swallows its own failures, so this service can be down during an export without
breaking it — the file is downloadable from the web either way.

## Configuration

Non-secret variables arrive through the ConfigMap `mailer-service-config`, rendered by
`scripts/render-env.sh` in the `gocast-infra` root repo; the SMTP password is a Secret. Sources
(`deploy/k8s/mailer/deployment.yaml`):

| Source | Variables |
| --- | --- |
| ConfigMap `mailer-service-config` (rendered by `render-env.sh`) | `REDIS_ADDR`, `REDIS_DB` (2), `WORKER_CONCURRENCY` (1), `WORKER_SHUTDOWN_TIMEOUT` (50m), `METRICS_ADDR`, `SMTP_ADDR`, `SMTP_USER`, `SMTP_FROM`, `FRONTEND_URL` |
| Secret `casbin-redis` | Redis password (lowercase key `redis-password`) |
| Secret `smtp-credentials` | `SMTP_PASS` (see below) |

The Mail block is read by `sharedconfig.LoadConfig()` (`SMTP_*` + `FRONTEND_URL`, default
`https://example.com`); the full shared variable list lives in `services/shared/README.md`.

### SMTP credentials (never committed)

The password is applied imperatively so it stays out of git and out of re-applied manifests:

```bash
make smtp-secret SMTP_PASS='<app-password>'
```

This creates/updates the `smtp-credentials` Secret in `go-app` with a single `SMTP_PASS` key. The
Secret is **not** part of `deploy/` — apply it once per environment.

## Project layout

```
mailer-service/
├── cmd/worker/main.go           # asynq server wiring, sender selection, metrics address
├── internal/
│   ├── mailer/                  # Email model, verification template, SMTPSender, LogSender
│   │   ├── email.go             # Email + EmailSender + NewVerificationEmail
│   │   ├── smtp_sender.go       # full SMTP session (implicit TLS 465 / STARTTLS)
│   │   └── log_sender.go        # dev log-only transport
│   ├── metrics/metrics.go       # mailer_sent_total, mailer_duration_seconds
│   └── queue/                   # payload + asynq handler (+ unit tests)
├── deploy/k8s/
│   ├── keda/scaledobject.yaml   # scale 0..1 on queue length (DB 2)
│   └── mailer/{deployment,service}.yaml
└── Dockerfile                   # alpine:3.18 + ca-certificates, non-root appuser (uid 1000)
```

Module: `github.com/mrhumster/mailer-service`, with
`replace github.com/mrhumster/go-shared => ../shared`. The Dockerfile build context is the parent
`services/` directory (it also needs `../shared`).

## Commands

```bash
make build    # docker build (VERSION/BUILD_DATE from git describe), tags VERSION and latest
make push     # push both tags
make deploy   # kubectl set image deployment/mailer-service (needs KUBECONFIG)
make test     # go test -v ./...
make logs     # tail worker logs
make clean    # remove local mailer-worker binary
```

Notes:

- `make build` builds only inside Docker; there is no local `go build` target, so `make clean` is
  normally a no-op (the binary it removes is gitignored and only produced by an out-of-band build).
- `make deploy` only runs `kubectl set image`; it does **not** wait for rollout. Because the image is
  pinned to `git describe --tags --always`, rebuilding an unchanged working tree reuses the same tag
  and `set image` becomes a no-op — use
  `kubectl rollout restart deployment/mailer-service -n go-app` in that case.
- `make logs` uses selector `-l app=mail`, but the Deployment/pods are labelled **`app=mailer-service`**
  — the log target as written matches nothing; use `-l app=mailer-service`.
- The Deployment references the `smtp-credentials` Secret; if it is missing, the pod stays in
  configuration-error state (`CreateContainerConfigError`).

## Tests

```bash
go test ./...
```

Covers the verification-email render/validation, base-URL trailing-slash trim, RFC5322 message
assembly, host/port detection (465 vs not), the asynq handler (success, send-error retry,
skip-retry on bad payload), and the log sender.

A real SMTP round-trip test is available but **skipped by default**. Enable it with
`SMOKE_SMTP_ADDR` set, `SMOKE_SMTP_PASS_FILE` pointing to a file containing the password (keeps the
secret off the command line), and `SMOKE_SMTP_FROM` as the sender:

```bash
SMOKE_SMTP_ADDR=smtp.gmail.com:465 \
SMOKE_SMTP_PASS_FILE=/path/to/pass \
SMOKE_SMTP_FROM=you@gmail.com \
go test ./internal/mailer/ -run TestSMTPSenderSmoke -v
```

## Deployment

```
deploy/k8s/
├── keda/
│   └── scaledobject.yaml   # mailer-keda: scale 0..1 on asynq:{emails}:pending/active
└── mailer/
    ├── deployment.yaml     # image xomrkob/mailer-service:<tag>, metrics :9090
    └── service.yaml        # ClusterIP service for metrics scraping
```

KEDA polls Redis DB 2 lists `asynq:{emails}:pending` (list) and `asynq:{emails}:active` (zset),
scaling when either exceeds `listLength: 2`, min 0 / max 1, with a 120s cooldown. Resources are
deliberately tiny (requests 50m CPU / 128Mi, limits 500m / 256Mi) for a 4-core homelab node.
`terminationGracePeriodSeconds: 120` and no `preStop`, so scale-down is quick (transcoder uses 300s +
`preStop`, thumbnail 3600s).

The ScaledObject references a `keda-redis-auth` TriggerAuthentication. This service ships only the
`scaledobject.yaml`; the auth object is currently provided by
`services/transcoder-service/deploy/k8s/keda/auth.yaml` (uses the `casbin-redis` secret). Apply it
(or an equivalent) before the ScaledObject.

## Known issues / caveats

- **`make logs` selector is wrong** (`app=mail` vs the real `app=mailer-service`).
- **IPv6 SMTP addresses are mis-parsed.** `host()`/`port()` split `SMTP_ADDR` on the last `:`, so an
  IPv6 literal like `[::1]:587` is read incorrectly. Use hostnames.
- **`Send` ignores `context`.** The producer's 30s Asynq timeout cannot interrupt a blocked SMTP
  call; a hung server blocks the worker until the OS TCP timeout.
- **The verification token is not URL-escaped** when interpolated into the link. Tokens are 64-hex
  today so this is safe in practice, but a token with reserved characters would break the URL.
- **Mailer-owned TTL copy.** The email says the link is valid 2 hours; the real TTL is identity's
  `VERIFY_TOKEN_TTL` (default 2h). Changing the TTL requires updating the template too.
- **The 2-hour TTL / one-time use lives in identity-service** (Redis `verify:<token>`), not here; the
  mailer neither validates nor persists tokens, so a delivered email stays valid for as long as
  identity's Redis entry survives.
