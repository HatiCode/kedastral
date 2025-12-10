// Package forecaster provides the core forecast loop orchestration.
package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/HatiCode/kedastral/pkg/httpx"
	"github.com/HatiCode/kedastral/pkg/storage"
)

// SetupRoutes configures HTTP endpoints for the forecaster.
func SetupRoutes(store storage.Store, staleAfter time.Duration, logger *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.Handle("/healthz", httpx.HealthHandler())

	// Forecast snapshot endpoint
	mux.HandleFunc("/forecast/current", handleGetSnapshot(store, staleAfter, logger))

	return mux
}

// handleGetSnapshot returns a handler for GET /forecast/current?workload=<name>.
func handleGetSnapshot(store storage.Store, staleAfter time.Duration, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workload := r.URL.Query().Get("workload")
		if workload == "" {
			httpx.WriteErrorMessage(w, http.StatusBadRequest, "workload parameter required")
			return
		}

		snapshot, found, err := store.GetLatest(workload)
		if err != nil {
			logger.Error("failed to get snapshot", "workload", workload, "error", err)
			httpx.WriteErrorMessage(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if !found {
			httpx.WriteErrorMessage(w, http.StatusNotFound, fmt.Sprintf("snapshot not found for workload %q", workload))
			return
		}

		// Check if stale per SPEC.md §3.1
		if time.Since(snapshot.GeneratedAt) > staleAfter {
			w.Header().Set("X-Kedastral-Stale", "true")
		}

		// Convert to API response format
		resp := map[string]any{
			"workload":        snapshot.Workload,
			"metric":          snapshot.Metric,
			"generatedAt":     snapshot.GeneratedAt.Format(time.RFC3339),
			"stepSeconds":     snapshot.StepSeconds,
			"horizonSeconds":  snapshot.HorizonSeconds,
			"values":          snapshot.Values,
			"desiredReplicas": snapshot.DesiredReplicas,
		}

		httpx.WriteJSON(w, http.StatusOK, resp)
	}
}
