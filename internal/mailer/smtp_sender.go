package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
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

func (s *SMTPSender) port() int {
	if i := strings.LastIndex(s.cfg.Addr, ":"); i >= 0 {
		if p, err := strconv.Atoi(s.cfg.Addr[i+1:]); err == nil {
			return p
		}
	}
	return 25
}

// isImplicitTLS reports whether the port expects a TLS handshake immediately
// (SMTPS, typically 465) instead of STARTTLS on a plain connection (587/25).
func (s *SMTPSender) isImplicitTLS() bool {
	return s.port() == 465
}

// dial opens the client connection: implicit TLS on port 465 (tls.Dial then
// smtp.NewClient), plain+STARTTLS otherwise — mirroring net/smtp.SendMail.
func (s *SMTPSender) dial() (*smtp.Client, error) {
	host := s.host()
	if s.isImplicitTLS() {
		conn, err := tls.Dial("tcp", s.cfg.Addr, &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		})
		if err != nil {
			return nil, fmt.Errorf("tls dial %s: %w", s.cfg.Addr, err)
		}
		return smtp.NewClient(conn, host)
	}

	conn, err := net.Dial("tcp", s.cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", s.cfg.Addr, err)
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("smtp client %s: %w", s.cfg.Addr, err)
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			_ = c.Close()
			return nil, fmt.Errorf("starttls %s: %w", s.cfg.Addr, err)
		}
	}
	return c, nil
}

func (s *SMTPSender) Send(ctx context.Context, msg Email) error {
	if s.cfg.From == "" || s.cfg.Addr == "" {
		return fmt.Errorf("smtp sender requires SMTP_ADDR and SMTP_FROM")
	}

	c, err := s.dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if s.cfg.User != "" {
		auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Pass, s.host())
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := c.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(s.buildMessage(msg)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp message submit: %w", err)
	}
	if err := c.Quit(); err != nil {
		// Quit failure (server dropped the connection) after a successful Data
		// does not mean the message was not delivered — ignore it.
		return nil
	}
	return nil
}