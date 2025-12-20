# Prometheus Data Collection Limits

## Overview

Kedastral fetches historical metrics from Prometheus using the `/api/v1/query_range` endpoint. The number of data points returned is determined by:

```
data_points = WINDOW / STEP
```

**Example:** `WINDOW=24h` and `STEP=1m` → `1440 data points`

## Prometheus Limits

Prometheus enforces a maximum number of samples per query:
- **Default limit:** 11,000 samples
- **Configurable via:** `--query.max-samples` flag

If your query exceeds this limit, Prometheus will return an error:
```
error: query processing would load too many samples into memory
```

## Recommended Configurations

### Safe Combinations ✅

| Use Case | Window | Step | Data Points | Notes |
|----------|--------|------|-------------|-------|
| **Short-term patterns** | 3h | 1m | 180 | Ideal for baseline model with hourly spikes |
| **Daily patterns** | 24h | 1m | 1,440 | Good for baseline model with daily cycles |
| **Weekly patterns** | 7d | 1m | 10,080 | Max for 1-min resolution (near limit) |
| **Weekly patterns** | 7d | 5m | 2,016 | Safer for ARIMA weekly patterns |
| **Monthly patterns** | 30d | 5m | 8,640 | ARIMA monthly cycles |
| **Long-term trends** | 30d | 15m | 2,880 | ARIMA with coarser resolution |

### Unsafe Combinations ⚠️

| Window | Step | Data Points | Issue |
|--------|------|-------------|-------|
| 30d | 1m | 43,200 | **Exceeds default limit** |
| 90d | 1m | 129,600 | **Exceeds default limit** |
| 7d | 30s | 20,160 | **Exceeds default limit** |

## Choosing Step Size

### For Baseline Model (3-24h window)

**Recommended:** `STEP=1m` (1 minute)

```bash
WINDOW=3h
STEP=1m
# → 180 data points ✅
```

**Why:**
- Fine-grained pattern detection
- Well under Prometheus limits
- Captures minute-of-hour seasonality

### For ARIMA Model (1-7 days window)

**Recommended:** `STEP=1m` to `5m` depending on window

**Weekly pattern:**
```bash
WINDOW=7d
STEP=1m
# → 10,080 data points ⚠️ (close to limit, but safe)
```

**Or safer:**
```bash
WINDOW=7d
STEP=5m
# → 2,016 data points ✅ (plenty of headroom)
```

**Why:**
- 5-minute resolution still captures hourly/daily patterns
- Much safer for long windows
- Reduces training time for ARIMA

### For ARIMA Model (30+ days window)

**Required:** `STEP=5m` or higher

```bash
WINDOW=30d
STEP=5m
# → 8,640 data points ✅
```

**Or:**
```bash
WINDOW=30d
STEP=15m
# → 2,880 data points ✅ (very safe)
```

## Auto-Scaling Step Based on Window

If you want to automatically adjust step size to stay under limits:

```bash
# For windows ≤ 7 days: use 1 minute
if [ $WINDOW_DAYS -le 7 ]; then
    STEP=1m
# For windows > 7 days: use 5 minutes
else
    STEP=5m
fi
```

**Calculation:**
```
max_points = 10000  # Leave 1000 point buffer below 11k limit
step_seconds = window_seconds / max_points
```

## Checking Your Configuration

### Calculate Data Points

```bash
# Your current config
WINDOW=24h
STEP=1m

# Calculate points
window_seconds=$((24 * 3600))    # 86400
step_seconds=60
data_points=$((window_seconds / step_seconds))

echo "Data points: $data_points"
# Output: Data points: 1440 ✅
```

### Test Prometheus Query

```bash
# Test if your configuration works
curl "http://prometheus:9090/api/v1/query_range?query=up&start=$(($(date +%s) - 86400))&end=$(date +%s)&step=60"

# Check for error
# If you see: "error": "query processing would load too many samples"
# → Reduce WINDOW or increase STEP
```

## Error Handling

If Kedastral encounters a Prometheus limit error, you'll see in the logs:

```
level=error msg="adapter collect failed" error="prometheus: status 422"
```

**Fix:**
1. Check Prometheus logs for exact error
2. Calculate your data points: `WINDOW / STEP`
3. If > 10,000: increase `STEP` or decrease `WINDOW`
4. Redeploy forecaster with new config

## Prometheus Configuration

If you control your Prometheus instance and need higher limits:

**Increase max samples:**
```yaml
# prometheus.yml or command-line flag
--query.max-samples=50000
```

**Or in Helm values:**
```yaml
prometheus:
  server:
    extraArgs:
      query.max-samples: 50000
```

**Caution:** Higher limits increase memory usage during queries!

## Best Practices

1. **Start conservative:** Use 1-minute step only for windows ≤ 7 days
2. **Monitor data points:** Log the number of rows returned by adapter
3. **Consider step size:** 5-minute resolution is sufficient for most patterns
4. **Test before deploying:** Verify your Prometheus query works manually
5. **Match to model needs:**
   - Baseline (intra-day patterns): 1-min step OK
   - ARIMA (weekly patterns): 5-min step sufficient

## Example Configurations

### Baseline: Hourly Spikes (Recommended)

```yaml
forecaster:
  window: 3h
  step: 1m
  # → 180 points ✅
```

### Baseline: Daily Patterns

```yaml
forecaster:
  window: 24h
  step: 1m
  # → 1,440 points ✅
```

### ARIMA: Weekly Patterns (Safe)

```yaml
forecaster:
  window: 7d
  step: 5m
  # → 2,016 points ✅
```

### ARIMA: Weekly Patterns (Maximum Detail)

```yaml
forecaster:
  window: 7d
  step: 1m
  # → 10,080 points ⚠️ (safe but close to limit)
```

### ARIMA: Monthly Patterns

```yaml
forecaster:
  window: 30d
  step: 5m
  # → 8,640 points ✅
```

## Summary

✅ **Kedastral correctly fetches the full WINDOW you specify**

⚠️ **But you must ensure:** `WINDOW / STEP ≤ 10,000 data points`

📊 **Best practice:** Use 1-minute step for ≤24h windows, 5-minute step for longer windows
