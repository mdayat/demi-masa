package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
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

		userID := fmt.Sprintf("%s", ctx.Value("userID"))
		err = queries.UpdateUserTimeZone(
			ctx,
			repository.UpdateUserTimeZoneParams{
				ID:       userID,
				TimeZone: repository.NullIndonesiaTimeZone{IndonesiaTimeZone: body.TimeZone, Valid: true}},
		)

		if err != nil {
			logWithCtx.Error().Err(err).Str("time_zone", string(body.TimeZone)).Msg("failed to update user time zone")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Str("time_zone", string(body.TimeZone)).Msg("successfully updated user time zone")
	}
}
