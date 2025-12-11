# Changelog

All notable changes to Kedastral will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
