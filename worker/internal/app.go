package internal

import (
	"github.com/hibiken/asynq"
	"github.com/mdayat/demi-masa/pkg/task"
	"github.com/mdayat/demi-masa/worker/configs/env"
)

func InitApp() (*asynq.Server, *asynq.ServeMux) {
	asynqServer := asynq.NewServer(
		asynq.RedisClientOpt{Addr: env.REDIS_URL},
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
