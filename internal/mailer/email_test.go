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
	require.Equal(t, 587, s.port())
	require.False(t, s.isImplicitTLS())
}

func TestSMTPSenderPortDetect(t *testing.T) {
	ssl := NewSMTPSender(SMTPConfig{Addr: "smtp.gmail.com:465"})
	require.True(t, ssl.isImplicitTLS())
	require.Equal(t, 465, ssl.port())

	noPort := NewSMTPSender(SMTPConfig{Addr: "smtp.example.com"})
	require.False(t, noPort.isImplicitTLS())
	require.Equal(t, 25, noPort.port())
}
func TestNewDownloadReadyEmail(t *testing.T) {
	msg, err := NewDownloadReadyEmail("owner@x.co", "https://example.com/streams/3d84828d", 1_437_004_800)
	require.NoError(t, err)
	require.Equal(t, "owner@x.co", msg.To)
	require.Equal(t, "Your GoCast video is ready to download", msg.Subject)
	require.Contains(t, msg.Body, "https://example.com/streams/3d84828d")
	require.Contains(t, msg.Body, "1.3 GiB")
	// The link must go to the page, not the file: the download is owner-only.
	require.NotContains(t, msg.Body, "minio")
	require.Contains(t, msg.Body, "download button")
}

func TestNewDownloadReadyEmailRequiresRecipientAndLink(t *testing.T) {
	_, err := NewDownloadReadyEmail("", "https://example.com/s/1", 10)
	require.Error(t, err)
	_, err = NewDownloadReadyEmail("owner@x.co", "", 10)
	require.Error(t, err)
}

func TestNewDownloadReadyEmailSizeFormatting(t *testing.T) {
	cases := map[string]struct {
		size int64
		want string
	}{
		"unknown size is omitted": {0, ""},
		"negative is omitted":    {-1, ""},
		"bytes":                  {512, "(512 B)"},
		"kibibytes":              {2048, "(2.0 KiB)"},
		"mebibytes":              {5 * 1024 * 1024, "(5.0 MiB)"},
		"gibibytes":              {3 * 1024 * 1024 * 1024, "(3.0 GiB)"},
		"tebibytes":              {2 * 1024 * 1024 * 1024 * 1024, "(2.0 TiB)"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			msg, err := NewDownloadReadyEmail("owner@x.co", "https://example.com/s/1", tc.size)
			require.NoError(t, err)
			if tc.want == "" {
				require.NotContains(t, msg.Body, "(")
				return
			}
			require.Contains(t, msg.Body, tc.want)
		})
	}
}
