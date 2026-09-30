package queue

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	TaskEmailVerification  = "email:verification"
	TaskEmailDownloadReady = "email:download_ready"
	TaskEmailQueue         = "emails"
)

type EmailVerificationPayload struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Token  string    `json:"token"`
}

// EmailDownloadReadyPayload is produced by the export worker once the
// single-file rendition exists in storage. StreamURL is built by the producer
// (it owns FRONTEND_URL); this service only renders the message.
type EmailDownloadReadyPayload struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	StreamID  uuid.UUID `json:"stream_id"`
	StreamURL string    `json:"stream_url"`
	Size      int64     `json:"size"`
}

func NewEmailVerificationTask(userID uuid.UUID, email, token string) (*asynq.Task, error) {
	payload, err := json.Marshal(EmailVerificationPayload{
		UserID: userID,
		Email:  email,
		Token:  token,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskEmailVerification, payload), nil
}