# Baseline Model

## Overview

The **Baseline Model** is Kedastral's default forecasting algorithm, designed for ease of use and broad applicability. It combines three powerful techniques to predict future workload:

1. **Trend Detection** - Detects if your workload is increasing, decreasing, or stable
2. **Momentum Detection** - Identifies acceleration or deceleration patterns
3. **Seasonality Learning** - Learns recurring patterns (e.g., hourly spikes, business hours cycles)

The baseline model is **training-optional** (learns patterns if provided historical data) and works well for most common autoscaling scenarios.

## When to Use Baseline Model

✅ **Use Baseline if you have:**
- Predictable traffic patterns (daily cycles, recurring spikes)
- 3-24 hours of historical metrics available
- Workloads with intra-day patterns (hourly, every-30-min, etc.)
- Need for fast, lightweight forecasting
- Simple deployment requirements (no complex tuning)

❌ **Consider ARIMA instead if:**
- Patterns span multiple days/weeks (e.g., weekend vs weekday traffic)
- Workload has complex multi-order autocorrelation
- You need more statistical rigor and model diagnostics

## How It Works

### Algorithm Overview

```
1. Trend Detection (Linear Regression)
   └─> Computes slope from last 10 data points
   └─> Identifies growth/decline rate (RPS per second)

2. Momentum Detection (Acceleration)
   └─> Compares recent trend vs older trend
   └─> Detects if load is accelerating upward

3. Seasonality Learning (Training Phase)
   ├─> Minute-of-hour patterns (0-59) → captures intra-hour cycles
   └─> Hour-of-day patterns (0-23) → captures daily cycles

4. Forecast Generation (Per step)
   ├─> Base prediction: current + trend*t + 0.5*momentum*t²
   ├─> Seasonal adjustment: look up pattern for future time
   └─> Adaptive weighting: 60-80% seasonal when strong pattern exists
```

### Example: Predicting 30-Minute Spikes

**Scenario:** Your API experiences traffic spikes every 30 minutes.

**Training:** (3 hours of data)
```
Minute 00: 500 RPS (spike)
Minute 01-29: 100 RPS (baseline)
Minute 30: 500 RPS (spike)
Minute 31-59: 100 RPS (baseline)
... repeats for 3 hours
```

**Learned Patterns:**
- `minuteSeasonality[0].mean = 500` (spike at :00)
- `minuteSeasonality[30].mean = 500` (spike at :30)
- `minuteSeasonality[1-29].mean = 100` (baseline)

**Prediction at minute 20:**
```
Current: 100 RPS
Trend: ~0 (flat between spikes)
Momentum: ~0 (no acceleration)

Forecast:
  minute 21-29: ~100 RPS (seasonal pattern says baseline)
  minute 30: ~500 RPS ⚡ (seasonal pattern predicts spike!)
  minute 31-39: ~100 RPS (back to baseline)
```

**Result:** KEDA scales up at minute 20, ready for the spike at minute 30! 🎯

## Configuration

### Environment Variables / Flags

The baseline model uses shared forecasting parameters:

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `MODEL` | `--model` | `baseline` | Model type (use `baseline`) |
| `METRIC` | `--metric` | *required* | Metric name to forecast |
| `STEP` | `--step` | `1m` | Time between forecast points |
| `HORIZON` | `--horizon` | `30m` | How far ahead to predict |
| `WINDOW` | `--window` | `30m` | Historical data window for training |

### Recommended Settings

#### For Recurring Spikes (30-min, hourly, etc.)
```bash
MODEL=baseline
STEP=1m              # Fine-grained predictions
HORIZON=30m          # Predict 30 minutes ahead
WINDOW=3h            # Learn from 3 hours (6 spike cycles)
```

**Why:** 3 hours provides 6 occurrences of a 30-min pattern, enough to learn reliably.

#### For Daily Traffic Cycles
```bash
MODEL=baseline
STEP=1m
HORIZON=60m          # Predict 1 hour ahead
WINDOW=12h           # Learn from half a day
```

**Why:** 12 hours captures business hours vs off-hours patterns.

#### For Fast-Ramping Workloads
```bash
MODEL=baseline
STEP=30s             # More frequent predictions
HORIZON=15m          # Shorter horizon for rapid changes
WINDOW=1h            # Less history for responsiveness
```

**Why:** Shorter window and horizon help model react quickly to trend changes.

## Pattern Learning Requirements

The model learns patterns during the **training phase**, which runs every forecast cycle:

### Minimum Data Requirements

| Pattern Type | Minimum Occurrences | Example |
|-------------|---------------------|---------|
| Minute-of-hour | 2 observations | 2+ spikes at minute :30 |
| Hour-of-day | 2 observations | 2+ days with 9am peak |

**Example:** To predict a spike every 30 minutes:
- Need at least **2 spikes** in training data
- With 3 hours of history @ 1min intervals = 180 data points
- Contains **6 spikes** at minutes :00 and :30
- ✅ Model learns the pattern reliably

### Data Quality Tips

1. **Consistent step size**: Prometheus queries should return evenly-spaced data
2. **Time features**: Feature builder automatically adds `hour` and `minute` fields
3. **No gaps**: Missing data points reduce pattern reliability
4. **Sufficient coverage**: More historical data = better pattern learning

## Limitations

### ⚠️ Known Constraints

| Limitation | Impact | Workaround |
|-----------|--------|------------|
| **Patterns > 24 hours** | Cannot learn weekly cycles | Use ARIMA model |
| **Very sparse data** | Need ≥2 occurrences to learn | Increase `WINDOW` duration |
| **Highly irregular workloads** | Predictions may be inaccurate | Consider reactive scaling or ARIMA |
| **Cold starts** | No patterns learned on first run | Allow 1-2 cycles for training |

### Pattern Detection Scope

The baseline model detects patterns within:
- ✅ Minute-of-hour (0-59): Excellent for 15min, 30min, hourly spikes
- ✅ Hour-of-day (0-23): Good for daily cycles (9am-5pm vs night)
- ❌ Day-of-week: Not supported (use ARIMA)
- ❌ Week-of-month: Not supported

## Forecasting Behavior

### Adaptive Weighting

The model intelligently blends trend and seasonal predictions:

```go
if seasonal_spike > 1.5x trend_prediction:
    forecast = 20% trend + 80% seasonal  // Trust the spike pattern!

else if seasonal ≈ trend:
    forecast = 50% trend + 50% seasonal  // Equal confidence

else:
    forecast = trend + momentum  // No strong pattern, use trend
```

### Non-Negative Guarantee

All predictions are clamped to `≥ 0`. Negative forecasts (from downward trends) become `0`.

### Trend Projection

Linear trends are projected forward using:
```
prediction(t) = current_value + slope*t + 0.5*acceleration*t²
```

Where:
- `slope` = linear regression over last 10 points
- `acceleration` = change in slope (momentum)
- `t` = time offset in seconds

## Example Configurations

### Example 1: E-commerce Site (Daily Patterns)

**Scenario:** Traffic peaks at 2pm, low overnight

```yaml
# Helm values.yaml or ConfigMap
forecaster:
  model: baseline
  metric: http_requests_per_second
  step: 1m
  horizon: 60m
  window: 24h        # Full day to learn 2pm peak
```

**Expected behavior:**
- At 1pm: Predicts increase toward 2pm peak
- At 2pm: Maintains high replica count
- At 3pm: Predicts decrease, scales down gradually
- Overnight: Minimal replicas

### Example 2: Batch Job Worker (Hourly Spikes)

**Scenario:** Jobs submitted on the hour (12:00, 13:00, etc.)

```yaml
forecaster:
  model: baseline
  metric: queue_depth
  step: 1m
  horizon: 30m
  window: 6h        # 6 hourly cycles
  leadTime: 10m     # Scale up 10 min before spike
```

**Expected behavior:**
- At :50 (10 min before hour): Predicts spike, begins scaling up
- At :00: Fully scaled, handles spike
- At :10: Predicts return to baseline, scales down

### Example 3: Gaming Server (Irregular Load)

**Scenario:** Load varies unpredictably, no clear patterns

```yaml
forecaster:
  model: baseline
  metric: active_players
  step: 30s
  horizon: 10m
  window: 2h        # Shorter window for responsiveness
```

**Expected behavior:**
- Relies primarily on **trend detection**
- Quickly adapts to sudden increases
- No seasonal adjustments (no patterns to learn)
- Acts like "smart reactive scaling" with short lead time

## Monitoring Model Performance

Check if the model is working:

```bash
# View forecaster logs
kubectl logs -f deployment/kedastral-forecaster

# Look for training confirmation
# "model training skipped" = no patterns learned (OK for first run)
# "predicted forecast" = successful prediction
```

### Key Metrics

```promql
# Prediction latency (should be < 100ms)
kedastral_model_predict_seconds

# Forecast age (should be < interval)
kedastral_forecast_age_seconds

# Desired replicas (should vary predictively)
kedastral_desired_replicas
```

### Debugging Tips

**Problem:** Predictions are flat (not varying)

**Diagnosis:**
```bash
# Check if seasonality was learned
# Add debug logging to see minuteSeasonality map
```

**Solutions:**
1. Increase `WINDOW` to capture more pattern occurrences
2. Verify Prometheus data has clear patterns
3. Check that feature builder is adding `hour` and `minute` fields

**Problem:** Predictions are too reactive (lagging behind actual load)

**Solutions:**
1. Decrease `WINDOW` for faster trend detection
2. Increase `LEAD_TIME` in capacity policy
3. Verify forecast `HORIZON` is long enough

## Comparison with ARIMA

| Aspect | Baseline | ARIMA |
|--------|----------|-------|
| **Setup complexity** | Zero tuning needed | Requires p,d,q parameters |
| **Training speed** | Fast (~10ms) | Slower (~100ms) |
| **Pattern types** | Intra-day only | Multi-day, weekly |
| **Data requirements** | 3-24 hours | 1-7 days |
| **Best for** | Hourly/daily patterns | Weekly/complex patterns |
| **Memory usage** | Low | Medium |

## Advanced: How Training Works

Each forecast cycle:

```
1. Collect historical data (WINDOW duration)
   └─> Example: Last 3 hours at 1-minute intervals = 180 points

2. Feature engineering
   ├─> Extract timestamp → hour (0-23), minute (0-59)
   └─> Normalize values

3. Train seasonality (parallel)
   ├─> Minute patterns: Group by minute-of-hour, compute stats
   │   └─> minuteSeasonality[0] = {mean: 500, max: 520, min: 480, count: 6}
   └─> Hour patterns: Group by hour-of-day, compute stats
       └─> hourSeasonality[14] = {mean: 350, max: 400, min: 300, count: 3}

4. Cache learned patterns
   └─> Patterns persist across predictions (in-memory)
```

### Pattern Statistics

For each time bucket, the model stores:

```go
type seasonalPattern struct {
    mean  float64  // Average load at this time
    max   float64  // Peak load observed
    min   float64  // Lowest load observed
    count int      // Number of observations
}
```

When predicting a spike, the model uses:
```
seasonal_value = 70% * mean + 30% * max  // Favor the max during upward momentum
```

## Summary

**The Baseline Model is ideal for:**
- ✅ Quick setup with zero tuning
- ✅ Intra-day patterns (spikes, hourly cycles)
- ✅ Workloads with 3-24 hours of usable history
- ✅ Most common autoscaling scenarios

**Key Strengths:**
- Automatically learns patterns from data
- Combines trend, momentum, and seasonality
- No parameter tuning required
- Fast training and prediction (<100ms total)

**Start Here!** Use the baseline model first. Only switch to ARIMA if you need weekly patterns or more statistical control.
