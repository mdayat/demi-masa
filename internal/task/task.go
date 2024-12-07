package task

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/pkg/errors"
)

const (
	TypeUserDowngrade      = "user:downgrade"
	TypePrayerReminder     = "prayer:remind"
	TypeLastPrayerReminder = "prayer:last-remind"
	TypePrayerRenewal      = "prayer:renew"
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

type PrayerReminderPayload struct {
	UserID          string
	PrayerName      string
	PrayerTimestamp int64
	LastDay         bool
}

func NewPrayerReminderTask(payload PrayerReminderPayload) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal prayer reminder task payload")
	}

	return asynq.NewTask(
		TypePrayerReminder,
		bytes,
		asynq.TaskID(fmt.Sprintf("%s:%s", payload.UserID, payload.PrayerName)),
		asynq.MaxRetry(3),
	), nil
}

type LastPrayerReminderPayload struct {
	UserID     string
	PrayerName string
}

func NewLastPrayerReminderTask(payload LastPrayerReminderPayload) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal last prayer reminder task payload")
	}

	return asynq.NewTask(
		TypeLastPrayerReminder,
		bytes,
		asynq.TaskID(fmt.Sprintf("%s:%s:last", payload.UserID, payload.PrayerName)),
		asynq.MaxRetry(3),
	), nil
}

type PrayerRenewalTask struct {
	TimeZone repository.IndonesiaTimeZone
}

func NewPrayerRenewalTask(payload PrayerRenewalTask) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal prayer renewal task payload")
	}

	return asynq.NewTask(TypePrayerRenewal, bytes, asynq.MaxRetry(3)), nil
}
