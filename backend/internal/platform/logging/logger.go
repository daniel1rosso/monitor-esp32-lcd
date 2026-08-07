// Package logging configures structured application logging.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

func New() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("DESK_LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})).With("service", "desk-monitor-backend")
}
