package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/mdayat/demi-masa-be/internal/prayer"
	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func deleteUserHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	userID := chi.URLParam(req, "userID")

	ctx := context.Background()
	_, err := queries.DeleteUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logWithCtx.Error().Err(err).Str("user_id", userID).Msg("user not found")
			http.Error(res, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			logWithCtx.Error().Err(err).Str("user_id", userID).Msg("failed to delete user by id")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}
	logWithCtx.Info().Str("user_id", userID).Msg("successfully deleted user")
}

func addUserToTaskQueue(ctx context.Context) (*prayer.Prayer, error) {
	timeZone := fmt.Sprintf("%s", ctx.Value("time_zone"))
	userID := fmt.Sprintf("%s", ctx.Value("userID"))

	prayerCalendar, err := prayer.GetPrayerCalendar(ctx, repository.IndonesiaTimeZone(timeZone))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get prayer calendar")
	}

	lastDayPrayer, err := prayer.GetLastDayPrayer(ctx, repository.IndonesiaTimeZone(timeZone))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get last day prayer")
	}

	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load time zone location")
	}

	now := time.Now().In(location)
	currentDay := now.Day()
	currentUnixTime := now.Unix()

	isLastDay := prayer.IsLastDay(&now)
	isPenultimateDay := prayer.IsPenultimateDay(&now)
	isyaPrayer := prayerCalendar[currentDay-1][5]

	if isLastDay {
		isyaPrayer = lastDayPrayer[5]
	}

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

	duration := time.Unix(nextPrayer.UnixTime, 0).Sub(now)
	err = prayer.SchedulePrayerReminder(
		&duration,
		task.PrayerReminderPayload{
			UserID:         userID,
			PrayerName:     nextPrayer.Name,
			PrayerUnixTime: nextPrayer.UnixTime,
			IsLastDay:      isNextPrayerLastDay,
		},
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to schedule prayer reminder")
	}

	return &nextPrayer, nil
}

func updateTimeZone(ctx context.Context) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start db tx")
	}

	userID := fmt.Sprintf("%s", ctx.Value("userID"))
	qtx := queries.WithTx(tx)
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

	var nextPrayer *prayer.Prayer
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
			asynqTaskID := task.PrayerReminderTaskID(userID, nextPrayer.Name)
			err = asynqInspector.DeleteTask(task.DefaultQueue, asynqTaskID)
			if err != nil {
				return errors.Wrap(err, "failed to delete prayer reminder")
			}

			return nil
		})
	}

	return nil
}

func updateTimeZoneHandler(res http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), time.Second*5)
	defer cancel()
	logWithCtx := log.Ctx(ctx).With().Logger()

	select {
	case <-ctx.Done():
		logWithCtx.Error().Err(errors.New("request timed out")).Send()
		http.Error(res, http.StatusText(http.StatusRequestTimeout), http.StatusRequestTimeout)
	default:
		var body struct {
			TimeZone repository.IndonesiaTimeZone `json:"time_zone" validate:"required"`
		}

		err := decodeAndValidateJSONBody(req, &body)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("invalid request body")
			http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		logWithCtx.Info().Msg("successfully decoded and validated request body")

		err = updateTimeZone(context.WithValue(ctx, "time_zone", body.TimeZone))
		if err != nil {
			logWithCtx.
				Error().
				Err(err).
				Str("time_zone", string(body.TimeZone)).
				Msg("failed to update user time zone and add user to task queue")

			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		logWithCtx.
			Info().
			Str("time_zone", string(body.TimeZone)).
			Msg("successfully updated user time zone and add user to task queue")
	}
}
