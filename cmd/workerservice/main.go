package main

import (
	"context"
	"sync"

	"github.com/odemimasa/backend/internal/config"
	"github.com/odemimasa/backend/internal/prayer"
	"github.com/odemimasa/backend/internal/services"
	"github.com/odemimasa/backend/internal/task"
	"github.com/odemimasa/backend/internal/workerservice"
	"github.com/odemimasa/backend/repository"
	"github.com/pkg/errors"
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

	services.InitRedis(config.Env.REDIS_URL)
	services.InitTwilio(config.Env.TWILIO_ACCOUNT_SID, config.Env.TWILIO_AUTH_TOKEN)
	services.InitAsynq(config.Env.REDIS_URL)

	var wg sync.WaitGroup
	errChan := make(chan error, 3)

	wg.Add(1)
	go func() {
		defer wg.Done()

		err = prayer.InitPrayerCalendar(repository.IndonesiaTimeZoneAsiaJakarta)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init WIB prayer calendar")
			return
		}

		err = prayer.InitPrayerReminder(repository.IndonesiaTimeZoneAsiaJakarta)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init WIB prayer reminder")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err = prayer.InitPrayerCalendar(repository.IndonesiaTimeZoneAsiaJayapura)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init WIT prayer calendar")
			return
		}

		err = prayer.InitPrayerReminder(repository.IndonesiaTimeZoneAsiaJayapura)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init WIT prayer reminder")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err = prayer.InitPrayerCalendar(repository.IndonesiaTimeZoneAsiaMakassar)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init WITA prayer calendar")
			return
		}

		err = prayer.InitPrayerReminder(repository.IndonesiaTimeZoneAsiaMakassar)
		if err != nil {
			errChan <- errors.Wrap(err, "failed to init WITA prayer reminder")
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
				log.Fatal().Err(err).Msg("failed to concurrently init prayer calendar and reminder")
			}
		}
	}

	err = task.ScheduleTaskRemovalTask()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to schedule task removal task")
	}

	err = prayer.ScheduleFirstPrayerUpdateTask()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to schedule first prayer update task")
	}

	service, mux := workerservice.New()
	if err := service.Run(mux); err != nil {
		log.Fatal().Stack().Err(err).Msg("failed to run asynq service")
	}
}
