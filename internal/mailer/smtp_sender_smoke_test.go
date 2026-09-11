package mailer

import (
	"context"
	"os"
	"testing"
)

// Dev-only smoke test: real SMTP round-trip, skipped unless SMOKE_SMTP_ADDR is
// set. SMOKE_SMTP_PASS_FILE points to a file with the password (keeps it off
// the command line / shell history).
func TestSMTPSenderSmoke(t *testing.T) {
	addr := os.Getenv("SMOKE_SMTP_ADDR")
	if addr == "" {
		t.Skip("SMOKE_SMTP_ADDR not set")
	}
	pass, err := os.ReadFile(os.Getenv("SMOKE_SMTP_PASS_FILE"))
	if err != nil {
		t.Skipf("SMOKE_SMTP_PASS_FILE unreadable: %v", err)
	}
	from := os.Getenv("SMOKE_SMTP_FROM")
	s := NewSMTPSender(SMTPConfig{
		Addr: addr,
		User: from,
		Pass: string(pass),
		From: from,
	})
	msg, err := NewVerificationEmail("https://example.com", from, "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), msg); err != nil {
		t.Fatalf("send failed: %v", err)
	}
}