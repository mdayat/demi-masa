package task

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/pkg/errors"
)

const (
	TypeUserDowngrade = "user:downgrade"
	TypeUserPrayer    = "user:prayer"
)

type UserDowngradePayload struct {
	UserID string
}

func NewUserDowngradeTask(payload UserDowngradePayload) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal user downgrade task payload")
	}

	return asynq.NewTask(
		TypeUserDowngrade,
		bytes,
		asynq.TaskID(payload.UserID),
		asynq.MaxRetry(3),
	), nil
}

type UserPrayerPayload struct {
	UserID     string
	PrayerName string
}

func NewUserPrayerTask(payload UserPrayerPayload) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal user prayer task payload")
	}

	return asynq.NewTask(
		TypeUserPrayer,
		bytes,
		asynq.TaskID(fmt.Sprintf("%s:%s", payload.UserID, payload.PrayerName)),
		asynq.MaxRetry(3),
	), nil
}
