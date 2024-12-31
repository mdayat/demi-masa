package env

import (
	"os"

	"github.com/joho/godotenv"
)

var (
	REDIS_URL         string
	ASYNQMON_BASE_URL string
)

func Init() error {
	err := godotenv.Load()
	if err != nil {
		return err
	}

	REDIS_URL = os.Getenv("REDIS_URL")
	ASYNQMON_BASE_URL = os.Getenv("ASYNQMON_BASE_URL")

	return nil
}
