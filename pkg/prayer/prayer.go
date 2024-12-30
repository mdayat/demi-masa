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

func ParseAladhanPrayerCalendar(prayerCalendar []aladhanPrayerCalendar, location *time.Location) (PrayerCalendar, error) {
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

func MakePrayerCalendarKey(timeZone string) string {
	return fmt.Sprintf("prayer:calendar:%s", timeZone)
}

func MakeLastDayPrayerKey(timeZone string) string {
	return fmt.Sprintf("prayer:last_day:%s", timeZone)
}

func MakePenultimateDayPrayerKey(timeZone string) string {
	return fmt.Sprintf("prayer:penultimate_day:%s", timeZone)
}

func GetPrayerCalendar(ctx context.Context, redisClient *redis.Client, timeZone string) (PrayerCalendar, error) {
	prayerCalendarJSON, err := redisClient.Get(ctx, MakePrayerCalendarKey(timeZone)).Result()
	if err != nil {
		return nil, err
	}

	var prayerCalendar PrayerCalendar
	err = json.Unmarshal([]byte(prayerCalendarJSON), &prayerCalendar)
	if err != nil {
		return nil, err
	}

	return prayerCalendar, nil
}

func GetLastDayPrayer(ctx context.Context, redisClient *redis.Client, timeZone string) (Prayers, error) {
	lastDayPrayerJSON, err := redisClient.Get(ctx, MakeLastDayPrayerKey(timeZone)).Result()
	if err != nil {
		return nil, err
	}

	var lastDayPrayer Prayers
	err = json.Unmarshal([]byte(lastDayPrayerJSON), &lastDayPrayer)
	if err != nil {
		return nil, err
	}

	return lastDayPrayer, nil
}

func GetPenultimateDayPrayer(ctx context.Context, redisClient *redis.Client, timeZone string) (Prayers, error) {
	penultimateDayPrayerJSON, err := redisClient.Get(ctx, MakeLastDayPrayerKey(timeZone)).Result()
	if err != nil {
		return nil, err
	}

	var penultimateDayPrayer Prayers
	err = json.Unmarshal([]byte(penultimateDayPrayerJSON), &penultimateDayPrayer)
	if err != nil {
		return nil, err
	}

	return penultimateDayPrayer, nil
}

func IsLastDay(currentTime *time.Time) bool {
	firstDayNextMonth := time.Date(currentTime.Year(), currentTime.Month()+1, 1, 0, 0, 0, 0, currentTime.Location())
	lastDayCurrentMonth := firstDayNextMonth.AddDate(0, 0, -1)
	return currentTime.Day() == lastDayCurrentMonth.Day()
}

func IsPenultimateDay(currentTime *time.Time) bool {
	firstDayNextMonth := time.Date(currentTime.Year(), currentTime.Month()+1, 1, 0, 0, 0, 0, currentTime.Location())
	penultimateDayCurrentMonth := firstDayNextMonth.AddDate(0, 0, -2)
	return currentTime.Day() == penultimateDayCurrentMonth.Day()
}
