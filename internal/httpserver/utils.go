package httpserver

import (
	"encoding/json"
	"net/http"
)

type successResponseParams struct {
	StatusCode int
	Data       interface{}
}

func sendJSONSuccessResponse(res http.ResponseWriter, params successResponseParams) error {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(params.StatusCode)
	err := json.NewEncoder(res).Encode(&params.Data)
	if err != nil {
		return err
	}
	return nil
}

type errorResponseParams struct {
	StatusCode int
	Message    string
}

func sendJSONErrorResponse(res http.ResponseWriter, params errorResponseParams) error {
	message := struct {
		Message string `json:"message"`
	}{
		Message: params.Message,
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(params.StatusCode)
	err := json.NewEncoder(res).Encode(&message)
	if err != nil {
		return err
	}
	return nil
}
