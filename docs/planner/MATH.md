# 🧮 Kedastral Capacity Planner Math

## Overview

Kedastral’s **capacity planner** converts a predicted workload (for example, requests per second)
into a number of **replicas** that Kubernetes should run, while respecting:

- **Lead time** (scale early)
- **Headroom** (safety margin)
- **Rounding** (pods are integers)
- **Change clamps** (avoid thrash)
- **Bounds** (min/max limits)

The result is a deterministic, explainable number of pods for each future time step.

---

## 1️⃣ Inputs

| Symbol | Variable | Meaning | Example |
|:-------|:----------|:---------|:---------|
| `v_i` | `forecast[i]` | Predicted metric value (RPS, CPU, etc.) | `[120,130,125,140,100]` |
| `T` | `TargetPerPod` | How much load a single pod can handle | `50` RPS/pod |
| `H` | `Headroom` | Multiplicative safety margin | `1.2` |
| `L` | `LeadTimeSeconds` | How far ahead we scale | `60` seconds |
| `S` | `stepSec` | Forecast step resolution | `60` seconds |
| `U` | `UpMaxFactorPerStep` | Maximum growth factor per step | `2.0` (2× per step) |
| `D` | `DownMaxPercentPerStep` | Maximum shrink percentage per step | `50` |
| `prev` | Previous output replicas | `2` |
| `min,max` | Bounds | `1, 100` |

---

## 2️⃣ Forecast Translation

For each time step `i`:

### Step A — Compute base requirement
```
rawPods_i = v_i / T
```

### Step B — Apply headroom
```
adjPods_i = rawPods_i * H
```

This provides the “adjusted” number of pods needed to safely handle the predicted load.

---

## 3️⃣ Lead Time Offset

Lead time shifts which forecast value is used for decision-making:

```
i0 = ceil(L / S)
```

Each decision step `i` uses the forecast value at `i + i0`
(to scale early by `L` seconds).

Example:
If `LeadTimeSeconds = 60` and `stepSec = 60`, then `i0 = 1`.

- At step 0 → use `forecast[1]` (130 RPS)
- At step 1 → use `forecast[2]` (125 RPS)
- etc.

---

## 4️⃣ Rounding

Pods are discrete, so we round up (default **ceil** mode):

```
roundedPods_i = ceil(adjPods_{i + i0})
```

Rounding up ensures we don’t under-provision.

---

## 5️⃣ Bounds

Clamp between min and max replicas:

```
bounded_i = min(max(roundedPods_i, MinReplicas), MaxReplicas)
```

If `MaxReplicas = 0`, it means “no upper bound”.

---

## 6️⃣ Change Clamps (Smoothing)

To avoid thrashing, the planner limits how much the replica count
can change between two consecutive steps.

Let `r_{i-1}` = previous step’s replicas (starting from `prev`).

### a. Upscaling Clamp

```
maxUp = ceil(r_{i-1} * U)
r_i   = min(bounded_i, maxUp)
```

### b. Downscaling Clamp

```
minDown = floor(r_{i-1} * (1 - D/100))
r_i     = max(r_i, minDown)
```

This guarantees:
- No growth > `UpMaxFactorPerStep`
- No shrink > `DownMaxPercentPerStep`

---

## 7️⃣ Final Bounds (Safety Net)

After clamps, apply global bounds again:

```
r_i = min(max(r_i, MinReplicas), MaxReplicas)
```

---

## 8️⃣ Full Formula (Summary)

Putting everything together:

```
r_i = ClampBounds(
        ClampChange(
            Round(H * forecast[i + i0] / T),
            r_{i-1},
            U,
            D
        ),
        MinReplicas,
        MaxReplicas
     )
```

---

## 9️⃣ Example Walkthrough

### Parameters

| Param | Value |
|:------|:------|
| `forecast` | `[120,130,125,140,100]` |
| `TargetPerPod` | `50` |
| `Headroom` | `1.2` |
| `LeadTimeSeconds` | `60` |
| `stepSec` | `60` |
| `UpMaxFactorPerStep` | `2.0` |
| `DownMaxPercentPerStep` | `50` |
| `MinReplicas` | `1` |
| `MaxReplicas` | `100` |
| `prev` | `2` |

### Step-by-step Calculation

| i | forecast used | rawPods | adjPods | ceil | clamp | result |
|---|----------------|---------|---------|------|--------|--------|
| 0 | 130 | 2.6 | 3.12 | 4 | up ok (2→4) | 4 |
| 1 | 125 | 2.5 | 3.00 | 3 | down ok (4→3) | 3 |
| 2 | 140 | 2.8 | 3.36 | 4 | up ok (3→4) | 4 |
| 3 | 100 | 2.0 | 2.40 | 3 | down ok (4→3) | 3 |
| 4 | 100 | 2.0 | 2.40 | 3 | steady | 3 |

✅ **Final Output:** `[4, 3, 4, 3, 3]`

---

## 🔍 Interpretation

| Component | Protects Against | Effect |
|------------|------------------|---------|
| `Headroom` | Forecast errors | Safer scaling up |
| `LeadTime` | Pod startup delay | Pre-scales |
| `Ceil rounding` | Fractional pods | Avoids under-scaling |
| `Up/Down clamps` | Rapid fluctuations | Smooths transitions |
| `Bounds` | Misconfiguration | Prevents extremes |

---

## 🔧 Design Principles

1. **Predictable:** each parameter influences only one aspect of scaling.
2. **Explainable:** all steps are linear and transparent.
3. **Tunable:** works for both steady and bursty workloads.
4. **Composable:** integrates with reactive KEDA/HPA — KEDA takes the *max* between predictive and reactive metrics.
5. **Deterministic:** same inputs → same outputs.

---

## 🧠 Mental Model

Imagine each pod is a bucket that can hold `TargetPerPod` requests per second.

We:
1. Predict how many buckets we’ll need in `LeadTime` seconds.
2. Add 20 % extra room (`Headroom`).
3. Always round up to a full bucket.
4. Never add or remove more than allowed by the change clamps.

---

## 🧩 Possible Future Enhancements

- **Windowed lead time:** look ahead over multiple steps (burst anticipation)
- **Dynamic headroom:** adjust based on forecast confidence
- **Cost-aware planning:** optimize for price vs. SLO
- **Alternative rounding modes:** `round`, `floor`, stochastic rounding

---

## ✅ Key Takeaway

The Kedastral planner is a **deterministic scaling function**:

```
forecast → safe, smooth, explainable replica plan
```

It bridges raw metric forecasts and Kubernetes autoscaling
in a way that’s **scientifically grounded** and **operationally practical**.
