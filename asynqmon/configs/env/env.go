package env

import (
	"os"

	"github.com/joho/godotenv"
)

var (
	REDIS_URL               string
	ASYNQMON_BASE_URL       string
	AUTHORIZED_EMAILS       string
	ACCESS_TOKEN_SECRET_KEY string
)

func Init() error {
	err := godotenv.Load()
	if err != nil {
		return err
	}

	REDIS_URL = os.Getenv("REDIS_URL")
	ASYNQMON_BASE_URL = os.Getenv("ASYNQMON_BASE_URL")
	AUTHORIZED_EMAILS = os.Getenv("AUTHORIZED_EMAILS")
	ACCESS_TOKEN_SECRET_KEY = os.Getenv("ACCESS_TOKEN_SECRET_KEY")

	return nil
}
