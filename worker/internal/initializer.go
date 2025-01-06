package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mdayat/demi-masa/pkg/prayer"
	"github.com/mdayat/demi-masa/pkg/task"
	"github.com/mdayat/demi-masa/worker/configs/services"
	"github.com/mdayat/demi-masa/worker/repository"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

func makeAladhanURL(year, month int, timeZone string) string {
	return fmt.Sprintf(
		"https://api.aladhan.com/v1/calendarByCity/%d/%d?country=Indonesia&city=%s",
		year,
		month,
		strings.Split(timeZone, "/")[1],
	)
}

func InitPrayerCalendar(ctx context.Context, location *time.Location) error {
	prayerRenewalTaskID := task.MakePrayerRenewalTaskID(location.String())
	_, err := services.AsynqInspector.GetTaskInfo(task.DefaultQueue, prayerRenewalTaskID)
	if err != nil && !errors.Is(err, asynq.ErrQueueNotFound) {
		return errors.Wrap(err, "failed to get prayer renewal task info by id")
	}

	if err != nil && errors.Is(err, asynq.ErrQueueNotFound) {
		return err
	}

	if err == nil {
		return nil
	}

	now := time.Now().In(location)
	year := now.Year()
	month, err := strconv.Atoi(fmt.Sprintf("%d", now.Month()))
	if err != nil {
		return errors.Wrap(err, "failed to convert string to integer")
	}

	timeZone := location.String()
	prayerCalendar, err := prayer.GetAladhanPrayerCalendar(makeAladhanURL(year, month, timeZone))
	if err != nil {
		return errors.Wrap(err, "failed to get aladhan prayer calendar")
	}

	parsedPrayerCalendar, err := prayer.ParseAladhanPrayerCalendar(prayerCalendar, location)
	if err != nil {
		return errors.Wrap(err, "failed to parse aladhan prayer calendar")
	}

	prayerCalendarJSON, err := json.Marshal(parsedPrayerCalendar)
	if err != nil {
		return errors.Wrap(err, "failed to marshal parsed aladhan prayer calendar")
	}

	penultimateDayPrayer := parsedPrayerCalendar[len(parsedPrayerCalendar)-2]
	penultimateDayPrayerJSON, err := json.Marshal(penultimateDayPrayer)
	if err != nil {
		return errors.Wrap(err, "failed to marshal penultimate day prayer of parsed aladhan prayer calendar")
	}

	lastDayPrayer := parsedPrayerCalendar[len(parsedPrayerCalendar)-1]
	lastDayPrayerJSON, err := json.Marshal(lastDayPrayer)
	if err != nil {
		return errors.Wrap(err, "failed to marshal last day prayer of parsed aladhan prayer calendar")
	}

	isLastDay := prayer.IsLastDay(now)
	if isLastDay {
		if month == 12 {
			year++
			month = 1
		} else {
			month++
		}

		prayerCalendar, err = prayer.GetAladhanPrayerCalendar(makeAladhanURL(year, month, timeZone))
		if err != nil {
			return errors.Wrap(err, "failed to get aladhan prayer calendar")
		}

		parsedPrayerCalendar, err = prayer.ParseAladhanPrayerCalendar(prayerCalendar, location)
		if err != nil {
			return errors.Wrap(err, "failed to parse aladhan prayer calendar")
		}

		prayerCalendarJSON, err = json.Marshal(parsedPrayerCalendar)
		if err != nil {
			return errors.Wrap(err, "failed to marshal parsed aladhan prayer calendar")
		}
	}

	err = services.RedisClient.Watch(ctx, func(tx *redis.Tx) error {
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, prayer.MakePrayerCalendarKey(timeZone), prayerCalendarJSON, 0)
			pipe.Set(ctx, prayer.MakeLastDayPrayerKey(timeZone), lastDayPrayerJSON, 0)
			pipe.Set(ctx, prayer.MakePenultimateDayPrayerKey(timeZone), penultimateDayPrayerJSON, 0)
			return nil
		})
		return err
	})

	if err != nil {
		return errors.Wrap(err, "failed to execute redis tx to set prayer calendar and last day prayer")
	}

	newAsynqTask, err := task.NewPrayerRenewalTask(task.PrayerRenewalTask{TimeZone: timeZone})
	if err != nil {
		return errors.Wrap(err, "failed to create prayer renewal task")
	}

	numOfDays := len(parsedPrayerCalendar)
	renewalDate := time.Date(year, time.Month(month), numOfDays, 0, 0, 0, 0, now.Location())

	_, err = services.AsynqClient.Enqueue(newAsynqTask, asynq.ProcessIn(renewalDate.Sub(now)))
	if err != nil {
		return errors.Wrap(err, "failed to enqueue prayer renewal task")
	}

	return nil
}

func InitPrayerReminder(ctx context.Context, location *time.Location) error {
	timeZone := location.String()
	prayerCalendar, err := prayer.GetPrayerCalendar(ctx, services.RedisClient, timeZone)
	if err != nil {
		return errors.Wrap(err, "failed to get prayer calendar")
	}

	lastDayPrayer, err := prayer.GetLastDayPrayer(ctx, services.RedisClient, timeZone)
	if err != nil {
		return errors.Wrap(err, "failed to get last day prayer")
	}

	users, err := services.Queries.GetUsersByTimeZone(
		ctx,
		repository.NullIndonesiaTimeZone{
			IndonesiaTimeZone: repository.IndonesiaTimeZone(timeZone),
			Valid:             true,
		},
	)

	if err != nil {
		return errors.Wrap(err, "failed to get users by time zone")
	}

	now := time.Now().In(location)
	currentDay := now.Day()

	isLastDay := prayer.IsLastDay(now)
	isPenultimateDay := prayer.IsPenultimateDay(now)
	isyaPrayer := prayerCalendar[currentDay-1][5]

	if isLastDay {
		isyaPrayer = lastDayPrayer[5]
	}

	for _, user := range users {
		now = time.Now().In(location)
		currentUnixTime := now.Unix()

		var isNextPrayerLastDay bool
		if (isPenultimateDay && currentUnixTime > isyaPrayer.UnixTime) ||
			(isLastDay && currentUnixTime < isyaPrayer.UnixTime) {
			isNextPrayerLastDay = true
		}

		var nextPrayer prayer.Prayer
		if isLastDay && currentUnixTime < isyaPrayer.UnixTime {
			nextPrayer = prayer.GetNextPrayer(prayerCalendar, lastDayPrayer, currentDay, currentUnixTime)
		} else {
			if isLastDay && currentUnixTime > isyaPrayer.UnixTime {
				currentDay = 1
			}

			nextPrayer = prayer.GetNextPrayer(prayerCalendar, nil, currentDay, currentUnixTime)
		}

		prayerReminderTaskID := task.MakePrayerReminderTaskID(user.ID, nextPrayer.Name)
		_, err := services.AsynqInspector.GetTaskInfo(task.DefaultQueue, prayerReminderTaskID)
		if err != nil && !errors.Is(err, asynq.ErrQueueNotFound) {
			return errors.Wrap(err, "failed to get prayer reminder task info by id")
		}

		if err != nil && errors.Is(err, asynq.ErrQueueNotFound) {
			return err
		}

		if err == nil {
			continue
		}

		nextPrayerTime := time.Unix(nextPrayer.UnixTime, 0).In(location)
		newAsynqTask, err := task.NewPrayerReminderTask(task.PrayerReminderPayload{
			UserID:         user.ID,
			PrayerName:     nextPrayer.Name,
			PrayerUnixTime: nextPrayer.UnixTime,
			IsLastDay:      isNextPrayerLastDay,
		})

		if err != nil {
			return errors.Wrap(err, "failed to create prayer reminder task")
		}

		_, err = services.AsynqClient.Enqueue(newAsynqTask, asynq.ProcessIn(nextPrayerTime.Sub(now)))
		if err != nil {
			return errors.Wrap(err, "failed to enqueue prayer reminder task")
		}
	}

	return nil
}

func InitPrayerUpdateTask(location *time.Location) error {
	prayerUpdateTaskID := task.MakePrayerUpdateTaskID()
	_, err := services.AsynqInspector.GetTaskInfo(task.DefaultQueue, prayerUpdateTaskID)
	if err != nil && !errors.Is(err, asynq.ErrQueueNotFound) {
		return errors.Wrap(err, "failed to get prayer update task info by id")
	}

	if err != nil && errors.Is(err, asynq.ErrQueueNotFound) {
		return err
	}

	if err == nil {
		return nil
	}

	now := time.Now().In(location)
	sixAMToday := time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, now.Location())

	var targetTime time.Time
	if now.Before(sixAMToday) {
		targetTime = sixAMToday
	} else {
		nextDay := now.Add(24 * time.Hour).In(now.Location())
		targetTime = time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 6, 0, 0, 0, nextDay.Location())
	}

	asynqTask, err := task.NewPrayerUpdateTask()
	if err != nil {
		return errors.Wrap(err, "failed to create prayer update task")
	}

	_, err = services.AsynqClient.Enqueue(asynqTask, asynq.ProcessIn(targetTime.Sub(now)))
	if err != nil {
		return errors.Wrap(err, "failed to enqueue prayer update task")
	}

	return nil
}

func InitTaskRemovalTask(location *time.Location) error {
	taskRemovalTaskID := task.MakeTaskRemovalTaskID()
	_, err := services.AsynqInspector.GetTaskInfo(task.DefaultQueue, taskRemovalTaskID)
	if err != nil && !errors.Is(err, asynq.ErrQueueNotFound) {
		return errors.Wrap(err, "failed to get task removal task info by id")
	}

	if err != nil && errors.Is(err, asynq.ErrQueueNotFound) {
		return err
	}

	if err == nil {
		return nil
	}

	now := time.Now().In(location)
	tomorrow := now.AddDate(0, 0, 1).In(location)
	midnight := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())

	asynqTask, err := task.NewTaskRemovalTask()
	if err != nil {
		return errors.Wrap(err, "failed to create task removal task")
	}

	_, err = services.AsynqClient.Enqueue(asynqTask, asynq.ProcessIn(midnight.Sub(now)))
	if err != nil {
		return errors.Wrap(err, "failed to enqueue task removal task")
	}

	return nil
}
