package workerserver

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mdayat/demi-masa-be/internal/prayer"
	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/pkg/errors"
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

	var prayerCalendar []prayer.PrayerCalendar
	if user.TimeZone.IndonesiaTimeZone == repository.IndonesiaTimeZone(prayer.WIBTimeZone) {
		prayerCalendar = prayer.WIBPrayerCalendar
	} else if user.TimeZone.IndonesiaTimeZone == repository.IndonesiaTimeZone(prayer.WITTimeZone) {
		prayerCalendar = prayer.WITPrayerCalendar
	} else {
		prayerCalendar = prayer.WITAPrayerCalendar
	}

	now := time.Now().In(location)
	currentDay := now.Day()
	currentTimestamp := now.Unix()

	nextPrayer, prevPrayer := prayer.GetNextAndPrevPrayer(prayerCalendar, currentDay, currentTimestamp)
	duration := time.Unix(nextPrayer.Timestamp, 0).Sub(now)
	err = prayer.SchedulePrayerReminder(
		&duration,
		task.PrayerReminderPayload{
			UserID:     payload.UserID,
			PrayerName: nextPrayer.Name,
		},
	)

	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to schedule prayer reminder")
		return err
	}
	logWithCtx.Info().Msg("successfully scheduled prayer reminder")

	if user.AccountType == repository.AccountTypePREMIUM {
		if payload.PrayerName == prayer.SubuhPrayerName {
			todayPrayer := prayerCalendar[currentDay-1]
			sunriseTime := time.Unix(todayPrayer[1].Timestamp, 0)
			subuhTime := time.Unix(todayPrayer[0].Timestamp, 0)

			prayerTimeDistance := sunriseTime.Sub(subuhTime)
			quarterTime := int(math.Round(prayerTimeDistance.Seconds() * 0.25))
			lastReminderDuration := duration - time.Duration(quarterTime)*time.Second

			err = prayer.ScheduleLastPrayerReminder(
				&lastReminderDuration,
				task.PrayerReminderPayload{
					UserID:     payload.UserID,
					PrayerName: payload.PrayerName,
				},
			)

			if err != nil {
				return errors.Wrap(err, "failed to schedule last prayer reminder")
			}
			logWithCtx.Info().Msg("successfully scheduled last prayer reminder")
		} else {
			prayerTimeDistance := time.Unix(nextPrayer.Timestamp, 0).Sub(time.Unix(prevPrayer.Timestamp, 0))
			quarterTime := int(math.Round(prayerTimeDistance.Seconds() * 0.25))
			lastReminderDuration := duration - time.Duration(quarterTime)*time.Second

			err = prayer.ScheduleLastPrayerReminder(
				&lastReminderDuration,
				task.PrayerReminderPayload{
					UserID:     payload.UserID,
					PrayerName: payload.PrayerName,
				},
			)

			if err != nil {
				return errors.Wrap(err, "failed to schedule last prayer reminder")
			}
			logWithCtx.Info().Msg("successfully scheduled last prayer reminder")
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

	_, err = twilioClient.Api.CreateMessage(&params)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to send prayer reminder")
		return err
	}
	logWithCtx.Info().Msg("successfully sent prayer reminder")

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
		logWithCtx.Error().Err(err).Msg("failed to send last prayer reminder")
		return err
	}
	logWithCtx.Info().Msg("successfully sent last prayer reminder")

	return nil
}
