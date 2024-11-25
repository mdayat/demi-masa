package services

import (
	"sync"

	"github.com/twilio/twilio-go"
)

var (
	twilioOnce   sync.Once
	twilioClient *twilio.RestClient
)

func InitTwilio(TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN string) {
	twilioOnce.Do(func() {
		twilioClient = twilio.NewRestClientWithParams(twilio.ClientParams{
			Username: TWILIO_ACCOUNT_SID,
			Password: TWILIO_AUTH_TOKEN,
		})
	})
}

func GetTwilio() *twilio.RestClient {
	return twilioClient
}
