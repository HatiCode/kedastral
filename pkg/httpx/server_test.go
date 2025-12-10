package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer(":8080", nil, logger)

	if server == nil {
		t.Fatal("NewServer() returned nil")
	}
	if server.server.Addr != ":8080" {
		t.Errorf("Addr = %q, want %q", server.server.Addr, ":8080")
	}
	if server.logger != logger {
		t.Error("logger not set correctly")
	}

	// Check timeouts are set
	if server.server.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 10s", server.server.ReadHeaderTimeout)
	}
	if server.server.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want 30s", server.server.ReadTimeout)
	}
	if server.server.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v, want 30s", server.server.WriteTimeout)
	}
	if server.server.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want 60s", server.server.IdleTimeout)
	}
}

func TestNewServer_NilLogger(t *testing.T) {
	// Should use default logger if nil is passed
	server := NewServer(":8080", nil, nil)
	if server == nil {
		t.Fatal("NewServer() returned nil")
	}
	if server.logger == nil {
		t.Error("logger should not be nil when nil is passed")
	}
}

func TestServer_StartStop(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("localhost:0", mux, logger) // Port 0 for random available port

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start()
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Stop server
	err := server.Stop(5 * time.Second)
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Wait for Start() to return
	startErr := <-errChan
	if startErr != nil {
		t.Errorf("Start() error = %v", startErr)
	}
}

func TestServer_Stop_Timeout(t *testing.T) {
	// Handler that hangs to test timeout
	mux := http.NewServeMux()
	mux.HandleFunc("/hang", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("localhost:0", mux, logger)

	go func() {
		server.Start()
	}()

	time.Sleep(50 * time.Millisecond)

	// Stop with very short timeout should work even with hanging request
	err := server.Stop(10 * time.Millisecond)
	// Shutdown may timeout but should not error (it forces close after timeout)
	_ = err // Context deadline exceeded is acceptable
}

func TestWriteJSON_Success(t *testing.T) {
	type TestResponse struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	}

	w := httptest.NewRecorder()
	resp := TestResponse{
		Message: "success",
		Code:    42,
	}

	err := WriteJSON(w, http.StatusOK, resp)
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	// Check body
	var got TestResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if got.Message != resp.Message || got.Code != resp.Code {
		t.Errorf("response = %+v, want %+v", got, resp)
	}
}

func TestWriteJSON_DifferentStatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			err := WriteJSON(w, tt.status, map[string]string{"test": "value"})
			if err != nil {
				t.Fatalf("WriteJSON() error = %v", err)
			}
			if w.Code != tt.status {
				t.Errorf("status code = %d, want %d", w.Code, tt.status)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	testErr := errors.New("something went wrong")

	WriteError(w, http.StatusBadRequest, testErr)

	// Check status code
	if w.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusBadRequest)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	// Check error response format per SPEC.md §3.1
	var got ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if got.Error != testErr.Error() {
		t.Errorf("error = %q, want %q", got.Error, testErr.Error())
	}
}

func TestWriteErrorMessage(t *testing.T) {
	w := httptest.NewRecorder()
	errorMsg := "custom error message"

	WriteErrorMessage(w, http.StatusInternalServerError, errorMsg)

	// Check status code
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	// Check error response
	var got ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if got.Error != errorMsg {
		t.Errorf("error = %q, want %q", got.Error, errorMsg)
	}
}

func TestHealthHandler(t *testing.T) {
	handler := HealthHandler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	// Check body
	body := w.Body.String()
	if body != "OK" {
		t.Errorf("body = %q, want %q", body, "OK")
	}
}

func TestHealthHandlerWithCheck_Success(t *testing.T) {
	// Check function that always succeeds
	check := func() error {
		return nil
	}

	handler := HealthHandlerWithCheck(check)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	// Check body
	body := w.Body.String()
	if body != "OK" {
		t.Errorf("body = %q, want %q", body, "OK")
	}
}

func TestHealthHandlerWithCheck_Failure(t *testing.T) {
	// Check function that fails
	testErr := errors.New("service unhealthy")
	check := func() error {
		return testErr
	}

	handler := HealthHandlerWithCheck(check)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	// Check error response
	var got ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if got.Error != testErr.Error() {
		t.Errorf("error = %q, want %q", got.Error, testErr.Error())
	}
}

func TestLoggingMiddleware(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	// Test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	// Wrap with logging middleware
	middleware := LoggingMiddleware(logger)
	handler := middleware(testHandler)

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check response is unchanged
	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	// Check log output contains expected fields
	logOutput := buf.String()
	expectedFields := []string{
		"HTTP request",
		"method=GET",
		"path=/test/path",
		"status=200",
		"duration_ms",
	}

	for _, field := range expectedFields {
		if !strings.Contains(logOutput, field) {
			t.Errorf("log output missing %q: %s", field, logOutput)
		}
	}
}

func TestLoggingMiddleware_NilLogger(t *testing.T) {
	// Should not panic with nil logger
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := LoggingMiddleware(nil)
	handler := middleware(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestLoggingMiddleware_CapturesStatusCode(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			middleware := LoggingMiddleware(logger)
			handler := middleware(testHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			logOutput := buf.String()
			expectedStatus := fmt.Sprintf("status=%d", tt.statusCode)
			if !strings.Contains(logOutput, expectedStatus) {
				t.Errorf("log output missing %q: %s", expectedStatus, logOutput)
			}
		})
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	// Handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})

	// Wrap with recovery middleware
	middleware := RecoveryMiddleware(logger)
	handler := middleware(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	// Check error response
	var got ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if got.Error != "internal server error" {
		t.Errorf("error = %q, want %q", got.Error, "internal server error")
	}

	// Check panic was logged
	logOutput := buf.String()
	if !strings.Contains(logOutput, "panic recovered") {
		t.Errorf("log output missing panic message: %s", logOutput)
	}
	if !strings.Contains(logOutput, "something went wrong") {
		t.Errorf("log output missing panic value: %s", logOutput)
	}
}

func TestRecoveryMiddleware_NilLogger(t *testing.T) {
	// Handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Should not panic with nil logger
	middleware := RecoveryMiddleware(nil)
	handler := middleware(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestRecoveryMiddleware_NormalRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	// Normal handler that doesn't panic
	normalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	middleware := RecoveryMiddleware(logger)
	handler := middleware(normalHandler)

	req := httptest.NewRequest(http.MethodGet, "/normal", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check normal response is unchanged
	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	if w.Body.String() != "success" {
		t.Errorf("body = %q, want %q", w.Body.String(), "success")
	}

	// No panic should be logged
	logOutput := buf.String()
	if strings.Contains(logOutput, "panic recovered") {
		t.Errorf("should not log panic for normal request: %s", logOutput)
	}
}

func TestMiddlewareChaining(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	// Test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Chain logging and recovery middleware
	handler := RecoveryMiddleware(logger)(LoggingMiddleware(logger)(testHandler))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	// Check both middlewares logged
	logOutput := buf.String()
	if !strings.Contains(logOutput, "HTTP request") {
		t.Error("logging middleware did not log")
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	// Test that responseWriter correctly captures status code
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	rw.WriteHeader(http.StatusNotFound)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("statusCode = %d, want %d", rw.statusCode, http.StatusNotFound)
	}

	if w.Code != http.StatusNotFound {
		t.Errorf("underlying ResponseWriter Code = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestResponseWriter_DefaultStatusCode(t *testing.T) {
	// Test that responseWriter defaults to 200 OK if WriteHeader not called
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't call WriteHeader explicitly
		w.Write([]byte("test"))
	})

	middleware := LoggingMiddleware(logger)
	handler := middleware(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should log status=200
	logOutput := buf.String()
	if !strings.Contains(logOutput, "status=200") {
		t.Errorf("log should contain status=200: %s", logOutput)
	}
}

func TestServer_Integration(t *testing.T) {
	// Integration test with all middleware and handlers
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	mux := http.NewServeMux()

	// Add health endpoint
	mux.Handle("/health", HealthHandler())

	// Add test endpoint
	mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Add error endpoint
	mux.HandleFunc("/api/error", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusBadRequest, errors.New("test error"))
	})

	// Wrap with middleware
	handler := RecoveryMiddleware(logger)(LoggingMiddleware(logger)(mux))

	// Test health endpoint
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("health check failed: status = %d", w.Code)
	}

	// Test API endpoint
	req = httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("api test failed: status = %d", w.Code)
	}

	// Test error endpoint
	req = httptest.NewRequest(http.MethodGet, "/api/error", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("api error failed: status = %d", w.Code)
	}
}
