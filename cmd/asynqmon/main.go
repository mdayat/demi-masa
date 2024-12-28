package main

import (
	"context"
	"net/http"

	"github.com/odemimasa/backend/internal/asynqmon"
	"github.com/odemimasa/backend/internal/config"
	"github.com/odemimasa/backend/internal/services"
	"github.com/rs/zerolog/log"
)

func main() {
	config.InitLogger()
	err := config.LoadEnv()
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to load %s file", ".env")
	}

	ctx := context.Background()
	err = services.InitFirebase(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init firebase")
	}

	service := asynqmon.New()
	err = http.ListenAndServe(":9090", service)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen and serve to port 9090")
	}
}
