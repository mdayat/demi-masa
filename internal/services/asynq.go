package services

import (
	"sync"

	"github.com/hibiken/asynq"
)

var (
	asynqOnce      sync.Once
	asynqClient    *asynq.Client
	asynqInspector *asynq.Inspector
)

func InitAsynq(REDIS_URL string) {
	asynqOnce.Do(func() {
		asynqClient = asynq.NewClient(asynq.RedisClientOpt{Addr: REDIS_URL})
		asynqInspector = asynq.NewInspector(asynq.RedisClientOpt{Addr: REDIS_URL})
	})
}

func GetAsynqClient() *asynq.Client {
	return asynqClient
}

func GetAsynqInspector() *asynq.Inspector {
	return asynqInspector
}
