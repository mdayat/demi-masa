package httpserver

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
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
