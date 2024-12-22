package httpservice

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

const (
	otpGenLimit         = 3
	otpSubmissionLimit  = 3
	otpDuration         = time.Minute * 2
	otpGenLimitDuration = time.Hour * 24
)

func makeOTPGenLimitKey(phoneNumber string) string {
	return fmt.Sprintf("%s:otp:gen_limit", phoneNumber)
}

func makeOTPSubLimitKey(phoneNumber string) string {
	return fmt.Sprintf("%s:otp:submission_limit", phoneNumber)
}

func makeOTPKey(phoneNumber string) string {
	return fmt.Sprintf("%s:otp", phoneNumber)
}

func generateOTP() string {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%d", 100000+rand.Intn(900000))
}

func generateOTPHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()
	body := struct {
		PhoneNumber string `json:"phone_number" validate:"required,e164"`
	}{}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusBadRequest).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	user, err := queries.GetUserByPhoneNumber(ctx, pgtype.Text{String: body.PhoneNumber, Valid: true})
	if err != nil && errors.Is(err, pgx.ErrNoRows) == false {
		errMsg := "failed to get user by phone number"
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if body.PhoneNumber == user.PhoneNumber.String {
		err = sendJSONErrorResponse(res, errorResponseParams{
			StatusCode: http.StatusConflict,
			Message:    "Nomor handphone telah digunakan"},
		)

		if err != nil {
			errMsg := "failed to send failed response body"
			logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	otpGenLimitKey := makeOTPGenLimitKey(body.PhoneNumber)
	otpSubmissionLimitKey := makeOTPSubLimitKey(body.PhoneNumber)
	otpKey := makeOTPKey(body.PhoneNumber)

	otp, err := redisClient.Get(ctx, otpKey).Result()
	if err != nil && err != redis.Nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to get otp")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if otp != "" {
		remainingTime, err := redisClient.TTL(ctx, otpKey).Result()
		if err != nil {
			errMsg := "failed to get remaining time of otp"
			logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		duration := int(remainingTime.Seconds())
		res.Header().Set("Retry-After", fmt.Sprintf("%d", duration))
		message := fmt.Sprintf("Kode OTP telah dikirim. Tunggu %d detik agar dapat mengirim ulang kode OTP", duration)

		err = sendJSONErrorResponse(res, errorResponseParams{StatusCode: http.StatusConflict, Message: message})
		if err != nil {
			errMsg := "failed to send failed response body"
			logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	err = redisClient.SetNX(ctx, otpGenLimitKey, 0, otpGenLimitDuration).Err()
	if err != nil {
		errMsg := "failed to set otp generation limit"
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	genCount, err := redisClient.Incr(ctx, otpGenLimitKey).Result()
	if err != nil {
		errMsg := "failed to increment otp generation limit"
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if genCount > int64(otpGenLimit) {
		remainingTime, err := redisClient.TTL(ctx, otpGenLimitKey).Result()
		if err != nil {
			errMsg := "failed to get remaining time of otp generation limit"
			logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		duration := int(remainingTime.Seconds())
		res.Header().Set("Retry-After", fmt.Sprintf("%d", duration))
		message := fmt.Sprintf(
			"Pengiriman kode OTP telah mencapai batas untuk hari ini. Tunggu %d detik agar dapat mengirim ulang kode OTP",
			duration,
		)

		err = sendJSONErrorResponse(res, errorResponseParams{StatusCode: http.StatusTooManyRequests, Message: message})
		if err != nil {
			errMsg := "failed to send failed response body"
			logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	otp = generateOTP()
	tx := redisClient.TxPipeline()
	tx.Set(ctx, otpKey, otp, otpDuration)
	tx.Set(ctx, otpSubmissionLimitKey, 0, otpDuration)
	_, err = tx.Exec(ctx)
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to create otp")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	params := twilioApi.CreateMessageParams{}
	params.SetFrom("whatsapp:+14155238886")
	params.SetTo(fmt.Sprintf("whatsapp:%s", body.PhoneNumber))
	params.SetBody(fmt.Sprintf("Berikut adalah kode OTP Anda: %s", otp))

	_, err = twilioClient.Api.CreateMessage(&params)
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to send otp")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusCreated)
	logWithCtx.Info().Dur("response_time", time.Since(start)).Msg("request completed")
}

func verifyOTPHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()
	body := struct {
		PhoneNumber string `json:"phone_number" validate:"required,e164"`
		UserOTP     string `json:"user_otp" validate:"required,len=6"`
	}{}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusBadRequest).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	otpGenLimitKey := makeOTPGenLimitKey(body.PhoneNumber)
	otpSubmissionLimitKey := makeOTPSubLimitKey(body.PhoneNumber)
	otpKey := makeOTPKey(body.PhoneNumber)

	otp, err := redisClient.Get(ctx, otpKey).Result()
	if err != nil {
		if err != redis.Nil {
			logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to get otp")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		message := fmt.Sprintln("Kamu belum memiliki kode OTP")
		err = sendJSONErrorResponse(res, errorResponseParams{StatusCode: http.StatusNotFound, Message: message})
		if err != nil {
			errMsg := "failed to send failed response body"
			logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	submissionCount, err := redisClient.Incr(ctx, otpSubmissionLimitKey).Result()
	if err != nil {
		errMsg := "failed to increment otp submission limit"
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if submissionCount > int64(otpSubmissionLimit) {
		remainingTime, err := redisClient.TTL(ctx, otpKey).Result()
		if err != nil {
			errMsg := "failed to get remaining time of otp"
			logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		duration := int(remainingTime.Seconds())
		res.Header().Set("Retry-After", fmt.Sprintf("%d", duration))
		message := fmt.Sprintf(
			"Verifikasi kode OTP telah mencapai batas. Tunggu %d detik agar dapat mengirim ulang kode OTP",
			duration,
		)

		err = sendJSONErrorResponse(res, errorResponseParams{StatusCode: http.StatusTooManyRequests, Message: message})
		if err != nil {
			errMsg := "failed to send failed response body"
			logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	if body.UserOTP != otp {
		message := fmt.Sprintf(
			"Kode OTP yang kamu masukkan salah. Kesempatan kamu tersisa %d",
			otpSubmissionLimit-int(submissionCount),
		)

		err = sendJSONErrorResponse(res, errorResponseParams{StatusCode: http.StatusUnauthorized, Message: message})
		if err != nil {
			errMsg := "failed to send failed response body"
			logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	tx := redisClient.TxPipeline()
	tx.Del(ctx, otpGenLimitKey)
	tx.Del(ctx, otpSubmissionLimitKey)
	tx.Del(ctx, otpKey)

	_, err = tx.Exec(ctx)
	if err != nil {
		errMsg := fmt.Sprintf("failed to delete %s, %s, and %s keys from redis", otpGenLimitKey, otpSubmissionLimitKey, otpKey)
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	userID := fmt.Sprintf("%s", req.Context().Value("userID"))
	err = queries.UpdateUserPhoneNumber(ctx, repository.UpdateUserPhoneNumberParams{
		ID:            userID,
		PhoneNumber:   pgtype.Text{String: body.PhoneNumber, Valid: true},
		PhoneVerified: true,
	})

	if err != nil {
		errMsg := "failed to update user phone number"
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg(errMsg)
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Dur("response_time", time.Since(start)).Msg("request completed")
}
