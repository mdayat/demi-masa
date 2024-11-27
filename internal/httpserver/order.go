package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mdayat/demi-masa-be/repository"
	"github.com/rs/zerolog/log"
)

type Order struct {
	ID                   string                   `json:"id,omitempty"`
	UserID               string                   `json:"user_id,omitempty"`
	TransactionID        string                   `json:"transaction_id,omitempty"`
	CouponCode           string                   `json:"coupon_code,omitempty"`
	Amount               int32                    `json:"amount,omitempty"`
	SubscriptionDuration int32                    `json:"subscription_duration,omitempty"`
	PaymentMethod        string                   `json:"payment_method,omitempty"`
	PaymentUrl           string                   `json:"payment_url,omitempty"`
	PaymentStatus        repository.PaymentStatus `json:"payment_status,omitempty"`
	CreatedAt            string                   `json:"created_at,omitempty"`
	PaidAt               string                   `json:"paid_at,omitempty"`
	ExpiredAt            string                   `json:"expired_at,omitempty"`
}

func getOrdersHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	ctx := context.Background()

	result, err := queries.GetOrders(ctx)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get orders")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully get orders")

	resultLen := len(result)
	orders := make([]Order, 0, resultLen)
	for i := 0; i < resultLen; i++ {
		orderID, err := result[i].ID.Value()
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to get pgtype.UUID value")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		order := Order{
			ID:                   fmt.Sprintf("%s", orderID),
			UserID:               result[i].UserID,
			TransactionID:        result[i].TransactionID,
			CouponCode:           result[i].CouponCode.String,
			Amount:               result[i].Amount,
			SubscriptionDuration: result[i].SubscriptionDuration,
			PaymentMethod:        result[i].PaymentMethod,
			PaymentStatus:        result[i].PaymentStatus,
			CreatedAt:            result[i].CreatedAt.Time.Format(time.RFC3339),
		}

		if result[i].PaidAt.Valid {
			order.PaidAt = result[i].PaidAt.Time.Format(time.RFC3339)
		}

		if result[i].ExpiredAt.Valid {
			order.ExpiredAt = result[i].ExpiredAt.Time.Format(time.RFC3339)
		}

		orders = append(orders, order)
	}

	respBody := struct {
		Orders []Order `json:"orders"`
	}{
		Orders: orders,
	}

	err = sendJSONSuccessResponse(res, successResponseParams{StatusCode: http.StatusOK, Data: &respBody})
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to send json success response")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
