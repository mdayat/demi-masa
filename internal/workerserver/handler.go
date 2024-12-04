package workerserver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mdayat/demi-masa-be/internal/prayer"
	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
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

func handleUserPrayer(ctx context.Context, asynqTask *asynq.Task) error {
	logWithCtx := log.Ctx(ctx).With().Logger()
	var payload task.UserPrayerPayload
	if err := json.Unmarshal(asynqTask.Payload(), &payload); err != nil {
		logWithCtx.Error().Err(err).Msg("failed to unmarshal user prayer task payload")
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
	nextPrayer := prayer.GetNextPrayer(&now, prayerCalendar)

	duration := nextPrayer.Time.Sub(now)
	err = prayer.ScheduleUserPrayer(
		&duration,
		task.UserPrayerPayload{
			UserID:     payload.UserID,
			PrayerName: nextPrayer.Name,
		},
	)

	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to schedule user prayer")
		return err
	}
	logWithCtx.Info().Msg("successfully scheduled user prayer")

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
	logWithCtx.Info().Msg("successful sent prayer reminder")

	return nil
}
