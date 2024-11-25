package httpserver

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		bearerToken := req.Header.Get("Authorization")
		if bearerToken == "" || strings.Contains(bearerToken, "Bearer") == false {
			log.Ctx(req.Context()).Error().Err(errors.New("invalid authorization header")).Msg("")
			http.Error(res, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		token, err := firebaseAuth.VerifyIDToken(context.Background(), strings.TrimPrefix(bearerToken, "Bearer "))
		if err != nil {
			log.Ctx(req.Context()).Error().Err(err).Msg("invalid id token")
			http.Error(res, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(req.Context(), "userID", token.UID)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

type loggerResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (res *loggerResponseWriter) WriteHeader(statusCode int) {
	res.statusCode = statusCode
	res.ResponseWriter.WriteHeader(statusCode)
}

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		lres := loggerResponseWriter{res, http.StatusOK}
		start := time.Now()

		subLogger := log.With().Str("req_id", uuid.New().String()).Logger()
		ctx := subLogger.WithContext(req.Context())

		hostname, err := os.Hostname()
		if err != nil {
			hostname = req.Host
			subLogger.Error().Err(err).Msg("failed to get hostname")
		}

		subLogger.Info().
			Str("method", req.Method).
			Str("path", req.URL.Path).
			Str("query", req.URL.RawQuery).
			Str("client_ip", req.RemoteAddr).
			Str("user_agent", req.UserAgent()).
			Str("hostname", hostname).
			Msg("request received")

		defer func() {
			subLogger.Info().
				Int("status_code", lres.statusCode).
				Dur("res_time", time.Since(start)).
				Msg("request completed")
		}()

		next.ServeHTTP(&lres, req.WithContext(ctx))
	})
}
