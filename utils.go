package main

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
)

type ErrorResponseParams struct {
	StatusCode int
	Message    string
}

func sendJSONErrorResponse(res http.ResponseWriter, params ErrorResponseParams) error {
	message := struct {
		Message string `json:"message"`
	}{
		Message: params.Message,
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(params.StatusCode)
	err := json.NewEncoder(res).Encode(&message)
	if err != nil {
		return errors.Wrap(err, "sendJSONErrorResponse(): failed to encode JSON")
	}
	return nil
}
