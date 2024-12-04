package workerserver

import (
	"github.com/hibiken/asynq"
	"github.com/mdayat/demi-masa-be/internal/config"
	"github.com/mdayat/demi-masa-be/internal/services"
	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/twilio/twilio-go"
)

var (
	queries      *repository.Queries
	twilioClient *twilio.RestClient
)

func New() (*asynq.Server, *asynq.ServeMux) {
	queries = services.GetQueries()
	twilioClient = services.GetTwilio()

	asynqServer := asynq.NewServer(
		asynq.RedisClientOpt{Addr: config.Env.REDIS_URL},
		asynq.Config{Concurrency: 10},
	)

	mux := asynq.NewServeMux()
	mux.Use(logger)
	mux.HandleFunc(task.TypeUserDowngrade, handleUserDowngrade)
	mux.HandleFunc(task.TypeUserPrayer, handleUserPrayer)

	return asynqServer, mux
}
