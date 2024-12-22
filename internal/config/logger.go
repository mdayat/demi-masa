package config

import (
	"path/filepath"
	"strconv"
	"sync"

	"github.com/rs/zerolog"
)

var loggerOnce sync.Once

type LogCallerHook struct{}

func (h LogCallerHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if level == zerolog.ErrorLevel {
		e.Caller()
	}
}

func InitLogger() {
	loggerOnce.Do(func() {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
			return filepath.Base(file) + ":" + strconv.Itoa(line)
		}
	})
}
