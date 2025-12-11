// Package config provides configuration parsing and management for the scaler.
//
// It handles both command-line flags and environment variables, with flags taking
// precedence over environment variables. The Config struct contains all runtime
// configuration needed by the scaler service.
//
// Supported configuration sources (in order of precedence):
//   1. Command-line flags
//   2. Environment variables
//   3. Default values
//
// Example usage:
//
//	cfg := config.ParseFlags()
//	// cfg now contains validated configuration
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Listen        string
	ForecasterURL string
	LeadTime      time.Duration
	LogFormat     string
	LogLevel      string
}

func ParseFlags() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.Listen, "listen", getEnv("SCALER_LISTEN", ":50051"), "gRPC listen address")
	flag.StringVar(&cfg.ForecasterURL, "forecaster-url", getEnv("FORECASTER_URL", "http://localhost:8081"), "Forecaster HTTP endpoint")
	flag.DurationVar(&cfg.LeadTime, "lead-time", getEnvDuration("LEAD_TIME", 5*time.Minute), "Lead time for forecast selection")
	flag.StringVar(&cfg.LogFormat, "log-format", getEnv("LOG_FORMAT", "text"), "Log format (text|json)")
	flag.StringVar(&cfg.LogLevel, "log-level", getEnv("LOG_LEVEL", "info"), "Log level (debug|info|warn|error)")

	flag.Parse()

	// Validation
	if cfg.ForecasterURL == "" {
		fmt.Fprintln(os.Stderr, "Error: -forecaster-url is required")
		flag.Usage()
		os.Exit(1)
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
