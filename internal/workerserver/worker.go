package workerserver

import (
	"github.com/hibiken/asynq"
	"github.com/mdayat/demi-masa-be/internal/config"
	"github.com/mdayat/demi-masa-be/internal/services"
	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
)

var (
	queries *repository.Queries
)

func New() (*asynq.Server, *asynq.ServeMux) {
	queries = services.GetQueries()

	asynqServer := asynq.NewServer(
		asynq.RedisClientOpt{Addr: config.Env.REDIS_URL},
		asynq.Config{Concurrency: 10},
	)

	mux := asynq.NewServeMux()
	mux.Use(logger)
	mux.HandleFunc(task.TypeUserDowngrade, handleUserDowngrade)

	return asynqServer, mux
}
