# Changelog

All notable changes to Kedastral will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### v0.2 Roadmap

Kedastral v0.2 will be delivered through three incremental releases:

#### [0.2.0] - Planned
**Target**: Production-ready Helm chart
- **Added**
  - Production-ready Helm chart for single-workload deployments
  - Multi-environment support (dev, prod values)
  - Conditional Redis deployment
  - ServiceMonitor for Prometheus Operator integration
  - Comprehensive chart documentation and examples
  - KEDA ScaledObject integration templates
  - Automated chart testing in CI/CD
  - Post-install NOTES with setup instructions

#### [0.1.5] - Planned
**Target**: ARIMA forecasting model
- **Added**
  - ARIMA (AutoRegressive Integrated Moving Average) forecasting model
  - Pure Go implementation (no external ML libraries)
  - Configurable ARIMA(p,d,q) parameters with auto-detection
  - Model selection via `--model=arima` flag
  - Training from historical data with periodic retraining
  - Better accuracy for workloads with trends and seasonality
  - Comprehensive test suite with synthetic workload generators
  - Model comparison documentation (baseline vs ARIMA)
  - Parameter tuning guide

#### [0.1.1] - Planned
**Target**: Optional Redis storage backend
- **Added**
  - Redis storage backend implementing Store interface
  - Multi-instance forecaster support (horizontal scaling)
  - Persistence of forecast snapshots beyond process lifetime
  - Configurable TTL for snapshot expiration
  - Storage selection via `--storage=redis` flag
  - Redis connection health checks and graceful degradation
  - Comprehensive integration tests with testcontainers
  - HA deployment examples and documentation
  - Migration guide for memory-to-Redis transition

---

## [0.1.0] - 2025-12-11

### Added
- Initial release of Kedastral predictive autoscaling framework
- Forecaster component with Prometheus integration
  - Statistical baseline forecasting model
  - Configurable capacity planning policies
  - In-memory forecast storage
  - HTTP API for forecast retrieval (`/forecast/current`)
  - Prometheus metrics endpoint (`/metrics`)
  - Health check endpoint (`/healthz`)
- Scaler component implementing KEDA External Scaler protocol
  - gRPC interface for KEDA integration
  - Forecast consumption with configurable lead time
  - HTTP metrics endpoint (`/metrics`)
  - Health check endpoint (`/healthz`)
- Comprehensive test suite
  - 81 unit tests covering core functionality
  - Integration tests using testcontainers
- Docker support
  - Multi-stage Dockerfiles for both components
  - Alpine-based runtime images for minimal footprint
- Kubernetes deployment examples
  - Complete deployment manifests
  - KEDA ScaledObject configuration examples
  - Detailed usage guide with troubleshooting
- Build automation
  - Makefile with multiple targets
  - Version injection via ldflags
  - CI/CD workflow for automated releases
- Documentation
  - Comprehensive README with architecture overview
  - Detailed examples and configuration reference
  - API documentation

### Technical Details
- Language: Go 1.25+
- Dependencies: Prometheus client, gRPC, testcontainers-go
- License: Apache-2.0
- Container registry: GitHub Container Registry (ghcr.io)

[0.1.0]: https://github.com/HatiCode/kedastral/releases/tag/v0.1.0
