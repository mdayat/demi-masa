package httpserver

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mdayat/demi-masa-be/internal/prayer"
	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type prayerRespBody struct {
	Name     string                       `json:"name"`
	UnixTime int64                        `json:"unix_time"`
	TimeZone repository.IndonesiaTimeZone `json:"time_zone"`
}

func getPrayersHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	userID := fmt.Sprintf("%s", req.Context().Value("userID"))

	userTimeZone, err := queries.GetUserTimeZoneByID(req.Context(), userID)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get user time zone by id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully got user time zone by id")

	location, err := time.LoadLocation(string(userTimeZone.IndonesiaTimeZone))
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to load time zone location")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully loaded time zone location")

	prayerCalendar, err := prayer.GetPrayerCalendar(req.Context(), userTimeZone.IndonesiaTimeZone)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get prayer calendar")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully got prayer calendar")

	now := time.Now().In(location)
	currentDay := now.Day()
	currentUnixTime := now.Unix()
	isLastDay := prayer.IsLastDay(&now)

	todayPrayer := prayerCalendar[currentDay-1]
	subuhPrayer := todayPrayer[0]
	var usedPrayer prayer.Prayers

	if currentDay == 1 && currentUnixTime < subuhPrayer.UnixTime ||
		isLastDay && currentUnixTime > subuhPrayer.UnixTime {
		usedPrayer, err = prayer.GetLastDayPrayer(req.Context(), userTimeZone.IndonesiaTimeZone)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to get last day prayer")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Msg("successfully got last day prayer")
	} else if isLastDay && currentUnixTime < subuhPrayer.UnixTime {
		usedPrayer, err = prayer.GetPenultimateDayPrayer(req.Context(), userTimeZone.IndonesiaTimeZone)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to get penultimate day prayer")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Msg("successfully got last day prayer")
	} else if isLastDay == false && currentUnixTime < subuhPrayer.UnixTime {
		yesterdayPrayer := prayerCalendar[currentDay-2]
		usedPrayer = yesterdayPrayer
	} else {
		usedPrayer = todayPrayer
	}

	respBody := make([]prayerRespBody, len(usedPrayer))
	for i, v := range usedPrayer {
		respBody[i] = prayerRespBody{
			Name:     v.Name,
			UnixTime: v.UnixTime,
			TimeZone: userTimeZone.IndonesiaTimeZone,
		}
	}

	err = sendJSONSuccessResponse(res, successResponseParams{StatusCode: http.StatusOK, Data: &respBody})
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to send successful response body")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully sent successful response body")
}

func createPrayerHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	var body struct {
		PrayerName     string                       `json:"prayer_name" validate:"required"`
		PrayerUnixTime int64                        `json:"prayer_unix_time" validate:"required"`
		TimeZone       repository.IndonesiaTimeZone `json:"time_zone" validate:"required"`
		CheckedAt      int64                        `json:"checked_at" validate:"required"`
	}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	logWithCtx.Info().Msg("successfully decoded and validated request body")

	prayerCalendar, err := prayer.GetPrayerCalendar(req.Context(), body.TimeZone)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get prayer calendar")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully got prayer calendar")

	checkedTime := time.Unix(body.CheckedAt, 0)
	isLastDay := prayer.IsLastDay(&checkedTime)
	prayerTime := time.Unix(body.PrayerUnixTime, 0)

	var isIsyaOfLastDay bool
	if isLastDay && body.PrayerName == prayer.IsyaPrayerName && checkedTime.Day() == prayerTime.Day() {
		isIsyaOfLastDay = true
	}

	var lastDayPrayer prayer.Prayers
	if isLastDay && isIsyaOfLastDay == false {
		lastDayPrayer, err = prayer.GetLastDayPrayer(req.Context(), body.TimeZone)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to get last day prayer")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Msg("successfully got last day prayer")
	}

	checkedDay := checkedTime.Day()
	if isLastDay && isIsyaOfLastDay {
		checkedDay = 1
	}

	var nextPrayer prayer.Prayer
	if body.PrayerName == prayer.SubuhPrayerName && isLastDay {
		lastDayPrayer, err = prayer.GetLastDayPrayer(req.Context(), body.TimeZone)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to get last day prayer")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		logWithCtx.Info().Msg("successfully got last day prayer")

		sunriseTime := lastDayPrayer[1]
		nextPrayer = sunriseTime
	} else if body.PrayerName == prayer.SubuhPrayerName && isLastDay == false {
		sunriseTime := prayerCalendar[checkedDay-1][1]
		nextPrayer = sunriseTime
	} else {
		nextPrayer = prayer.GetNextPrayer(prayerCalendar, lastDayPrayer, checkedDay, body.PrayerUnixTime)
	}

	nextPrayerTime := time.Unix(nextPrayer.UnixTime, 0)
	prayersDistance := nextPrayerTime.Sub(prayerTime)

	distanceQuarter := int(math.Round(prayersDistance.Seconds() * 0.25))
	distanceToNextPrayer := nextPrayerTime.Sub(checkedTime)

	prayerStatus := repository.PrayerStatusLATE
	if distanceToNextPrayer.Seconds()-float64(distanceQuarter) > 0 {
		prayerStatus = repository.PrayerStatusONTIME
	}

	userID := fmt.Sprintf("%s", req.Context().Value("userID"))
	userAccountType, err := queries.GetUserSubsByID(req.Context(), userID)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get user account type by id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully got user account type by id")

	err = queries.CreatePrayer(req.Context(), repository.CreatePrayerParams{
		UserID: userID,
		Name:   body.PrayerName,
		Status: prayerStatus,
		Year:   int16(prayerTime.Year()),
		Month:  int16(prayerTime.Month()),
		Day:    int16(prayerTime.Day()),
	})

	if err != nil {
		ErrUniqueViolation := "23505"
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == ErrUniqueViolation {
			errMsg := fmt.Sprintf(
				"Kamu telah mencentang salat %s, kamu tidak dapat mencentang lebih dari sekali.",
				body.PrayerName,
			)

			logWithCtx.Warn().Msg(pgErr.Message)
			http.Error(res, errMsg, http.StatusBadRequest)
		} else {
			logWithCtx.Error().Err(err).Msg("failed to create prayer")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}
	logWithCtx.Info().Msg("successfully created prayer")

	if userAccountType == repository.AccountTypePREMIUM {
		asynqTaskID := task.LastPrayerReminderTaskID(userID, body.PrayerName)
		err = asynqInspector.DeleteTask(task.DefaultQueue, asynqTaskID)
		if err != nil {
			if errors.Is(err, asynq.ErrQueueNotFound) {
				logWithCtx.Error().Err(err).Str("queue_name", task.DefaultQueue).Send()
			} else if errors.Is(err, asynq.ErrTaskNotFound) {
				logWithCtx.Error().Err(err).Str("task_id", asynqTaskID).Send()
			} else {
				logWithCtx.Error().Err(err).Str("task_id", asynqTaskID).Msg("failed to delete last prayer reminder")
				http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}
		logWithCtx.Info().Str("task_id", asynqTaskID).Msg("successfully deleted last prayer reminder")
	}
}
