package main

import (
	"context"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/mdayat/demi-masa/asynqmon/configs/env"
	"github.com/mdayat/demi-masa/asynqmon/configs/services"
	"github.com/mdayat/demi-masa/asynqmon/internal"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}

	logger := log.With().Caller().Logger()
	err := env.Init()
	if err != nil {
		logger.Fatal().Err(err).Send()
	}

	ctx := context.TODO()
	err = services.InitFirebase(ctx)
	if err != nil {
		logger.Fatal().Err(err).Send()
	}

	app := internal.InitApp()
	err = http.ListenAndServe(":9090", app)
	if err != nil {
		logger.Fatal().Err(err).Send()
	}
}
