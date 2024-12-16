package httpserver

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mdayat/demi-masa-be/internal/prayer"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/rs/zerolog/log"
)

type prayerRespBody struct {
	Name     string                       `json:"name"`
	UnixTime int64                        `json:"unix_time"`
	TimeZone repository.IndonesiaTimeZone `json:"time_zone"`
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

	location, err := time.LoadLocation(string(userTimeZone.IndonesiaTimeZone))
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to load time zone location")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully loaded time zone location")

	prayerCalendar, err := prayer.GetPrayerCalendar(req.Context(), userTimeZone.IndonesiaTimeZone)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get prayer calendar")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully got prayer calendar")

	now := time.Now().In(location)
	currentDay := now.Day()
	currentUnixTime := now.Unix()
	isLastDay := prayer.IsLastDay(&now)

	todayPrayer := prayerCalendar[currentDay-1]
	subuhPrayer := todayPrayer[0]
	var usedPrayer prayer.Prayers

	if currentDay == 1 && currentUnixTime < subuhPrayer.UnixTime ||
		isLastDay && currentUnixTime > subuhPrayer.UnixTime {
		usedPrayer, err = prayer.GetLastDayPrayer(req.Context(), userTimeZone.IndonesiaTimeZone)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to get last day prayer")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Msg("successfully got last day prayer")
	} else if isLastDay && currentUnixTime < subuhPrayer.UnixTime {
		usedPrayer, err = prayer.GetPenultimateDayPrayer(req.Context(), userTimeZone.IndonesiaTimeZone)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to get penultimate day prayer")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Msg("successfully got last day prayer")
	} else if isLastDay == false && currentUnixTime < subuhPrayer.UnixTime {
		yesterdayPrayer := prayerCalendar[currentDay-2]
		usedPrayer = yesterdayPrayer
	} else {
		usedPrayer = todayPrayer
	}

	respBody := make([]prayerRespBody, 0, len(usedPrayer)-1)
	for _, v := range usedPrayer {
		if v.Name == prayer.SunriseTimeName {
			continue
		}

		respBody = append(respBody, prayerRespBody{
			Name:     v.Name,
			UnixTime: v.UnixTime,
			TimeZone: userTimeZone.IndonesiaTimeZone,
		})
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
		Name   string                  `json:"name" validate:"required"`
		Status repository.PrayerStatus `json:"status" validate:"required"`
	}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	logWithCtx.Info().Msg("successfully decoded and validated request body")

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
		Status: body.Status,
	})

	if err != nil {
		logWithCtx.Error().Err(err).Str("prayer_id", prayerID).Msg("failed to update prayer status by id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Str("prayer_id", prayerID).Msg("successfully updated prayer status by id")

	userID := fmt.Sprintf("%s", req.Context().Value("userID"))
	asynqTaskID := fmt.Sprintf("%s:%s:last", userID, body.Name)

	err = asynqInspector.DeleteTask("default", asynqTaskID)
	if err != nil {
		logWithCtx.Error().Err(err).Str("task_id", asynqTaskID).Msg("failed to delete last prayer reminder")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Str("task_id", asynqTaskID).Msg("successfully deleted last prayer reminder")
}
