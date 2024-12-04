package main

import (
	"context"

	"github.com/mdayat/demi-masa-be/internal/config"
	"github.com/mdayat/demi-masa-be/internal/services"
	"github.com/mdayat/demi-masa-be/internal/workerserver"
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

	services.InitTwilio(config.Env.TWILIO_ACCOUNT_SID, config.Env.TWILIO_AUTH_TOKEN)
	services.InitAsynq(config.Env.REDIS_URL)

	server, mux := workerserver.New()
	if err := server.Run(mux); err != nil {
		log.Fatal().Stack().Err(err).Msg("failed to run asynq server")
	}
}
