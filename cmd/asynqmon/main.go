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
	err = services.InitFirebase(ctx)
	if err != nil {
		log.Fatal().Stack().Err(err).Msg("failed to init firebase")
	}

	services.InitAsynq(config.Env.REDIS_URL)

	service := httpservice.New()
	http.ListenAndServe(":8080", service)
}
