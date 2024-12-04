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
)

type PrayerCalendar struct {
	Subuh  int64
	Zuhur  int64
	Asar   int64
	Magrib int64
	Isya   int64
}

var (
	WIBPrayerCalendar  []PrayerCalendar
	WITPrayerCalendar  []PrayerCalendar
	WITAPrayerCalendar []PrayerCalendar
	WIBTimeZone        = "Asia/Jakarta"
	WITTimeZone        = "Asia/Makassar"
	WITATimeZone       = "Asia/Jayapura"
)

type aladhanAPIResp struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

type aladhanPrayerCalendar struct {
	Timings struct {
		Fajr    string `json:"Fajr"`
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

func ParseAladhanPrayerCalendar(prayerCalendar *[]aladhanPrayerCalendar, location *time.Location, timeZone string) (*[]PrayerCalendar, error) {
	parsedPrayerCalendar := make([]PrayerCalendar, len(*prayerCalendar))
	for i, prayer := range *prayerCalendar {
		timestamp, err := strconv.ParseInt(prayer.Date.TimestampStr, 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse unix timestamp string to int64")
		}

		subuh, err := parsePrayerTime(location, timestamp, prayer.Timings.Fajr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse subuh prayer time")
		}

		zuhur, err := parsePrayerTime(location, timestamp, prayer.Timings.Dhuhr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse zuhur prayer time")
		}

		asar, err := parsePrayerTime(location, timestamp, prayer.Timings.Asr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse asar prayer time")
		}

		magrib, err := parsePrayerTime(location, timestamp, prayer.Timings.Maghrib)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse magrib prayer time")
		}

		isya, err := parsePrayerTime(location, timestamp, prayer.Timings.Isha)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse isya prayer time")
		}

		parsedPrayerCalendar[i].Subuh = subuh.Unix()
		parsedPrayerCalendar[i].Zuhur = zuhur.Unix()
		parsedPrayerCalendar[i].Asar = asar.Unix()
		parsedPrayerCalendar[i].Magrib = magrib.Unix()
		parsedPrayerCalendar[i].Isya = isya.Unix()
	}

	return &parsedPrayerCalendar, nil
}

func GetAladhanPrayerCalendar(url string) (*[]aladhanPrayerCalendar, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	body, err := retry.DoWithData(
		func() (*[]aladhanPrayerCalendar, error) {
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

			return &data, nil
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
		WIBPrayerCalendar = *parsedPrayerCalendar
	} else if timeZone == WITTimeZone {
		WITPrayerCalendar = *parsedPrayerCalendar
	} else {
		WITAPrayerCalendar = *parsedPrayerCalendar
	}

	return nil
}
