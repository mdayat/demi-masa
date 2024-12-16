package prayer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mdayat/demi-masa-be/internal/services"
	"github.com/mdayat/demi-masa-be/repository"
)

func MakePrayerCalendarKey(timeZone repository.IndonesiaTimeZone) string {
	return fmt.Sprintf("prayer:calendar:%s", timeZone)
}

func MakeLastDayPrayerKey(timeZone repository.IndonesiaTimeZone) string {
	return fmt.Sprintf("prayer:last_day:%s", timeZone)
}

func MakePenultimateDayPrayerKey(timeZone repository.IndonesiaTimeZone) string {
	return fmt.Sprintf("prayer:penultimate_day:%s", timeZone)
}

func GetPrayerCalendar(ctx context.Context, timeZone repository.IndonesiaTimeZone) (PrayerCalendar, error) {
	prayerCalendarJSON, err := services.GetRedis().Get(ctx, MakePrayerCalendarKey(timeZone)).Result()
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

func GetLastDayPrayer(ctx context.Context, timeZone repository.IndonesiaTimeZone) (Prayers, error) {
	lastDayPrayerJSON, err := services.GetRedis().Get(ctx, MakeLastDayPrayerKey(timeZone)).Result()
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

func GetPenultimateDayPrayer(ctx context.Context, timeZone repository.IndonesiaTimeZone) (Prayers, error) {
	penultimateDayPrayerJSON, err := services.GetRedis().Get(ctx, MakeLastDayPrayerKey(timeZone)).Result()
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
