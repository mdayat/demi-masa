package internal

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

func logger(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		subLogger := log.
			With().
			Str("task_id", task.ResultWriter().TaskID()).
			Str("task_type", task.Type()).
			Logger()

		err := next.ProcessTask(subLogger.WithContext(ctx), task)
		if err != nil {
			return err
		}

		return nil
	})
}
