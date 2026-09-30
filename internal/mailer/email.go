package mailer

import (
	"context"
	"fmt"
	"strings"
)

type Email struct {
	To      string
	Subject string
	Body    string
}

// EmailSender delivers outgoing messages. The SMTP implementation performs an
// actual send; the transport is an interface so tests can use a fake. A send
// error means the message was not delivered and the caller should retry.
type EmailSender interface {
	Send(ctx context.Context, msg Email) error
}

// NewVerificationEmail builds the verification email which points the user to
// the frontend /verify page carrying the one-time token.
func NewVerificationEmail(baseURL, to, token string) (Email, error) {
	if to == "" || token == "" {
		return Email{}, fmt.Errorf("verification email requires non-empty to and token")
	}
	verifyURL := fmt.Sprintf("%s/verify?token=%s", strings.TrimRight(baseURL, "/"), token)
	body := fmt.Sprintf(
		`<div style="font-family:monospace;max-width:480px;margin:0 auto;">
<p>Hi!</p>
<p>Please confirm your email to start publishing streams on GoCast:</p>
<p><a href="%[1]s" style="background:#111;color:#ffd23f;padding:10px 16px;text-decoration:none;">VERIFY EMAIL</a></p>
<p>Or copy this link into your browser:<br/><a href="%[1]s">%[1]s</a></p>
<p>The link is valid for 2 hours and can be used only once.</p>
</div>`,
		verifyURL,
	)
	return Email{
		To:      to,
		Subject: "Verify your GoCast email",
		Body:    body,
	}, nil
}

// NewDownloadReadyEmail tells the owner that the MP4 of their stream is ready.
// The link goes to the stream page, not straight at the file: the download is
// owner-only and the browser has to ask for it, so a one-shot signed URL would
// be both expiring and shareable.
func NewDownloadReadyEmail(to, streamURL string, size int64) (Email, error) {
	if to == "" || streamURL == "" {
		return Email{}, fmt.Errorf("download email requires non-empty to and stream url")
	}
	body := fmt.Sprintf(
		`<div style="font-family:monospace;max-width:480px;margin:0 auto;">
<p>Hi!</p>
<p>The MP4 of your stream is ready to download%[2]s:</p>
<p><a href="%[1]s" style="background:#111;color:#ffd23f;padding:10px 16px;text-decoration:none;">OPEN STREAM</a></p>
<p>Or copy this link into your browser:<br/><a href="%[1]s">%[1]s</a></p>
<p>Open the page and press the download button &mdash; the file is served from your
account, so the link works only while you are signed in as the owner.</p>
</div>`,
		streamURL,
		sizeSuffix(size),
	)
	return Email{
		To:      to,
		Subject: "Your GoCast video is ready to download",
		Body:    body,
	}, nil
}

// sizeSuffix renders the file size for humans, or an empty string when the
// producer did not report one.
func sizeSuffix(size int64) string {
	if size <= 0 {
		return ""
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf(" (%d B)", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf(" (%.1f %ciB)", float64(size)/float64(div), "KMGT"[exp])
}