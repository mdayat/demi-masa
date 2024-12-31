package internal

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/mdayat/demi-masa/web/configs/services"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		logWithCtx := log.Ctx(req.Context()).With().Logger()
		bearerToken := req.Header.Get("Authorization")
		if bearerToken == "" || strings.Contains(bearerToken, "Bearer") == false {
			err := errors.New("invalid authorization header")
			logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusUnauthorized).Send()
			http.Error(res, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		token, err := services.FirebaseAuth.VerifyIDToken(context.Background(), strings.TrimPrefix(bearerToken, "Bearer "))
		if err != nil {
			logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusUnauthorized).Msg("invalid id token")
			http.Error(res, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(res, req.WithContext(context.WithValue(req.Context(), "userID", token.UID)))
	})
}

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		subLogger := log.
			With().
			Str("request_id", uuid.New().String()).
			Str("method", req.Method).
			Str("path", req.URL.Path).
			Str("client_ip", req.RemoteAddr).
			Logger()

		next.ServeHTTP(res, req.WithContext(subLogger.WithContext(req.Context())))
	})
}
