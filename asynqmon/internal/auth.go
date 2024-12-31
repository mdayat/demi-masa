package internal

import (
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mdayat/demi-masa/asynqmon/configs/env"
	"github.com/mdayat/demi-masa/asynqmon/configs/services"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func createAccessToken(email string, duration int64) (string, error) {
	signingMethod := jwt.SigningMethodHS256
	claims := jwt.MapClaims{
		"email": email,
		"iss":   env.ASYNQMON_BASE_URL,
		"exp":   duration,
	}

	token := jwt.NewWithClaims(signingMethod, claims)
	tokenString, err := token.SignedString([]byte(env.ACCESS_TOKEN_SECRET_KEY))
	if err != nil {
		return "", errors.Wrap(err, "failed to sign access token")
	}

	return tokenString, nil
}

func loginHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()
	var body struct {
		IDToken string `json:"id_token" validate:"required,jwt"`
		Email   string `json:"email" validate:"required,email"`
	}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusBadRequest).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	emails := strings.Split(env.AUTHORIZED_EMAILS, ",")
	var isUserAuthorized bool

	for _, v := range emails {
		if body.Email == v {
			isUserAuthorized = true
			break
		}
	}

	if isUserAuthorized == false {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusForbidden).Msg("unauthorized email")
		http.Error(res, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	_, err = services.FirebaseAuth.VerifyIDToken(ctx, body.IDToken)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusUnauthorized).Msg("invalid id token")
		http.Error(res, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	accessTokenDuration := time.Now().Add(30 * 24 * time.Hour)
	accessToken, err := createAccessToken(body.Email, accessTokenDuration.Unix())

	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to create access token")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	cookie := http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Expires:  accessTokenDuration,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Domain:   strings.TrimPrefix(env.ASYNQMON_BASE_URL, "https://"),
		Path:     "/",
	}

	http.SetCookie(res, &cookie)
	logWithCtx.Info().Int("status_code", http.StatusOK).Dur("response_time", time.Since(start)).Msg("request completed")
}
