package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/EternisAI/silo-proxy/internal/api/http"
)

const (
	LOG_LEVEL_ERROR   = "ERROR"
	LOG_LEVEL_WARNING = "WARNING"
	LOG_LEVEL_INFO    = "INFO"
	LOG_LEVEL_DEBUG   = "DEBUG"
)

type LogConfig struct {
	Level string
	Http  http.Config
}

func initLogger(logLevel string) {
	var level slog.Level
	switch strings.ToUpper(logLevel) {
	case LOG_LEVEL_ERROR:
		level = slog.LevelError
	case LOG_LEVEL_WARNING:
		level = slog.LevelWarn
	case LOG_LEVEL_INFO:
		level = slog.LevelInfo
	case LOG_LEVEL_DEBUG:
		level = slog.LevelDebug
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)

	slog.SetDefault(logger)
}
