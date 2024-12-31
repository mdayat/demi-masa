package internal

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mdayat/demi-masa/web/configs/services"
	"github.com/rs/zerolog/log"
)

type subscriptionPlan struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Price            int    `json:"price"`
	DurationInMonths int    `json:"duration_in_months"`
	CreatedAt        string `json:"created_at"`
}

func getSubsPlansHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()

	result, err := services.Queries.GetSubsPlans(ctx)
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to get subscription plans")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resultLen := len(result)
	subsPlans := make([]subscriptionPlan, 0, resultLen)
	for i := 0; i < resultLen; i++ {
		subsPlanID, err := result[i].ID.Value()
		if err != nil {
			logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to get subscription plan UUID from pgtype.UUID")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		subsPlan := subscriptionPlan{
			ID:               fmt.Sprintf("%s", subsPlanID),
			Name:             result[i].Name,
			Price:            int(result[i].Price),
			DurationInMonths: int(result[i].DurationInMonths),
			CreatedAt:        result[i].CreatedAt.Time.Format(time.RFC3339),
		}

		subsPlans = append(subsPlans, subsPlan)
	}

	err = sendJSONSuccessResponse(res, successResponseParams{StatusCode: http.StatusOK, Data: &subsPlans})
	if err != nil {
		logWithCtx.Error().Err(err).Caller().Int("status_code", http.StatusInternalServerError).Msg("failed to send successful response body")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Int("status_code", http.StatusOK).Dur("response_time", time.Since(start)).Msg("request completed")
}
