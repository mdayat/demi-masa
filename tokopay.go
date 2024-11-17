package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

var QRISPaymentMethod = "QRIS"

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
	var body struct {
		Amount int `json:"amount"`
	}

	err := json.NewDecoder(req.Body).Decode(&body)
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "createOrderHandler()")).Msg("invalid json body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	MERCHANT_ID := os.Getenv("Merchant_ID")
	SECRET_KEY := os.Getenv("Secret_Key")
	refID := uuid.New().String()

	orderURL := fmt.Sprintf(
		"https://api.tokopay.id/v1/order?merchant=%s&secret=%s&ref_id=%s&nominal=%d&metode=%s",
		MERCHANT_ID,
		SECRET_KEY,
		refID,
		body.Amount,
		QRISPaymentMethod,
	)

	response, err := http.Get(orderURL)
	if err != nil {
		errMsg := "failed to make http get request to create order"
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "createOrderHandler()")).Msg(errMsg)
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer response.Body.Close()

	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "createOrderHandler()")).Msg("failed to read order response")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fmt.Println(string(bytes))

	var responseStatus struct {
		Status interface{} `json:"status"`
	}

	if err := json.Unmarshal(bytes, &responseStatus); err != nil {
		log.
			Ctx(req.Context()).
			Error().
			Err(errors.Wrap(err, "createOrderHandler()")).
			Msg("failed to unmarshal order response status")

		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	switch responseStatus.Status.(type) {
	case string:
		var orderSuccess OrderSuccess
		if err := json.Unmarshal(bytes, &orderSuccess); err != nil {
			log.
				Ctx(req.Context()).
				Error().
				Err(errors.Wrap(err, "createOrderHandler()")).
				Msg("failed to unmarshal successful order")

			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		log.
			Ctx(req.Context()).
			Info().
			Str("order_id", refID).
			Str("transaction_id", orderSuccess.Data.TrxID).
			Int("amount", body.Amount).
			Msg("successfully created order")

		res.WriteHeader(http.StatusCreated)

	case float64:
		var orderError OrderError
		if err := json.Unmarshal(bytes, &orderError); err != nil {
			log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "createOrderHandler()")).Msg("failed to unmarshal failed order")
		} else {
			log.Ctx(req.Context()).Error().Err(errors.New("createOrderHandler(): " + orderError.ErrorMsg)).Msg("")
		}
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

	default:
		log.
			Ctx(req.Context()).
			Error().
			Err(errors.Wrap(err, "createOrderHandler()")).
			Msgf("unexpected order response status type: %T", responseStatus)

		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
