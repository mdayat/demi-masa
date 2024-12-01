package workerserver

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/rs/zerolog/log"
)

func handleUserDowngrade(ctx context.Context, asynqTask *asynq.Task) error {
	logWithCtx := log.Ctx(ctx).With().Logger()
	var payload task.UserDowngradePayload
	if err := json.Unmarshal(asynqTask.Payload(), &payload); err != nil {
		logWithCtx.Error().Err(err).Msg("failed to unmarshal user downgrade task payload")
		return err
	}

	err := queries.UpdateUserSubs(ctx, repository.UpdateUserSubsParams{
		ID:          payload.UserID,
		AccountType: repository.AccountTypeFREE,
	})

	if err != nil {
		logWithCtx.Error().Err(err).Str("user_id", payload.UserID).Msg("failed to update user subscription to FREE")
		return err
	}
	logWithCtx.Info().Str("user_id", payload.UserID).Msg("successfully updated user subscription to FREE")

	return nil
}
