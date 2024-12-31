package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
	"github.com/rs/zerolog/log"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

func handleUserDowngrade(ctx context.Context, asynqTask *asynq.Task) error {
	start := time.Now()
	logWithCtx := log.Ctx(ctx).With().Logger()
	var payload task.UserDowngradePayload
	if err := json.Unmarshal(asynqTask.Payload(), &payload); err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to unmarshal user downgrade task payload")
		return err
	}

	err := services.Queries.UpdateUserSubs(ctx, repository.UpdateUserSubsParams{
		ID:          payload.UserID,
		AccountType: repository.AccountTypeFREE,
	})

	if err != nil {
		logWithCtx.Error().Err(err).Caller().Str("user_id", payload.UserID).Msg("failed to update user subscription to FREE")
		return err
	}

	logWithCtx.Info().Dur("response_time", time.Since(start)).Msg("task completed")
	return nil
}

func handlePrayerReminder(ctx context.Context, asynqTask *asynq.Task) error {
	start := time.Now()
	logWithCtx := log.Ctx(ctx).With().Logger()
	var payload task.PrayerReminderPayload
	if err := json.Unmarshal(asynqTask.Payload(), &payload); err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to unmarshal prayer reminder task payload")
		return err
	}

	user, err := services.Queries.GetUserPrayerByID(ctx, payload.UserID)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Str("user_id", payload.UserID).Msg("failed to get user prayer by id")
		return err
	}

	location, err := time.LoadLocation(string(user.TimeZone.IndonesiaTimeZone))
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to load time zone location")
		return err
	}

	prayerCalendar, err := prayer.GetPrayerCalendar(ctx, services.RedisClient, string(user.TimeZone.IndonesiaTimeZone))
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to get prayer calendar")
		return err
	}

	prayerTime := time.Unix(payload.PrayerUnixTime, 0).In(location)
	isLastDay := prayer.IsLastDay(&prayerTime)
	isPenultimateDay := prayer.IsPenultimateDay(&prayerTime)

	var lastDayPrayer prayer.Prayers
	if payload.IsLastDay && payload.PrayerName != prayer.IsyaPrayerName {
		lastDayPrayer, err = prayer.GetLastDayPrayer(ctx, services.RedisClient, string(user.TimeZone.IndonesiaTimeZone))
		if err != nil {
			logWithCtx.Error().Err(err).Caller().Msg("failed to get last day prayer")
			return err
		}
	}

	var isNextPrayerLastDay bool
	if (isPenultimateDay && payload.PrayerName == prayer.IsyaPrayerName) ||
		(isLastDay && payload.PrayerName != prayer.IsyaPrayerName) {
		isNextPrayerLastDay = true
	}

	prayerDay := prayerTime.Day()
	if payload.IsLastDay && payload.PrayerName == prayer.IsyaPrayerName {
		prayerDay = 1
	}

	nextPrayer := prayer.GetNextPrayer(prayerCalendar, lastDayPrayer, prayerDay, payload.PrayerUnixTime)
	now := time.Now().In(location)
	nowUnixTime := now.Unix()

	nextPrayerTime := time.Unix(nextPrayer.UnixTime, 0).In(location)
	newAsynqTask, err := task.NewPrayerReminderTask(task.PrayerReminderPayload{
		UserID:         payload.UserID,
		PrayerName:     nextPrayer.Name,
		PrayerUnixTime: nextPrayer.UnixTime,
		IsLastDay:      isNextPrayerLastDay,
		Day:            nextPrayerTime.Day(),
	})

	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to create prayer reminder task")
		return err
	}

	_, err = services.AsynqClient.Enqueue(newAsynqTask, asynq.ProcessIn(nextPrayerTime.Sub(now)))
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to enqueue prayer reminder task")
		return err
	}

	if user.AccountType == repository.AccountTypePREMIUM {
		var prayerTimeDistance int64
		var nowToNextPrayerDistance int64

		if payload.PrayerName == prayer.SubuhPrayerName && isLastDay {
			sunriseUnixTime := lastDayPrayer[1].UnixTime
			prayerTimeDistance = sunriseUnixTime - payload.PrayerUnixTime
			nowToNextPrayerDistance = sunriseUnixTime - nowUnixTime
		} else if payload.PrayerName == prayer.SubuhPrayerName && isLastDay == false {
			todayPrayer := prayerCalendar[prayerDay-1]
			sunriseUnixTime := todayPrayer[1].UnixTime
			prayerTimeDistance = sunriseUnixTime - payload.PrayerUnixTime
			nowToNextPrayerDistance = sunriseUnixTime - nowUnixTime
		} else {
			prayerTimeDistance = nextPrayer.UnixTime - payload.PrayerUnixTime
			nowToNextPrayerDistance = nextPrayer.UnixTime - nowUnixTime
		}

		newAsynqTask, err = task.NewLastPrayerReminderTask(task.LastPrayerReminderPayload{
			UserID:     payload.UserID,
			PrayerName: payload.PrayerName,
			Day:        prayerTime.Day(),
		})

		if err != nil {
			logWithCtx.Error().Err(err).Caller().Msg("failed to create last prayer reminder task")
			return err
		}

		quarterTime := math.Round(float64(prayerTimeDistance) * 0.25)
		lastReminderDuration := time.Duration(nowToNextPrayerDistance-int64(quarterTime)) * time.Second

		_, err = services.AsynqClient.Enqueue(newAsynqTask, asynq.ProcessIn(lastReminderDuration))
		if err != nil {
			logWithCtx.Error().Err(err).Caller().Msg("failed to enqueue last prayer reminder task")
			return err
		}
	}

	params := twilioApi.CreateMessageParams{}
	params.SetFrom("whatsapp:+14155238886")
	params.SetTo(fmt.Sprintf("whatsapp:%s", user.PhoneNumber.String))
	msg := fmt.Sprintf(
		"Hai! Sudah waktunya salat %s nih... Yuk segera tunaikan dan jangan lupa untuk memperbarui kemajuan kamu di aplikasi Demi Masa.",
		payload.PrayerName,
	)
	params.SetBody(msg)

	_, err = services.TwilioClient.Api.CreateMessage(&params)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Str("phone_number", user.PhoneNumber.String).Msg("failed to send prayer reminder")
		return err
	}
	logWithCtx.Info().Dur("response_time", time.Since(start)).Msg("task completed")

	return nil
}

func handleLastPrayerReminder(ctx context.Context, asynqTask *asynq.Task) error {
	start := time.Now()
	logWithCtx := log.Ctx(ctx).With().Logger()
	var payload task.PrayerReminderPayload
	if err := json.Unmarshal(asynqTask.Payload(), &payload); err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to unmarshal last prayer reminder task payload")
		return err
	}

	userPhone, err := services.Queries.GetUserPhoneByID(ctx, payload.UserID)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Str("user_id", payload.UserID).Msg("failed to get user phone by id")
		return err
	}

	params := twilioApi.CreateMessageParams{}
	params.SetFrom("whatsapp:+14155238886")
	params.SetTo(fmt.Sprintf("whatsapp:%s", userPhone.String))
	msg := fmt.Sprintf(
		"Waktu salat %s sudah hampir habis. Yuk, segera lakukan sebelum terlambat.",
		payload.PrayerName,
	)
	params.SetBody(msg)

	_, err = services.TwilioClient.Api.CreateMessage(&params)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Str("phone_number", userPhone.String).Msg("failed to send last prayer reminder")
		return err
	}
	logWithCtx.Info().Dur("response_time", time.Since(start)).Msg("task completed")

	return nil
}

func handlePrayerRenewal(ctx context.Context, asynqTask *asynq.Task) error {
	start := time.Now()
	logWithCtx := log.Ctx(ctx).With().Logger()
	var payload task.PrayerRenewalTask
	if err := json.Unmarshal(asynqTask.Payload(), &payload); err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to unmarshal prayer renewal task payload")
		return err
	}

	location, err := time.LoadLocation(string(payload.TimeZone))
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to load time zone location")
		return err
	}

	now := time.Now().In(location)
	year := now.Year()
	month, err := strconv.Atoi(fmt.Sprintf("%d", now.Month()))
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to convert string to integer")
		return err
	}

	if month == 12 {
		year++
		month = 1
	} else {
		month++
	}

	URL := fmt.Sprintf(
		"https://api.aladhan.com/v1/calendarByCity/%d/%d?country=Indonesia&city=%s",
		year,
		month,
		strings.Split(string(payload.TimeZone), "/")[1],
	)

	unparsedPrayerCalendar, err := prayer.GetAladhanPrayerCalendar(URL)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to get aladhan prayer calendar")
		return err
	}

	parsedPrayerCalendar, err := prayer.ParseAladhanPrayerCalendar(unparsedPrayerCalendar, location)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to parse aladhan prayer calendar")
		return err
	}

	parsedPrayerCalendarJSON, err := json.Marshal(parsedPrayerCalendar)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to marshal parsed aladhan prayer calendar")
		return err
	}

	err = services.RedisClient.Watch(ctx, func(tx *redis.Tx) error {
		oldPrayerCalendar, err := prayer.GetPrayerCalendar(ctx, services.RedisClient, payload.TimeZone)
		if err != nil {
			logWithCtx.Error().Err(err).Caller().Msg("failed to get prayer calendar")
			return err
		}

		penultimateDayPrayer := parsedPrayerCalendar[len(parsedPrayerCalendar)-2]
		penultimateDayPrayerJSON, err := json.Marshal(penultimateDayPrayer)
		if err != nil {
			logWithCtx.Error().Err(err).Caller().Msg("failed to marshal penultimate day prayer of old prayer calendar")
			return err
		}

		lastDayPrayer := oldPrayerCalendar[len(oldPrayerCalendar)-1]
		lastDayPrayerJSON, err := json.Marshal(lastDayPrayer)
		if err != nil {
			logWithCtx.Error().Err(err).Caller().Msg("failed to marshal last day prayer of old prayer calendar")
			return err
		}

		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, prayer.MakePrayerCalendarKey(payload.TimeZone), parsedPrayerCalendarJSON, 0)
			pipe.Set(ctx, prayer.MakeLastDayPrayerKey(payload.TimeZone), lastDayPrayerJSON, 0)
			pipe.Set(ctx, prayer.MakePenultimateDayPrayerKey(payload.TimeZone), penultimateDayPrayerJSON, 0)
			return nil
		})

		return err
	})

	newAsynqTask, err := task.NewPrayerRenewalTask(task.PrayerRenewalTask{TimeZone: payload.TimeZone})
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to create prayer renewal task")
		return err
	}

	numOfDays := len(parsedPrayerCalendar)
	renewalDate := time.Date(year, time.Month(month), numOfDays, 0, 0, 0, 0, now.Location())

	_, err = services.AsynqClient.Enqueue(newAsynqTask, asynq.ProcessIn(renewalDate.Sub(now)))
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to enqueue prayer renewal task")
		return err
	}

	logWithCtx.Info().Dur("response_time", time.Since(start)).Msg("task completed")
	return nil
}

func handleTaskRemoval(ctx context.Context, _ *asynq.Task) error {
	start := time.Now()
	logWithCtx := log.Ctx(ctx).With().Logger()
	err := services.Queries.RemoveCheckedTask(ctx)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to delete checked tasks")
		return err
	}

	location, err := time.LoadLocation(string(repository.IndonesiaTimeZoneAsiaJakarta))
	if err != nil {
		return errors.Wrap(err, "failed to load time zone location")
	}

	now := time.Now().In(location)
	tomorrow := now.AddDate(0, 0, 1).In(location)
	midnight := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())

	newAsynqTask, err := task.NewTaskRemovalTask()
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to create task removal task")
		return err
	}

	_, err = services.AsynqClient.Enqueue(newAsynqTask, asynq.ProcessIn(midnight.Sub(now)))
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to enqueue task removal task")
		return err
	}

	logWithCtx.Info().Dur("response_time", time.Since(start)).Msg("task completed")
	return nil
}

func handlePrayerUpdate(ctx context.Context, _ *asynq.Task) error {
	start := time.Now()
	logWithCtx := log.Ctx(ctx).With().Logger()
	location, err := time.LoadLocation(string(repository.IndonesiaTimeZoneAsiaJakarta))
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to load time zone location")
		return err
	}

	now := time.Now().In(location)
	err = services.Queries.UpdatePrayersToMissed(ctx, repository.UpdatePrayersToMissedParams{
		Day:   int16(now.Day()),
		Month: int16(now.Month()),
		Year:  int16(now.Year()),
	})

	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to update prayers status to missed")
		return err
	}

	nextDay := now.Add(24 * time.Hour).In(now.Location())
	nextDayAtSix := time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 6, 0, 0, 0, nextDay.Location())

	newAsynqTask, err := task.NewPrayerUpdateTask()
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to create prayer update task")
		return err
	}

	_, err = services.AsynqClient.Enqueue(newAsynqTask, asynq.ProcessIn(nextDayAtSix.Sub(now)))
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Msg("failed to enqueue prayer update task")
		return err
	}

	logWithCtx.Info().Dur("response_time", time.Since(start)).Msg("task completed")
	return nil
}
