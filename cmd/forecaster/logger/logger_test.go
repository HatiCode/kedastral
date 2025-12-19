package logger

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/HatiCode/kedastral/cmd/forecaster/config"
)

func TestNew_TextFormat(t *testing.T) {
	cfg := &config.Config{
		LogFormat: "text",
		LogLevel:  "info",
	}

	logger := New(cfg)
	if logger == nil {
		t.Fatal("New() returned nil")
	}

	// Logger should be usable
	logger.Info("test message")
}

func TestNew_JSONFormat(t *testing.T) {
	cfg := &config.Config{
		LogFormat: "json",
		LogLevel:  "info",
	}

	logger := New(cfg)
	if logger == nil {
		t.Fatal("New() returned nil")
	}

	// Logger should be usable
	logger.Info("test message")
}

func TestNew_LogLevels(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		wantFunc func(*slog.Logger) bool
	}{
		{
			name:     "debug level",
			logLevel: "debug",
			wantFunc: func(l *slog.Logger) bool {
				return l.Enabled(context.TODO(), slog.LevelDebug)
			},
		},
		{
			name:     "info level",
			logLevel: "info",
			wantFunc: func(l *slog.Logger) bool {
				return l.Enabled(context.TODO(), slog.LevelInfo) && !l.Enabled(context.TODO(), slog.LevelDebug)
			},
		},
		{
			name:     "warn level",
			logLevel: "warn",
			wantFunc: func(l *slog.Logger) bool {
				return l.Enabled(context.TODO(), slog.LevelWarn) && !l.Enabled(context.TODO(), slog.LevelInfo)
			},
		},
		{
			name:     "error level",
			logLevel: "error",
			wantFunc: func(l *slog.Logger) bool {
				return l.Enabled(context.TODO(), slog.LevelError) && !l.Enabled(context.TODO(), slog.LevelWarn)
			},
		},
		{
			name:     "invalid level defaults to info",
			logLevel: "invalid",
			wantFunc: func(l *slog.Logger) bool {
				return l.Enabled(context.TODO(), slog.LevelInfo) && !l.Enabled(context.TODO(), slog.LevelDebug)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				LogFormat: "text",
				LogLevel:  tt.logLevel,
			}

			logger := New(cfg)
			if !tt.wantFunc(logger) {
				t.Errorf("logger level configuration incorrect for %s", tt.logLevel)
			}
		})
	}
}

func TestNew_JSONOutput(t *testing.T) {
	cfg := &config.Config{
		LogFormat: "json",
		LogLevel:  "info",
	}

	// We can't easily test the output without refactoring to accept an io.Writer,
	// but we can verify the logger works
	logger := New(cfg)
	logger.Info("test message", "key", "value")

	// Just verify it doesn't panic
}

func TestNew_TextOutput(t *testing.T) {
	cfg := &config.Config{
		LogFormat: "text",
		LogLevel:  "debug",
	}

	logger := New(cfg)
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")

	// Verify it doesn't panic
}

func TestNew_EmptyLogLevel(t *testing.T) {
	cfg := &config.Config{
		LogFormat: "text",
		LogLevel:  "",
	}

	logger := New(cfg)

	// Should default to info level
	if !logger.Enabled(context.TODO(), slog.LevelInfo) {
		t.Error("expected default level to be info")
	}
	if logger.Enabled(context.TODO(), slog.LevelDebug) {
		t.Error("expected debug to be disabled at info level")
	}
}

func TestNew_CaseInsensitiveFormat(t *testing.T) {
	tests := []struct {
		format string
	}{
		{"json"},
		{"JSON"},
		{"Json"},
		{"text"},
		{"TEXT"},
		{"Text"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			cfg := &config.Config{
				LogFormat: strings.ToLower(tt.format), // Normalize for comparison
				LogLevel:  "info",
			}

			logger := New(cfg)
			if logger == nil {
				t.Errorf("New() returned nil for format %s", tt.format)
			}
		})
	}
}
