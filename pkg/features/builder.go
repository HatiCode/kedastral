// Package features provides utilities for building feature frames from raw metric data.
package features

import (
	"fmt"
	"time"

	"github.com/HatiCode/kedastral/pkg/adapters"
	"github.com/HatiCode/kedastral/pkg/models"
)

// Builder constructs feature frames from DataFrames, extracting time-based features
// and transforming raw metric data into a format suitable for forecasting models.
type Builder struct{}

// NewBuilder creates a new feature builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// BuildFeatures converts a DataFrame from an adapter into a FeatureFrame for a model.
// It extracts the following features from each row:
//   - value: the metric value (required)
//   - timestamp: Unix timestamp in seconds (extracted from "ts" field if present)
//   - hour: hour of day (0-23) extracted from timestamp
//   - day: day of week (0-6, Sunday=0) extracted from timestamp
//
// Rows without a "value" field are skipped.
// If "ts" field is missing, features derived from timestamps are not included.
func (b *Builder) BuildFeatures(df adapters.DataFrame) (models.FeatureFrame, error) {
	if len(df.Rows) == 0 {
		return models.FeatureFrame{}, fmt.Errorf("dataframe is empty")
	}

	rows := make([]map[string]float64, 0, len(df.Rows))

	for _, row := range df.Rows {
		valueRaw, hasValue := row["value"]
		if !hasValue {
			continue
		}

		value, ok := toFloat64(valueRaw)
		if !ok {
			continue
		}

		features := map[string]float64{
			"value": value,
		}

		if tsRaw, hasTs := row["ts"]; hasTs {
			if timestamp, err := parseTimestamp(tsRaw); err == nil {
				features["timestamp"] = float64(timestamp.Unix())

				// Extract time-based features
				features["hour"] = float64(timestamp.Hour())
				features["day"] = float64(timestamp.Weekday())
			}
		}

		rows = append(rows, features)
	}

	if len(rows) == 0 {
		return models.FeatureFrame{}, fmt.Errorf("no valid rows with 'value' field")
	}

	return models.FeatureFrame{Rows: rows}, nil
}

// toFloat64 attempts to convert any numeric type to float64.
// Handles float64, float32, int, int64, and string representations.
func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}

// parseTimestamp attempts to parse a timestamp from various formats.
// Supports:
//   - RFC3339 strings (e.g., "2023-01-01T12:00:00Z")
//   - Unix timestamps as float64, int, int64
//   - time.Time objects
func parseTimestamp(v any) (time.Time, error) {
	switch val := v.(type) {
	case string:
		t, err := time.Parse(time.RFC3339, val)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid timestamp string: %w", err)
		}
		return t, nil

	case float64:
		return time.Unix(int64(val), 0), nil

	case int:
		return time.Unix(int64(val), 0), nil

	case int64:
		return time.Unix(val, 0), nil

	case time.Time:
		return val, nil

	default:
		return time.Time{}, fmt.Errorf("unsupported timestamp type: %T", v)
	}
}

// FillMissingValues fills missing values in a FeatureFrame using forward fill strategy.
// For each feature column, missing values (represented as NaN or not present) are
// replaced with the last valid value seen.
//
// This is a simple implementation for v0.1. More sophisticated imputation
// strategies (mean, interpolation) can be added later if needed.
func FillMissingValues(frame models.FeatureFrame) models.FeatureFrame {
	if len(frame.Rows) == 0 {
		return frame
	}

	keys := make(map[string]bool)
	for _, row := range frame.Rows {
		for k := range row {
			keys[k] = true
		}
	}

	for key := range keys {
		var lastValid float64
		hasLastValid := false

		for i := range frame.Rows {
			if val, exists := frame.Rows[i][key]; exists {
				lastValid = val
				hasLastValid = true
			} else if hasLastValid {
				frame.Rows[i][key] = lastValid
			}
		}
	}

	return frame
}
