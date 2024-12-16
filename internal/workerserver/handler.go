package workerserver

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mdayat/demi-masa-be/internal/prayer"
	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

func handleUserDowngrade(ctx context.Context, asynqTask *asynq.Task) error {
	logWithCtx := log.Ctx(ctx).With().Logger()
	var payload task.UserDowngradePayload
	if err := json.Unmarshal(asynqTask.Payload(), &payload); err != nil {
		logWithCtx.Error().Err(err).Msg("failed to unmarshal user downgrade task payload")
		return err
	}

	err := queries.UpdateUserSubs(ctx, repository.UpdateUserSubsParams{
		ID:          payload.UserID,
		AccountType: repository.AccountTypeFREE,
	})

	if err != nil {
		logWithCtx.Error().Err(err).Str("user_id", payload.UserID).Msg("failed to update user subscription to FREE")
		return err
	}
	logWithCtx.Info().Str("user_id", payload.UserID).Msg("successfully updated user subscription to FREE")

	return nil
}

func handlePrayerReminder(ctx context.Context, asynqTask *asynq.Task) error {
	logWithCtx := log.Ctx(ctx).With().Logger()
	var payload task.PrayerReminderPayload
	if err := json.Unmarshal(asynqTask.Payload(), &payload); err != nil {
		logWithCtx.Error().Err(err).Msg("failed to unmarshal prayer reminder task payload")
		return err
	}

	user, err := queries.GetUserPrayerByID(ctx, payload.UserID)
	if err != nil {
		logWithCtx.Error().Err(err).Str("user_id", payload.UserID).Msg("failed to get user prayer by id")
		return err
	}
	logWithCtx.Info().Str("user_id", payload.UserID).Msg("successfully get user prayer by id")

	location, err := time.LoadLocation(string(user.TimeZone.IndonesiaTimeZone))
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to load time zone location")
		return err
	}

	prayerCalendar, err := prayer.GetPrayerCalendar(ctx, user.TimeZone.IndonesiaTimeZone)
	if err != nil {
		return errors.Wrap(err, "failed to get prayer calendar")
	}

	prayerTime := time.Unix(payload.PrayerUnixTime, 0)
	isLastDay := prayer.IsLastDay(&prayerTime)
	isPenultimateDay := prayer.IsPenultimateDay(&prayerTime)

	var lastDayPrayer prayer.Prayers
	if payload.IsLastDay && payload.PrayerName != prayer.IsyaPrayerName {
		lastDayPrayer, err = prayer.GetLastDayPrayer(ctx, user.TimeZone.IndonesiaTimeZone)
		if err != nil {
			return errors.Wrap(err, "failed to get last day prayer")
		}
	}

	var isNextPrayerLastDay bool
	if (isPenultimateDay && payload.PrayerName == prayer.IsyaPrayerName) ||
		(isLastDay && payload.PrayerName != prayer.IsyaPrayerName) {
		isNextPrayerLastDay = true
	}

	prayerDay := prayerTime.Day()
	if isLastDay && payload.PrayerName == prayer.IsyaPrayerName {
		prayerDay = 1
	}

	nextPrayer := prayer.GetNextPrayer(prayerCalendar, lastDayPrayer, prayerDay, payload.PrayerUnixTime)
	nextPrayerTime := time.Unix(nextPrayer.UnixTime, 0)

	now := time.Now().In(location)
	duration := nextPrayerTime.Sub(now)
	err = prayer.SchedulePrayerReminder(
		&duration,
		task.PrayerReminderPayload{
			UserID:         payload.UserID,
			PrayerName:     nextPrayer.Name,
			PrayerUnixTime: nextPrayer.UnixTime,
			IsLastDay:      isNextPrayerLastDay,
		},
	)

	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to schedule prayer reminder")
		return err
	}
	logWithCtx.Info().Msg("successfully scheduled prayer reminder")

	if user.AccountType == repository.AccountTypePREMIUM {
		var prayerTimeDistance time.Duration
		if payload.PrayerName == prayer.SubuhPrayerName && isLastDay {
			sunriseTime := time.Unix(lastDayPrayer[1].UnixTime, 0)
			prayerTimeDistance = sunriseTime.Sub(prayerTime)
		} else if payload.PrayerName == prayer.SubuhPrayerName && isLastDay == false {
			todayPrayer := prayerCalendar[prayerDay-1]
			sunriseTime := time.Unix(todayPrayer[1].UnixTime, 0)
			prayerTimeDistance = sunriseTime.Sub(prayerTime)
		} else {
			prayerTimeDistance = nextPrayerTime.Sub(prayerTime)
		}

		quarterTime := int(math.Round(prayerTimeDistance.Seconds() * 0.25))
		lastReminderDuration := duration - time.Duration(quarterTime)

		err = prayer.ScheduleLastPrayerReminder(
			&lastReminderDuration,
			task.LastPrayerReminderPayload{
				UserID:     payload.UserID,
				PrayerName: payload.PrayerName,
			},
		)

		if err != nil {
			return errors.Wrap(err, "failed to schedule last prayer reminder")
		}
		logWithCtx.Info().Msg("successfully scheduled last prayer reminder")
	}

	params := twilioApi.CreateMessageParams{}
	params.SetFrom("whatsapp:+14155238886")
	params.SetTo(fmt.Sprintf("whatsapp:%s", user.PhoneNumber.String))
	msg := fmt.Sprintf(
		"Hai! Sudah waktunya salat %s nih... Yuk segera tunaikan dan jangan lupa untuk memperbarui kemajuan kamu di aplikasi Demi Masa.",
		payload.PrayerName,
	)
	params.SetBody(msg)

	_, err = twilioClient.Api.CreateMessage(&params)
	if err != nil {
		logWithCtx.Error().Err(err).Str("phone_number", user.PhoneNumber.String).Msg("failed to send prayer reminder")
		return err
	}
	logWithCtx.Info().Str("phone_number", user.PhoneNumber.String).Msg("successfully sent prayer reminder")

	return nil
}

func handleLastPrayerReminder(ctx context.Context, asynqTask *asynq.Task) error {
	logWithCtx := log.Ctx(ctx).With().Logger()
	var payload task.PrayerReminderPayload
	if err := json.Unmarshal(asynqTask.Payload(), &payload); err != nil {
		logWithCtx.Error().Err(err).Msg("failed to unmarshal last prayer reminder task payload")
		return err
	}

	userPhone, err := queries.GetUserPhoneByID(ctx, payload.UserID)
	if err != nil {
		logWithCtx.Error().Err(err).Str("user_id", payload.UserID).Msg("failed to get user phone by id")
		return err
	}
	logWithCtx.Info().Str("user_id", payload.UserID).Msg("successfully get user phone by id")

	params := twilioApi.CreateMessageParams{}
	params.SetFrom("whatsapp:+14155238886")
	params.SetTo(fmt.Sprintf("whatsapp:%s", userPhone.String))
	msg := fmt.Sprintf(
		"Waktu salat %s sudah hampir habis. Yuk, segera lakukan sebelum terlambat.",
		payload.PrayerName,
	)
	params.SetBody(msg)

	_, err = twilioClient.Api.CreateMessage(&params)
	if err != nil {
		logWithCtx.Error().Err(err).Str("phone_number", userPhone.String).Msg("failed to send last prayer reminder")
		return err
	}
	logWithCtx.Info().Str("phone_number", userPhone.String).Msg("successfully sent last prayer reminder")

	return nil
}

func handlePrayerRenewal(ctx context.Context, asynqTask *asynq.Task) error {
	logWithCtx := log.Ctx(ctx).With().Logger()
	var payload task.PrayerRenewalTask
	if err := json.Unmarshal(asynqTask.Payload(), &payload); err != nil {
		logWithCtx.Error().Err(err).Msg("failed to unmarshal prayer renewal task payload")
		return err
	}

	location, err := time.LoadLocation(string(payload.TimeZone))
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to load time zone location")
		return err
	}
	logWithCtx.Info().Msg("successfully loaded time zone location")

	now := time.Now().In(location)
	year := now.Year()
	month, err := strconv.Atoi(fmt.Sprintf("%d", now.Month()))
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to convert string to integer")
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
		logWithCtx.Error().Err(err).Msg("failed to get aladhan prayer calendar")
		return err
	}
	logWithCtx.Info().Msg("successfully get aladhan prayer calendar")

	parsedPrayerCalendar, err := prayer.ParseAladhanPrayerCalendar(unparsedPrayerCalendar, location, payload.TimeZone)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to parse aladhan prayer calendar")
		return err
	}
	logWithCtx.Info().Msg("successfully parsed aladhan prayer calendar")

	parsedPrayerCalendarJSON, err := json.Marshal(parsedPrayerCalendar)
	if err != nil {
		return errors.Wrap(err, "failed to marshal parsed aladhan prayer calendar")
	}

	err = redisClient.Watch(ctx, func(tx *redis.Tx) error {
		oldPrayerCalendar, err := prayer.GetPrayerCalendar(ctx, payload.TimeZone)
		if err != nil {
			return errors.Wrap(err, "failed to get prayer calendar")
		}

		penultimateDayPrayer := parsedPrayerCalendar[len(parsedPrayerCalendar)-2]
		penultimateDayPrayerJSON, err := json.Marshal(penultimateDayPrayer)
		if err != nil {
			return errors.Wrap(err, "failed to marshal penultimate day prayer of old prayer calendar")
		}

		lastDayPrayer := oldPrayerCalendar[len(oldPrayerCalendar)-1]
		lastDayPrayerJSON, err := json.Marshal(lastDayPrayer)
		if err != nil {
			return errors.Wrap(err, "failed to marshal last day prayer of old prayer calendar")
		}

		duration := time.Hour * 24 * 3
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, prayer.MakePrayerCalendarKey(payload.TimeZone), parsedPrayerCalendarJSON, 0)
			pipe.Set(ctx, prayer.MakeLastDayPrayerKey(payload.TimeZone), lastDayPrayerJSON, duration)
			pipe.Set(ctx, prayer.MakePenultimateDayPrayerKey(payload.TimeZone), penultimateDayPrayerJSON, duration)
			return nil
		})

		return err
	})

	numOfDays := len(parsedPrayerCalendar)
	err = prayer.SchedulePrayerRenewal(
		numOfDays,
		location,
		&now,
		task.PrayerRenewalTask{TimeZone: payload.TimeZone},
	)

	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to schedule prayer renewal")
		return err
	}
	logWithCtx.Info().Msg("successfully scheduled prayer renewal")

	return nil
}

func handleTaskRemoval(ctx context.Context, asynqTask *asynq.Task) error {
	logWithCtx := log.Ctx(ctx).With().Logger()
	err := queries.RemoveCheckedTask(ctx)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to delete checked tasks")
		return err
	}
	logWithCtx.Info().Msg("successfully deleted checked tasks")

	err = task.ScheduleTaskRemovalTask()
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to schedule task removal task")
		return err
	}
	logWithCtx.Info().Msg("successfully scheduled task removal task")

	return nil
}
