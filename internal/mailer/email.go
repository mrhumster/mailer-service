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