package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func getUserHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	userID := chi.URLParam(req, "userID")
	ctx := context.Background()

	user, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logWithCtx.Info().Str("user_id", userID).Msg("user not found")
			errMsg := fmt.Sprintf("user dengan id %s tidak ditemukan", userID)
			http.Error(res, errMsg, http.StatusNotFound)
		} else {
			logWithCtx.Error().Err(err).Str("user_id", userID).Msg("failed to get user by id")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}
	logWithCtx.Info().Str("user_id", userID).Msg("successfully get user by id")

	err = sendJSONSuccessResponse(res, SuccessResponseParams{StatusCode: http.StatusOK, Data: user})
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to send json success response")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
