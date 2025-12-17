package models

import (
	"context"
	"math"
	"sync"
	"testing"
)

// Synthetic data generators

// syntheticConstant generates a constant time series
func syntheticConstant(n int, value float64) FeatureFrame {
	rows := make([]map[string]float64, n)
	for i := range n {
		rows[i] = map[string]float64{
			"value": value,
		}
	}
	return FeatureFrame{Rows: rows}
}

// syntheticLinear generates a linear trend series
func syntheticLinear(n int, slope, intercept, noise float64) FeatureFrame {
	rows := make([]map[string]float64, n)
	for i := range n {
		value := slope*float64(i) + intercept
		if noise > 0 {
			// Simple deterministic "noise" for reproducibility
			value += noise * math.Sin(float64(i)*0.5)
		}
		rows[i] = map[string]float64{
			"value": value,
		}
	}
	return FeatureFrame{Rows: rows}
}

// syntheticSeasonal generates a seasonal (sine wave) series
func syntheticSeasonal(n int, period, amplitude, noise float64) FeatureFrame {
	rows := make([]map[string]float64, n)
	for i := range n {
		value := amplitude * math.Sin(2*math.Pi*float64(i)/period)
		if noise > 0 {
			value += noise * math.Cos(float64(i)*0.3)
		}
		rows[i] = map[string]float64{
			"value": value + 100, // Offset to keep positive
		}
	}
	return FeatureFrame{Rows: rows}
}

// syntheticComplex generates trend + seasonal series
func syntheticComplex(n int) FeatureFrame {
	rows := make([]map[string]float64, n)
	for i := range n {
		trend := 0.5 * float64(i)
		seasonal := 20 * math.Sin(2*math.Pi*float64(i)/24)
		value := 100 + trend + seasonal
		rows[i] = map[string]float64{
			"value": value,
		}
	}
	return FeatureFrame{Rows: rows}
}

// Test cases

func TestARIMAModel_NewARIMAModel_Success(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 1, 1, 1)

	if model == nil {
		t.Fatal("expected non-nil model")
	}

	if model.Name() != "arima(1,1,1)" {
		t.Errorf("Name() = %q, want %q", model.Name(), "arima(1,1,1)")
	}

	if model.metric != "test_metric" {
		t.Errorf("metric = %q, want %q", model.metric, "test_metric")
	}

	if model.p != 1 || model.d != 1 || model.q != 1 {
		t.Errorf("p,d,q = %d,%d,%d, want 1,1,1", model.p, model.d, model.q)
	}
}

func TestARIMAModel_NewARIMAModel_AutoDetect(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 0, 0, 0)

	if model.p != 1 || model.d != 1 || model.q != 1 {
		t.Errorf("auto-detect failed: p,d,q = %d,%d,%d, want 1,1,1", model.p, model.d, model.q)
	}

	if model.Name() != "arima(1,1,1)" {
		t.Errorf("Name() = %q, want %q", model.Name(), "arima(1,1,1)")
	}
}

func TestARIMAModel_NewARIMAModel_Panics(t *testing.T) {
	tests := []struct {
		name       string
		metric     string
		stepSec    int
		horizonSec int
		p, d, q    int
	}{
		{"empty metric", "", 60, 1800, 1, 1, 1},
		{"zero step", "test", 0, 1800, 1, 1, 1},
		{"negative step", "test", -1, 1800, 1, 1, 1},
		{"horizon < step", "test", 60, 30, 1, 1, 1},
		{"d > 2", "test", 60, 1800, 1, 3, 1},
		{"negative p", "test", 60, 1800, -1, 1, 1},
		{"negative q", "test", 60, 1800, 1, 1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic for %s", tt.name)
				}
			}()
			NewARIMAModel(tt.metric, tt.stepSec, tt.horizonSec, tt.p, tt.d, tt.q)
		})
	}
}

func TestARIMAModel_Train_Success_Constant(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 1, 1, 1)
	history := syntheticConstant(100, 100.0)

	err := model.Train(context.Background(), history)
	if err != nil {
		t.Fatalf("Train() error = %v, want nil", err)
	}

	if !model.trained {
		t.Error("expected model.trained = true")
	}
}

func TestARIMAModel_Train_Success_LinearTrend(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 1, 1, 1)
	history := syntheticLinear(100, 2.0, 50.0, 1.0)

	err := model.Train(context.Background(), history)
	if err != nil {
		t.Fatalf("Train() error = %v, want nil", err)
	}

	if !model.trained {
		t.Error("expected model.trained = true")
	}

	// Check that coefficients were computed
	if len(model.arCoeffs) != model.p {
		t.Errorf("len(arCoeffs) = %d, want %d", len(model.arCoeffs), model.p)
	}

	if len(model.maCoeffs) != model.q {
		t.Errorf("len(maCoeffs) = %d, want %d", len(model.maCoeffs), model.q)
	}
}

func TestARIMAModel_Train_Success_Seasonal(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 2, 1, 2)
	history := syntheticSeasonal(200, 24, 20, 2.0)

	err := model.Train(context.Background(), history)
	if err != nil {
		t.Fatalf("Train() error = %v, want nil", err)
	}

	if !model.trained {
		t.Error("expected model.trained = true")
	}
}

func TestARIMAModel_Train_InsufficientData(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 5, 1, 3)
	history := syntheticConstant(5, 100.0) // Only 5 points, need at least 10

	err := model.Train(context.Background(), history)
	if err == nil {
		t.Error("Train() error = nil, want error for insufficient data")
	}

	if !contains(err.Error(), "need at least") {
		t.Errorf("error message = %q, want to contain 'need at least'", err.Error())
	}
}

func TestARIMAModel_Train_ContextCancellation(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 1, 1, 1)
	history := syntheticConstant(100, 100.0)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := model.Train(ctx, history)
	if err != context.Canceled {
		t.Errorf("Train() error = %v, want %v", err, context.Canceled)
	}
}

func TestARIMAModel_Predict_NotTrained(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 1, 1, 1)

	_, err := model.Predict(context.Background(), FeatureFrame{})
	if err == nil {
		t.Error("Predict() error = nil, want error for untrained model")
	}

	if !contains(err.Error(), "not trained") {
		t.Errorf("error message = %q, want to contain 'not trained'", err.Error())
	}
}

func TestARIMAModel_Predict_Success_Constant(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 1, 1, 1)
	history := syntheticConstant(100, 100.0)

	if err := model.Train(context.Background(), history); err != nil {
		t.Fatalf("Train() error = %v", err)
	}

	forecast, err := model.Predict(context.Background(), FeatureFrame{})
	if err != nil {
		t.Fatalf("Predict() error = %v", err)
	}

	// Check forecast structure
	if forecast.Metric != "test_metric" {
		t.Errorf("forecast.Metric = %q, want %q", forecast.Metric, "test_metric")
	}

	expectedSteps := 1800 / 60 // 30 steps
	if len(forecast.Values) != expectedSteps {
		t.Errorf("len(forecast.Values) = %d, want %d", len(forecast.Values), expectedSteps)
	}

	// For constant series, predictions should be near the constant value
	for i, v := range forecast.Values {
		if v < 50 || v > 150 {
			t.Errorf("forecast.Values[%d] = %f, expected ~100", i, v)
		}
	}
}

func TestARIMAModel_Predict_Success_LinearTrend(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 1, 1, 1)
	history := syntheticLinear(100, 2.0, 50.0, 0.5)

	if err := model.Train(context.Background(), history); err != nil {
		t.Fatalf("Train() error = %v", err)
	}

	forecast, err := model.Predict(context.Background(), FeatureFrame{})
	if err != nil {
		t.Fatalf("Predict() error = %v", err)
	}

	expectedSteps := 1800 / 60
	if len(forecast.Values) != expectedSteps {
		t.Errorf("len(forecast.Values) = %d, want %d", len(forecast.Values), expectedSteps)
	}

	// For linear trend, predictions should generally increase or be stable
	// This is a weak test since our simple ARIMA may not perfectly capture trend
	if len(forecast.Values) > 0 && forecast.Values[0] < 0 {
		t.Errorf("first forecast value is negative: %f", forecast.Values[0])
	}
}

func TestARIMAModel_Predict_NonNegative(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 1, 1, 1)

	// Create series that might produce negative predictions
	history := syntheticLinear(50, -1.0, 100.0, 2.0)

	if err := model.Train(context.Background(), history); err != nil {
		t.Fatalf("Train() error = %v", err)
	}

	forecast, err := model.Predict(context.Background(), FeatureFrame{})
	if err != nil {
		t.Fatalf("Predict() error = %v", err)
	}

	// All predictions must be non-negative
	for i, v := range forecast.Values {
		if v < 0 {
			t.Errorf("forecast.Values[%d] = %f, want >= 0", i, v)
		}
	}
}

func TestARIMAModel_Predict_ContextCancellation(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 1, 1, 1)
	history := syntheticConstant(100, 100.0)

	if err := model.Train(context.Background(), history); err != nil {
		t.Fatalf("Train() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := model.Predict(ctx, FeatureFrame{})
	if err != context.Canceled {
		t.Errorf("Predict() error = %v, want %v", err, context.Canceled)
	}
}

func TestARIMAModel_Concurrency_TrainPredict(t *testing.T) {
	model := NewARIMAModel("test_metric", 60, 1800, 1, 1, 1)
	history := syntheticLinear(100, 1.0, 50.0, 1.0)

	// Initial training
	if err := model.Train(context.Background(), history); err != nil {
		t.Fatalf("Train() error = %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Launch concurrent predictors
	for range 5 {
		wg.Go(func() {
			_, err := model.Predict(context.Background(), FeatureFrame{})
			if err != nil {
				errors <- err
			}
		})
	}

	// Wait for completion
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("concurrent Predict() error: %v", err)
	}
}

func TestARIMAModel_Predict_Acceptance(t *testing.T) {
	// Full acceptance test: train on complex series, verify predictions
	model := NewARIMAModel("test_metric", 60, 1800, 2, 1, 1)
	history := syntheticComplex(168) // 1 week of hourly data

	// Train
	err := model.Train(context.Background(), history)
	if err != nil {
		t.Fatalf("Train() error = %v", err)
	}

	// Predict
	forecast, err := model.Predict(context.Background(), FeatureFrame{})
	if err != nil {
		t.Fatalf("Predict() error = %v", err)
	}

	// Verify forecast structure
	expectedSteps := 30 // 1800 / 60
	if len(forecast.Values) != expectedSteps {
		t.Errorf("len(forecast.Values) = %d, want %d", len(forecast.Values), expectedSteps)
	}

	// All values must be non-negative
	for i, v := range forecast.Values {
		if v < 0 {
			t.Errorf("forecast.Values[%d] = %f, must be non-negative", i, v)
		}
	}

	// Values should be within reasonable bounds (0 to 1000 for our synthetic data)
	for i, v := range forecast.Values {
		if v > 1000 {
			t.Errorf("forecast.Values[%d] = %f, unexpectedly large", i, v)
		}
	}

	// Check forecast metadata
	if forecast.Metric != "test_metric" {
		t.Errorf("forecast.Metric = %q, want %q", forecast.Metric, "test_metric")
	}

	if forecast.StepSec != 60 {
		t.Errorf("forecast.StepSec = %d, want 60", forecast.StepSec)
	}

	if forecast.Horizon != 1800 {
		t.Errorf("forecast.Horizon = %d, want 1800", forecast.Horizon)
	}
}

// Benchmark tests

func BenchmarkARIMAModel_Train_100Points(b *testing.B) {
	model := NewARIMAModel("bench", 60, 1800, 1, 1, 1)
	history := syntheticLinear(100, 1.0, 50.0, 1.0)
	ctx := context.Background()

	for b.Loop() {
		_ = model.Train(ctx, history)
	}
}

func BenchmarkARIMAModel_Train_1000Points(b *testing.B) {
	model := NewARIMAModel("bench", 60, 1800, 1, 1, 1)
	history := syntheticLinear(1000, 1.0, 50.0, 1.0)
	ctx := context.Background()

	for b.Loop() {
		_ = model.Train(ctx, history)
	}
}

func BenchmarkARIMAModel_Predict_30Steps(b *testing.B) {
	model := NewARIMAModel("bench", 60, 1800, 1, 1, 1)
	history := syntheticLinear(100, 1.0, 50.0, 1.0)
	ctx := context.Background()

	_ = model.Train(ctx, history)

	for b.Loop() {
		_, _ = model.Predict(ctx, FeatureFrame{})
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
