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
