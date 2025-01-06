package internal

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/mdayat/demi-masa/pkg/prayer"
	"github.com/mdayat/demi-masa/pkg/task"
	"github.com/mdayat/demi-masa/web/configs/services"
	"github.com/mdayat/demi-masa/web/repository"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func deleteUserHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()

	userID := chi.URLParam(req, "userID")
	_, err := services.Queries.DeleteUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusNotFound).Msg("user not found")
			http.Error(res, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to delete user by id")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}
	logWithCtx.Info().Int("status_code", http.StatusOK).Dur("response_time", time.Since(start)).Msg("request completed")
}

func addUserToTaskQueue(ctx context.Context) (nextPrayer prayer.Prayer, err error) {
	timeZone := fmt.Sprintf("%s", ctx.Value("time_zone"))
	userID := fmt.Sprintf("%s", ctx.Value("userID"))

	prayerCalendar, err := prayer.GetPrayerCalendar(ctx, services.RedisClient, timeZone)
	if err != nil {
		return nextPrayer, errors.Wrap(err, "failed to get prayer calendar")
	}

	lastDayPrayer, err := prayer.GetLastDayPrayer(ctx, services.RedisClient, timeZone)
	if err != nil {
		return nextPrayer, errors.Wrap(err, "failed to get last day prayer")
	}

	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return nextPrayer, errors.Wrap(err, "failed to load time zone location")
	}

	now := time.Now().In(location)
	currentDay := now.Day()
	currentUnixTime := now.Unix()

	isLastDay := prayer.IsLastDay(now)
	isPenultimateDay := prayer.IsPenultimateDay(now)
	isyaPrayer := prayerCalendar[currentDay-1][5]

	if isLastDay {
		isyaPrayer = lastDayPrayer[5]
	}

	var isNextPrayerLastDay bool
	if (isPenultimateDay && currentUnixTime > isyaPrayer.UnixTime) ||
		(isLastDay && currentUnixTime < isyaPrayer.UnixTime) {
		isNextPrayerLastDay = true
	}

	if isLastDay && currentUnixTime < isyaPrayer.UnixTime {
		nextPrayer = prayer.GetNextPrayer(prayerCalendar, lastDayPrayer, currentDay, currentUnixTime)
	} else {
		if isLastDay && currentUnixTime > isyaPrayer.UnixTime {
			currentDay = 1
		}

		nextPrayer = prayer.GetNextPrayer(prayerCalendar, nil, currentDay, currentUnixTime)
	}

	asynqTask, err := task.NewPrayerReminderTask(task.PrayerReminderPayload{
		UserID:         userID,
		PrayerName:     nextPrayer.Name,
		PrayerUnixTime: nextPrayer.UnixTime,
		IsLastDay:      isNextPrayerLastDay,
	})

	if err != nil {
		return nextPrayer, errors.Wrap(err, "failed to create prayer reminder task")
	}

	duration := time.Duration(nextPrayer.UnixTime-currentUnixTime) * time.Second
	_, err = services.AsynqClient.Enqueue(asynqTask, asynq.ProcessIn(duration))
	if err != nil {
		return nextPrayer, errors.Wrap(err, "failed to enqueue prayer reminder task")
	}

	return nextPrayer, nil
}

func updateTimeZone(ctx context.Context, userID string) error {
	tx, err := services.DB.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start db tx")
	}

	qtx := services.Queries.WithTx(tx)
	userTimeZone, err := qtx.GetUserTimeZoneByID(ctx, userID)
	if err != nil {
		return errors.Wrap(err, "failed to get user time zone by id")
	}

	timeZone := fmt.Sprintf("%s", ctx.Value("time_zone"))
	err = qtx.UpdateUserTimeZone(ctx, repository.UpdateUserTimeZoneParams{
		ID:       userID,
		TimeZone: repository.NullIndonesiaTimeZone{IndonesiaTimeZone: repository.IndonesiaTimeZone(timeZone), Valid: true}},
	)
	if err != nil {
		return errors.Wrap(err, "failed to update user time zone")
	}

	var nextPrayer prayer.Prayer
	if userTimeZone.Valid == false {
		nextPrayer, err = addUserToTaskQueue(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to add user to task queue")
		}
	}

	err = tx.Commit(ctx)
	if err != nil && userTimeZone.Valid {
		return errors.Wrap(err, "failed to commit db tx")
	}

	if err != nil && userTimeZone.Valid == false {
		return retry.Do(func() error {
			asynqTaskID := task.MakePrayerReminderTaskID(userID, nextPrayer.Name)
			err = services.AsynqInspector.DeleteTask(task.DefaultQueue, asynqTaskID)
			if err != nil {
				return errors.Wrap(err, "failed to delete prayer reminder")
			}

			return nil
		})
	}

	return nil
}

func updateTimeZoneHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()
	var body struct {
		TimeZone repository.IndonesiaTimeZone `json:"time_zone" validate:"required"`
	}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusBadRequest).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	userID := fmt.Sprintf("%s", ctx.Value("userID"))
	err = updateTimeZone(context.WithValue(ctx, "time_zone", body.TimeZone), userID)
	if err != nil {
		logWithCtx.
			Error().
			Err(err).
			Caller().
			Int("status_code", http.StatusInternalServerError).
			Str("user_id", userID).
			Str("time_zone", string(body.TimeZone)).
			Msg("failed to update user time zone and add user to task queue")

		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Int("status_code", http.StatusOK).Dur("response_time", time.Since(start)).Msg("request completed")
}
