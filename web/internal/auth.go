package internal

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mdayat/demi-masa/web/configs/services"
	"github.com/mdayat/demi-masa/web/repository"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type idTokenClaims struct {
	Name  string
	Email string
}

func loginHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()
	var body struct {
		IDToken string `json:"id_token" validate:"required,jwt"`
	}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusBadRequest).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token, err := services.FirebaseAuth.VerifyIDToken(ctx, body.IDToken)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusUnauthorized).Msg("invalid id token")
		http.Error(res, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var idTokenClaims idTokenClaims
	err = mapstructure.Decode(token.Claims, &idTokenClaims)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to convert id token claims map to struct")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	user, err := services.Queries.GetUserByID(ctx, token.UID)
	if err != nil && errors.Is(err, pgx.ErrNoRows) == false {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to get user by id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	statusCode := http.StatusOK
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		user, err = services.Queries.CreateUser(ctx, repository.CreateUserParams{
			ID:    token.UID,
			Name:  idTokenClaims.Name,
			Email: idTokenClaims.Email,
		})

		if err != nil {
			logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to create new user")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		res.Header().Set("Location", fmt.Sprintf("/users/%s", user.ID))
		statusCode = http.StatusCreated
	}

	respBody := struct {
		PhoneNumber   string                       `json:"phone_number,omitempty"`
		PhoneVerified bool                         `json:"phone_verified"`
		AccountType   repository.AccountType       `json:"account_type"`
		TimeZone      repository.IndonesiaTimeZone `json:"time_zone,omitempty"`
	}{
		PhoneNumber:   user.PhoneNumber.String,
		PhoneVerified: user.PhoneVerified,
		AccountType:   user.AccountType,
		TimeZone:      user.TimeZone.IndonesiaTimeZone,
	}

	err = sendJSONSuccessResponse(res, successResponseParams{StatusCode: statusCode, Data: respBody})
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to send successful response body")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Int("status_code", statusCode).Dur("response_time", time.Since(start)).Msg("request completed")
}
