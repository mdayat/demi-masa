package main

import (
	"context"
	"net/http"
	"path/filepath"
	"strconv"
	_ "time/tzdata"

	"github.com/mdayat/demi-masa/web/configs/env"
	"github.com/mdayat/demi-masa/web/configs/services"
	"github.com/mdayat/demi-masa/web/internal"
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

	err = services.InitFirebase(ctx)
	if err != nil {
		logger.Fatal().Err(err).Send()
	}

	services.InitRedis(env.REDIS_URL)
	services.InitTwilio(env.TWILIO_ACCOUNT_SID, env.TWILIO_AUTH_TOKEN)
	services.InitAsynq(env.REDIS_URL)

	app := internal.InitApp()
	err = http.ListenAndServe(":8080", app)
	if err != nil {
		logger.Fatal().Err(err).Send()
	}
}
