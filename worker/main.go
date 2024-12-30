package main

import (
	"context"
	"path/filepath"
	"strconv"

	"github.com/mdayat/demi-masa/worker/configs/env"
	"github.com/mdayat/demi-masa/worker/configs/services"
	"github.com/mdayat/demi-masa/worker/internal"
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
	db, err := services.InitDB(ctx, env.DATABASE_URL)
	if err != nil {
		logger.Fatal().Err(err).Send()
	}
	defer db.Close()

	services.InitRedis(env.REDIS_URL)
	services.InitTwilio(env.TWILIO_ACCOUNT_SID, env.TWILIO_AUTH_TOKEN)
	services.InitAsynq(env.REDIS_URL)

	app, mux := internal.InitApp()
	if err := app.Run(mux); err != nil {
		logger.Fatal().Err(err).Send()
	}
}
