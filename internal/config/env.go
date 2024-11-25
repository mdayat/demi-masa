package config

import (
	"os"
	"sync"

	"github.com/joho/godotenv"
)

var (
	envOnce sync.Once
	err     error
	Env     struct {
		DATABASE_URL       string
		TWILIO_ACCOUNT_SID string
		TWILIO_AUTH_TOKEN  string
		REDIS_URL          string
		MERCHANT_ID        string
		SECRET_KEY         string
	}
)

func LoadEnv() error {
	envOnce.Do(func() {
		err = godotenv.Load()
		if err != nil {
			return
		}

		Env.DATABASE_URL = os.Getenv("DATABASE_URL")
		Env.TWILIO_ACCOUNT_SID = os.Getenv("TWILIO_ACCOUNT_SID")
		Env.TWILIO_AUTH_TOKEN = os.Getenv("TWILIO_AUTH_TOKEN")
		Env.REDIS_URL = os.Getenv("REDIS_URL")
		Env.MERCHANT_ID = os.Getenv("MERCHANT_ID")
		Env.SECRET_KEY = os.Getenv("SECRET_KEY")
	})

	return err
}
