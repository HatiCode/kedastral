package features

import (
	"testing"
	"time"

	"github.com/HatiCode/kedastral/pkg/adapters"
	"github.com/HatiCode/kedastral/pkg/models"
)

func TestNewBuilder(t *testing.T) {
	builder := NewBuilder()
	if builder == nil {
		t.Fatal("NewBuilder() returned nil")
	}
}

func TestBuilder_BuildFeatures_Success(t *testing.T) {
	builder := NewBuilder()

	// Create a DataFrame with timestamp and value
	now := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC) // Monday, 14:30
	df := adapters.DataFrame{
		Rows: []adapters.Row{
			{"ts": now.Format(time.RFC3339), "value": 100.0},
			{"ts": now.Add(time.Minute).Format(time.RFC3339), "value": 110.0},
			{"ts": now.Add(2 * time.Minute).Format(time.RFC3339), "value": 120.0},
		},
	}

	frame, err := builder.BuildFeatures(df)
	if err != nil {
		t.Fatalf("BuildFeatures() error = %v", err)
	}

	if len(frame.Rows) != 3 {
		t.Errorf("len(Rows) = %d, want 3", len(frame.Rows))
	}

	// Check first row
	row := frame.Rows[0]
	if row["value"] != 100.0 {
		t.Errorf("value = %f, want 100.0", row["value"])
	}
	if row["hour"] != 14.0 {
		t.Errorf("hour = %f, want 14.0", row["hour"])
	}
	if row["day"] != 1.0 { // Monday
		t.Errorf("day = %f, want 1.0 (Monday)", row["day"])
	}
	if row["timestamp"] != float64(now.Unix()) {
		t.Errorf("timestamp = %f, want %f", row["timestamp"], float64(now.Unix()))
	}
}

func TestBuilder_BuildFeatures_UnixTimestamp(t *testing.T) {
	builder := NewBuilder()

	now := time.Now()
	df := adapters.DataFrame{
		Rows: []adapters.Row{
			{"ts": float64(now.Unix()), "value": 100.0},
			{"ts": int64(now.Unix()), "value": 110.0},
			{"ts": int(now.Unix()), "value": 120.0},
		},
	}

	frame, err := builder.BuildFeatures(df)
	if err != nil {
		t.Fatalf("BuildFeatures() error = %v", err)
	}

	if len(frame.Rows) != 3 {
		t.Errorf("len(Rows) = %d, want 3", len(frame.Rows))
	}

	for i, row := range frame.Rows {
		if _, hasTimestamp := row["timestamp"]; !hasTimestamp {
			t.Errorf("row %d missing timestamp feature", i)
		}
		if _, hasHour := row["hour"]; !hasHour {
			t.Errorf("row %d missing hour feature", i)
		}
		if _, hasDay := row["day"]; !hasDay {
			t.Errorf("row %d missing day feature", i)
		}
	}
}

func TestBuilder_BuildFeatures_NoTimestamp(t *testing.T) {
	builder := NewBuilder()

	// DataFrame without timestamp field
	df := adapters.DataFrame{
		Rows: []adapters.Row{
			{"value": 100.0},
			{"value": 110.0},
		},
	}

	frame, err := builder.BuildFeatures(df)
	if err != nil {
		t.Fatalf("BuildFeatures() error = %v", err)
	}

	if len(frame.Rows) != 2 {
		t.Errorf("len(Rows) = %d, want 2", len(frame.Rows))
	}

	// Should have value but no time features
	row := frame.Rows[0]
	if row["value"] != 100.0 {
		t.Errorf("value = %f, want 100.0", row["value"])
	}
	if _, hasHour := row["hour"]; hasHour {
		t.Error("should not have hour feature without timestamp")
	}
}

func TestBuilder_BuildFeatures_EmptyDataFrame(t *testing.T) {
	builder := NewBuilder()

	df := adapters.DataFrame{Rows: []adapters.Row{}}

	_, err := builder.BuildFeatures(df)
	if err == nil {
		t.Error("Expected error for empty dataframe")
	}
}

func TestBuilder_BuildFeatures_NoValueField(t *testing.T) {
	builder := NewBuilder()

	df := adapters.DataFrame{
		Rows: []adapters.Row{
			{"ts": "2024-01-01T00:00:00Z"},
			{"timestamp": 123456},
		},
	}

	_, err := builder.BuildFeatures(df)
	if err == nil {
		t.Error("Expected error when no rows have 'value' field")
	}
}

func TestBuilder_BuildFeatures_MixedRows(t *testing.T) {
	builder := NewBuilder()

	// Mix of valid and invalid rows
	df := adapters.DataFrame{
		Rows: []adapters.Row{
			{"value": 100.0, "ts": "2024-01-01T00:00:00Z"},
			{"other": "no value"}, // Skipped
			{"value": 110.0, "ts": "2024-01-01T00:01:00Z"},
			{"value": "invalid"}, // Skipped
			{"value": 120.0, "ts": "2024-01-01T00:02:00Z"},
		},
	}

	frame, err := builder.BuildFeatures(df)
	if err != nil {
		t.Fatalf("BuildFeatures() error = %v", err)
	}

	// Should have 3 valid rows
	if len(frame.Rows) != 3 {
		t.Errorf("len(Rows) = %d, want 3", len(frame.Rows))
	}
}

func TestBuilder_BuildFeatures_NumericTypes(t *testing.T) {
	builder := NewBuilder()

	df := adapters.DataFrame{
		Rows: []adapters.Row{
			{"value": float64(100.5)},
			{"value": float32(110.5)},
			{"value": int(120)},
			{"value": int64(130)},
			{"value": int32(140)},
		},
	}

	frame, err := builder.BuildFeatures(df)
	if err != nil {
		t.Fatalf("BuildFeatures() error = %v", err)
	}

	if len(frame.Rows) != 5 {
		t.Errorf("len(Rows) = %d, want 5", len(frame.Rows))
	}

	// Verify values are correctly converted
	expectedValues := []float64{100.5, 110.5, 120.0, 130.0, 140.0}
	for i, expected := range expectedValues {
		if frame.Rows[i]["value"] != expected {
			t.Errorf("row %d value = %f, want %f", i, frame.Rows[i]["value"], expected)
		}
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  float64
		ok    bool
	}{
		{"float64", float64(123.45), 123.45, true},
		{"float32", float32(123.45), float64(float32(123.45)), true},
		{"int", int(123), 123.0, true},
		{"int64", int64(123), 123.0, true},
		{"int32", int32(123), 123.0, true},
		{"string", "123", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			if ok != tt.ok {
				t.Errorf("toFloat64() ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("toFloat64() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		input   any
		want    time.Time
		wantErr bool
	}{
		{
			name:    "RFC3339 string",
			input:   now.Format(time.RFC3339),
			want:    now,
			wantErr: false,
		},
		{
			name:    "Unix timestamp float64",
			input:   float64(now.Unix()),
			want:    time.Unix(now.Unix(), 0),
			wantErr: false,
		},
		{
			name:    "Unix timestamp int64",
			input:   int64(now.Unix()),
			want:    time.Unix(now.Unix(), 0),
			wantErr: false,
		},
		{
			name:    "Unix timestamp int",
			input:   int(now.Unix()),
			want:    time.Unix(now.Unix(), 0),
			wantErr: false,
		},
		{
			name:    "time.Time",
			input:   now,
			want:    now,
			wantErr: false,
		},
		{
			name:    "invalid string",
			input:   "not a timestamp",
			wantErr: true,
		},
		{
			name:    "unsupported type",
			input:   true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimestamp(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("parseTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFillMissingValues(t *testing.T) {
	tests := []struct {
		name  string
		input models.FeatureFrame
		want  models.FeatureFrame
	}{
		{
			name: "no missing values",
			input: models.FeatureFrame{
				Rows: []map[string]float64{
					{"value": 100, "hour": 0},
					{"value": 110, "hour": 1},
				},
			},
			want: models.FeatureFrame{
				Rows: []map[string]float64{
					{"value": 100, "hour": 0},
					{"value": 110, "hour": 1},
				},
			},
		},
		{
			name: "missing value in middle",
			input: models.FeatureFrame{
				Rows: []map[string]float64{
					{"value": 100, "hour": 0},
					{"hour": 1}, // Missing value
					{"value": 120, "hour": 2},
				},
			},
			want: models.FeatureFrame{
				Rows: []map[string]float64{
					{"value": 100, "hour": 0},
					{"value": 100, "hour": 1}, // Filled with last valid
					{"value": 120, "hour": 2},
				},
			},
		},
		{
			name: "empty frame",
			input: models.FeatureFrame{
				Rows: []map[string]float64{},
			},
			want: models.FeatureFrame{
				Rows: []map[string]float64{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FillMissingValues(tt.input)

			if len(got.Rows) != len(tt.want.Rows) {
				t.Errorf("len(Rows) = %d, want %d", len(got.Rows), len(tt.want.Rows))
				return
			}

			for i := range got.Rows {
				for key, wantVal := range tt.want.Rows[i] {
					gotVal, exists := got.Rows[i][key]
					if !exists {
						t.Errorf("row %d missing key %q", i, key)
						continue
					}
					if gotVal != wantVal {
						t.Errorf("row %d key %q = %f, want %f", i, key, gotVal, wantVal)
					}
				}
			}
		})
	}
}

func TestBuilder_BuildFeatures_TimeFeatures(t *testing.T) {
	builder := NewBuilder()

	// Test different hours and days
	testCases := []struct {
		time     time.Time
		wantHour float64
		wantDay  float64
	}{
		{
			time:     time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), // Monday midnight
			wantHour: 0,
			wantDay:  1, // Monday
		},
		{
			time:     time.Date(2024, 1, 14, 23, 59, 0, 0, time.UTC), // Sunday 23:59
			wantHour: 23,
			wantDay:  0, // Sunday
		},
		{
			time:     time.Date(2024, 1, 20, 12, 30, 0, 0, time.UTC), // Saturday noon
			wantHour: 12,
			wantDay:  6, // Saturday
		},
	}

	for _, tc := range testCases {
		df := adapters.DataFrame{
			Rows: []adapters.Row{
				{"ts": tc.time.Format(time.RFC3339), "value": 100.0},
			},
		}

		frame, err := builder.BuildFeatures(df)
		if err != nil {
			t.Fatalf("BuildFeatures() error = %v", err)
		}

		row := frame.Rows[0]
		if row["hour"] != tc.wantHour {
			t.Errorf("time %v: hour = %f, want %f", tc.time, row["hour"], tc.wantHour)
		}
		if row["day"] != tc.wantDay {
			t.Errorf("time %v: day = %f, want %f", tc.time, row["day"], tc.wantDay)
		}
	}
}
