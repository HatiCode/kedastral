# ARIMA Model

## Overview

The **ARIMA Model** (AutoRegressive Integrated Moving Average) is Kedastral's advanced forecasting algorithm for workloads with complex patterns and long-term trends. It provides more statistical rigor than the baseline model and can handle multi-day patterns.

**ARIMA(p, d, q)** where:
- **p** = AutoRegressive order (how many past values influence the forecast)
- **d** = Differencing order (trend removal: 0=none, 1=linear, 2=quadratic)
- **q** = Moving Average order (how many past forecast errors to incorporate)

## When to Use ARIMA Model

✅ **Use ARIMA if you have:**
- Weekly patterns (weekday vs weekend traffic)
- Multi-day cycles (monthly billing spikes, payday effects)
- Complex autocorrelation requiring statistical modeling
- 1-7 days of historical metrics
- Need for trend removal and stationarity

❌ **Use Baseline instead if:**
- Patterns are within 24 hours (hourly, daily cycles)
- You want zero-configuration setup
- Training data is limited (< 1 day)
- Prediction latency must be minimal

## How It Works

### Algorithm Overview

ARIMA combines three components:

```
1. AR (AutoRegressive): p
   └─> Predicts based on weighted sum of p previous values
   └─> Example: AR(2) = φ₁*y(t-1) + φ₂*y(t-2)

2. I (Integrated): d
   └─> Removes trends by differencing
   └─> d=0: No differencing (stationary data)
   └─> d=1: First difference (removes linear trends)
   └─> d=2: Second difference (removes quadratic trends)

3. MA (Moving Average): q
   └─> Predicts based on weighted sum of q past forecast errors
   └─> Example: MA(1) = θ₁*ε(t-1)
```

### Mathematical Formulation

Given a time series `y(t)`:

1. **Differencing** (make stationary):
   ```
   d=1: y'(t) = y(t) - y(t-1)
   d=2: y''(t) = y'(t) - y'(t-1)
   ```

2. **AR Component** (past values):
   ```
   AR(p): ŷ(t) = φ₁y(t-1) + φ₂y(t-2) + ... + φₚy(t-p)
   ```

3. **MA Component** (past errors):
   ```
   MA(q): ŷ(t) += θ₁ε(t-1) + θ₂ε(t-2) + ... + θ_qε(t-q)
   ```

4. **Final prediction:**
   ```
   ARIMA(p,d,q): ŷ(t) = AR(p) + MA(q) + mean
   ```

5. **Invert differencing** to get actual forecast

### Training Process

```
1. Extract metric values from historical data
   └─> Minimum: max(p+d, q+d, 10) data points required

2. Apply differencing d times
   └─> Achieve stationarity (constant mean/variance)

3. Compute mean of stationary series
   └─> Used to center the data

4. Fit AR coefficients using Yule-Walker equations
   └─> Solves autocorrelation matrix for φ₁...φₚ

5. Fit MA coefficients using innovations algorithm
   └─> Estimates θ₁...θ_q from residuals

6. Store last p values and q errors
   └─> Needed for recursive prediction
```

### Prediction Process

```
1. Start with last p known values and q errors
2. For each future step:
   ├─> Apply AR: weighted sum of previous values
   ├─> Apply MA: weighted sum of previous errors
   ├─> Add mean
   └─> Invert differencing to get actual value
3. Update sliding window for next prediction
4. Clamp to non-negative values
```

## Configuration

### Environment Variables / Flags

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `MODEL` | `--model` | `baseline` | Set to `arima` |
| `ARIMA_P` | `--arima-p` | `0` (auto→1) | AutoRegressive order |
| `ARIMA_D` | `--arima-d` | `0` (auto→1) | Differencing order |
| `ARIMA_Q` | `--arima-q` | `0` (auto→1) | Moving Average order |
| `METRIC` | `--metric` | *required* | Metric name |
| `STEP` | `--step` | `1m` | Forecast step size |
| `HORIZON` | `--horizon` | `30m` | Forecast horizon |
| `WINDOW` | `--window` | `30m` | Training window |

### Parameter Selection Guide

#### **p (AutoRegressive order)**

**What it controls:** How many past values influence the forecast

| Value | Use Case | Example |
|-------|----------|---------|
| `p=0` | Auto (→1) | Default for most cases |
| `p=1` | Short memory | Tomorrow depends only on today |
| `p=2` | Medium memory | Tomorrow depends on today and yesterday |
| `p=3+` | Long memory | Complex multi-day dependencies |

**When to increase p:**
- Workload has clear correlation with several previous days
- Autocorrelation plot shows slow decay
- Need more historical context

**Caution:** Higher p requires more training data (`p+d` minimum points)

#### **d (Differencing order)**

**What it controls:** Trend removal

| Value | Use Case | Trend Type |
|-------|----------|------------|
| `d=0` | Already stationary | Oscillating around constant mean |
| `d=1` | Linear trend (default) | Steadily increasing/decreasing |
| `d=2` | Quadratic trend | Accelerating growth/decline |

**When to use d=1:** (Most common)
- Workload is generally growing or declining
- Mean shifts over time
- Weekday traffic higher than weekend

**When to use d=2:**
- Exponential growth patterns
- Launch periods (rapid acceleration)
- Rare - usually d=1 is sufficient

**When to use d=0:**
- Load oscillates around stable mean
- No long-term trend
- Seasonality only

#### **q (Moving Average order)**

**What it controls:** How forecast errors improve predictions

| Value | Use Case | Example |
|-------|----------|---------|
| `q=0` | Auto (→1) | Default |
| `q=1` | Recent errors | Adjust for yesterday's misprediction |
| `q=2` | Medium error memory | Learn from 2-day error pattern |
| `q=3+` | Complex error patterns | Rare |

**When to increase q:**
- Forecasts systematically over/under-predict
- Shocks take multiple periods to absorb
- Complex error autocorrelation

## Recommended Configurations

### 1. Weekly Traffic Pattern (Weekday vs Weekend)

**Scenario:** B2B SaaS with high weekday, low weekend traffic

```bash
MODEL=arima
ARIMA_P=7          # 7-day lookback (weekly cycle)
ARIMA_D=1          # Remove linear trend
ARIMA_Q=1          # Adjust for recent errors
WINDOW=7d          # One full week of data
STEP=1h            # Hourly predictions
HORIZON=24h        # Predict one day ahead
```

**Why:**
- `p=7`: Monday depends on last Monday (7 days ago)
- `d=1`: Remove growth trend
- `q=1`: Correct systematic errors
- `window=7d`: Capture full weekly cycle

**Minimum training data:** 7 days worth

### 2. Monthly Billing Spike

**Scenario:** Payment processing spike on 1st of month

```bash
MODEL=arima
ARIMA_P=30         # Monthly lookback
ARIMA_D=1
ARIMA_Q=2
WINDOW=60d         # Two months to learn pattern
STEP=1h
HORIZON=48h        # Predict 2 days ahead
```

**Why:**
- `p=30`: Today depends on same day last month
- `q=2`: Handle irregular error patterns
- Long window ensures 2+ pattern occurrences

### 3. Rapidly Growing Startup

**Scenario:** 20% week-over-week growth

```bash
MODEL=arima
ARIMA_P=1
ARIMA_D=2          # Quadratic trend (acceleration)
ARIMA_Q=1
WINDOW=14d
STEP=1h
HORIZON=24h
```

**Why:**
- `d=2`: Captures accelerating growth
- Shorter `p`: Recent data more relevant
- Regular retraining keeps up with growth

### 4. General Purpose (Start Here)

**Scenario:** Unknown pattern, experimenting

```bash
MODEL=arima
ARIMA_P=0          # Auto → 1
ARIMA_D=0          # Auto → 1
ARIMA_Q=0          # Auto → 1
WINDOW=24h
STEP=1m
HORIZON=30m
```

**Why:**
- Auto-detection uses sensible defaults
- ARIMA(1,1,1) works for many cases
- Easy to tune later based on results

## Data Requirements

### Minimum Data Points

The model requires at least:
```
min_points = max(max(p+d, q+d), 10)
```

**Examples:**
- ARIMA(1,1,1): min 10 points
- ARIMA(2,1,1): min 10 points (max of 3, 2, 10)
- ARIMA(7,1,1): min 10 points (max of 8, 2, 10)
- ARIMA(30,1,1): min 31 points

**Practical rule:** Provide at least **2-3x** the minimum for stable training.

### Recommended Training Windows

| Pattern Cycle | Recommended Window | Minimum Window |
|--------------|-------------------|----------------|
| Hourly | 12-24 hours | 6 hours |
| Daily | 7-14 days | 3 days |
| Weekly | 4-8 weeks | 2 weeks |
| Monthly | 3-6 months | 2 months |

## Performance Characteristics

### Training Time

| Data Points | p,d,q | Training Time |
|------------|-------|---------------|
| 100 | (1,1,1) | ~50ms |
| 1000 | (1,1,1) | ~100ms |
| 1000 | (7,1,1) | ~150ms |
| 10000 | (1,1,1) | ~500ms |

**Note:** Training happens every forecast cycle. For large windows, consider caching.

### Memory Usage

- **Baseline:** Stores AR coefficients (p floats) + MA coefficients (q floats) + last values
- **Typical:** ARIMA(7,1,1) ≈ 200 bytes
- **Large:** ARIMA(30,1,2) ≈ 500 bytes

## Limitations

### ⚠️ Known Constraints

| Limitation | Impact | Workaround |
|-----------|--------|------------|
| **Max d=2** | Cannot handle higher-order trends | Use d=2 or transform data |
| **Training required** | Fails without sufficient data | Ensure min_points available |
| **Numerical instability** | Rare: matrix inversion fails | Reduce p/q or add more data |
| **Longer prediction time** | ~100ms vs baseline ~10ms | Acceptable for most cases |
| **No automatic p,d,q selection** | Manual tuning needed | Start with (1,1,1) |

### Edge Cases

**Sparse or irregular data:**
- ARIMA assumes evenly-spaced observations
- Gaps in Prometheus data may degrade predictions
- Solution: Ensure consistent scrape intervals

**Non-stationary series:**
- If d=2 insufficient, model may fail
- Solution: Pre-transform data (log scale) or use baseline model

## Monitoring & Debugging

### Check Training Success

```bash
kubectl logs deployment/kedastral-forecaster | grep -i arima

# Look for:
# "initializing ARIMA model" p=X d=Y q=Z
# "predicted forecast" model=arima(X,Y,Z)
```

### Common Errors

**Error:** `need at least N points for ARIMA(p,d,q), got M`

**Cause:** Insufficient training data

**Fix:**
1. Increase `WINDOW` duration
2. Reduce `p`, `d`, or `q`
3. Wait for more Prometheus data to accumulate

---

**Error:** `numerical instability during coefficient estimation`

**Cause:** Ill-conditioned autocorrelation matrix

**Fix:**
1. Reduce `p` or `q`
2. Increase training data
3. Check for duplicate or constant values in data

---

**Error:** `model not trained, call Train() first`

**Cause:** Training failed on previous cycle

**Fix:**
1. Check logs for training error
2. Verify data availability
3. Ensure feature frame has 'value' field

## Model Diagnostics

### Evaluating Fit Quality

After running for several cycles, check:

```promql
# Prediction latency
histogram_quantile(0.95, kedastral_model_predict_seconds)

# Forecast accuracy (compare to actuals)
abs(kedastral_desired_replicas - actual_replicas) / actual_replicas

# Training failures
rate(kedastral_errors_total{component="model"}[5m])
```

### Tuning Iteration

1. **Start with defaults:** ARIMA(1,1,1)
2. **Run for 1-2 days**
3. **Compare forecasts vs actuals**
4. **Adjust:**
   - Forecasts lag behind → increase `p` or `q`
   - Forecasts overshoot → decrease `p` or `q`
   - Trend not captured → check `d` (usually 1 is fine)
5. **Re-deploy and repeat**

## Advanced: Mathematical Details

### Yule-Walker Equations (AR Coefficients)

For AR(p), the coefficients φ₁...φₚ are solved from:

```
Γφ = γ

Where:
  Γ = autocorrelation matrix (p×p)
  γ = autocorrelation vector (p×1)
  φ = AR coefficient vector (p×1)
```

Implementation uses:
```go
func fitAR(series []float64, p int) ([]float64, error)
    └─> Compute autocorrelation r(k) for k=0..p
    └─> Build Γ matrix from r(1)..r(p)
    └─> Solve Γφ = [r(1), r(2), ..., r(p)]ᵀ
    └─> Return φ coefficients
```

### Innovations Algorithm (MA Coefficients)

For MA(q), coefficients are estimated using:

```
1. Compute residuals from AR model
2. Apply innovations algorithm to find θ₁...θ_q
3. Minimize sum of squared errors
```

### Differencing Implementation

```go
func difference(series []float64, d int) []float64
    For d=1: y'(t) = y(t) - y(t-1)
    For d=2: y''(t) = y'(t) - y'(t-1)
```

Inverse differencing reconstructs original scale:
```go
func inverseDifference(diffed []float64, original []float64, d int)
    Cumulative sum for d=1
    Double cumulative sum for d=2
```

## Comparison with Baseline

| Aspect | ARIMA | Baseline |
|--------|-------|----------|
| **Pattern types** | Weekly, monthly, complex | Intra-day only |
| **Setup** | Requires p,d,q tuning | Zero config |
| **Training** | ~100ms | ~10ms |
| **Prediction** | ~100ms | ~10ms |
| **Memory** | Medium (coefficients) | Low (patterns) |
| **Data needs** | 1-7 days | 3-24 hours |
| **Best for** | Multi-day patterns | Hourly/daily patterns |
| **Accuracy** | Higher (if tuned) | Good (auto-tuned) |

## Example: Weekday/Weekend Pattern

**Scenario:** E-commerce site with 5x more traffic on weekends

### Data Pattern
```
Mon-Fri: 100 RPS
Sat-Sun: 500 RPS
```

### Configuration
```yaml
forecaster:
  model: arima
  arima-p: 7        # Look back 7 days
  arima-d: 0        # No trend, just weekly cycle
  arima-q: 1        # Small error correction
  window: 14d       # Two weeks to learn pattern
  step: 1h
  horizon: 24h
```

### Expected Behavior

**On Friday 6pm:**
```
Forecast for Saturday:
  └─> ARIMA looks back to last Saturday (p=7)
  └─> Last Saturday: 500 RPS
  └─> Current Friday: 100 RPS
  └─> Prediction: ~500 RPS for tomorrow ✅
```

**Result:** KEDA scales up Friday night, ready for Saturday traffic!

**On Sunday 6pm:**
```
Forecast for Monday:
  └─> ARIMA looks back to last Monday (p=7)
  └─> Last Monday: 100 RPS
  └─> Current Sunday: 500 RPS
  └─> Prediction: ~100 RPS for tomorrow ✅
```

**Result:** KEDA scales down Sunday night, saves cost on Monday!

## Summary

**The ARIMA Model is ideal for:**
- ✅ Weekly or monthly patterns
- ✅ Multi-day autocorrelation
- ✅ Workloads with 1-7 days of history
- ✅ Scenarios requiring statistical rigor

**Key Strengths:**
- Handles complex temporal dependencies
- Proven statistical methodology
- Flexible parameter tuning
- Good for long-cycle patterns

**When to Use:**
- You've tried baseline and need more sophistication
- Patterns span multiple days
- You have sufficient training data (≥ 2 pattern cycles)
- Prediction latency <200ms is acceptable

**Quick Start:** Use ARIMA(1,1,1) with 24-hour window as a starting point, then tune based on observed performance.
