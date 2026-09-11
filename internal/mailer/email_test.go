package mailer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewVerificationEmail(t *testing.T) {
	msg, err := NewVerificationEmail("https://example.com", "user@x.co", "abc123")
	require.NoError(t, err)
	require.Equal(t, "user@x.co", msg.To)
	require.Equal(t, "Verify your GoCast email", msg.Subject)
	require.Contains(t, msg.Body, "/verify?token=abc123")
	require.Contains(t, msg.Body, "https://example.com/verify?token=abc123")
}

func TestNewVerificationEmailTrimBaseURL(t *testing.T) {
	msg, err := NewVerificationEmail("https://example.com/", "user@x.co", "tok")
	require.NoError(t, err)
	require.NotContains(t, msg.Body, "//verify")
	require.Contains(t, msg.Body, "https://example.com/verify?token=tok")
}

func TestNewVerificationEmailInvalid(t *testing.T) {
	_, err := NewVerificationEmail("https://example.com", "", "tok")
	require.Error(t, err)
	_, err = NewVerificationEmail("https://example.com", "a@b.c", "")
	require.Error(t, err)
}

func TestSMTPSenderBuildMessage(t *testing.T) {
	s := NewSMTPSender(SMTPConfig{Addr: "smtp.example.com:587", From: "no-reply@example.com"})
	msg, err := NewVerificationEmail("https://example.com", "user@x.co", "tok")
	require.NoError(t, err)
	out := string(s.buildMessage(msg))
	require.True(t, strings.HasPrefix(out, "From: no-reply@example.com"))
	require.Contains(t, out, "Subject: Verify your GoCast email")
	require.Contains(t, out, "Content-Type: text/html; charset=UTF-8")
	require.Contains(t, out, "https://example.com/verify?token=tok")
}

func TestSMTPSenderHost(t *testing.T) {
	s := NewSMTPSender(SMTPConfig{Addr: "smtp.example.com:587"})
	require.Equal(t, "smtp.example.com", s.host())
}