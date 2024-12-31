package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mdayat/demi-masa/web/configs/services"
	"github.com/mdayat/demi-masa/web/repository"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type transaction struct {
	ID               string                       `json:"id"`
	Status           repository.TransactionStatus `json:"status"`
	QrUrl            string                       `json:"qr_url"`
	PaidAt           string                       `json:"paid_at"`
	ExpiredAt        string                       `json:"expired_at"`
	Price            int                          `json:"price"`
	DurationInMonths int                          `json:"duration_in_months"`
}

func getTransactionsHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()

	userID := fmt.Sprintf("%s", ctx.Value("userID"))
	result, err := services.Queries.GetTxByUserID(ctx, userID)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to get transactions by user id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resultLen := len(result)
	transactions := make([]transaction, 0, resultLen)
	for i := 0; i < resultLen; i++ {
		transactionID, err := result[i].TransactionID.Value()
		if err != nil {
			logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to get transaction UUID from pgtype.UUID")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		transaction := transaction{
			ID:               fmt.Sprintf("%s", transactionID),
			Status:           result[i].Status,
			QrUrl:            result[i].QrUrl,
			Price:            int(result[i].Price),
			DurationInMonths: int(result[i].DurationInMonths),
			ExpiredAt:        result[i].ExpiredAt.Time.Format(time.RFC3339),
		}

		if result[i].CouponCode.Valid {
			transaction.Price = int(math.Round(float64(transaction.Price) * 0.7))
		}

		if result[i].PaidAt.Valid {
			transaction.PaidAt = result[i].PaidAt.Time.Format(time.RFC3339)
		}

		transactions = append(transactions, transaction)
	}

	err = sendJSONSuccessResponse(res, successResponseParams{StatusCode: http.StatusOK, Data: &transactions})
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to send successful response body")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Int("status_code", http.StatusOK).Dur("response_time", time.Since(start)).Msg("request completed")
}

func applyCoupon(ctx context.Context, couponCode string) (bool, error) {
	_, err := services.Queries.DecrementCouponQuota(ctx, couponCode)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func createTxHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()

	var shouldRollbackQuota bool
	var couponCode pgtype.Text

	defer func() {
		if shouldRollbackQuota {
			err := retry.Do(
				func() error {
					err := services.Queries.IncrementCouponQuota(ctx, couponCode.String)
					if err != nil {
						return err
					}

					return nil
				},
				retry.Attempts(3),
				retry.Context(ctx),
			)

			if err != nil {
				logWithCtx.
					Error().
					Err(err).
					Int("status_code", http.StatusInternalServerError).
					Str("coupon_code", couponCode.String).
					Msg("failed to roll back coupon quota")
			}
		}
	}()

	var body struct {
		SubsPlanID       string `json:"subs_plan_id" validate:"required,uuid4"`
		SubsPlanName     string `json:"subs_plan_name" validate:"required"`
		SubsPlanPrice    int    `json:"subs_plan_price" validate:"required"`
		SubsPlanDuration int    `json:"subs_plan_duration" validate:"required"`
		CouponCode       string `json:"coupon_code"`
		CustomerName     string `json:"customer_name" validate:"required"`
		CustomerEmail    string `json:"customer_email" validate:"required,email"`
		CustomerPhone    string `json:"customer_phone" validate:"required,e164"`
	}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusBadRequest).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if body.CouponCode != "" {
		valid, err := applyCoupon(ctx, body.CouponCode)
		if err != nil {
			logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to decrement coupon quota")
			return
		}

		if valid {
			couponCode = pgtype.Text{String: body.CouponCode, Valid: true}
		}
	}

	if couponCode.Valid {
		body.SubsPlanPrice = int(math.Round(float64(body.SubsPlanPrice) * 0.7))
	}

	merchantRef := uuid.New()
	merchantRefString := merchantRef.String()
	signature := createTripayTxSig(merchantRefString, body.SubsPlanPrice)
	oneHourExpiration := time.Now().Unix() + int64(time.Hour.Seconds())

	params := createTripayTxParams{
		Method:        QRIS_PAYMENT_METHOD,
		MerchantRef:   merchantRefString,
		Amount:        body.SubsPlanPrice,
		CustomerName:  body.CustomerName,
		CustomerEmail: body.CustomerEmail,
		CustomerPhone: body.CustomerPhone,
		Signature:     signature,
		ExpiredTime:   int(oneHourExpiration),
		OrderItems: []tripayOrderItem{
			{
				SubscriptionPlanID: body.SubsPlanID,
				Name:               body.SubsPlanName,
				Price:              body.SubsPlanPrice,
				Quantity:           1,
			},
		},
	}

	resp, err := createTripayTx(&params)
	if err != nil {
		if couponCode.Valid {
			shouldRollbackQuota = true
		}

		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to create tripay transaction")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if resp.Success {
		var data tripayTxData
		err = json.Unmarshal(resp.Data, &data)
		if err != nil {
			if couponCode.Valid {
				shouldRollbackQuota = true
			}

			logWithCtx.
				Error().
				Err(err).
				Caller().
				Int("status_code", http.StatusInternalServerError).
				Str("merchant_ref", merchantRefString).
				Msg("failed to unmarshal successful tripay transaction")

			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		userID := fmt.Sprintf("%s", ctx.Value("userID"))
		expiredAt := time.Unix(oneHourExpiration, 0)

		subsPlanIDBytes, err := uuid.Parse(body.SubsPlanID)
		if err != nil {
			errMsg := "failed to parse subscription plan uuid string to bytes"
			logWithCtx.
				Error().
				Err(err).
				Caller().
				Int("status_code", http.StatusInternalServerError).
				Str("merchant_ref", merchantRefString).
				Msg(errMsg)

			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		err = services.Queries.CreateTx(ctx, repository.CreateTxParams{
			ID:                 pgtype.UUID{Bytes: merchantRef, Valid: true},
			UserID:             userID,
			SubscriptionPlanID: pgtype.UUID{Bytes: subsPlanIDBytes, Valid: true},
			RefID:              data.Reference,
			CouponCode:         couponCode,
			PaymentMethod:      QRIS_PAYMENT_METHOD,
			QrUrl:              fmt.Sprintf("%s", data.QrURL),
			ExpiredAt:          pgtype.Timestamptz{Time: expiredAt, Valid: true},
		})

		if err != nil {
			if couponCode.Valid {
				shouldRollbackQuota = true
			}

			logWithCtx.
				Error().
				Err(err).
				Caller().
				Int("status_code", http.StatusInternalServerError).
				Str("merchant_ref", merchantRefString).
				Msg("failed to create transaction")

			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		respBody := transaction{
			ID:               fmt.Sprintf("%s", merchantRefString),
			Status:           repository.TransactionStatusUNPAID,
			QrUrl:            data.QrURL,
			Price:            body.SubsPlanPrice,
			DurationInMonths: body.SubsPlanDuration,
			ExpiredAt:        expiredAt.Format(time.RFC3339),
		}

		err = sendJSONSuccessResponse(res, successResponseParams{StatusCode: http.StatusCreated, Data: &respBody})
		if err != nil {
			logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to send successful response body")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Int("status_code", http.StatusCreated).Dur("response_time", time.Since(start)).Msg("request completed")
	} else {
		if couponCode.Valid {
			shouldRollbackQuota = true
		}

		err := errors.New(resp.Message)
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to create tripay transaction")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
