package asynqmon

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/odemimasa/backend/internal/config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

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

func verifyAccessToken(accessTokenString string) error {
	accessToken, err := jwt.Parse(accessTokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); ok == false {
			return nil, errors.New("invalid signing method")
		}
		return []byte(config.Env.ACCESS_TOKEN_SECRET_KEY), nil
	})

	if err != nil {
		return errors.Wrap(err, "failed to parse access token")
	}

	claims, ok := accessToken.Claims.(jwt.MapClaims)
	if ok == false {
		return errors.New("invalid access token claims")
	}

	email := claims["email"].(string)
	emails := strings.Split(config.Env.AUTHORIZED_EMAILS, ",")
	var isUserAuthorized bool

	for _, v := range emails {
		if email == v {
			isUserAuthorized = true
			break
		}
	}

	if isUserAuthorized == false {
		return errors.New("invalid access token")
	}

	return nil
}

func authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		logWithCtx := log.Ctx(req.Context()).With().Logger()

		cookie, err := req.Cookie("access_token")
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) == false {
				errMsg := "failed to get access token from cookie"
				logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg(errMsg)
				http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			ext := filepath.Ext(req.URL.Path)
			if req.URL.Path != "/" && ext == "" {
				http.Redirect(res, req, "/", http.StatusFound)
			} else {
				next.ServeHTTP(res, req)
			}
			return
		}

		if err := cookie.Valid(); err != nil {
			cookie := http.Cookie{
				Name:     "access_token",
				Value:    "",
				Expires:  time.Unix(0, 0),
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteStrictMode,
				Domain:   strings.TrimPrefix(config.Env.ASYNQMON_BASE_URL, "https://"),
				Path:     "/",
			}

			logWithCtx.Error().Err(err).Msg("invalid cookie")
			http.SetCookie(res, &cookie)
			http.Redirect(res, req, "/", http.StatusFound)
			return
		}

		err = verifyAccessToken(cookie.Value)
		if err != nil {
			cookie := http.Cookie{
				Name:     "access_token",
				Value:    "",
				Expires:  time.Unix(0, 0),
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteStrictMode,
				Domain:   strings.TrimPrefix(config.Env.ASYNQMON_BASE_URL, "https://"),
				Path:     "/",
			}

			logWithCtx.Error().Err(err).Msg("invalid access token")
			http.SetCookie(res, &cookie)
			http.Redirect(res, req, "/", http.StatusFound)
			return
		}

		if req.URL.Path == "/" {
			http.Redirect(res, req, "/monitoring", http.StatusFound)
			return
		}

		next.ServeHTTP(res, req)
	})
}
