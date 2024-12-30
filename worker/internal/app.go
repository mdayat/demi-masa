package internal

import (
	"github.com/hibiken/asynq"
	"github.com/mdayat/demi-masa/worker/configs/env"
)

func InitApp() (*asynq.Server, *asynq.ServeMux) {
	asynqServer := asynq.NewServer(
		asynq.RedisClientOpt{Addr: env.REDIS_URL},
		asynq.Config{Concurrency: 10},
	)

	mux := asynq.NewServeMux()

	return asynqServer, mux
}
