package internal

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
)

func decodeAndValidateJSONBody(req *http.Request, dst interface{}) error {
	err := json.NewDecoder(req.Body).Decode(&dst)
	if err != nil {
		return errors.Wrap(err, "failed to decode request body")
	}

	if err := validator.New(validator.WithRequiredStructEnabled()).Struct(dst); err != nil {
		return err
	}

	return nil
}
