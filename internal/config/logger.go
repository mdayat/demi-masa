package config

import (
	"path/filepath"
	"strconv"
	"sync"

	"github.com/rs/zerolog"
)

var loggerOnce sync.Once

func InitLogger() {
	loggerOnce.Do(func() {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
			return filepath.Base(file) + ":" + strconv.Itoa(line)
		}
	})
}
