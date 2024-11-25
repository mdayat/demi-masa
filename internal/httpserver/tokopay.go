package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

const qrisPaymentMethod = "QRISREALTIME"

type orderSuccess struct {
	Data struct {
		Other             string `json:"other"`
		PanduanPembayaran string `json:"panduan_pembayaran"`
		PayURL            string `json:"pay_url"`
		QrLink            string `json:"qr_link"`
		QrString          string `json:"qr_string"`
		TotalBayar        int    `json:"total_bayar"`
		TotalDiterima     int    `json:"total_diterima"`
		TrxID             string `json:"trx_id"`
	} `json:"data"`
	Status string `json:"status"`
}

type orderError struct {
	ErrorMsg string `json:"error_msg"`
	Status   int    `json:"status"`
}

type orderResponseStatus struct {
	Status interface{} `json:"status"`
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

func createTokopayOrder(orderURL string) (orderResponseStatus, *[]byte, error) {
	response, err := http.Get(orderURL)
	if err != nil {
		return orderResponseStatus{}, nil, errors.Wrap(err, "failed to make http get request to create order")
	}
	defer response.Body.Close()

	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		return orderResponseStatus{}, nil, errors.Wrap(err, "failed to read tokopay order response")
	}

	var responseStatus struct {
		Status interface{} `json:"status"`
	}

	if err := json.Unmarshal(bytes, &responseStatus); err != nil {
		return orderResponseStatus{}, nil, errors.Wrap(err, "failed to unmarshal tokopay order response status")
	}

	return responseStatus, &bytes, nil
}

func createOrderAndEnqueueTask(ctx context.Context, order repository.CreateOrderParams) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start db transaction to create order and enqueue task")
	}

	qtx := queries.WithTx(tx)
	err = qtx.CreateOrder(ctx, order)

	if err != nil {
		return errors.Wrap(err, "failed to create order")
	}

	orderID := fmt.Sprintf("%s", ctx.Value("order_id"))
	asynqTask, err := task.NewCleanupOrderTask(task.OrderTaskPayload{OrderID: orderID})
	if err != nil {
		return errors.Wrap(err, "failed to create task")
	}

	_, err = asynqClient.Enqueue(asynqTask)
	if err != nil {
		return errors.Wrap(err, "failed to enqueue task")
	}

	err = tx.Commit(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to commit db transaction to create order and enqueue task")
	}

	return nil
}

func createOrderHandler(res http.ResponseWriter, req *http.Request) {
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
					logWithCtx.Info().Str("coupon_code", couponCode.String).Msg("successfully rolled back coupon quota")
					return
				}

				logWithCtx.
					Info().
					Str("coupon_code", couponCode.String).
					Int("attempt", i).
					Msg("failed to increment coupon quota")
				time.Sleep(retryDelay)
			}

			logWithCtx.Error().Err(err).Str("coupon_code", couponCode.String).Msg("failed to roll back coupon quota")
		}
	}()

	var body struct {
		Amount               int    `json:"amount"`
		CouponCode           string `json:"coupon_code"`
		SubscriptionDuration int    `json:"subscription_duration"`
	}

	err := json.NewDecoder(req.Body).Decode(&body)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("invalid json body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	if body.CouponCode != "" {
		valid, err := applyCoupon(ctx, body.CouponCode)
		if err != nil {
			logWithCtx.Error().Err(err).Str("coupon_code", body.CouponCode).Msg("failed to decrement coupon quota")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if valid {
			couponCode.String = body.CouponCode
			couponCode.Valid = true
			body.Amount = int(math.Round(float64(body.Amount) * 0.7))
			logWithCtx.Info().Str("coupon_code", body.CouponCode).Msg("successfully decremented coupon quota")
		} else {
			logWithCtx.Info().Str("coupon_code", body.CouponCode).Msg("invalid coupon code or exhausted coupon quota")
		}
	}

	MERCHANT_ID := os.Getenv("MERCHANT_ID")
	SECRET_KEY := os.Getenv("SECRET_KEY")

	refID := uuid.New()
	refIDString := refID.String()

	orderURL := fmt.Sprintf(
		"https://api.tokopay.id/v1/order?merchant=%s&secret=%s&ref_id=%s&nominal=%d&metode=%s",
		MERCHANT_ID,
		SECRET_KEY,
		refIDString,
		body.Amount,
		qrisPaymentMethod,
	)

	responseStatus, bytes, err := createTokopayOrder(orderURL)
	if err != nil {
		if couponCode.Valid {
			shouldRollbackQuota = true
		}

		logWithCtx.Error().Err(err).Str("order_id", refIDString).Msg("failed to create tokopay order")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	switch responseStatus.Status.(type) {
	case string:
		var orderSuccess orderSuccess
		if err := json.Unmarshal(*bytes, &orderSuccess); err != nil {
			if couponCode.Valid {
				shouldRollbackQuota = true
			}

			logWithCtx.Error().Err(err).Str("order_id", refIDString).Msg("failed to unmarshal tokopay successful order")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Str("order_id", refIDString).Msg("successfully created tokopay order")

		userID := fmt.Sprintf("%s", req.Context().Value("userID"))
		oneDay := time.Unix(time.Now().Unix()+int64(time.Hour.Seconds()*24), 0)

		order := repository.CreateOrderParams{
			ID:                   pgtype.UUID{Bytes: refID, Valid: true},
			UserID:               userID,
			TransactionID:        orderSuccess.Data.TrxID,
			CouponCode:           couponCode,
			Amount:               int32(body.Amount),
			SubscriptionDuration: int32(body.SubscriptionDuration),
			PaymentMethod:        qrisPaymentMethod,
			PaymentUrl:           orderSuccess.Data.QrLink,
			ExpiredAt:            pgtype.Timestamptz{Time: oneDay, Valid: true},
		}

		err = createOrderAndEnqueueTask(context.WithValue(ctx, "order_id", refIDString), order)
		if err != nil {
			if couponCode.Valid {
				shouldRollbackQuota = true
			}

			logWithCtx.Error().Err(err).Str("order_id", refIDString).Msg("failed to create order and enqueue task")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Str("order_id", refIDString).Msg("successfully created order and enqueue task")

		respBody := struct {
			QRLink string `json:"qr_link"`
		}{
			QRLink: orderSuccess.Data.QrLink,
		}

		err = sendJSONSuccessResponse(res, successResponseParams{StatusCode: http.StatusCreated, Data: respBody})
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to send json success response")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

	case float64:
		var orderError orderError
		if err := json.Unmarshal(*bytes, &orderError); err != nil {
			logWithCtx.Error().Err(err).Str("order_id", refIDString).Msg("failed to unmarshal tokopay failed order")
		} else {
			logWithCtx.Error().Err(errors.New(orderError.ErrorMsg)).Str("order_id", refIDString).Msg("")
		}

		if couponCode.Valid {
			shouldRollbackQuota = true
		}
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

	default:
		logWithCtx.
			Error().
			Err(err).
			Str("order_id", refIDString).
			Str("order_payload", string(*bytes)).
			Msgf("unknown tokopay order response status type: %T", responseStatus.Status)

		if couponCode.Valid {
			shouldRollbackQuota = true
		}
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

type TokopayWebhook struct {
	Data struct {
		CreatedAt      string `json:"created_at"`
		CustomerEmail  string `json:"customer_email"`
		CustomerName   string `json:"customer_name"`
		CustomerPhone  string `json:"customer_phone"`
		MerchantID     string `json:"merchant_id"`
		PaymentChannel string `json:"payment_channel"`
		TotalDibayar   int    `json:"total_dibayar"`
		TotalDiterima  int    `json:"total_diterima"`
		UpdatedAt      string `json:"updated_at"`
	} `json:"data"`
	Reference string `json:"reference"`
	ReffID    string `json:"reff_id"`
	Signature string `json:"signature"`
	Status    string `json:"status"`
}

// This function do three things:
// (1) update order status,
// (2) update user subscription, and
// (3) delete a task from queue based on order id
func updateOrderStatusAndUserSubs(
	ctx context.Context,
	updatedOrder repository.UpdateOrderStatusParams,
	updatedUser repository.UpdateUserSubscriptionParams,
) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		msg := "failed to start db transaction to update order status, user subscription, and task deletion"
		return errors.Wrap(err, msg)
	}
	defer tx.Rollback(ctx)

	qtx := queries.WithTx(tx)
	err = qtx.UpdateOrderStatus(ctx, updatedOrder)
	if err != nil {
		return errors.Wrap(err, "failed to update order status")
	}

	err = qtx.UpdateUserSubscription(ctx, updatedUser)
	if err != nil {
		return errors.Wrap(err, "failed to update user subscription")
	}

	orderID := fmt.Sprintf("%s", ctx.Value("order_id"))
	err = asynqInspector.DeleteTask("default", orderID)
	if err != nil {
		return errors.Wrap(err, "failed to delete task")
	}

	err = tx.Commit(ctx)
	if err != nil {
		msg := "failed to commit db transaction to update order status, user subscription, and task deletion"
		return errors.Wrap(err, msg)
	}

	return nil
}

func webhookHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	bytes, err := io.ReadAll(req.Body)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to read tokopay webhook request")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var requestStatus struct {
		Status interface{} `json:"status"`
	}

	if err := json.Unmarshal(bytes, &requestStatus); err != nil {
		logWithCtx.Error().Err(err).Msg("failed to unmarshal tokopay webhook request status")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	switch requestStatus.Status.(type) {
	case string:
		if requestStatus.Status != "Success" && requestStatus.Status != "Completed" {
			logWithCtx.
				Error().
				Err(err).
				Str("webhook_payload", string(bytes)).
				Msgf("unknown tokopay webhook request status value: %s", requestStatus.Status)

			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		var body TokopayWebhook
		if err := json.Unmarshal(bytes, &body); err != nil {
			logWithCtx.Error().Err(err).Msg("failed to unmarshal tokopay webhook request")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		ctx := context.Background()
		refIDBytes, err := uuid.Parse(body.ReffID)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to parse uuid string to uuid bytes")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		orderWithUser, err := queries.GetOrderByIDWithUser(ctx, pgtype.UUID{Bytes: refIDBytes, Valid: true})
		if err != nil {
			logWithCtx.Error().Err(err).Str("order_id", body.ReffID).Msg("failed to get order with user by order id")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		updatedOrder := repository.UpdateOrderStatusParams{
			ID:            pgtype.UUID{Bytes: refIDBytes, Valid: true},
			PaymentStatus: repository.PaymentStatusPaid,
			PaidAt:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}

		upgradedAt := time.Now()
		expiredAt := time.Unix(upgradedAt.Unix()+int64(orderWithUser.SubscriptionDuration), 0)
		updatedUser := repository.UpdateUserSubscriptionParams{
			ID:          orderWithUser.UserID,
			AccountType: repository.AccountTypePremium,
			UpgradedAt:  pgtype.Timestamptz{Time: upgradedAt, Valid: true},
			ExpiredAt:   pgtype.Timestamptz{Time: expiredAt, Valid: true},
		}

		err = updateOrderStatusAndUserSubs(context.WithValue(ctx, "order_id", body.ReffID), updatedOrder, updatedUser)
		if err != nil {
			logWithCtx.
				Error().
				Err(err).
				Str("order_id", body.ReffID).
				Str("user_id", orderWithUser.UserID).
				Msg("failed to update order status, user subscription, and task deletion")

			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		logWithCtx.
			Info().
			Str("order_id", body.ReffID).
			Str("user_id", orderWithUser.UserID).
			Msg("successfully updated order status, user subscription, and task deletion")

		respBody := struct {
			Status bool `json:"status"`
		}{
			Status: true,
		}

		err = sendJSONSuccessResponse(res, successResponseParams{StatusCode: http.StatusOK, Data: respBody})
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to send json success response")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

	default:
		logWithCtx.
			Error().
			Err(err).
			Str("webhook_payload", string(bytes)).
			Msgf("unknown tokopay webhook request status type: %T", requestStatus.Status)

		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
