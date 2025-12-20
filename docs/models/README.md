# Kedastral Forecasting Models

This directory contains documentation for Kedastral's forecasting models. Each model uses different algorithms and is suited for different workload patterns.

## Available Models

### 🎯 [Baseline Model](./baseline.md) — **Recommended for Most Users**

Fast, zero-configuration forecasting combining trend detection, momentum, and seasonality learning.

**Best for:**
- Hourly or daily traffic patterns
- Recurring spikes (every 15min, 30min, hourly)
- Workloads with 3-24 hours of historical data
- Quick setup without tuning

**Quick start:**
```bash
MODEL=baseline
WINDOW=3h
HORIZON=30m
```

[→ Full Baseline Documentation](./baseline.md)

---

### 📊 [ARIMA Model](./arima.md) — **Advanced Statistical Forecasting**

AutoRegressive Integrated Moving Average model for complex patterns and long-term trends.

**Best for:**
- Weekly patterns (weekday vs weekend)
- Monthly cycles (billing spikes, payroll)
- Multi-day autocorrelation
- Workloads with 1-7 days of historical data

**Quick start:**
```bash
MODEL=arima
ARIMA_P=7          # Weekly lookback
ARIMA_D=1          # Remove trend
ARIMA_Q=1          # Error correction
WINDOW=7d
HORIZON=24h
```

[→ Full ARIMA Documentation](./arima.md)

---

## Model Comparison

| Feature | Baseline | ARIMA |
|---------|----------|-------|
| **Setup Complexity** | ✅ Zero config | ⚙️ Requires p,d,q tuning |
| **Pattern Detection** | Intra-day (hourly, daily) | Multi-day (weekly, monthly) |
| **Training Speed** | ⚡ ~10ms | 🔄 ~100ms |
| **Prediction Speed** | ⚡ ~10ms | 🔄 ~100ms |
| **Min Training Data** | 3 hours | 1-7 days |
| **Optimal Data Window** | 3-24 hours | 1-7 days |
| **Memory Usage** | Low | Medium |
| **Accuracy** | Good (auto-tuned) | Higher (if properly tuned) |
| **Best Use Case** | 30-min spikes, daily cycles | Weekly traffic, monthly billing |

## Choosing a Model

### Start with Baseline if:

- ✅ You're new to Kedastral
- ✅ Your workload has predictable hourly/daily patterns
- ✅ You want zero-configuration setup
- ✅ You have < 24 hours of historical data
- ✅ Prediction latency matters

**Example scenarios:**
- API with lunch-hour traffic spikes
- Batch jobs running every 30 minutes
- Microservice with business-hours peak (9am-5pm)

### Switch to ARIMA if:

- 📈 Baseline predictions aren't accurate enough
- 📅 Your pattern spans multiple days (weekday vs weekend)
- 🔬 You need statistical rigor and parameter control
- 💾 You have 1+ weeks of training data
- ⏱️ You can tolerate ~200ms prediction latency

**Example scenarios:**
- B2B SaaS with weekend downtime
- Payment processor with month-end spikes
- Gaming platform with weekly maintenance windows

## Configuration Reference

### Shared Parameters

These apply to both models:

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `MODEL` | `--model` | `baseline` | Model type: `baseline` or `arima` |
| `METRIC` | `--metric` | *required* | Metric name to forecast |
| `STEP` | `--step` | `1m` | Time between predictions |
| `HORIZON` | `--horizon` | `30m` | How far ahead to predict |
| `WINDOW` | `--window` | `30m` | Historical data for training |
| `INTERVAL` | `--interval` | `30s` | How often to run forecast loop |

### ARIMA-Specific Parameters

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `ARIMA_P` | `--arima-p` | `0` (auto→1) | AutoRegressive order |
| `ARIMA_D` | `--arima-d` | `0` (auto→1) | Differencing order |
| `ARIMA_Q` | `--arima-q` | `0` (auto→1) | Moving Average order |

## Quick Start Examples

### Example 1: Baseline for Hourly Spikes

```yaml
# deploy/examples/baseline-hourly-spikes.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kedastral-forecaster-config
data:
  MODEL: baseline
  METRIC: http_requests_per_second
  WINDOW: 6h
  STEP: 1m
  HORIZON: 30m
  INTERVAL: 30s
```

### Example 2: ARIMA for Weekly Pattern

```yaml
# deploy/examples/arima-weekly-pattern.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kedastral-forecaster-config
data:
  MODEL: arima
  METRIC: queue_depth
  ARIMA_P: 7
  ARIMA_D: 1
  ARIMA_Q: 1
  WINDOW: 14d
  STEP: 1h
  HORIZON: 24h
  INTERVAL: 5m
```

## Monitoring Model Performance

### Key Metrics to Watch

```promql
# Prediction latency (Histogram)
kedastral_model_predict_seconds

# Training/prediction errors (Counter)
kedastral_errors_total{component="model"}

# Forecast age (Gauge - should be < interval)
kedastral_forecast_age_seconds

# Desired replicas (Gauge - should vary predictively)
kedastral_desired_replicas
```

### Grafana Dashboard

See [deploy/grafana/kedastral-dashboard.json](../../deploy/grafana/kedastral-dashboard.json) for:
- Model prediction latency over time
- Forecast vs actual comparison
- Training success rate
- Replica count prediction accuracy

## Debugging Tips

### Problem: Flat Predictions (Not Varying)

**Baseline:**
1. Check if training data has clear patterns
2. Increase `WINDOW` to capture more pattern occurrences
3. Verify Prometheus query returns varying data
4. Check logs for "model training skipped" (means no patterns learned)

**ARIMA:**
1. Verify sufficient data points (min: max(p+d, q+d, 10))
2. Try increasing `WINDOW`
3. Check for "numerical instability" errors
4. Reduce `p` or `q` if data is sparse

### Problem: Predictions Lag Behind Reality

**Both models:**
1. Decrease `WINDOW` for faster trend adaptation
2. Increase `LEAD_TIME` in capacity planning policy
3. Decrease `INTERVAL` to forecast more frequently
4. For ARIMA: reduce `p` (less historical dependency)

### Problem: Training Errors

**Check logs:**
```bash
kubectl logs deployment/kedastral-forecaster | grep -i error
```

**Common errors:**
- `"need at least N points"` → Increase `WINDOW`
- `"numerical instability"` → Reduce ARIMA p/q parameters
- `"no 'value' field"` → Check Prometheus adapter configuration

## Implementation Details

### Model Interface

Both models implement:

```go
type Model interface {
    Train(ctx context.Context, history FeatureFrame) error
    Predict(ctx context.Context, features FeatureFrame) (Forecast, error)
    Name() string
}
```

**Training:** Optional for Baseline (learns patterns if available), Required for ARIMA

**Prediction:** Returns forecast with:
- `Metric`: string
- `Values`: []float64 (length = horizon/step)
- `StepSec`: int
- `Horizon`: int

### Feature Engineering

The forecaster automatically enriches data with time features:

```go
Input (from Prometheus):
  {value: 123.45, timestamp: 1234567890}

Output (features):
  {
    value: 123.45,
    timestamp: 1234567890,
    hour: 14,        // 0-23
    minute: 30,      // 0-59
    dayOfWeek: 2,    // 0-6 (Sunday=0)
  }
```

These time features enable seasonality learning.

## Future Models

Planned for future releases:

- **Prophet**: Facebook's forecasting model with holiday effects
- **Ensemble**: Combine multiple models with weighted voting
- **ML-based**: Neural networks for very complex patterns
- **BYOM (Bring Your Own Model)**: HTTP endpoint contract

## Contributing

To add a new model:

1. Implement the `Model` interface in `pkg/models/`
2. Add constructor in `cmd/forecaster/models/model.go`
3. Add configuration flags in `cmd/forecaster/config/config.go`
4. Write comprehensive tests
5. Create documentation in `docs/models/your-model.md`
6. Update this README with comparison table

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for details.

---

## Additional Resources

- [Architecture Overview](../architecture.md)
- [Capacity Planning](../capacity-planning.md)
- [Prometheus Adapter Configuration](../adapters/prometheus.md)
- [Quick Start Guide](../quickstart.md)

---

**Need help choosing?** Start with **Baseline** and only switch to ARIMA if you need multi-day patterns!
