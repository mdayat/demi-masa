package task

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/pkg/errors"
)

const (
	TypeUserDowngrade      = "user:downgrade"
	TypePrayerReminder     = "prayer:remind"
	TypeLastPrayerReminder = "prayer:last_remind"
	TypePrayerRenewal      = "prayer:renew"
	TypePrayerUpdate       = "prayer:update"
	TypeTaskRemoval        = "task:remove"
)

const (
	DefaultQueue = "default"
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

func MakePrayerReminderTaskID(userID string, prayerName string) string {
	return fmt.Sprintf("%s:%s", userID, prayerName)
}

type PrayerReminderPayload struct {
	UserID         string
	PrayerName     string
	PrayerUnixTime int64
	IsLastDay      bool
}

func NewPrayerReminderTask(payload PrayerReminderPayload) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal prayer reminder task payload")
	}

	return asynq.NewTask(
		TypePrayerReminder,
		bytes,
		asynq.TaskID(MakePrayerReminderTaskID(payload.UserID, payload.PrayerName)),
		asynq.MaxRetry(3),
	), nil
}

func MakeLastPrayerReminderTaskID(userID string, prayerName string) string {
	return fmt.Sprintf("%s:%s:last", userID, prayerName)
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
		asynq.TaskID(MakeLastPrayerReminderTaskID(payload.UserID, payload.PrayerName)),
		asynq.MaxRetry(3),
	), nil
}

func MakePrayerRenewalTaskID(timeZone string) string {
	return fmt.Sprintf("%s:%s", TypePrayerRenewal, timeZone)
}

type PrayerRenewalTask struct {
	TimeZone string
}

func NewPrayerRenewalTask(payload PrayerRenewalTask) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal prayer renewal task payload")
	}

	return asynq.NewTask(
		TypePrayerRenewal,
		bytes,
		asynq.TaskID(MakePrayerRenewalTaskID(payload.TimeZone)),
		asynq.MaxRetry(3),
	), nil
}

func MakeTaskRemovalTaskID() string {
	return TypeTaskRemoval
}

func NewTaskRemovalTask() (*asynq.Task, error) {
	return asynq.NewTask(
		TypeTaskRemoval,
		nil,
		asynq.TaskID(MakeTaskRemovalTaskID()),
		asynq.MaxRetry(3),
	), nil
}

func MakePrayerUpdateTaskID() string {
	return TypePrayerUpdate
}

func NewPrayerUpdateTask() (*asynq.Task, error) {
	return asynq.NewTask(
		TypePrayerUpdate,
		nil,
		asynq.TaskID(MakePrayerUpdateTaskID()),
		asynq.MaxRetry(3),
	), nil
}
