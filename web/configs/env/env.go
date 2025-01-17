package env

import (
	"os"

	"github.com/joho/godotenv"
)

var (
	DATABASE_URL         string
	TWILIO_ACCOUNT_SID   string
	TWILIO_AUTH_TOKEN    string
	TWILIO_SENDER        string
	REDIS_URL            string
	TRIPAY_MERCHANT_CODE string
	TRIPAY_API_KEY       string
	TRIPAY_PRIVATE_KEY   string
	ALLOWED_ORIGINS      string
)

func Init() error {
	err := godotenv.Load()
	if err != nil {
		return err
	}

	DATABASE_URL = os.Getenv("DATABASE_URL")
	TWILIO_ACCOUNT_SID = os.Getenv("TWILIO_ACCOUNT_SID")
	TWILIO_AUTH_TOKEN = os.Getenv("TWILIO_AUTH_TOKEN")
	TWILIO_SENDER = os.Getenv("TWILIO_SENDER")
	REDIS_URL = os.Getenv("REDIS_URL")
	TRIPAY_MERCHANT_CODE = os.Getenv("TRIPAY_MERCHANT_CODE")
	TRIPAY_API_KEY = os.Getenv("TRIPAY_API_KEY")
	TRIPAY_PRIVATE_KEY = os.Getenv("TRIPAY_PRIVATE_KEY")
	ALLOWED_ORIGINS = os.Getenv("ALLOWED_ORIGINS")

	return nil
}
