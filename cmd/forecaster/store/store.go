// Package store provides storage backend initialization for the forecaster.
//
// This package acts as a factory for creating storage.Store implementations
// based on the forecaster configuration. It supports two storage backends:
//
//   - Memory: In-memory storage (default) - suitable for single-instance
//     deployments and development. Data is lost on restart.
//
//   - Redis: Distributed Redis storage - required for multi-instance
//     deployments and production HA setups. Provides persistence and
//     shared state across multiple forecaster instances.
//
// The store factory performs fail-fast initialization, validating storage
// connectivity during startup and exiting immediately if the backend is
// unavailable. This ensures the forecaster never runs with a broken storage
// configuration.
//
// Usage:
//
//	store := store.New(cfg, logger)
//	defer func() {
//	    if closer, ok := store.(interface{ Close() error }); ok {
//	        closer.Close()
//	    }
//	}()
package store

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/HatiCode/kedastral/cmd/forecaster/config"
	"github.com/HatiCode/kedastral/pkg/storage"
)

// New creates and initializes a storage backend based on the provided configuration.
//
// The function performs fail-fast validation, establishing and verifying the
// storage connection during initialization. If the backend is unavailable or
// misconfigured, the process exits immediately with os.Exit(1).
//
// Supported storage backends:
//
//   - "memory": In-memory storage with optional TTL-based expiration.
//     No external dependencies. Data lost on restart.
//
//   - "redis": Redis-backed storage with connection pooling and health checks.
//     Requires Redis server. Connection parameters from cfg.Redis*.
//
// Parameters:
//
//   - cfg: Forecaster configuration containing storage backend selection
//     and connection parameters (RedisAddr, RedisPassword, RedisDB, RedisTTL)
//
//   - logger: Structured logger for initialization events and errors
//
// Returns:
//
//	storage.Store implementation ready for use. Never returns nil.
//	Calls os.Exit(1) on initialization failure.
//
// Example:
//
//	store := store.New(cfg, logger)
//	snapshot, found, err := store.GetLatest("my-workload")
func New(cfg *config.Config, logger *slog.Logger) storage.Store {
	switch cfg.Storage {
	case "redis":
		logger.Info("initializing redis storage",
			"addr", cfg.RedisAddr,
			"db", cfg.RedisDB,
			"ttl", cfg.RedisTTL,
		)
		redisStore, err := storage.NewRedisStore(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.RedisTTL)
		if err != nil {
			logger.Error("failed to connect to redis", "error", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := redisStore.Ping(ctx); err != nil {
			logger.Error("redis health check failed", "error", err)
			os.Exit(1)
		}
		logger.Info("redis storage initialized successfully")

		return redisStore
	case "memory":
		logger.Info("initializing in-memory storage")
		return storage.NewMemoryStore()

	default:
		logger.Error("invalid storage type", "storage", cfg.Storage)
		os.Exit(1)
	}

	return nil
}
