package services

import (
	"github.com/redis/go-redis/v9"
)

var (
	RedisClient *redis.Client
)

func InitRedis(REDIS_URL string) {
	RedisClient = redis.NewClient(&redis.Options{Addr: REDIS_URL})
}
