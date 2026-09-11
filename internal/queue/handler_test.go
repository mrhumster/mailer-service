package queue

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mrhumster/mailer-service/internal/mailer"
	"github.com/stretchr/testify/require"
)

type fakeSender struct {
	mu   sync.Mutex
	sent []mailer.Email
	err  error
}

func (f *fakeSender) Send(_ context.Context, msg mailer.Email) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, msg)
	return nil
}

func (f *fakeSender) calls() []mailer.Email {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]mailer.Email(nil), f.sent...)
}

func makeTask(t *testing.T, p EmailVerificationPayload) *asynq.Task {
	t.Helper()
	task, err := NewEmailVerificationTask(p.UserID, p.Email, p.Token)
	require.NoError(t, err)
	return task
}

func TestHandleEmailVerificationTaskSuccess(t *testing.T) {
	s := &fakeSender{}
	h := NewHandleEmailVerification(s, "https://example.com")

	err := h.HandleEmailVerificationTask(context.Background(), makeTask(t, EmailVerificationPayload{
		UserID: uuid.New(),
		Email:  "user@x.co",
		Token:  "tok123",
	}))
	require.NoError(t, err)
	calls := s.calls()
	require.Len(t, calls, 1)
	require.Equal(t, "user@x.co", calls[0].To)
	require.Contains(t, calls[0].Body, "token=tok123")
}

func TestHandleEmailVerificationTaskSendErrorRetries(t *testing.T) {
	s := &fakeSender{err: context.DeadlineExceeded}
	h := NewHandleEmailVerification(s, "https://example.com")

	err := h.HandleEmailVerificationTask(context.Background(), makeTask(t, EmailVerificationPayload{
		UserID: uuid.New(),
		Email:  "user@x.co",
		Token:  "tok123",
	}))
	require.Error(t, err)
	require.NotErrorIs(t, err, asynq.SkipRetry)
}

func TestHandleEmailVerificationTaskInvalidPayloadSkipRetry(t *testing.T) {
	s := &fakeSender{}
	h := NewHandleEmailVerification(s, "https://example.com")

	err := h.HandleEmailVerificationTask(context.Background(), makeTask(t, EmailVerificationPayload{
		UserID: uuid.New(),
		Email:  "",
		Token:  "",
	}))
	require.Error(t, err)
	require.ErrorIs(t, err, asynq.SkipRetry)
	require.Empty(t, s.calls())
}

func TestLogSenderSmoke(t *testing.T) {
	err := mailer.LogSender{}.Send(context.Background(), mailer.Email{
		To:      "a@b.c",
		Subject: "s",
		Body:    "b",
	})
	require.NoError(t, err)
}