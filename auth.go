package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/go-redis/redis"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

type IDTokenClaims struct {
	Name  string
	Email string
}

func loginHandler(res http.ResponseWriter, req *http.Request) {
	body := struct {
		IDToken string `json:"id_token"`
	}{}

	err := json.NewDecoder(req.Body).Decode(&body)
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "loginHandler()")).Msg("invalid json body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token, err := firebaseAuth.VerifyIDToken(context.Background(), body.IDToken)
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "loginHandler()")).Msg("invalid id token")
		http.Error(res, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	log.Ctx(req.Context()).Info().Msg("successfully verified id token")

	var idTokenClaims IDTokenClaims
	err = mapstructure.Decode(token.Claims, &idTokenClaims)
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "loginHandler()")).Msg("failed to convert map of id token claims to struct")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	ctx := context.Background()
	_, err = queries.GetUserByID(ctx, token.UID)
	if err != nil && errors.Is(err, pgx.ErrNoRows) == false {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "loginHandler()")).Msg("failed to get user by UID")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	log.Ctx(req.Context()).Info().Str("user_id", token.UID).Msg("successfully get user by UID")

	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		user := repository.CreateUserParams{
			ID:    token.UID,
			Name:  idTokenClaims.Name,
			Email: idTokenClaims.Email,
		}

		err = queries.CreateUser(ctx, user)
		if err != nil {
			log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "loginHandler()")).Msg("failed to create new user")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		log.Ctx(req.Context()).Info().Str("user_id", token.UID).Msg("successfully created new user")

		res.Header().Set("Location", fmt.Sprintf("/users/%s", user.ID))
		res.WriteHeader(http.StatusCreated)
		return
	}

	log.Ctx(req.Context()).Info().Str("user_id", token.UID).Msg("successfully signed in")
	res.WriteHeader(http.StatusOK)
}

var OTP_GEN_LIMIT = 3
var OTP_SUBMISSION_LIMIT = 3
var OTP_DURATION = time.Minute * 3
var OTP_GEN_LIMIT_DURATION = time.Hour * 24

func generateOTP() string {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%d", 100000+rand.Intn(900000))
}

func generateOTPHandler(res http.ResponseWriter, req *http.Request) {
	body := struct {
		PhoneNumber string `json:"phone_number"`
	}{}

	err := json.NewDecoder(req.Body).Decode(&body)
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "generateOTPHandler()")).Msg("invalid json body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	user, err := queries.GetUserByPhoneNumber(ctx, pgtype.Text{String: body.PhoneNumber, Valid: true})
	if err != nil && errors.Is(err, pgx.ErrNoRows) == false {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "generateOTPHandler()")).Msg("failed to get user by phone number")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if user.PhoneNumber.String == body.PhoneNumber {
		log.Ctx(req.Context()).Info().Str("phone_number", body.PhoneNumber).Msg("phone number already used")
		http.Error(res, http.StatusText(http.StatusConflict), http.StatusConflict)
		return
	}

	otpGenLimitKey := fmt.Sprintf("%s:otp:gen_limit", body.PhoneNumber)
	otpSubmissionLimitKey := fmt.Sprintf("%s:otp:submission_limit", body.PhoneNumber)
	otpKey := fmt.Sprintf("%s:otp", body.PhoneNumber)

	otp, err := redisClient.Get(otpKey).Result()
	if err != nil && err != redis.Nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "generateOTPHandler()")).Msg("failed to get otp")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if otp != "" {
		remainingTime, err := redisClient.TTL(otpKey).Result()
		if err != nil {
			log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "generateOTPHandler()")).Msg("failed to get the remaining time of otp")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		log.Ctx(req.Context()).Info().Str("phone_number", body.PhoneNumber).Msg("otp already exist")
		res.Header().Set("Retry-After", fmt.Sprintf("%d", int(remainingTime.Seconds())))
		http.Error(res, http.StatusText(http.StatusConflict), http.StatusConflict)
		return
	}

	err = redisClient.SetNX(otpGenLimitKey, 0, OTP_GEN_LIMIT_DURATION).Err()
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "generateOTPHandler()")).Msg("failed to set otp generation limit")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	genCount, err := redisClient.Incr(otpGenLimitKey).Result()
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "generateOTPHandler()")).Msg("failed to increment otp generation limit")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if genCount > int64(OTP_GEN_LIMIT) {
		remainingTime, err := redisClient.TTL(otpGenLimitKey).Result()
		if err != nil {
			log.
				Ctx(req.Context()).
				Error().
				Err(errors.Wrap(err, "generateOTPHandler()")).
				Msg("failed to get the remaining time of otp generation limit")

			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		log.Ctx(req.Context()).Info().Str("phone_number", body.PhoneNumber).Msg("otp generation already reached its limit")
		res.Header().Set("Retry-After", fmt.Sprintf("%d", int(remainingTime.Seconds())))
		http.Error(res, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}

	otp = generateOTP()
	tx := redisClient.TxPipeline()
	tx.Set(otpKey, otp, OTP_DURATION)
	tx.Set(otpSubmissionLimitKey, 0, OTP_DURATION)
	_, err = tx.Exec()
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "generateOTPHandler()")).Msg("failed to create otp")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	log.Ctx(req.Context()).Info().Str("phone_number", body.PhoneNumber).Msg("successfully created otp")

	params := twilioApi.CreateMessageParams{}
	params.SetFrom("whatsapp:+14155238886")
	params.SetTo(fmt.Sprintf("whatsapp:%s", body.PhoneNumber))
	params.SetBody(fmt.Sprintf("Berikut adalah kode OTP Anda: %s", otp))

	_, err = twilioClient.Api.CreateMessage(&params)
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "generateOTPHandler()")).Msg("failed to send otp")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	log.Ctx(req.Context()).Info().Str("phone_number", body.PhoneNumber).Msg("successfully sent otp")
	res.WriteHeader(http.StatusCreated)
}

func verifyOTPHandler(res http.ResponseWriter, req *http.Request) {
	body := struct {
		PhoneNumber string `json:"phone_number"`
		UserOTP     string `json:"user_otp"`
	}{}

	err := json.NewDecoder(req.Body).Decode(&body)
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "verifyOTPHandler()")).Msg("invalid json body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	otpGenLimitKey := fmt.Sprintf("%s:otp:gen_limit", body.PhoneNumber)
	otpSubmissionLimitKey := fmt.Sprintf("%s:otp:submission_limit", body.PhoneNumber)
	otpKey := fmt.Sprintf("%s:otp", body.PhoneNumber)

	otp, err := redisClient.Get(otpKey).Result()
	if err != nil {
		if err == redis.Nil {
			log.Ctx(req.Context()).Info().Str("phone_number", body.PhoneNumber).Msg("otp not found")
			http.Error(res, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "verifyOTPHandler()")).Msg("failed to get otp")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	submissionCount, err := redisClient.Incr(otpSubmissionLimitKey).Result()
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "verifyOTPHandler()")).Msg("failed to increment otp submission limit")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if submissionCount > int64(OTP_SUBMISSION_LIMIT) {
		remainingTime, err := redisClient.TTL(otpKey).Result()
		if err != nil {
			log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "verifyOTPHandler()")).Msg("failed to get the remaining time of otp")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		log.Ctx(req.Context()).Info().Str("phone_number", body.PhoneNumber).Msg("otp submission already reached its limit")
		res.Header().Set("Retry-After", fmt.Sprintf("%d", int(remainingTime.Seconds())))
		http.Error(res, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}

	if body.UserOTP != otp {
		log.Ctx(req.Context()).Info().Str("phone_number", body.PhoneNumber).Msg("invalid otp")
		http.Error(res, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	log.Ctx(req.Context()).Info().Str("phone_number", body.PhoneNumber).Msg("successfully verified otp")

	tx := redisClient.TxPipeline()
	tx.Del(otpGenLimitKey)
	tx.Del(otpSubmissionLimitKey)
	tx.Del(otpKey)

	_, err = tx.Exec()
	if err != nil {
		log.
			Ctx(req.Context()).
			Error().
			Err(errors.Wrap(err, "verifyOTPHandler()")).
			Msgf("failed to delete %s, %s, and %s", otpGenLimitKey, otpSubmissionLimitKey, otpKey)

		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	userID := fmt.Sprintf("%s", req.Context().Value("userID"))
	ctx := context.Background()

	err = queries.UpdateUserPhoneNumber(
		ctx,
		repository.UpdateUserPhoneNumberParams{
			ID:            userID,
			PhoneNumber:   pgtype.Text{String: body.PhoneNumber, Valid: true},
			PhoneVerified: true,
		},
	)
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "verifyOTPHandler()")).Msg("failed to update user phone number")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	log.
		Ctx(req.Context()).
		Info().
		Str("user_id", userID).
		Str("phone_number", body.PhoneNumber).
		Msg("successfully updated user phone number")

	res.WriteHeader(http.StatusOK)
}
