# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1] - 2025-12-12

### Added

- **Redis Storage Backend**: Added Redis as an optional storage backend for forecast snapshots, enabling multi-instance deployments and persistent storage
  - New `pkg/storage/redis.go` implementing the Store interface with Redis
  - Comprehensive test suite with 14 tests using testcontainers for integration testing
  - Configuration flags: `--storage`, `--redis-addr`, `--redis-password`, `--redis-db`, `--redis-ttl`
  - TTL-based expiration, connection pooling, and automatic health checks
  - Idempotent `Close()` implementation for graceful shutdown
- Redis deployment example in `examples/deployment-redis.yaml` showing HA setup with 2 forecaster replicas
- Storage backend factory in `cmd/forecaster/store` package with fail-fast initialization
- Package-level documentation for all forecaster and scaler packages (godoc/pkg.go.dev compatible)
- Comprehensive function headers following Go documentation conventions

### Changed

- Forecaster now uses pointer type for `capacity.Policy` for consistency
- Storage initialization moved to dedicated factory package
- Updated README.md with storage backends comparison table and usage examples
- Improved error handling and logging throughout storage initialization

### Fixed

- Fixed store cleanup to prevent premature connection closing
- Fixed type consistency between Forecaster struct and constructor

### Dependencies

- Added `github.com/redis/go-redis/v9` v9.7.0
- Added `github.com/testcontainers/testcontainers-go/modules/redis` v0.40.0
- Bumped `golang.org/x/crypto` from 0.43.0 to 0.45.0

### Documentation

- Added detailed package documentation for `cmd/forecaster/store`
- Added function headers for all exported functions in forecaster and scaler packages
- Updated README with storage backend section
- Added Redis deployment example with annotations

## [0.1.0] - 2024-12-09

### Added

- Initial release of Kedastral predictive autoscaler
- Baseline forecasting model with trend detection
- Prometheus adapter for metrics collection
- Capacity planning with configurable policies
- HTTP API for forecast snapshots
- KEDA External Scaler integration
- In-memory storage for single-instance deployments
- Comprehensive test suite
- Kubernetes deployment examples
- Documentation and getting started guide

[0.1.1]: https://github.com/HatiCode/kedastral/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/HatiCode/kedastral/releases/tag/v0.1.0
