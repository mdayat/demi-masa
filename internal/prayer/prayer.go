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
	"github.com/redis/go-redis/v9"
)

type Prayer struct {
	Name     string
	UnixTime int64
}

type Prayers []Prayer
type PrayerCalendar []Prayers

var (
	SubuhPrayerName  = "Subuh"
	ZuhurPrayerName  = "Zuhur"
	AsarPrayerName   = "Asar"
	MagribPrayerName = "Magrib"
	IsyaPrayerName   = "Isya"
	SunriseTimeName  = "Sunrise"
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
	prayerTime = time.Date(now.Year(), now.Month(), now.Day(), prayerTime.Hour(), prayerTime.Minute(), 0, 0, now.Location())
	return &prayerTime, nil
}

func ParseAladhanPrayerCalendar(
	prayerCalendar []aladhanPrayerCalendar,
	location *time.Location,
	timeZone repository.IndonesiaTimeZone,
) (PrayerCalendar, error) {
	parsedPrayerCalendar := make(PrayerCalendar, len(prayerCalendar))
	for i, prayer := range prayerCalendar {
		timestamp, err := strconv.ParseInt(prayer.Date.TimestampStr, 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse unix timestamp string to int64")
		}

		prayers := make(Prayers, 6)
		subuh, err := parsePrayerTime(location, timestamp, prayer.Timings.Fajr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse subuh prayer time")
		}

		prayers[0] = Prayer{
			Name:     SubuhPrayerName,
			UnixTime: subuh.Unix(),
		}

		sunrise, err := parsePrayerTime(location, timestamp, prayer.Timings.Sunrise)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse sunrise prayer time")
		}

		prayers[1] = Prayer{
			Name:     SunriseTimeName,
			UnixTime: sunrise.Unix(),
		}

		zuhur, err := parsePrayerTime(location, timestamp, prayer.Timings.Dhuhr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse zuhur prayer time")
		}

		prayers[2] = Prayer{
			Name:     ZuhurPrayerName,
			UnixTime: zuhur.Unix(),
		}

		asar, err := parsePrayerTime(location, timestamp, prayer.Timings.Asr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse asar prayer time")
		}

		prayers[3] = Prayer{
			Name:     AsarPrayerName,
			UnixTime: asar.Unix(),
		}

		magrib, err := parsePrayerTime(location, timestamp, prayer.Timings.Maghrib)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse magrib prayer time")
		}

		prayers[4] = Prayer{
			Name:     MagribPrayerName,
			UnixTime: magrib.Unix(),
		}

		isya, err := parsePrayerTime(location, timestamp, prayer.Timings.Isha)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse isya prayer time")
		}

		prayers[5] = Prayer{
			Name:     IsyaPrayerName,
			UnixTime: isya.Unix(),
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

func SchedulePrayerRenewal(
	numOfDays int,
	now *time.Time,
	payload task.PrayerRenewalTask,
) error {
	asynqTask, err := task.NewPrayerRenewalTask(payload)
	if err != nil {
		return errors.Wrap(err, "failed to create prayer renewal task")
	}

	renewalDate := time.Date(now.Year(), now.Month(), numOfDays, 0, 0, 0, 0, now.Location())
	renewalDuration := renewalDate.Unix() - now.Unix()
	_, err = services.GetAsynqClient().Enqueue(asynqTask, asynq.ProcessIn(time.Duration(renewalDuration)*time.Second))
	if err != nil {
		return errors.Wrap(err, "failed to enqueue prayer renewal task")
	}

	return nil
}

func InitPrayerCalendar(timeZone repository.IndonesiaTimeZone) error {
	location, err := time.LoadLocation(string(timeZone))
	if err != nil {
		return errors.Wrap(err, "failed to load time zone location")
	}

	now := time.Now().In(location)
	URL := fmt.Sprintf(
		"https://api.aladhan.com/v1/calendarByCity/%d/%d?country=Indonesia&city=%s",
		now.Year(),
		now.Month(),
		strings.Split(string(timeZone), "/")[1],
	)

	prayerCalendar, err := GetAladhanPrayerCalendar(URL)
	if err != nil {
		return errors.Wrap(err, "failed to get aladhan prayer calendar")
	}

	parsedPrayerCalendar, err := ParseAladhanPrayerCalendar(prayerCalendar, location, timeZone)
	if err != nil {
		return errors.Wrap(err, "failed to parse aladhan prayer calendar")
	}

	prayerCalendarJSON, err := json.Marshal(parsedPrayerCalendar)
	if err != nil {
		return errors.Wrap(err, "failed to marshal parsed aladhan prayer calendar")
	}

	penultimateDayPrayer := parsedPrayerCalendar[len(parsedPrayerCalendar)-2]
	penultimateDayPrayerJSON, err := json.Marshal(penultimateDayPrayer)
	if err != nil {
		return errors.Wrap(err, "failed to marshal penultimate day prayer of parsed aladhan prayer calendar")
	}

	lastDayPrayer := parsedPrayerCalendar[len(parsedPrayerCalendar)-1]
	lastDayPrayerJSON, err := json.Marshal(lastDayPrayer)
	if err != nil {
		return errors.Wrap(err, "failed to marshal last day prayer of parsed aladhan prayer calendar")
	}

	err = services.GetRedis().Watch(context.TODO(), func(tx *redis.Tx) error {
		_, err = tx.TxPipelined(context.TODO(), func(pipe redis.Pipeliner) error {
			pipe.Set(context.TODO(), MakePrayerCalendarKey(timeZone), prayerCalendarJSON, 0)
			pipe.Set(context.TODO(), MakeLastDayPrayerKey(timeZone), lastDayPrayerJSON, 0)
			pipe.Set(context.TODO(), MakePenultimateDayPrayerKey(timeZone), penultimateDayPrayerJSON, 0)
			return nil
		})
		return err
	})

	if err != nil {
		return errors.Wrap(err, "failed to execute redis tx to set prayer calendar and last day prayer")
	}

	numOfDays := len(parsedPrayerCalendar)
	err = SchedulePrayerRenewal(
		numOfDays,
		&now,
		task.PrayerRenewalTask{TimeZone: timeZone},
	)

	if err != nil {
		return errors.Wrap(err, "failed to schedule prayer renewal")
	}

	return nil
}

func GetNextPrayer(
	prayerCalendar PrayerCalendar,
	lastPrayers Prayers,
	currentDay int,
	currentTimestamp int64,
) Prayer {
	var nextPrayer Prayer

	if lastPrayers == nil {
		todayPrayer := prayerCalendar[currentDay-1]
		for _, prayer := range todayPrayer {
			if prayer.Name == SunriseTimeName {
				continue
			}

			if prayer.UnixTime > currentTimestamp {
				nextPrayer = prayer
				break
			}
		}

		if nextPrayer.Name == "" {
			currentDay++
			tomorrowPrayer := prayerCalendar[currentDay-1]
			nextPrayer = tomorrowPrayer[0]
		}
	} else {
		for _, prayer := range lastPrayers {
			if prayer.Name == SunriseTimeName {
				continue
			}

			if prayer.UnixTime > currentTimestamp {
				nextPrayer = prayer
				break
			}
		}

		if nextPrayer.Name == "" {
			tomorrowPrayer := prayerCalendar[0]
			nextPrayer = tomorrowPrayer[0]
		}
	}

	return nextPrayer
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

func ScheduleLastPrayerReminder(duration *time.Duration, payload task.LastPrayerReminderPayload) error {
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

func InitPrayerReminder(timeZone repository.IndonesiaTimeZone) error {
	prayerCalendar, err := GetPrayerCalendar(context.TODO(), timeZone)
	if err != nil {
		return errors.Wrap(err, "failed to get prayer calendar")
	}

	lastDayPrayer, err := GetLastDayPrayer(context.TODO(), timeZone)
	if err != nil {
		return errors.Wrap(err, "failed to get last day prayer")
	}

	users, err := services.GetQueries().GetUsersByTimeZone(
		context.TODO(),
		repository.NullIndonesiaTimeZone{
			IndonesiaTimeZone: timeZone,
			Valid:             true,
		},
	)

	if err != nil {
		return errors.Wrap(err, "failed to get users by time zone")
	}

	location, err := time.LoadLocation(string(timeZone))
	if err != nil {
		return errors.Wrap(err, "failed to load time zone location")
	}

	now := time.Now().In(location)
	currentDay := now.Day()

	isLastDay := IsLastDay(&now)
	isPenultimateDay := IsPenultimateDay(&now)
	isyaPrayer := prayerCalendar[currentDay-1][5]

	if isLastDay {
		isyaPrayer = lastDayPrayer[5]
	}

	for _, user := range users {
		now = time.Now().In(location)
		currentUnixTime := now.Unix()

		var isNextPrayerLastDay bool
		if (isPenultimateDay && currentUnixTime > isyaPrayer.UnixTime) ||
			(isLastDay && currentUnixTime < isyaPrayer.UnixTime) {
			isNextPrayerLastDay = true
		}

		var nextPrayer Prayer
		if isLastDay && currentUnixTime < isyaPrayer.UnixTime {
			nextPrayer = GetNextPrayer(prayerCalendar, lastDayPrayer, currentDay, currentUnixTime)
		} else {
			if isLastDay && currentUnixTime > isyaPrayer.UnixTime {
				currentDay = 1
			}

			nextPrayer = GetNextPrayer(prayerCalendar, nil, currentDay, currentUnixTime)
		}

		duration := time.Duration(nextPrayer.UnixTime-currentUnixTime) * time.Second
		err = SchedulePrayerReminder(
			&duration,
			task.PrayerReminderPayload{
				UserID:         user.ID,
				PrayerName:     nextPrayer.Name,
				PrayerUnixTime: nextPrayer.UnixTime,
				IsLastDay:      isNextPrayerLastDay,
			},
		)

		if err != nil {
			return errors.Wrap(err, "failed to schedule prayer reminder")
		}
	}

	return nil
}

func ScheduleFirstPrayerUpdateTask() error {
	location, err := time.LoadLocation(string(repository.IndonesiaTimeZoneAsiaJakarta))
	if err != nil {
		return errors.Wrap(err, "failed to load time zone location")
	}

	now := time.Now().In(location)
	sixAMToday := time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, now.Location())

	var targetTime time.Time
	if now.Before(sixAMToday) {
		targetTime = sixAMToday
	} else {
		nextDay := now.Add(24 * time.Hour).In(now.Location())
		targetTime = time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 6, 0, 0, 0, nextDay.Location())
	}

	asynqTask, err := task.NewPrayerUpdateTask()
	if err != nil {
		return errors.Wrap(err, "failed to create prayer update task")
	}

	_, err = services.GetAsynqClient().Enqueue(asynqTask, asynq.ProcessIn(targetTime.Sub(now)))
	if err != nil {
		return errors.Wrap(err, "failed to enqueue prayer update task")
	}

	return nil
}

func SchedulePrayerUpdateTask(now *time.Time) error {
	nextDay := now.Add(24 * time.Hour).In(now.Location())
	nextDayAtSix := time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 6, 0, 0, 0, nextDay.Location())

	asynqTask, err := task.NewPrayerUpdateTask()
	if err != nil {
		return errors.Wrap(err, "failed to create prayer update task")
	}

	_, err = services.GetAsynqClient().Enqueue(asynqTask, asynq.ProcessIn(nextDayAtSix.Sub(*now)))
	if err != nil {
		return errors.Wrap(err, "failed to enqueue prayer update task")
	}

	return nil
}
