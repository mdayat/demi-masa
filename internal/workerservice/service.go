package workerservice

import (
	"github.com/hibiken/asynq"
	"github.com/mdayat/demi-masa-be/internal/config"
	"github.com/mdayat/demi-masa-be/internal/services"
	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/redis/go-redis/v9"
	"github.com/twilio/twilio-go"
)

var (
	queries      *repository.Queries
	twilioClient *twilio.RestClient
	redisClient  *redis.Client
)

func New() (*asynq.Server, *asynq.ServeMux) {
	queries = services.GetQueries()
	twilioClient = services.GetTwilio()
	redisClient = services.GetRedis()

	asynqServer := asynq.NewServer(
		asynq.RedisClientOpt{Addr: config.Env.REDIS_URL},
		asynq.Config{Concurrency: 10},
	)

	mux := asynq.NewServeMux()
	mux.Use(logger)
	mux.HandleFunc(task.TypeUserDowngrade, handleUserDowngrade)
	mux.HandleFunc(task.TypePrayerReminder, handlePrayerReminder)
	mux.HandleFunc(task.TypeLastPrayerReminder, handleLastPrayerReminder)
	mux.HandleFunc(task.TypePrayerRenewal, handlePrayerRenewal)
	mux.HandleFunc(task.TypeTaskRemoval, handleTaskRemoval)
	mux.HandleFunc(task.TypePrayerUpdate, handlePrayerUpdate)

	return asynqServer, mux
}