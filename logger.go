package main

import (
	"os"

	"github.com/rs/zerolog"
)

var (
	Logger zerolog.Logger
)

func SetGlobalLevel(level string) {
	// Logger runs beforce env is initialized
	// so we must use the os.Getenv
	switch level {
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "warning", "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "trace":
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	case "off", "no":
		zerolog.SetGlobalLevel(zerolog.NoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

func initializeLogger() zerolog.Logger {
	// Logger runs beforce env is initialized
	// so we must use the os.Getenv
	logLevel := os.Getenv("LOG_LEVEL")
	SetGlobalLevel(logLevel)

	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat == "json" {
		Logger = zerolog.New(os.Stdout)
	} else {
		Logger = zerolog.New(zerolog.NewConsoleWriter())
	}

	Logger = Logger.With().Timestamp().Caller().Logger()
	return Logger
}
