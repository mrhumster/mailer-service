package mailer

import (
	"context"
	"fmt"
	"math/rand"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

type SMTPConfig struct {
	Addr string // host:port of the SMTP server
	User string // optional auth username
	Pass string // optional auth password
	From string // sender envelope/header address
}

type SMTPSender struct {
	cfg SMTPConfig
}

func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	return &SMTPSender{cfg: cfg}
}

// buildMessage renders the RFC5322 message bytes for the given email.
func (s *SMTPSender) buildMessage(msg Email) []byte {
	var b strings.Builder
	h := textproto.MIMEHeader{}
	h.Set("From", s.cfg.From)
	h.Set("To", msg.To)
	h.Set("Subject", msg.Subject)
	h.Set("Message-ID", fmt.Sprintf("<%d.%d@%s>", time.Now().UnixNano(), rand.Int63(), s.host()))
	h.Set("Date", time.Now().Format(time.RFC1123Z))
	h.Set("MIME-Version", "1.0")
	h.Set("Content-Type", "text/html; charset=UTF-8")
	b.WriteString("From: " + s.cfg.From + "\r\n")
	b.WriteString("To: " + msg.To + "\r\n")
	b.WriteString("Subject: " + msg.Subject + "\r\n")
	b.WriteString("Message-ID: " + h.Get("Message-ID") + "\r\n")
	b.WriteString("Date: " + h.Get("Date") + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(msg.Body)
	return []byte(b.String())
}

func (s *SMTPSender) host() string {
	addr := s.cfg.Addr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		return addr[:i]
	}
	return addr
}

func (s *SMTPSender) Send(_ context.Context, msg Email) error {
	if s.cfg.From == "" || s.cfg.Addr == "" {
		return fmt.Errorf("smtp sender requires SMTP_ADDR and SMTP_FROM")
	}
	auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Pass, s.host())
	return smtp.SendMail(s.cfg.Addr, auth, s.cfg.From, []string{msg.To}, s.buildMessage(msg))
}