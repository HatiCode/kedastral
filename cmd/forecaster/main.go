// Package main implements the Kedastral forecaster service.
// The forecaster collects metrics, predicts future workload, calculates desired replicas,
// and serves forecast snapshots via HTTP API.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HatiCode/kedastral/cmd/forecaster/config"
	"github.com/HatiCode/kedastral/cmd/forecaster/logger"
	"github.com/HatiCode/kedastral/cmd/forecaster/router"
	"github.com/HatiCode/kedastral/pkg/adapters"
	"github.com/HatiCode/kedastral/pkg/capacity"
	"github.com/HatiCode/kedastral/pkg/features"
	"github.com/HatiCode/kedastral/pkg/httpx"
	"github.com/HatiCode/kedastral/pkg/models"
	"github.com/HatiCode/kedastral/pkg/storage"
)

func main() {
	cfg := config.ParseFlags()

	logger := logger.New(cfg)
	slog.SetDefault(logger)

	logger.Info("starting kedastral forecaster",
		"version", "v0.1.0",
		"workload", cfg.Workload,
		"metric", cfg.Metric,
	)

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

	staleAfter := 2 * cfg.Interval // Snapshot is stale if older than 2x the interval
	mux := router.SetupRoutes(store, staleAfter, logger)
	httpServer := httpx.NewServer(cfg.Listen, mux, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := f.Run(ctx, cfg.Interval); err != nil && err != context.Canceled {
			logger.Error("forecast loop failed", "error", err)
		}
	}()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- httpServer.Start()
	}()

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

	logger.Info("shutting down")
	cancel()

	if err := httpServer.Stop(10 * time.Second); err != nil {
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
