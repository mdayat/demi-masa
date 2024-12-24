package task

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/odemimasa/backend/internal/services"
	"github.com/odemimasa/backend/repository"
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

type PrayerReminderPayload struct {
	UserID         string
	PrayerName     string
	PrayerUnixTime int64
	IsLastDay      bool
}

func PrayerReminderTaskID(userID string, prayerName string) string {
	return fmt.Sprintf("%s:%s", userID, prayerName)
}

func NewPrayerReminderTask(payload PrayerReminderPayload) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal prayer reminder task payload")
	}

	return asynq.NewTask(
		TypePrayerReminder,
		bytes,
		asynq.TaskID(PrayerReminderTaskID(payload.UserID, payload.PrayerName)),
		asynq.MaxRetry(3),
	), nil
}

type LastPrayerReminderPayload struct {
	UserID     string
	PrayerName string
}

func LastPrayerReminderTaskID(userID string, prayerName string) string {
	return fmt.Sprintf("%s:%s:last", userID, prayerName)
}

func NewLastPrayerReminderTask(payload LastPrayerReminderPayload) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal last prayer reminder task payload")
	}

	return asynq.NewTask(
		TypeLastPrayerReminder,
		bytes,
		asynq.TaskID(LastPrayerReminderTaskID(payload.UserID, payload.PrayerName)),
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

func ScheduleTaskRemovalTask() error {
	location, err := time.LoadLocation(string(repository.IndonesiaTimeZoneAsiaJakarta))
	if err != nil {
		return errors.Wrap(err, "failed to load time zone location")
	}

	now := time.Now().In(location)
	tomorrow := now.AddDate(0, 0, 1).In(location)
	midnight := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())
	asynqTask, err := NewTaskRemovalTask()
	if err != nil {
		return errors.Wrap(err, "failed to create task removal task")
	}

	_, err = services.GetAsynqClient().Enqueue(asynqTask, asynq.ProcessIn(midnight.Sub(now)))
	if err != nil {
		return errors.Wrap(err, "failed to enqueue task removal task")
	}

	return nil
}

func NewTaskRemovalTask() (*asynq.Task, error) {
	return asynq.NewTask(TypeTaskRemoval, nil, asynq.MaxRetry(3)), nil
}

func NewPrayerUpdateTask() (*asynq.Task, error) {
	return asynq.NewTask(TypePrayerUpdate, nil, asynq.MaxRetry(3)), nil
}
