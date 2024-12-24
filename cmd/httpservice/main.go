package main

import (
	"context"

	"net/http"

	"github.com/odemimasa/backend/internal/config"
	"github.com/odemimasa/backend/internal/httpservice"
	"github.com/odemimasa/backend/internal/services"
	"github.com/rs/zerolog/log"
)

func main() {
	config.InitLogger()
	err := config.LoadEnv()
	if err != nil {
		log.Fatal().Stack().Err(err).Msgf("failed to load %s file", ".env")
	}

	ctx := context.Background()
	db, err := services.InitDB(ctx, config.Env.DATABASE_URL)
	if err != nil {
		log.Fatal().Stack().Err(err).Msg("failed to init PostgreSQL database")
	}
	defer db.Close()

	err = services.InitFirebase(ctx)
	if err != nil {
		log.Fatal().Stack().Err(err).Msg("failed to init firebase")
	}

	services.InitRedis(config.Env.REDIS_URL)
	services.InitTwilio(config.Env.TWILIO_ACCOUNT_SID, config.Env.TWILIO_AUTH_TOKEN)
	services.InitAsynq(config.Env.REDIS_URL)

	service := httpservice.New()
	http.ListenAndServe(":8080", service)
}
