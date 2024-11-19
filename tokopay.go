package main

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
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

var QRISPaymentMethod = "QRISREALTIME"

type OrderSuccess struct {
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

type OrderError struct {
	ErrorMsg string `json:"error_msg"`
	Status   int    `json:"status"`
}

func createOrderHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	var body struct {
		Amount               int    `json:"amount"`
		CouponCode           string `json:"coupon_code"`
		SubscriptionDuration int    `json:"subscription_duration"`
	}

	err := json.NewDecoder(req.Body).Decode(&body)
	if err != nil {
		logWithCtx.Error().Err(errors.Wrap(err, "createOrderHandler()")).Msg("invalid json body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	var couponCode pgtype.Text

	coupon, err := queries.GetCoupon(ctx, body.CouponCode)
	if err != nil && errors.Is(err, pgx.ErrNoRows) == false {
		logWithCtx.
			Error().
			Err(errors.Wrap(err, "createOrderHandler()")).
			Str("coupon_code", body.CouponCode).
			Msg("failed to get coupon by coupon code")

		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err == nil && body.CouponCode == coupon.Code {
		couponCode.String = body.CouponCode
		couponCode.Valid = true
		body.Amount = int(math.Round(float64(body.Amount) * 0.7))
	}

	MERCHANT_ID := os.Getenv("Merchant_ID")
	SECRET_KEY := os.Getenv("Secret_Key")

	refID := uuid.New()
	refIDString := refID.String()

	orderURL := fmt.Sprintf(
		"https://api.tokopay.id/v1/order?merchant=%s&secret=%s&ref_id=%s&nominal=%d&metode=%s",
		MERCHANT_ID,
		SECRET_KEY,
		refIDString,
		body.Amount,
		QRISPaymentMethod,
	)

	response, err := http.Get(orderURL)
	if err != nil {
		errMsg := "failed to make http get request to create order"
		logWithCtx.Error().Err(errors.Wrap(err, "createOrderHandler()")).Msg(errMsg)
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer response.Body.Close()

	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		logWithCtx.Error().Err(errors.Wrap(err, "createOrderHandler()")).Msg("failed to read order response")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var responseStatus struct {
		Status interface{} `json:"status"`
	}

	if err := json.Unmarshal(bytes, &responseStatus); err != nil {
		logWithCtx.Error().Err(errors.Wrap(err, "createOrderHandler()")).Msg("failed to unmarshal order response status")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	switch responseStatus.Status.(type) {
	case string:
		var orderSuccess OrderSuccess
		if err := json.Unmarshal(bytes, &orderSuccess); err != nil {
			logWithCtx.
				Error().
				Err(errors.Wrap(err, "createOrderHandler()")).
				Str("order_id", refIDString).
				Str("transaction_id", orderSuccess.Data.TrxID).
				Msg("failed to unmarshal successful order")

			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		logWithCtx.
			Info().
			Str("order_id", refIDString).
			Str("transaction_id", orderSuccess.Data.TrxID).
			Int("amount", body.Amount).
			Msg("successfully created order")

		tx, err := db.Begin(ctx)
		if err != nil {
			logWithCtx.Error().Err(errors.Wrap(err, "createOrderHandler()")).Msg("failed to start db transaction")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback(ctx)

		userID := fmt.Sprintf("%s", req.Context().Value("userID"))
		qtx := queries.WithTx(tx)

		err = qtx.CreateOrder(
			ctx,
			repository.CreateOrderParams{
				ID:                   pgtype.UUID{Bytes: refID, Valid: true},
				UserID:               userID,
				TransactionID:        orderSuccess.Data.TrxID,
				CouponCode:           couponCode,
				Amount:               int32(body.Amount),
				SubscriptionDuration: int32(body.SubscriptionDuration),
				PaymentMethod:        QRISPaymentMethod,
			},
		)

		if err != nil {
			logWithCtx.
				Error().
				Err(errors.Wrap(err, "createOrderHandler()")).
				Str("order_id", refIDString).
				Msg("failed to insert order to database")

			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Str("order_id", refIDString).Msg("successfully inserted order to database")

		if couponCode.Valid {
			err = qtx.DecrementCouponQuota(ctx, couponCode.String)
			if err != nil {
				logWithCtx.
					Error().
					Err(errors.Wrap(err, "createOrderHandler()")).
					Str("coupon_code", body.CouponCode).
					Msg("failed to decrement coupon quota")

				http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			logWithCtx.Info().Str("coupon_code", body.CouponCode).Msg("successfully decremented coupon quota")
		}

		err = tx.Commit(ctx)
		if err != nil {
			logWithCtx.Error().Err(errors.Wrap(err, "webhookHandler()")).Msg("failed to commit transaction")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(res).Encode(struct {
			QRLink string `json:"qr_link"`
		}{QRLink: orderSuccess.Data.QrLink})

		if err != nil {
			logWithCtx.Error().Err(errors.Wrap(err, "createOrderHandler()")).Msg("failed to encode successful order to json")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusCreated)

	case float64:
		var orderError OrderError
		if err := json.Unmarshal(bytes, &orderError); err != nil {
			logWithCtx.Error().Err(errors.Wrap(err, "createOrderHandler()")).Msg("failed to unmarshal failed order")
		} else {
			logWithCtx.Error().Err(errors.New("createOrderHandler(): " + orderError.ErrorMsg)).Msg("")
		}
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

	default:
		logWithCtx.
			Error().
			Err(errors.Wrap(err, "createOrderHandler()")).
			Msgf("unexpected order response status type: %T", responseStatus)

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

func webhookHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	var body TokopayWebhook

	err := json.NewDecoder(req.Body).Decode(&body)
	if err != nil {
		logWithCtx.Error().Err(errors.Wrap(err, "webhookHandler()")).Msg("invalid json body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	refIDBytes, err := uuid.Parse(body.ReffID)
	if err != nil {
		logWithCtx.Error().Err(errors.Wrap(err, "webhookHandler()")).Msg("failed to parse uuid string to uuid bytes")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	ctx := context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		logWithCtx.Error().Err(errors.Wrap(err, "webhookHandler()")).Msg("failed to start db transaction")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	qtx := queries.WithTx(tx)
	orderWithUser, err := qtx.GetOrderByIDWithUser(ctx, pgtype.UUID{Bytes: refIDBytes, Valid: true})
	if err != nil {
		logWithCtx.
			Error().
			Err(errors.Wrap(err, "webhookHandler()")).
			Str("order_id", body.ReffID).
			Msg("failed to get order with user by order id")

		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	err = qtx.UpdateOrderStatus(
		ctx,
		repository.UpdateOrderStatusParams{
			ID:            orderWithUser.OrderID,
			PaymentStatus: repository.PaymentStatusSuccess,
		},
	)

	if err != nil {
		logWithCtx.
			Error().
			Err(errors.Wrap(err, "webhookHandler()")).
			Str("order_id", body.ReffID).
			Msg("failed to update order status")

		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	upgradedAt := time.Now()
	expiredAt := time.Unix(upgradedAt.Unix()+int64(orderWithUser.SubscriptionDuration), 0)
	err = qtx.UpdateUserSubscription(
		ctx,
		repository.UpdateUserSubscriptionParams{
			ID:          orderWithUser.UserID,
			AccountType: repository.AccountTypePremium,
			UpgradedAt:  pgtype.Timestamptz{Time: upgradedAt, Valid: true},
			ExpiredAt:   pgtype.Timestamptz{Time: expiredAt, Valid: true},
		},
	)

	if err != nil {
		logWithCtx.
			Error().
			Err(errors.Wrap(err, "webhookHandler()")).
			Str("user_id", orderWithUser.UserID).
			Msg("failed to update user subscription")

		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	err = tx.Commit(ctx)
	if err != nil {
		logWithCtx.
			Error().
			Err(errors.Wrap(err, "webhookHandler()")).
			Str("order_id", body.ReffID).
			Str("user_id", orderWithUser.UserID).
			Msg("failed to commit transaction to update order status and user subscription")

		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	logWithCtx.
		Info().
		Str("order_id", body.ReffID).
		Str("user_id", orderWithUser.UserID).
		Msg("successfully updated order status and user subscription")

	err = json.NewEncoder(res).Encode(struct {
		Status bool `json:"status"`
	}{Status: true})

	if err != nil {
		logWithCtx.Error().Err(errors.Wrap(err, "webhookHandler()")).Msg("failed to encode webhook response")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "application/json")
}
