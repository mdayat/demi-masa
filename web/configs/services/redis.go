package services

import (
	"github.com/redis/go-redis/v9"
)

var (
	RedisClient *redis.Client
)

func InitRedis(REDIS_URL string) *redis.Client {
	RedisClient = redis.NewClient(&redis.Options{Addr: REDIS_URL})
	return RedisClient
}
