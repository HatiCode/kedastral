// Package main implements the Kedastral forecaster service.
// The forecaster collects metrics, predicts future workload, calculates desired replicas,
// and serves forecast snapshots via HTTP API.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HatiCode/kedastral/cmd/forecaster/server"
	"github.com/HatiCode/kedastral/pkg/adapters"
	"github.com/HatiCode/kedastral/pkg/capacity"
	"github.com/HatiCode/kedastral/pkg/features"
	"github.com/HatiCode/kedastral/pkg/httpx"
	"github.com/HatiCode/kedastral/pkg/models"
	"github.com/HatiCode/kedastral/pkg/storage"
)

// Config holds all forecaster configuration.
type Config struct {
	// Server
	Listen string

	// Workload
	Workload string
	Metric   string

	// Forecast parameters
	Horizon  time.Duration
	Step     time.Duration
	LeadTime time.Duration

	// Capacity policy
	TargetPerPod          float64
	Headroom              float64
	MinReplicas           int
	MaxReplicas           int
	UpMaxFactorPerStep    float64
	DownMaxPercentPerStep int

	// Prometheus adapter
	PromURL   string
	PromQuery string

	// Timing
	Interval time.Duration
	Window   time.Duration

	// Logging
	LogFormat string // "text" or "json"
	LogLevel  string // "debug", "info", "warn", "error"
}

func main() {
	cfg := parseFlags()

	// Set up logging
	logger := setupLogger(cfg)
	slog.SetDefault(logger)

	logger.Info("starting kedastral forecaster",
		"version", "v0.1.0",
		"workload", cfg.Workload,
		"metric", cfg.Metric,
	)

	// Initialize components
	adapter := &adapters.PrometheusAdapter{
		ServerURL:   cfg.PromURL,
		Query:       cfg.PromQuery,
		StepSeconds: int(cfg.Step.Seconds()),
	}
	model := models.NewBaselineModel(
		cfg.Metric,
		int(cfg.Step.Seconds()),
		int(cfg.Horizon.Seconds()),
	)
	builder := features.NewBuilder()
	store := storage.NewMemoryStore()

	policy := capacity.Policy{
		TargetPerPod:          cfg.TargetPerPod,
		Headroom:              cfg.Headroom,
		LeadTimeSeconds:       int(cfg.LeadTime.Seconds()),
		MinReplicas:           cfg.MinReplicas,
		MaxReplicas:           cfg.MaxReplicas,
		UpMaxFactorPerStep:    cfg.UpMaxFactorPerStep,
		DownMaxPercentPerStep: cfg.DownMaxPercentPerStep,
	}

	// Create forecaster
	f := New(
		cfg.Workload,
		adapter,
		model,
		builder,
		store,
		policy,
		cfg.Horizon,
		cfg.Step,
		cfg.Window,
		logger,
	)

	// Create HTTP server
	staleAfter := 2 * cfg.Interval // Snapshot is stale if older than 2x the interval
	mux := server.SetupRoutes(store, staleAfter, logger)
	httpServer := httpx.NewServer(cfg.Listen, mux, logger)

	// Run forecaster
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start forecast loop
	go func() {
		if err := f.Run(ctx, cfg.Interval); err != nil && err != context.Canceled {
			logger.Error("forecast loop failed", "error", err)
		}
	}()

	// Start HTTP server
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- httpServer.Start()
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigCh:
		logger.Info("received shutdown signal", "signal", sig)
	case err := <-serverErr:
		if err != nil {
			logger.Error("server failed", "error", err)
		}
	}

	// Graceful shutdown
	logger.Info("shutting down")
	cancel() // Stop forecast loop

	if err := httpServer.Stop(10 * time.Second); err != nil {
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}

func parseFlags() Config {
	cfg := Config{}

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

	// Validate required fields
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

func setupLogger(cfg Config) *slog.Logger {
	// Parse log level
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

// Helper functions for env parsing
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
