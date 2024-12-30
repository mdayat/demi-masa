package env

import (
	"os"

	"github.com/joho/godotenv"
)

var (
	DATABASE_URL           string
	TWILIO_ACCOUNT_SID     string
	TWILIO_AUTH_TOKEN      string
	TWILIO_WHATSAPP_SENDER string
	REDIS_URL              string
)

func Init() error {
	err := godotenv.Load()
	if err != nil {
		return err
	}

	DATABASE_URL = os.Getenv("DATABASE_URL")
	TWILIO_ACCOUNT_SID = os.Getenv("TWILIO_ACCOUNT_SID")
	TWILIO_AUTH_TOKEN = os.Getenv("TWILIO_AUTH_TOKEN")
	TWILIO_WHATSAPP_SENDER = os.Getenv("TWILIO_WHATSAPP_SENDER")
	REDIS_URL = os.Getenv("REDIS_URL")

	return nil
}
