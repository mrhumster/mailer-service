package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mrhumster/mailer-service/internal/mailer"
	"github.com/mrhumster/mailer-service/internal/metrics"
)

type HandleEmailVerification struct {
	sender  mailer.EmailSender
	baseURL string
}

func NewHandleEmailVerification(sender mailer.EmailSender, baseURL string) *HandleEmailVerification {
	return &HandleEmailVerification{sender: sender, baseURL: baseURL}
}

func (h *HandleEmailVerification) HandleEmailVerificationTask(ctx context.Context, t *asynq.Task) error {
	var p EmailVerificationPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json unmarshal failed: %w", asynq.SkipRetry)
	}

	msg, err := mailer.NewVerificationEmail(h.baseURL, p.Email, p.Token)
	if err != nil {
		return fmt.Errorf("build verification email: %w", asynq.SkipRetry)
	}

	start := time.Now()
	if err := h.sender.Send(ctx, msg); err != nil {
		metrics.SentError()
		return fmt.Errorf("send verification email: %w", err)
	}
	metrics.SentSuccess()
	metrics.Duration.Observe(time.Since(start).Seconds())
	return nil
}