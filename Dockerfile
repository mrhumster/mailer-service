FROM golang:1.25-alpine AS builder
ARG VERSION=0.0.1
ARG BUILD_DATE=11.03.2026

WORKDIR /app
COPY mailer-service/go.mod ./
RUN if [ -f mailer-service/go.sum ]; then cp mailer-service/go.sum .; fi
COPY shared /shared
RUN go mod download
COPY mailer-service/. .
RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags="-w -s -X main.version=$VERSION -X main.buildDate=$BUILD_DATE" \
  -o mailer-worker ./cmd/worker/main.go

FROM alpine:3.18
ARG VERSION=0.0.1
ARG BUILD_DATE=11.03.2026
LABEL version=$VERSION \
  build-date=$BUILD_DATE \
  maintainer="me@xomrkob.ru"
RUN apk add --no-cache ca-certificates
RUN addgroup -g 1000 appgroup && \
  adduser -D -u 1000 -G appgroup appuser
WORKDIR /app
COPY --from=builder --chown=appuser:appgroup /app/mailer-worker .
USER appuser
CMD ["/app/mailer-worker"]