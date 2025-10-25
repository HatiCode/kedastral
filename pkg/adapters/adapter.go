package adapters

import (
	"context"
	"time"
)

// Row represents a single time-series or tabular observation.
// Example: {"ts": "2025-10-25T17:00:00Z", "value": 312.4, "playerA": "Nadal"}
type Row map[string]any

// DataFrame is a lightweight structure for tabular data returned by adapters.
// Each adapter collects data over a time window and returns it in this format.
type DataFrame struct {
	Rows []Row
}

// Adapter is the interface that all Kedastral adapters must implement.
//
// Adapters are responsible for fetching raw data from an external system
// (Prometheus, Kafka, HTTP API, etc.), shaping it into a DataFrame, and
// returning it for feature building and forecasting.
//
// The Collect() call is synchronous and should respect context cancellation
// and deadlines.
type Adapter interface {
	// Collect fetches metrics or events for the last windowSeconds and returns them
	// as a DataFrame. It must handle transient errors gracefully and never panic.
	Collect(ctx context.Context, windowSeconds int) (*DataFrame, error)

	// Name returns a short, unique identifier for the adapter.
	// Example: "prometheus", "schedule", "http".
	Name() string
}

// Optional: helper to align timestamps to a consistent step duration.
func AlignTimestamp(ts time.Time, stepSec int) time.Time {
	return ts.Truncate(time.Duration(stepSec) * time.Second)
}
