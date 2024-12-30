package services

import (
	"github.com/twilio/twilio-go"
)

var (
	TwilioClient *twilio.RestClient
)

func InitTwilio(accountSID, authToken string) {
	TwilioClient = twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: accountSID,
		Password: authToken,
	})
}
