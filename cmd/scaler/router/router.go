// Package router configures HTTP routes for the scaler's HTTP server.
//
// The scaler exposes an auxiliary HTTP server (separate from the main gRPC service)
// that provides health checks and Prometheus metrics. This package sets up the
// routes for that HTTP server.
//
// Routes configured:
//   - GET /healthz - Health check endpoint (returns 200 OK)
//   - GET /metrics - Prometheus metrics endpoint
package router

import (
	"log/slog"
	"net/http"

	"github.com/HatiCode/kedastral/pkg/httpx"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SetupRoutes configures HTTP routes for the scaler
func SetupRoutes(logger *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("/healthz", httpx.HealthHandler())

	mux.Handle("/metrics", promhttp.Handler())

	return mux
}
