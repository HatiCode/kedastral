package config

import (
	"flag"
	"os"
	"testing"
	"time"
)

func TestConfig_Defaults(t *testing.T) {
	// Reset flag package for testing
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	os.Args = []string{"cmd"}

	cfg := ParseFlags()

	// Check defaults
	if cfg.Listen != ":50051" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, ":50051")
	}
	if cfg.ForecasterURL != "http://localhost:8081" {
		t.Errorf("ForecasterURL = %q, want %q", cfg.ForecasterURL, "http://localhost:8081")
	}
	if cfg.LeadTime != 5*time.Minute {
		t.Errorf("LeadTime = %v, want 5m", cfg.LeadTime)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "text")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestConfig_CustomValues(t *testing.T) {
	// Reset flag package for testing
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	os.Args = []string{
		"cmd",
		"-listen=:9090",
		"-forecaster-url=http://forecaster:8081",
		"-lead-time=10m",
		"-log-format=json",
		"-log-level=debug",
	}

	cfg := ParseFlags()

	if cfg.Listen != ":9090" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, ":9090")
	}
	if cfg.ForecasterURL != "http://forecaster:8081" {
		t.Errorf("ForecasterURL = %q, want %q", cfg.ForecasterURL, "http://forecaster:8081")
	}
	if cfg.LeadTime != 10*time.Minute {
		t.Errorf("LeadTime = %v, want 10m", cfg.LeadTime)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{
			name:         "environment variable set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "from-env",
			want:         "from-env",
		},
		{
			name:         "environment variable not set",
			key:          "NONEXISTENT_VAR",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		want         int
	}{
		{
			name:         "valid integer",
			key:          "TEST_INT",
			defaultValue: 10,
			envValue:     "42",
			want:         42,
		},
		{
			name:         "invalid integer",
			key:          "TEST_INT",
			defaultValue: 10,
			envValue:     "not-a-number",
			want:         10,
		},
		{
			name:         "not set",
			key:          "NONEXISTENT_INT",
			defaultValue: 99,
			envValue:     "",
			want:         99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnvInt(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetEnvDuration(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue time.Duration
		envValue     string
		want         time.Duration
	}{
		{
			name:         "valid duration",
			key:          "TEST_DURATION",
			defaultValue: 1 * time.Minute,
			envValue:     "5m",
			want:         5 * time.Minute,
		},
		{
			name:         "invalid duration",
			key:          "TEST_DURATION",
			defaultValue: 30 * time.Second,
			envValue:     "not-a-duration",
			want:         30 * time.Second,
		},
		{
			name:         "not set",
			key:          "NONEXISTENT_DURATION",
			defaultValue: 10 * time.Second,
			envValue:     "",
			want:         10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnvDuration(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}
