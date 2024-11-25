package main

import (
	"context"

	"net/http"

	"github.com/mdayat/demi-masa-be/internal/config"
	"github.com/mdayat/demi-masa-be/internal/httpserver"
	"github.com/mdayat/demi-masa-be/internal/services"
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

	server := httpserver.NewServer()
	http.ListenAndServe(":80", server)
}
