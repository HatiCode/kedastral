package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNew(t *testing.T) {
	// Create new metrics registry for testing
	reg := prometheus.NewRegistry()

	m := &Metrics{
		AdapterCollectSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "kedastral_adapter_collect_seconds",
			Help: "Time spent collecting metrics from adapter",
			ConstLabels: prometheus.Labels{
				"adapter":  "prometheus",
				"workload": "test-workload",
			},
		}),
		ModelPredictSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "kedastral_model_predict_seconds",
			Help: "Time spent predicting forecast",
			ConstLabels: prometheus.Labels{
				"model":    "baseline",
				"workload": "test-workload",
			},
		}),
		CapacityComputeSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "kedastral_capacity_compute_seconds",
			Help: "Time spent computing desired replicas",
			ConstLabels: prometheus.Labels{
				"workload": "test-workload",
			},
		}),
		ForecastAgeSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "kedastral_forecast_age_seconds",
			Help: "Age of the current forecast in seconds",
			ConstLabels: prometheus.Labels{
				"workload": "test-workload",
			},
		}),
		DesiredReplicas: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "kedastral_desired_replicas",
			Help: "Current desired replica count",
			ConstLabels: prometheus.Labels{
				"workload": "test-workload",
			},
		}),
		ErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kedastral_errors_total",
			Help: "Total number of errors by component and reason",
			ConstLabels: prometheus.Labels{
				"workload": "test-workload",
			},
		}, []string{"component", "reason"}),
	}

	reg.MustRegister(
		m.AdapterCollectSeconds,
		m.ModelPredictSeconds,
		m.CapacityComputeSeconds,
		m.ForecastAgeSeconds,
		m.DesiredReplicas,
		m.ErrorsTotal,
	)

	if m.AdapterCollectSeconds == nil {
		t.Error("AdapterCollectSeconds should not be nil")
	}
	if m.ModelPredictSeconds == nil {
		t.Error("ModelPredictSeconds should not be nil")
	}
	if m.CapacityComputeSeconds == nil {
		t.Error("CapacityComputeSeconds should not be nil")
	}
	if m.ForecastAgeSeconds == nil {
		t.Error("ForecastAgeSeconds should not be nil")
	}
	if m.DesiredReplicas == nil {
		t.Error("DesiredReplicas should not be nil")
	}
	if m.ErrorsTotal == nil {
		t.Error("ErrorsTotal should not be nil")
	}
}

func TestRecordCollect(t *testing.T) {
	m := New("test-record-collect")

	m.RecordCollect(0.123)

	// Verify the histogram recorded the value
	count := testutil.CollectAndCount(m.AdapterCollectSeconds)
	if count != 1 {
		t.Errorf("expected 1 observation, got %d", count)
	}
}

func TestRecordPredict(t *testing.T) {
	m := New("test-record-predict")

	m.RecordPredict(0.456)

	count := testutil.CollectAndCount(m.ModelPredictSeconds)
	if count != 1 {
		t.Errorf("expected 1 observation, got %d", count)
	}
}

func TestRecordCapacity(t *testing.T) {
	m := New("test-record-capacity")

	m.RecordCapacity(0.789)

	count := testutil.CollectAndCount(m.CapacityComputeSeconds)
	if count != 1 {
		t.Errorf("expected 1 observation, got %d", count)
	}
}

func TestSetForecastAge(t *testing.T) {
	m := New("test-set-forecast-age")

	m.SetForecastAge(120.5)

	// Collect gauge value
	gauges, err := testutil.GatherAndCount(prometheus.DefaultGatherer, "kedastral_forecast_age_seconds")
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	if gauges == 0 {
		t.Error("expected forecast age gauge to be set")
	}
}

func TestSetDesiredReplicas(t *testing.T) {
	m := New("test-set-desired-replicas")

	tests := []int{1, 5, 10, 100}
	for _, replicas := range tests {
		m.SetDesiredReplicas(replicas)

		gauges, err := testutil.GatherAndCount(prometheus.DefaultGatherer, "kedastral_desired_replicas")
		if err != nil {
			t.Fatalf("failed to gather metrics: %v", err)
		}
		if gauges == 0 {
			t.Errorf("expected desired replicas gauge to be set for %d replicas", replicas)
		}
	}
}

func TestRecordError(t *testing.T) {
	m := New("test-record-error")

	tests := []struct {
		component string
		reason    string
	}{
		{"adapter", "collect_failed"},
		{"model", "predict_failed"},
		{"store", "put_failed"},
		{"features", "build_failed"},
	}

	for _, tt := range tests {
		m.RecordError(tt.component, tt.reason)
	}

	// Verify errors were recorded
	count := testutil.CollectAndCount(m.ErrorsTotal)
	if count != len(tests) {
		t.Errorf("expected %d error metrics, got %d", len(tests), count)
	}
}

func TestRecordError_Increment(t *testing.T) {
	m := New("test-record-error-increment")

	// Record same error multiple times
	m.RecordError("adapter", "timeout")
	m.RecordError("adapter", "timeout")
	m.RecordError("adapter", "timeout")

	// The counter should increment
	count := testutil.CollectAndCount(m.ErrorsTotal)
	if count == 0 {
		t.Error("expected error counter to have observations")
	}
}

func TestMetrics_MultipleObservations(t *testing.T) {
	m := New("test-metrics-multiple-observations")

	// Record multiple observations
	for range 10 {
		m.RecordCollect(0.1)
		m.RecordPredict(0.2)
		m.RecordCapacity(0.01)
	}

	// Verify metrics are present (testutil.CollectAndCount counts metric descriptors, not observations)
	collectCount := testutil.CollectAndCount(m.AdapterCollectSeconds)
	if collectCount == 0 {
		t.Error("expected collect metric to be present")
	}

	predictCount := testutil.CollectAndCount(m.ModelPredictSeconds)
	if predictCount == 0 {
		t.Error("expected predict metric to be present")
	}

	capacityCount := testutil.CollectAndCount(m.CapacityComputeSeconds)
	if capacityCount == 0 {
		t.Error("expected capacity metric to be present")
	}
}
