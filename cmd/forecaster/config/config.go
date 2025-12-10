// Package config implements the Kedastral forecaster config.
package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// Config holds all forecaster configuration.
type Config struct {
	Listen                string
	Workload              string
	Metric                string
	Horizon               time.Duration
	Step                  time.Duration
	LeadTime              time.Duration
	TargetPerPod          float64
	Headroom              float64
	MinReplicas           int
	MaxReplicas           int
	UpMaxFactorPerStep    float64
	DownMaxPercentPerStep int
	PromURL               string
	PromQuery             string
	Interval              time.Duration
	Window                time.Duration
	LogFormat             string
	LogLevel              string
}

// ParseFlags parses command-line flags and environment variables into a Config.
// Exits with status 1 if required flags (workload, metric, prom-query) are missing.
// Environment variables are used as fallbacks when flags are not provided.
func ParseFlags() *Config {
	cfg := &Config{}

	// Server
	flag.StringVar(&cfg.Listen, "listen", getEnv("LISTEN", ":8081"), "HTTP listen address")

	// Workload
	flag.StringVar(&cfg.Workload, "workload", getEnv("WORKLOAD", ""), "Workload name (required)")
	flag.StringVar(&cfg.Metric, "metric", getEnv("METRIC", ""), "Metric name (required)")

	// Forecast parameters
	flag.DurationVar(&cfg.Horizon, "horizon", getEnvDuration("HORIZON", 30*time.Minute), "Forecast horizon")
	flag.DurationVar(&cfg.Step, "step", getEnvDuration("STEP", 1*time.Minute), "Forecast step size")
	flag.DurationVar(&cfg.LeadTime, "lead-time", getEnvDuration("LEAD_TIME", 5*time.Minute), "Lead time for pre-scaling")

	// Capacity policy
	flag.Float64Var(&cfg.TargetPerPod, "target-per-pod", getEnvFloat("TARGET_PER_POD", 100.0), "Target metric value per pod")
	flag.Float64Var(&cfg.Headroom, "headroom", getEnvFloat("HEADROOM", 1.2), "Headroom multiplier")
	flag.IntVar(&cfg.MinReplicas, "min", getEnvInt("MIN_REPLICAS", 1), "Minimum replicas")
	flag.IntVar(&cfg.MaxReplicas, "max", getEnvInt("MAX_REPLICAS", 100), "Maximum replicas")
	flag.Float64Var(&cfg.UpMaxFactorPerStep, "up-max-factor", getEnvFloat("UP_MAX_FACTOR", 2.0), "Max scale-up factor per step")
	flag.IntVar(&cfg.DownMaxPercentPerStep, "down-max-percent", getEnvInt("DOWN_MAX_PERCENT", 50), "Max scale-down percent per step")

	// Prometheus
	flag.StringVar(&cfg.PromURL, "prom-url", getEnv("PROM_URL", "http://localhost:9090"), "Prometheus URL")
	flag.StringVar(&cfg.PromQuery, "prom-query", getEnv("PROM_QUERY", ""), "Prometheus query (required)")

	// Timing
	flag.DurationVar(&cfg.Interval, "interval", getEnvDuration("INTERVAL", 30*time.Second), "Forecast interval")
	flag.DurationVar(&cfg.Window, "window", getEnvDuration("WINDOW", 30*time.Minute), "Historical window")

	// Logging
	flag.StringVar(&cfg.LogFormat, "log-format", getEnv("LOG_FORMAT", "text"), "Log format: text or json")
	flag.StringVar(&cfg.LogLevel, "log-level", getEnv("LOG_LEVEL", "info"), "Log level: debug, info, warn, error")

	flag.Parse()

	if cfg.Workload == "" {
		fmt.Fprintln(os.Stderr, "Error: --workload is required")
		os.Exit(1)
	}
	if cfg.Metric == "" {
		fmt.Fprintln(os.Stderr, "Error: --metric is required")
		os.Exit(1)
	}
	if cfg.PromQuery == "" {
		fmt.Fprintln(os.Stderr, "Error: --prom-query is required")
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
		var i int
		if _, err := fmt.Sscanf(value, "%d", &i); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		var f float64
		if _, err := fmt.Sscanf(value, "%f", &f); err == nil {
			return f
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
