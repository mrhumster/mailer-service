package mailer

import (
	"context"
	"log/slog"
)

// LogSender is the development fallback used when no SMTP server is
// configured (SMTP_ADDR empty). It logs the full rendered message instead of
// delivering it, so flows can be exercised end-to-end without a mail server.
type LogSender struct{}

func (LogSender) Send(_ context.Context, msg Email) error {
	slog.Info("email (log sender)",
		"to", msg.To,
		"subject", msg.Subject,
		"body", msg.Body)
	return nil
}