// Package metrics provides Prometheus metrics instrumentation for the scaler.
//
// It exposes operational metrics about the scaler's gRPC service performance,
// forecast fetching behavior, and scaling decisions. All metrics are exposed
// via the /metrics HTTP endpoint for Prometheus scraping.
//
// Metrics exposed:
//   - kedastral_scaler_grpc_requests_total: Counter of gRPC requests by method and status
//   - kedastral_scaler_grpc_request_duration_seconds: Histogram of gRPC request durations
//   - kedastral_scaler_forecast_fetch_duration_seconds: Histogram of forecast fetch latency
//   - kedastral_scaler_forecast_fetch_errors_total: Counter of forecast fetch errors
//   - kedastral_scaler_desired_replicas_returned: Gauge of last replica count returned to KEDA
//   - kedastral_scaler_forecast_age_seen_seconds: Gauge of forecast data age
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	GRPCRequestsTotal       *prometheus.CounterVec
	GRPCRequestDuration     *prometheus.HistogramVec
	ForecastFetchDuration   prometheus.Histogram
	ForecastFetchErrors     prometheus.Counter
	DesiredReplicasReturned prometheus.Gauge
	ForecastAgeSeen         prometheus.Gauge
}

func New() *Metrics {
	return &Metrics{
		GRPCRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "kedastral_scaler_grpc_requests_total",
			Help: "Total number of gRPC requests by method and status",
		}, []string{"method", "status"}),

		GRPCRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kedastral_scaler_grpc_request_duration_seconds",
			Help:    "Duration of gRPC requests by method",
			Buckets: prometheus.DefBuckets,
		}, []string{"method"}),

		ForecastFetchDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "kedastral_scaler_forecast_fetch_duration_seconds",
			Help:    "Duration of forecast fetch from forecaster",
			Buckets: prometheus.DefBuckets,
		}),

		ForecastFetchErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "kedastral_scaler_forecast_fetch_errors_total",
			Help: "Total number of errors fetching forecasts",
		}),

		DesiredReplicasReturned: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "kedastral_scaler_desired_replicas_returned",
			Help: "Last desired replicas value returned to KEDA",
		}),

		ForecastAgeSeen: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "kedastral_scaler_forecast_age_seen_seconds",
			Help: "Age of forecast data seen from forecaster",
		}),
	}
}

func (m *Metrics) RecordGRPCRequest(method, status string) {
	m.GRPCRequestsTotal.WithLabelValues(method, status).Inc()
}

func (m *Metrics) ObserveGRPCDuration(method string, seconds float64) {
	m.GRPCRequestDuration.WithLabelValues(method).Observe(seconds)
}

func (m *Metrics) ObserveForecastFetch(seconds float64) {
	m.ForecastFetchDuration.Observe(seconds)
}

func (m *Metrics) RecordForecastFetchError() {
	m.ForecastFetchErrors.Inc()
}

func (m *Metrics) SetDesiredReplicas(replicas int) {
	m.DesiredReplicasReturned.Set(float64(replicas))
}

func (m *Metrics) SetForecastAge(seconds float64) {
	m.ForecastAgeSeen.Set(seconds)
}
