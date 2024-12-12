package httpserver

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/rs/zerolog/log"
)

type prayer struct {
	ID       string                       `json:"id"`
	Name     string                       `json:"name"`
	Time     int64                        `json:"time"`
	TimeZone repository.IndonesiaTimeZone `json:"time_zone"`
	Status   repository.PrayerStatus      `json:"status"`
	Year     int16                        `json:"year"`
	Month    int16                        `json:"month"`
	Day      int16                        `json:"day"`
}

func getPrayersHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	userID := fmt.Sprintf("%s", req.Context().Value("userID"))

	prayers, err := queries.GetAndSortPrayers(req.Context(), userID)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get prayers based on user id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully get prayers based on user id")

	respBody := make([]prayer, len(prayers))
	for i, v := range prayers {
		prayerID, err := v.ID.Value()
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to get prayer UUID from pgtype.UUID")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		respBody[i] = prayer{
			ID:       fmt.Sprintf("%s", prayerID),
			Name:     v.Name,
			Time:     v.Time,
			TimeZone: v.TimeZone,
			Status:   v.Status,
			Year:     v.Year,
			Month:    v.Month,
			Day:      v.Day,
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
