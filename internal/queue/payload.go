package queue

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	TaskEmailVerification = "email:verification"
	TaskEmailQueue        = "emails"
)

type EmailVerificationPayload struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Token  string    `json:"token"`
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