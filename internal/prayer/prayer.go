package prayer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/hibiken/asynq"
	"github.com/mdayat/demi-masa-be/internal/services"
	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/pkg/errors"
)

type Prayer struct {
	Name      string
	Timestamp int64
}

type PrayerCalendar [6]Prayer

var (
	WIBPrayerCalendar  []PrayerCalendar
	WITPrayerCalendar  []PrayerCalendar
	WITAPrayerCalendar []PrayerCalendar
	WIBTimeZone        = "Asia/Jakarta"
	WITTimeZone        = "Asia/Makassar"
	WITATimeZone       = "Asia/Jayapura"
	SubuhPrayerName    = "Subuh"
	ZuhurPrayerName    = "Zuhur"
	AsarPrayerName     = "Asar"
	MagribPrayerName   = "Magrib"
	IsyaPrayerName     = "Isya"
	SunriseTimeName    = "Sunrise"
)

type aladhanAPIResp struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

type aladhanPrayerCalendar struct {
	Timings struct {
		Fajr    string `json:"Fajr"`
		Sunrise string `json:"Sunrise"`
		Dhuhr   string `json:"Dhuhr"`
		Asr     string `json:"Asr"`
		Maghrib string `json:"Maghrib"`
		Isha    string `json:"Isha"`
	} `json:"timings"`
	Date struct {
		TimestampStr string `json:"timestamp"`
	} `json:"date"`
}

func parsePrayerTime(location *time.Location, timestamp int64, timeValue string) (*time.Time, error) {
	prayerTime, err := time.ParseInLocation("15:04", strings.Split(timeValue, " ")[0], location)
	if err != nil {
		return nil, err
	}

	now := time.Unix(timestamp, 0).In(location)
	prayerTime = time.Date(now.Year(), now.Month(), now.Day(), prayerTime.Hour(), prayerTime.Minute(), 0, 0, location)
	return &prayerTime, nil
}

func ParseAladhanPrayerCalendar(prayerCalendar []aladhanPrayerCalendar, location *time.Location, timeZone string) ([]PrayerCalendar, error) {
	parsedPrayerCalendar := make([]PrayerCalendar, len(prayerCalendar))
	for i, prayer := range prayerCalendar {
		timestamp, err := strconv.ParseInt(prayer.Date.TimestampStr, 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse unix timestamp string to int64")
		}

		var prayers [6]Prayer
		subuh, err := parsePrayerTime(location, timestamp, prayer.Timings.Fajr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse subuh prayer time")
		}

		prayers[0] = Prayer{
			Name:      SubuhPrayerName,
			Timestamp: subuh.Unix(),
		}

		sunrise, err := parsePrayerTime(location, timestamp, prayer.Timings.Sunrise)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse sunrise prayer time")
		}

		prayers[1] = Prayer{
			Name:      SunriseTimeName,
			Timestamp: sunrise.Unix(),
		}

		zuhur, err := parsePrayerTime(location, timestamp, prayer.Timings.Dhuhr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse zuhur prayer time")
		}

		prayers[2] = Prayer{
			Name:      ZuhurPrayerName,
			Timestamp: zuhur.Unix(),
		}

		asar, err := parsePrayerTime(location, timestamp, prayer.Timings.Asr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse asar prayer time")
		}

		prayers[3] = Prayer{
			Name:      AsarPrayerName,
			Timestamp: asar.Unix(),
		}

		magrib, err := parsePrayerTime(location, timestamp, prayer.Timings.Maghrib)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse magrib prayer time")
		}

		prayers[4] = Prayer{
			Name:      MagribPrayerName,
			Timestamp: magrib.Unix(),
		}

		isya, err := parsePrayerTime(location, timestamp, prayer.Timings.Isha)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse isya prayer time")
		}

		prayers[5] = Prayer{
			Name:      IsyaPrayerName,
			Timestamp: isya.Unix(),
		}

		parsedPrayerCalendar[i] = prayers
	}

	return parsedPrayerCalendar, nil
}

func GetAladhanPrayerCalendar(url string) ([]aladhanPrayerCalendar, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	body, err := retry.DoWithData(
		func() ([]aladhanPrayerCalendar, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create new request")
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return nil, errors.Wrap(err, "failed execute http get request")
			}
			defer resp.Body.Close()

			var payload aladhanAPIResp
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				return nil, errors.Wrap(err, "failed to decode response body")
			}

			if payload.Code != http.StatusOK {
				var data string
				if err = json.Unmarshal(payload.Data, &data); err != nil {
					return nil, errors.Wrap(err, "failed to unmarshal failed http get request")
				}

				if data != "" {
					return nil, errors.New(data)
				} else {
					return nil, errors.New(fmt.Sprintf("unexpected status code: %d", payload.Code))
				}
			}

			var data []aladhanPrayerCalendar
			if err = json.Unmarshal(payload.Data, &data); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal successful http get request")
			}

			return data, nil
		},
		retry.Attempts(3),
		retry.Context(ctx),
	)

	if err != nil {
		ctxErr := ctx.Err()
		if ctx.Err() == context.DeadlineExceeded {
			return nil, ctxErr
		} else {
			return nil, err
		}
	}

	return body, nil
}

func InitPrayerCalendar(timeZone string) error {
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return errors.Wrap(err, "failed to load time zone location")
	}

	now := time.Now().In(location)
	URL := fmt.Sprintf(
		"https://api.aladhan.com/v1/calendarByCity/%d/%d?country=Indonesia&city=%s",
		now.Year(),
		now.Month(),
		strings.Split(timeZone, "/")[1],
	)

	prayerCalendar, err := GetAladhanPrayerCalendar(URL)
	if err != nil {
		return errors.Wrap(err, "failed to get aladhan prayer calendar")
	}

	parsedPrayerCalendar, err := ParseAladhanPrayerCalendar(prayerCalendar, location, timeZone)
	if err != nil {
		return errors.Wrap(err, "failed to parse aladhan prayer calendar")
	}

	if timeZone == WIBTimeZone {
		WIBPrayerCalendar = parsedPrayerCalendar
	} else if timeZone == WITTimeZone {
		WITPrayerCalendar = parsedPrayerCalendar
	} else {
		WITAPrayerCalendar = parsedPrayerCalendar
	}

	return nil
}

func GetNextAndPrevPrayer(prayerCalendar []PrayerCalendar, currentDay int, currentTimestamp int64) (Prayer, Prayer) {
	var nextPrayer Prayer
	var prevPrayer Prayer

	todayPrayer := prayerCalendar[currentDay-1]
	for _, prayer := range todayPrayer {
		if prayer.Name == SunriseTimeName {
			continue
		}

		if prayer.Timestamp > currentTimestamp {
			nextPrayer = prayer
			break
		} else {
			prevPrayer = prayer
		}
	}

	if nextPrayer.Name == "" {
		currentDay++
		tomorrowPrayer := prayerCalendar[currentDay-1]
		nextPrayer = tomorrowPrayer[0]
	}

	return nextPrayer, prevPrayer
}

func SchedulePrayerReminder(duration *time.Duration, payload task.PrayerReminderPayload) error {
	asynqTask, err := task.NewPrayerReminderTask(payload)
	if err != nil {
		return errors.Wrap(err, "failed to create prayer reminder task")
	}

	_, err = services.GetAsynqClient().Enqueue(asynqTask, asynq.ProcessIn(*duration))
	if err != nil {
		return errors.Wrap(err, "failed to enqueue prayer reminder task")
	}

	return nil
}

func ScheduleLastPrayerReminder(duration *time.Duration, payload task.PrayerReminderPayload) error {
	asynqTask, err := task.NewLastPrayerReminderTask(payload)
	if err != nil {
		return errors.Wrap(err, "failed to create last prayer reminder task")
	}

	_, err = services.GetAsynqClient().Enqueue(asynqTask, asynq.ProcessIn(*duration))
	if err != nil {
		return errors.Wrap(err, "failed to enqueue last prayer reminder task")
	}

	return nil
}

func InitPrayerReminder(prayerCalendar []PrayerCalendar, timeZone string) error {
	users, err := services.GetQueries().GetUsersByTimeZone(
		context.TODO(),
		repository.NullIndonesiaTimeZone{
			IndonesiaTimeZone: repository.IndonesiaTimeZone(timeZone),
			Valid:             true,
		},
	)

	if err != nil {
		return errors.Wrap(err, "failed to get users by time zone")
	}

	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return errors.Wrap(err, "failed to load time zone location")
	}

	for _, user := range users {
		now := time.Now().In(location)
		currentDay := now.Day()
		currentTimestamp := now.Unix()

		nextPrayer, _ := GetNextAndPrevPrayer(prayerCalendar, currentDay, currentTimestamp)
		duration := time.Unix(nextPrayer.Timestamp, 0).Sub(now)
		err = SchedulePrayerReminder(
			&duration,
			task.PrayerReminderPayload{
				UserID:     user.ID,
				PrayerName: nextPrayer.Name,
			},
		)

		if err != nil {
			return errors.Wrap(err, "failed to schedule prayer reminder")
		}
	}

	return nil
}
