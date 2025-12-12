// Package logger provides structured logging configuration for the forecaster.
//
// It creates slog.Logger instances configured according to the forecaster's Config,
// supporting both text and JSON output formats, and configurable log levels
// (debug, info, warn, error).
//
// The logger uses Go's standard library slog package for structured logging,
// ensuring consistent log output across the forecaster service. All logs are
// written to stdout for container-friendly log collection.
package logger

import (
	"log/slog"
	"os"

	"github.com/HatiCode/kedastral/cmd/forecaster/config"
)

func New(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
