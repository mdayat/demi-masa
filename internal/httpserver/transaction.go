package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

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
			var err error
			ctx := context.Background()
			maxRetries := 3
			retryDelay := time.Second * 2

			for i := 1; i <= maxRetries; i++ {
				err = queries.IncrementCouponQuota(ctx, couponCode.String)
				if err == nil {
					logWithCtx.Info().Str("coupon_code", couponCode.String).Msg("successfully rollback coupon quota")
					return
				}

				logWithCtx.
					Info().
					Str("coupon_code", couponCode.String).
					Int("attempt", i).
					Msg("failed to increment coupon quota")
				time.Sleep(retryDelay)
			}

			logWithCtx.Error().Err(err).Str("coupon_code", couponCode.String).Msg("failed to rollback coupon quota")
		}
	}()

	var body struct {
		SubscriptionPlanID string `json:"subscription_plan_id" validate:"required,uuid4"`
		CouponCode         string `json:"coupon_code"`
		CustomerName       string `json:"customer_name" validate:"required"`
		CustomerEmail      string `json:"customer_email" validate:"required,email"`
		CustomerPhone      string `json:"customer_phone" validate:"required,e164"`
	}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("invalid json body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	subsPlanChan := make(chan repository.SubscriptionPlan, 1)
	couponCodeChan := make(chan pgtype.Text, 1)
	errChan := make(chan error, 2)

	var wg sync.WaitGroup
	ctx := context.Background()

	wg.Add(1)
	go func() {
		defer wg.Done()

		subsPlanIDBytes, err := uuid.Parse(body.SubscriptionPlanID)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to parse subscription plan uuid string to bytes")
			errChan <- err
			return
		}

		result, err := queries.GetSubsPlanByID(ctx, pgtype.UUID{Bytes: subsPlanIDBytes, Valid: true})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				logWithCtx.Error().Err(err).Msg("subscription plan not found")
				errChan <- err
			} else {
				logWithCtx.
					Error().
					Err(err).
					Str("subscription_plan_id", body.SubscriptionPlanID).
					Msg("failed to get subscription plan by id")

				errChan <- err
			}
			return
		}

		logWithCtx.
			Info().
			Str("subscription_plan_id", body.SubscriptionPlanID).
			Msg("successfully get subscription plan by id")

		subsPlanChan <- result
	}()

	if body.CouponCode != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()

			valid, err := applyCoupon(ctx, body.CouponCode)
			if err != nil {
				logWithCtx.Error().Err(err).Msg("failed to decrement coupon quota")
				errChan <- err
				return
			}

			if valid {
				logWithCtx.Info().Str("coupon_code", body.CouponCode).Msg("successfully decremented coupon quota")
				couponCodeChan <- pgtype.Text{String: body.CouponCode, Valid: true}
			} else {
				logWithCtx.Info().Str("coupon_code", body.CouponCode).Msg("invalid coupon code or exhausted coupon quota")
			}
		}()
	} else {
		close(couponCodeChan)
	}

	go func() {
		wg.Wait()
		close(subsPlanChan)
		close(couponCodeChan)
		close(errChan)
	}()

	var subsPlan repository.SubscriptionPlan
	for i := 0; i < 2; i++ {
		select {
		case err = <-errChan:
			if err != nil {
				if couponCode.Valid {
					shouldRollbackQuota = true
				}

				logWithCtx.Error().Err(err).Msg("")
				http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

		case subsPlan = <-subsPlanChan:
		case couponCode = <-couponCodeChan:
		}
	}

	if couponCode.Valid {
		subsPlan.Price = int32(math.Round(float64(subsPlan.Price) * 0.7))
	}

	merchantRef := uuid.New()
	merchantRefString := merchantRef.String()
	signature := createTripayTxSig(merchantRefString, int(subsPlan.Price))
	oneHourExpiration := time.Now().Unix() + int64(time.Hour.Seconds())

	params := createTripayTxParams{
		Method:        QRIS_PAYMENT_METHOD,
		MerchantRef:   merchantRefString,
		Amount:        int(subsPlan.Price),
		CustomerName:  body.CustomerName,
		CustomerEmail: body.CustomerEmail,
		CustomerPhone: body.CustomerPhone,
		Signature:     signature,
		ExpiredTime:   int(oneHourExpiration),
		OrderItems: []tripayOrderItem{
			{
				SubscriptionPlanID: body.SubscriptionPlanID,
				Name:               subsPlan.Name,
				Price:              int(subsPlan.Price),
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

		err = queries.CreateTx(ctx, repository.CreateTxParams{
			ID:                 pgtype.UUID{Bytes: merchantRef, Valid: true},
			UserID:             userID,
			SubscriptionPlanID: subsPlan.ID,
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
			Price:            int(subsPlan.Price),
			DurationInMonths: int(subsPlan.DurationInMonths),
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
