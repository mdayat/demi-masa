package services

import "github.com/hibiken/asynq"

var (
	AsynqClient    *asynq.Client
	AsynqInspector *asynq.Inspector
)

func InitAsynq(redisURL string) {
	AsynqClient = asynq.NewClient(asynq.RedisClientOpt{Addr: redisURL})
	AsynqInspector = asynq.NewInspector(asynq.RedisClientOpt{Addr: redisURL})
}
