package workerservice

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

func logger(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		start := time.Now()
		subLogger := log.
			With().
			Str("task_id", task.ResultWriter().TaskID()).
			Str("task_type", task.Type()).
			Logger()

		subLogger.Info().Msg("task processed")

		err := next.ProcessTask(subLogger.WithContext(ctx), task)
		if err != nil {
			return err
		}
		subLogger.Info().Dur("res_time", time.Since(start)).Msg("task completed")
		return nil
	})
}
