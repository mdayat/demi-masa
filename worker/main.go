package main

import (
	"context"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/mdayat/demi-masa/worker/configs/env"
	"github.com/mdayat/demi-masa/worker/configs/services"
	"github.com/mdayat/demi-masa/worker/internal"
	"github.com/mdayat/demi-masa/worker/repository"
	"github.com/pkg/errors"
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

	var wg sync.WaitGroup
	errChan := make(chan error, 3)

	wg.Add(1)
	go func() {
		defer wg.Done()

		location, err := time.LoadLocation(string(repository.IndonesiaTimeZoneAsiaJakarta))
		if err != nil {
			errChan <- errors.Wrap(err, "failed to load WIB time zone location")
			return
		}

		err = internal.InitPrayerCalendar(ctx, location)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init WIB prayer calendar")
			return
		}

		err = internal.InitPrayerReminder(ctx, location)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init WIB prayer reminder")
			return
		}

		err = internal.InitTaskRemovalTask(location)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init task removal task")
			return
		}

		err = internal.InitPrayerUpdateTask(location)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init prayer update task")
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		location, err := time.LoadLocation(string(repository.IndonesiaTimeZoneAsiaJayapura))
		if err != nil {
			errChan <- errors.Wrap(err, "failed to load WIT time zone location")
			return
		}

		err = internal.InitPrayerCalendar(ctx, location)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init WIT prayer calendar")
			return
		}

		err = internal.InitPrayerReminder(ctx, location)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init WIT prayer reminder")
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		location, err := time.LoadLocation(string(repository.IndonesiaTimeZoneAsiaMakassar))
		if err != nil {
			errChan <- errors.Wrap(err, "failed to load WITA time zone location")
			return
		}

		err = internal.InitPrayerCalendar(ctx, location)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init WITA prayer calendar")
			return
		}

		err = internal.InitPrayerReminder(ctx, location)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init WITA prayer reminder")
			return
		}
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for i := 0; i < 3; i++ {
		select {
		case err = <-errChan:
			if err != nil {
				log.Fatal().Caller().Err(err).Send()
			}
		}
	}

	app, mux := internal.InitApp()
	if err := app.Run(mux); err != nil {
		logger.Fatal().Err(err).Send()
	}
}
