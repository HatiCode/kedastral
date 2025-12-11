package router

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetupRoutes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := SetupRoutes(logger)

	if mux == nil {
		t.Fatal("SetupRoutes() returned nil")
	}
}

func TestHealthEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := SetupRoutes(logger)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if body != "OK" {
		t.Errorf("body = %q, want %q", body, "OK")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := SetupRoutes(logger)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	// Metrics endpoint should return prometheus text format
	contentType := w.Header().Get("Content-Type")
	if contentType == "" {
		t.Error("Content-Type header should be set for metrics endpoint")
	}
}
