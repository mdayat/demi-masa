package httpserver

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
	"github.com/mdayat/demi-masa-be/repository"
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
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	ctx := context.Background()
	userID := fmt.Sprintf("%s", req.Context().Value("userID"))

	result, err := queries.GetTxByUserID(ctx, userID)
	if err != nil {
		logWithCtx.Error().Err(err).Str("user_id", userID).Msg("failed to get transactions by user id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Str("user_id", userID).Msg("successfully get transactions by user id")

	resultLen := len(result)
	transactions := make([]transaction, 0, resultLen)
	for i := 0; i < resultLen; i++ {
		transactionID, err := result[i].TransactionID.Value()
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to get transaction pgtype.UUID value")
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
		logWithCtx.Error().Err(err).Msg("failed to send json success response")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func applyCoupon(ctx context.Context, couponCode string) (bool, error) {
	_, err := queries.DecrementCouponQuota(ctx, couponCode)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func createTxHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	var shouldRollbackQuota bool
	var couponCode pgtype.Text

	defer func() {
		if shouldRollbackQuota {
			err := retry.Do(
				func() error {
					err := queries.IncrementCouponQuota(req.Context(), couponCode.String)
					if err != nil {
						return err
					}

					return nil
				},
				retry.Attempts(3),
				retry.Context(req.Context()),
			)

			if err != nil {
				logWithCtx.Error().Err(err).Str("coupon_code", couponCode.String).Msg("failed to roll back coupon quota")
				return
			}
			logWithCtx.Info().Msg("successfully rolled back coupon quota")
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
		logWithCtx.Error().Err(err).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if body.CouponCode != "" {
		valid, err := applyCoupon(req.Context(), body.CouponCode)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to decrement coupon quota")
			return
		}

		if valid {
			logWithCtx.Info().Msg("successfully decremented coupon quota")
			couponCode = pgtype.Text{String: body.CouponCode, Valid: true}
		} else {
			logWithCtx.Info().Msg("invalid coupon code or exhausted coupon quota")
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

		logWithCtx.Error().Err(err).Str("merchant_ref", merchantRefString).Msg("failed to create tripay transaction")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if resp.Success {
		logWithCtx.Info().Str("merchant_ref", merchantRefString).Msg("successfully created tripay transaction")
		var data tripayTxData
		err = json.Unmarshal(resp.Data, &data)
		if err != nil {
			if couponCode.Valid {
				shouldRollbackQuota = true
			}

			logWithCtx.Error().Err(err).Str("merchant_ref", merchantRefString).Msg("failed to unmarshal successful tripay transaction")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		userID := fmt.Sprintf("%s", req.Context().Value("userID"))
		expiredAt := time.Unix(oneHourExpiration, 0)

		subsPlanIDBytes, err := uuid.Parse(body.SubsPlanID)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to parse subscription plan uuid string to bytes")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Msg("successfully parsed subscription plan uuid string to bytes")

		err = queries.CreateTx(req.Context(), repository.CreateTxParams{
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

			logWithCtx.Error().Err(err).Str("merchant_ref", merchantRefString).Msg("failed to create transaction")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Str("transaction_id", merchantRefString).Msg("successfully created transaction")

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
			logWithCtx.Error().Err(err).Msg("failed to send json success response")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	} else {
		if couponCode.Valid {
			shouldRollbackQuota = true
		}

		logWithCtx.Error().Err(errors.New(resp.Message)).Msg("failed to create tripay transaction")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
