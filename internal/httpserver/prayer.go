package httpserver

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mdayat/demi-masa-be/internal/prayer"
	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type prayerRespBody struct {
	ID       string                  `json:"id"`
	Name     string                  `json:"name"`
	Status   repository.PrayerStatus `json:"status"`
	UnixTime int64                   `json:"unix_time"`
}

func getUsedPrayers(ctx context.Context, timeZone repository.IndonesiaTimeZone) (prayer.Prayers, error) {
	location, err := time.LoadLocation(string(timeZone))
	if err != nil {
		return nil, errors.Wrap(err, "failed to load time zone location")
	}

	prayerCalendar, err := prayer.GetPrayerCalendar(ctx, timeZone)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get prayer calendar")
	}

	now := time.Now().In(location)
	currentDay := now.Day()
	currentUnixTime := now.Unix()
	isLastDay := prayer.IsLastDay(&now)

	todayPrayer := prayerCalendar[currentDay-1]
	subuhPrayer := todayPrayer[0]
	var usedPrayers prayer.Prayers

	if currentDay == 1 && currentUnixTime < subuhPrayer.UnixTime ||
		isLastDay && currentUnixTime > subuhPrayer.UnixTime {
		usedPrayers, err = prayer.GetLastDayPrayer(ctx, timeZone)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get last day prayer")
		}
	} else if isLastDay && currentUnixTime < subuhPrayer.UnixTime {
		usedPrayers, err = prayer.GetPenultimateDayPrayer(ctx, timeZone)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get penultimate day prayer")
		}
	} else if isLastDay == false && currentUnixTime < subuhPrayer.UnixTime {
		yesterdayPrayer := prayerCalendar[currentDay-2]
		usedPrayers = yesterdayPrayer
	} else {
		usedPrayers = todayPrayer
	}

	return usedPrayers, nil
}

type bulkInsertPrayerParams struct {
	userID string
	year   int16
	month  int16
	day    int16
}

func bulkInsertPrayer(
	ctx context.Context,
	usedPrayers prayer.Prayers,
	arg *bulkInsertPrayerParams,
) ([]repository.GetTodayPrayersRow, error) {
	createPrayersParams := make([]repository.CreatePrayersParams, 0, len(usedPrayers)-1)
	todayPrayers := make([]repository.GetTodayPrayersRow, 0, len(usedPrayers)-1)

	for _, v := range usedPrayers {
		if v.Name == prayer.SunriseTimeName {
			continue
		}

		prayerUUID := uuid.New()
		createPrayersParams = append(createPrayersParams, repository.CreatePrayersParams{
			ID:     pgtype.UUID{Bytes: prayerUUID, Valid: true},
			UserID: arg.userID,
			Name:   v.Name,
			Year:   arg.year,
			Month:  arg.month,
			Day:    arg.day,
		})

		todayPrayers = append(todayPrayers, repository.GetTodayPrayersRow{
			ID:     pgtype.UUID{Bytes: prayerUUID, Valid: true},
			Name:   v.Name,
			Status: repository.PrayerStatusMISSED,
		})
	}

	_, err := queries.CreatePrayers(ctx, createPrayersParams)
	if err != nil {
		return nil, errors.Wrap(err, "failed to bulk insert today prayers")
	}

	return todayPrayers, nil
}

func getPrayersHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	userID := fmt.Sprintf("%s", req.Context().Value("userID"))

	userTimeZone, err := queries.GetUserTimeZoneByID(req.Context(), userID)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get user time zone by id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully got user time zone by id")

	usedPrayers, err := getUsedPrayers(req.Context(), userTimeZone.IndonesiaTimeZone)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get used prayers")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully got used prayers")

	subuhTime := time.Unix(usedPrayers[0].UnixTime, 0)
	usedPrayersYear := subuhTime.Year()
	usedPrayersMonth := subuhTime.Month()
	usedPrayersDay := subuhTime.Day()

	todayPrayers, err := queries.GetTodayPrayers(req.Context(), repository.GetTodayPrayersParams{
		UserID: userID,
		Year:   int16(usedPrayersYear),
		Month:  int16(usedPrayersMonth),
		Day:    int16(usedPrayersDay),
	})

	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get today prayers")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully got today prayer")

	if len(todayPrayers) == 0 {
		todayPrayers, err = bulkInsertPrayer(req.Context(), usedPrayers, &bulkInsertPrayerParams{
			userID: userID,
			year:   int16(usedPrayersYear),
			month:  int16(usedPrayersMonth),
			day:    int16(usedPrayersDay),
		})

		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to bulk insert today prayers")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Msg("successfully bulk inserted today prayers")
	}

	respBody := make([]prayerRespBody, 0, len(todayPrayers))
	for _, v := range usedPrayers {
		if v.Name == prayer.SunriseTimeName {
			continue
		}

		for _, p := range todayPrayers {
			if p.Name != v.Name {
				continue
			}

			prayerID, err := p.ID.Value()
			if err != nil {
				logWithCtx.Error().Err(err).Msg("failed to get prayer UUID from pgtype.UUID")
				http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			respBody = append(respBody, prayerRespBody{
				ID:       fmt.Sprintf("%s", prayerID),
				Name:     v.Name,
				Status:   p.Status,
				UnixTime: v.UnixTime,
			})
			break
		}
	}

	err = sendJSONSuccessResponse(res, successResponseParams{StatusCode: http.StatusOK, Data: &respBody})
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to send successful response body")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully sent successful response body")
}

func updatePrayerHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	var body struct {
		PrayerName     string                       `json:"prayer_name" validate:"required"`
		PrayerUnixTime int64                        `json:"prayer_unix_time" validate:"required"`
		TimeZone       repository.IndonesiaTimeZone `json:"time_zone" validate:"required"`
		CheckedAt      int64                        `json:"checked_at" validate:"required"`
		AccountType    repository.AccountType       `json:"account_type" validate:"required"`
	}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	logWithCtx.Info().Msg("successfully decoded and validated request body")

	prayerCalendar, err := prayer.GetPrayerCalendar(req.Context(), body.TimeZone)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get prayer calendar")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully got prayer calendar")

	prayerTime := time.Unix(body.PrayerUnixTime, 0)
	prayerDay := prayerTime.Day()
	isLastDayPrayer := prayer.IsLastDay(&prayerTime)
	isPenultimateDayPrayer := prayer.IsPenultimateDay(&prayerTime)

	checkedTime := time.Unix(body.CheckedAt, 0)
	isCheckedAtLastDay := prayer.IsLastDay(&checkedTime)

	var usedPrayers prayer.Prayers
	if isPenultimateDayPrayer && isCheckedAtLastDay && body.PrayerName != prayer.IsyaPrayerName {
		usedPrayers, err = prayer.GetPenultimateDayPrayer(req.Context(), body.TimeZone)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to get penultimate day prayer")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Msg("successfully got penultimate day prayer")
	} else if isPenultimateDayPrayer && isCheckedAtLastDay && body.PrayerName == prayer.IsyaPrayerName ||
		isLastDayPrayer && body.PrayerName != prayer.IsyaPrayerName {
		usedPrayers, err = prayer.GetLastDayPrayer(req.Context(), body.TimeZone)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to get last day prayer")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Msg("successfully got last day prayer")
	} else if isLastDayPrayer && body.PrayerName == prayer.IsyaPrayerName {
		prayerDay = 1
	}

	var nextPrayer prayer.Prayer
	if body.PrayerName == prayer.SubuhPrayerName {
		var sunriseTime prayer.Prayer
		if usedPrayers != nil {
			sunriseTime = usedPrayers[1]
		} else {
			sunriseTime = prayerCalendar[prayerDay-1][1]
		}
		nextPrayer = sunriseTime
	} else {
		nextPrayer = prayer.GetNextPrayer(prayerCalendar, usedPrayers, prayerDay, body.PrayerUnixTime)
	}

	nextPrayerTime := time.Unix(nextPrayer.UnixTime, 0)
	prayersDistance := nextPrayerTime.Sub(prayerTime)

	distanceQuarter := int(math.Round(prayersDistance.Seconds() * 0.25))
	distanceToNextPrayer := nextPrayerTime.Sub(checkedTime)

	var prayerStatus repository.PrayerStatus
	if body.CheckedAt > nextPrayer.UnixTime {
		prayerStatus = repository.PrayerStatusMISSED
	} else if distanceToNextPrayer.Seconds()-float64(distanceQuarter) > 0 {
		prayerStatus = repository.PrayerStatusONTIME
	} else {
		prayerStatus = repository.PrayerStatusLATE
	}

	prayerID := chi.URLParam(req, "prayerID")
	prayerIDBytes, err := uuid.Parse(prayerID)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to parse prayer uuid string to bytes")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully parsed prayer uuid string to bytes")

	err = queries.UpdatePrayer(req.Context(), repository.UpdatePrayerParams{
		ID:     pgtype.UUID{Bytes: prayerIDBytes, Valid: true},
		Status: prayerStatus,
	})

	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to update prayer status")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully updated prayer status")

	userID := fmt.Sprintf("%s", req.Context().Value("userID"))
	if body.AccountType == repository.AccountTypePREMIUM {
		asynqTaskID := task.LastPrayerReminderTaskID(userID, body.PrayerName)
		err = asynqInspector.DeleteTask(task.DefaultQueue, asynqTaskID)
		if err != nil {
			if errors.Is(err, asynq.ErrQueueNotFound) {
				logWithCtx.Error().Err(err).Str("queue_name", task.DefaultQueue).Send()
			} else if errors.Is(err, asynq.ErrTaskNotFound) {
				logWithCtx.Error().Err(err).Str("task_id", asynqTaskID).Send()
			} else {
				logWithCtx.Error().Err(err).Str("task_id", asynqTaskID).Msg("failed to delete last prayer reminder")
				http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		} else {
			logWithCtx.Info().Str("task_id", asynqTaskID).Msg("successfully deleted last prayer reminder")
		}
	}
}
