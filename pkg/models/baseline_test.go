package models

import (
	"context"
	"testing"
)

func TestBaselineModel_Name(t *testing.T) {
	model := NewBaselineModel("test_metric", 60, 1800)
	if got := model.Name(); got != "baseline" {
		t.Errorf("Name() = %q, want %q", got, "baseline")
	}
}

func TestBaselineModel_Predict_Basic(t *testing.T) {
	tests := []struct {
		name        string
		metric      string
		stepSec     int
		horizon     int
		features    FeatureFrame
		wantLen     int
		wantErr     bool
		checkValues func(t *testing.T, values []float64)
	}{
		{
			name:     "simple constant series",
			metric:   "http_rps",
			stepSec:  60,
			horizon:  300, // 5 minutes
			features: makeFeatureFrame([]float64{100, 100, 100, 100, 100}),
			wantLen:  5, // 300/60 = 5 steps
			wantErr:  false,
			checkValues: func(t *testing.T, values []float64) {
				// All predictions should be close to 100
				for i, v := range values {
					if v < 90 || v > 110 {
						t.Errorf("value[%d] = %.2f, want ~100", i, v)
					}
				}
			},
		},
		{
			name:     "empty features",
			metric:   "http_rps",
			stepSec:  60,
			horizon:  300,
			features: FeatureFrame{Rows: []map[string]float64{}},
			wantLen:  0,
			wantErr:  true,
		},
		{
			name:    "features without value field",
			metric:  "http_rps",
			stepSec: 60,
			horizon: 300,
			features: FeatureFrame{
				Rows: []map[string]float64{
					{"timestamp": 1000},
					{"timestamp": 2000},
				},
			},
			wantLen: 0,
			wantErr: true,
		},
		{
			name:     "single point",
			metric:   "http_rps",
			stepSec:  60,
			horizon:  180,
			features: makeFeatureFrame([]float64{150}),
			wantLen:  3, // 180/60 = 3
			wantErr:  false,
			checkValues: func(t *testing.T, values []float64) {
				// Forecast should be constant at ~150
				for i, v := range values {
					if v < 140 || v > 160 {
						t.Errorf("value[%d] = %.2f, want ~150", i, v)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewBaselineModel(tt.metric, tt.stepSec, tt.horizon)
			forecast, err := model.Predict(context.Background(), tt.features)

			if (err != nil) != tt.wantErr {
				t.Errorf("Predict() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error, test passed
			}

			if len(forecast.Values) != tt.wantLen {
				t.Errorf("len(Values) = %d, want %d", len(forecast.Values), tt.wantLen)
			}

			if forecast.Metric != tt.metric {
				t.Errorf("Metric = %q, want %q", forecast.Metric, tt.metric)
			}

			if forecast.StepSec != tt.stepSec {
				t.Errorf("StepSec = %d, want %d", forecast.StepSec, tt.stepSec)
			}

			if forecast.Horizon != tt.horizon {
				t.Errorf("Horizon = %d, want %d", forecast.Horizon, tt.horizon)
			}

			if tt.checkValues != nil {
				tt.checkValues(t, forecast.Values)
			}
		})
	}
}

// TestBaselineModel_Predict_AcceptanceTest implements SPEC.md §11.2:
// GIVEN: Monotonically increasing series 100..200 over 30m
// EXPECT: Predictions must be non-decreasing and within [last, last*1.5]
func TestBaselineModel_Predict_AcceptanceTest(t *testing.T) {
	// Create monotonically increasing series: 100, 105, 110, ..., 200
	// That's 21 points (100 to 200 in steps of 5)
	values := make([]float64, 21)
	for i := range values {
		values[i] = 100 + float64(i*5)
	}

	model := NewBaselineModel("http_rps", 60, 1800) // 30m horizon
	features := makeFeatureFrame(values)

	forecast, err := model.Predict(context.Background(), features)
	if err != nil {
		t.Fatalf("Predict() error = %v", err)
	}

	lastInput := values[len(values)-1] // 200

	// Check all predictions are within [last, last*1.5] = [200, 300]
	for i, v := range forecast.Values {
		if v < lastInput {
			t.Errorf("value[%d] = %.2f < %.2f (last input), want >= last", i, v, lastInput)
		}
		if v > lastInput*1.5 {
			t.Errorf("value[%d] = %.2f > %.2f (last*1.5), want <= last*1.5", i, v, lastInput*1.5)
		}
	}

	// Check predictions are non-decreasing
	for i := 1; i < len(forecast.Values); i++ {
		if forecast.Values[i] < forecast.Values[i-1] {
			t.Errorf("values not non-decreasing: value[%d]=%.2f < value[%d]=%.2f",
				i, forecast.Values[i], i-1, forecast.Values[i-1])
		}
	}
}

func TestBaselineModel_Predict_NonNegative(t *testing.T) {
	// Test that predictions are always non-negative, even with edge cases
	tests := []struct {
		name   string
		values []float64
	}{
		{
			name:   "all positive",
			values: []float64{10, 20, 30, 40, 50},
		},
		{
			name:   "with zeros",
			values: []float64{0, 0, 10, 20, 30},
		},
		{
			name:   "very small values",
			values: []float64{0.1, 0.2, 0.3, 0.4, 0.5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewBaselineModel("test", 60, 600)
			features := makeFeatureFrame(tt.values)

			forecast, err := model.Predict(context.Background(), features)
			if err != nil {
				t.Fatalf("Predict() error = %v", err)
			}

			for i, v := range forecast.Values {
				if v < 0 {
					t.Errorf("value[%d] = %.2f is negative, want non-negative", i, v)
				}
			}
		})
	}
}

func TestBaselineModel_Train_Seasonality(t *testing.T) {
	model := NewBaselineModel("http_rps", 60, 1800)

	// Create training data with clear hourly patterns
	// Hour 9: high traffic (~200), Hour 14: moderate (~150), Hour 22: low (~100)
	history := FeatureFrame{
		Rows: []map[string]float64{
			{"value": 200, "hour": 9},
			{"value": 210, "hour": 9},
			{"value": 190, "hour": 9},
			{"value": 150, "hour": 14},
			{"value": 160, "hour": 14},
			{"value": 140, "hour": 14},
			{"value": 100, "hour": 22},
			{"value": 110, "hour": 22},
			{"value": 90, "hour": 22},
		},
	}

	err := model.Train(context.Background(), history)
	if err != nil {
		t.Fatalf("Train() error = %v", err)
	}

	// Verify seasonality was learned
	if len(model.seasonality) == 0 {
		t.Error("expected seasonality to be populated after training")
	}

	// Check specific hours
	if mean9, ok := model.seasonality[9]; ok {
		want := 200.0 // (200+210+190)/3
		if mean9 < 195 || mean9 > 205 {
			t.Errorf("seasonality[9] = %.2f, want ~%.2f", mean9, want)
		}
	} else {
		t.Error("expected seasonality for hour 9")
	}
}

func TestBaselineModel_Train_EmptyHistory(t *testing.T) {
	model := NewBaselineModel("http_rps", 60, 1800)

	// Training with empty history should not error
	err := model.Train(context.Background(), FeatureFrame{})
	if err != nil {
		t.Errorf("Train() with empty history error = %v, want nil", err)
	}
}

func TestComputeEMA(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		n      int
		want   float64
	}{
		{
			name:   "constant values",
			values: []float64{100, 100, 100, 100, 100},
			n:      5,
			want:   100,
		},
		{
			name:   "increasing values",
			values: []float64{10, 20, 30, 40, 50},
			n:      5,
			want:   34.0, // approximately (actual EMA calculation)
		},
		{
			name:   "fewer values than n",
			values: []float64{10, 20, 30},
			n:      5,
			want:   22.5, // approximately (actual EMA calculation)
		},
		{
			name:   "empty values",
			values: []float64{},
			n:      5,
			want:   0,
		},
		{
			name:   "single value",
			values: []float64{42},
			n:      5,
			want:   42,
		},
		{
			name:   "more values than n",
			values: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			n:      5,
			want:   8.33, // approximately, uses last 5 values
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeEMA(tt.values, tt.n)

			// Allow 1% tolerance for floating point
			tolerance := tt.want * 0.01
			if tolerance == 0 {
				tolerance = 0.01
			}

			if got < tt.want-tolerance || got > tt.want+tolerance {
				t.Errorf("computeEMA() = %.2f, want ~%.2f (±%.2f)", got, tt.want, tolerance)
			}
		})
	}
}

// makeFeatureFrame is a helper to create a FeatureFrame from a slice of values
func makeFeatureFrame(values []float64) FeatureFrame {
	rows := make([]map[string]float64, len(values))
	for i, v := range values {
		rows[i] = map[string]float64{
			"value":     v,
			"timestamp": float64(i * 60), // 1 minute intervals
		}
	}
	return FeatureFrame{Rows: rows}
}
