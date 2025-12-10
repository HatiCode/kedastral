package models

import (
	"context"
	"fmt"
)

// BaselineModel implements a simple forecasting model using exponential moving averages
// and optional hour-of-day seasonality patterns.
//
// Algorithm:
//  1. Compute EMA5m and EMA30m over recent window
//  2. Base forecast = 0.7*EMA5m + 0.3*EMA30m
//  3. Optional seasonality: if sufficient hour-of-day data exists,
//     compute Mean_h and blend: yhat = 0.8*Base + 0.2*Mean_h
//  4. All values are non-negative
//
// This model is stateless and requires no training.
type BaselineModel struct {
	// metric is the name of the metric being forecast
	metric string

	// stepSec is the interval in seconds between forecast points
	stepSec int

	// horizon is the total forecast window in seconds
	horizon int

	// seasonality stores hour-of-day means if available
	// map key is hour (0-23), value is mean for that hour
	seasonality map[int]float64
}

// NewBaselineModel creates a new baseline forecasting model.
// The model uses EMA-based forecasting with optional seasonality.
func NewBaselineModel(metric string, stepSec, horizon int) *BaselineModel {
	return &BaselineModel{
		metric:      metric,
		stepSec:     stepSec,
		horizon:     horizon,
		seasonality: make(map[int]float64),
	}
}

// Name returns the model identifier.
func (m *BaselineModel) Name() string {
	return "baseline"
}

// Train extracts seasonality patterns from historical data.
// For the baseline model, this computes hour-of-day means if sufficient data exists.
// Returns nil (training is optional for baseline).
func (m *BaselineModel) Train(ctx context.Context, history FeatureFrame) error {
	if len(history.Rows) == 0 {
		return nil
	}

	hourSums := make(map[int]float64)
	hourCounts := make(map[int]int)

	for _, row := range history.Rows {
		value, hasValue := row["value"]
		hour, hasHour := row["hour"]

		if hasValue && hasHour {
			h := int(hour)
			if h >= 0 && h < 24 {
				hourSums[h] += value
				hourCounts[h]++
			}
		}
	}

	for h := range 24 {
		if count := hourCounts[h]; count >= 2 {
			m.seasonality[h] = hourSums[h] / float64(count)
		}
	}

	return nil
}

// Predict generates a forecast using EMA-based prediction with optional seasonality.
//
// The features FeatureFrame should contain recent historical values with:
//   - "value": the metric value (required)
//   - "hour": hour of day 0-23 (optional, for seasonality)
//   - "timestamp": Unix timestamp (optional, for ordering)
//
// Returns a Forecast with Values of length horizon/stepSec, all non-negative.
func (m *BaselineModel) Predict(ctx context.Context, features FeatureFrame) (Forecast, error) {
	if len(features.Rows) == 0 {
		return Forecast{}, fmt.Errorf("features cannot be empty")
	}

	values := make([]float64, 0, len(features.Rows))
	for _, row := range features.Rows {
		if v, ok := row["value"]; ok {
			values = append(values, v)
		}
	}

	if len(values) == 0 {
		return Forecast{}, fmt.Errorf("no 'value' field found in features")
	}

	ema5 := computeEMA(values, 5)
	ema30 := computeEMA(values, 30)

	baseForecast := 0.7*ema5 + 0.3*ema30

	lastValue := values[len(values)-1]
	if len(values) >= 2 {
		if lastValue > baseForecast {
			baseForecast = lastValue
		}
	}

	if baseForecast < 0 {
		baseForecast = 0
	}

	numSteps := m.horizon / m.stepSec
	if numSteps <= 0 {
		numSteps = 1
	}

	forecastValues := make([]float64, numSteps)

	currentHour := -1
	if len(features.Rows) > 0 {
		lastRow := features.Rows[len(features.Rows)-1]
		if h, ok := lastRow["hour"]; ok {
			currentHour = int(h)
		}
	}

	for i := 0; i < numSteps; i++ {
		value := baseForecast

		if currentHour >= 0 && len(m.seasonality) > 0 {
			hoursAhead := (i * m.stepSec) / 3600
			futureHour := (currentHour + hoursAhead) % 24

			if seasonalMean, ok := m.seasonality[futureHour]; ok {
				value = 0.8*baseForecast + 0.2*seasonalMean
			}
		}

		if value < 0 {
			value = 0
		}

		forecastValues[i] = value
	}

	return Forecast{
		Metric:  m.metric,
		Values:  forecastValues,
		StepSec: m.stepSec,
		Horizon: m.horizon,
	}, nil
}

// computeEMA calculates the exponential moving average over the most recent n points.
// If there are fewer than n points, uses all available points.
// Returns 0 if values is empty.
//
// EMA formula: EMA_t = α * value_t + (1-α) * EMA_{t-1}
// where α = 2 / (n + 1)
func computeEMA(values []float64, n int) float64 {
	if len(values) == 0 {
		return 0
	}

	start := 0
	if len(values) > n {
		start = len(values) - n
	}
	window := values[start:]

	if len(window) == 0 {
		return 0
	}

	alpha := 2.0 / float64(len(window)+1)
	ema := window[0]

	for i := 1; i < len(window); i++ {
		ema = alpha*window[i] + (1-alpha)*ema
	}

	return ema
}
