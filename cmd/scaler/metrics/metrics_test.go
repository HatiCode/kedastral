package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// Shared metrics instance for all tests to avoid duplicate registration
var testMetrics = New()

func TestNew(t *testing.T) {
	m := testMetrics

	if m.GRPCRequestsTotal == nil {
		t.Error("GRPCRequestsTotal should not be nil")
	}
	if m.GRPCRequestDuration == nil {
		t.Error("GRPCRequestDuration should not be nil")
	}
	if m.ForecastFetchDuration == nil {
		t.Error("ForecastFetchDuration should not be nil")
	}
	if m.ForecastFetchErrors == nil {
		t.Error("ForecastFetchErrors should not be nil")
	}
	if m.DesiredReplicasReturned == nil {
		t.Error("DesiredReplicasReturned should not be nil")
	}
	if m.ForecastAgeSeen == nil {
		t.Error("ForecastAgeSeen should not be nil")
	}
}

func TestRecordGRPCRequest(t *testing.T) {
	m := testMetrics

	m.RecordGRPCRequest("GetMetrics", "success")
	m.RecordGRPCRequest("GetMetrics", "error")
	m.RecordGRPCRequest("IsActive", "active")

	count := testutil.CollectAndCount(m.GRPCRequestsTotal)
	if count == 0 {
		t.Error("expected gRPC request metrics to be recorded")
	}
}

func TestObserveGRPCDuration(t *testing.T) {
	m := testMetrics

	m.ObserveGRPCDuration("GetMetrics", 0.123)
	m.ObserveGRPCDuration("IsActive", 0.045)

	count := testutil.CollectAndCount(m.GRPCRequestDuration)
	if count == 0 {
		t.Error("expected gRPC duration metrics to be recorded")
	}
}

func TestObserveForecastFetch(t *testing.T) {
	m := testMetrics

	m.ObserveForecastFetch(0.250)

	count := testutil.CollectAndCount(m.ForecastFetchDuration)
	if count != 1 {
		t.Errorf("expected 1 observation, got %d", count)
	}
}

func TestRecordForecastFetchError(t *testing.T) {
	m := testMetrics

	m.RecordForecastFetchError()
	m.RecordForecastFetchError()
	m.RecordForecastFetchError()

	count := testutil.CollectAndCount(m.ForecastFetchErrors)
	if count != 1 {
		t.Errorf("expected 1 counter, got %d", count)
	}
}

func TestSetDesiredReplicas(t *testing.T) {
	m := testMetrics

	tests := []int{1, 5, 10, 100}
	for _, replicas := range tests {
		m.SetDesiredReplicas(replicas)

		count := testutil.CollectAndCount(m.DesiredReplicasReturned)
		if count != 1 {
			t.Errorf("expected 1 gauge, got %d", count)
		}
	}
}

func TestSetForecastAge(t *testing.T) {
	m := testMetrics

	m.SetForecastAge(120.5)

	count := testutil.CollectAndCount(m.ForecastAgeSeen)
	if count != 1 {
		t.Errorf("expected 1 gauge, got %d", count)
	}
}

func TestMetrics_MultipleObservations(t *testing.T) {
	m := testMetrics

	// Record multiple observations
	for range 10 {
		m.RecordGRPCRequest("GetMetrics", "success")
		m.ObserveGRPCDuration("GetMetrics", 0.1)
		m.ObserveForecastFetch(0.2)
	}

	// Verify metrics are present
	grpcCount := testutil.CollectAndCount(m.GRPCRequestsTotal)
	if grpcCount == 0 {
		t.Error("expected gRPC request metrics to be present")
	}

	durationCount := testutil.CollectAndCount(m.GRPCRequestDuration)
	if durationCount == 0 {
		t.Error("expected gRPC duration metrics to be present")
	}

	fetchCount := testutil.CollectAndCount(m.ForecastFetchDuration)
	if fetchCount == 0 {
		t.Error("expected forecast fetch metrics to be present")
	}
}
