package services

import (
	"sync"

	"github.com/redis/go-redis/v9"
)

var (
	redisOnce   sync.Once
	redisClient *redis.Client
)

func InitRedis(REDIS_URL string) {
	redisOnce.Do(func() {
		redisClient = redis.NewClient(&redis.Options{Addr: REDIS_URL})
	})
}

func GetRedis() *redis.Client {
	return redisClient
}
