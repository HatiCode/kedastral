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
| \(v_i\) | `forecast[i]` | predicted metric value (RPS, CPU, etc.) | `[120,130,125,140,100]` |
| \(T\) | `TargetPerPod` | how much load a single pod can handle | `50` RPS/pod |
| \(H\) | `Headroom` | multiplicative safety margin | `1.2` |
| \(L\) | `LeadTimeSeconds` | how far ahead we scale | `60` seconds |
| \(S\) | `stepSec` | forecast step resolution | `60` seconds |
| \(U\) | `UpMaxFactorPerStep` | maximum growth factor per step | `2.0` (2× per step) |
| \(D\) | `DownMaxPercentPerStep` | maximum shrink percentage per step | `50` |
| `prev` | previous output replicas | `2` |
| `min,max` | bounds | `1, 100` |

---

## 2️⃣ Forecast Translation

For each time step \(i\):

### Step A — Compute base requirement
\[
\text{rawPods}_i = \frac{v_i}{T}
\]

### Step B — Apply headroom
\[
\text{adjPods}_i = \text{rawPods}_i \times H
\]

---

## 3️⃣ Lead Time Offset

Lead time shifts which forecast value is used for decision-making:

\[
i_0 = \left\lceil \frac{L}{S} \right\rceil
\]

Each decision step \(i\) uses the forecast value at \(i + i_0\)
(to scale early by \(L\) seconds).

Example:
If `LeadTimeSeconds=60` and `stepSec=60`, then \(i_0=1\).

- At step 0 → use `forecast[1]` (130 RPS)
- At step 1 → use `forecast[2]` (125 RPS)
- etc.

---

## 4️⃣ Rounding

Pods are discrete, so we round up (default “ceil” mode):

\[
\text{roundedPods}_i = \lceil \text{adjPods}_{i + i_0} \rceil
\]

Rounding up (instead of down) ensures we don’t under-provision.

---

## 5️⃣ Bounds

Clamp between min/max replicas:

\[
\text{bounded}_i = \min( \max(\text{roundedPods}_i, \text{MinReplicas}), \text{MaxReplicas})
\]

If `MaxReplicas=0`, it means “no upper bound”.

---

## 6️⃣ Change Clamps (Smoothing)

To avoid thrashing, we limit how much the replica count
can change between two consecutive steps.

Let \(r_{i-1}\) = previous step’s replicas (starting from `prev`).

### a. Upscaling Clamp

\[
\text{maxUp} = \lceil r_{i-1} \times U \rceil
\]
\[
r_i = \min(\text{bounded}_i, \text{maxUp})
\]

### b. Downscaling Clamp

\[
\text{minDown} = \lfloor r_{i-1} \times (1 - \frac{D}{100}) \rfloor
\]
\[
r_i = \max(r_i, \text{minDown})
\]

This guarantees:
- No growth > `UpMaxFactorPerStep`
- No shrink > `DownMaxPercentPerStep`

---

## 7️⃣ Final Bounds (Safety Net)

After clamps, re-apply global bounds to enforce the policy limits:

\[
r_i = \min(\max(r_i, \text{MinReplicas}), \text{MaxReplicas})
\]

---

## 8️⃣ Full Formula (summary)

Putting it all together:

\[
r_i = \text{ClampBounds}\Big(
\text{ClampChange}\big(
\text{Round}(H \cdot \frac{v_{i+i_0}}{T}),
r_{i-1},
U,
D
\big),
\text{MinReplicas},
\text{MaxReplicas}
\Big)
\]

---

## 🧩 Example Walkthrough

### Given:

| Param | Value |
|:------|:------|
| `forecast` | `[120,130,125,140,100]` |
| `TargetPerPod` | 50 |
| `Headroom` | 1.2 |
| `LeadTimeSeconds` | 60 |
| `stepSec` | 60 |
| `UpMaxFactorPerStep` | 2.0 |
| `DownMaxPercentPerStep` | 50 |
| `MinReplicas` | 1 |
| `MaxReplicas` | 100 |
| `prev` | 2 |

### Step-by-step

| i | forecast used | rawPods | adjPods | ceil | clamp | result |
|---|----------------|---------|---------|------|--------|--------|
| 0 | 130 | 2.6 | 3.12 | 4 | up ok (2→4) | 4 |
| 1 | 125 | 2.5 | 3.00 | 3 | down ok (4→3) | 3 |
| 2 | 140 | 2.8 | 3.36 | 4 | up ok (3→4) | 4 |
| 3 | 100 | 2.0 | 2.40 | 3 | down ok (4→3) | 3 |
| 4 | 100 | 2.0 | 2.40 | 3 | steady | 3 |

✅ Final output: `[4, 3, 4, 3, 3]`

---

## 9️⃣ Interpretation

| Component | Protects Against | Effect |
|------------|------------------|---------|
| `Headroom` | Forecast errors | Safer scaling up |
| `LeadTime` | Latency to warm pods | Pre-scales |
| `Ceil rounding` | Fractional pods | Avoids under-scaling |
| `Up/Down clamps` | Spikes and thrash | Smooths transitions |
| `Bounds` | Bad configs | Cost + SLO safety |

---

## 🔍 Design Principles

1. **Predictable:** each parameter changes only one behavior dimension.
2. **Explainable:** all steps are linear and transparent.
3. **Tunable:** can match different workloads (steady APIs vs bursty games).
4. **Composable:** works with reactive KEDA/HPA — KEDA will take the *max* between predictive and reactive metrics.
5. **Deterministic:** same inputs → same outputs, no stochastic ML noise.

---

## 🧠 Pro Tip: “Mental Model”

Imagine each pod is a bucket that can hold `TargetPerPod` requests/sec.

We:
1. Predict how many buckets we’ll need in 1 minute (`LeadTime`).
2. Add 20% extra water space (`Headroom`).
3. Always round up to a whole bucket.
4. Don’t buy or throw away more than 2× or 50% of buckets per minute.

---

## 🧩 Optional Extensions

Future versions could include:
- **Windowed lead time** (for burst anticipation)
- **Dynamic headroom** (learned from forecast confidence)
- **Cost-aware planners** (optimize for cloud price/SLO trade-off)
- **Custom rounding modes** (bankers, stochastic)

---

## ✅ Key Takeaway

Kedastral’s planner is a **deterministic scaling function**:

\[
\text{forecast} \to \text{safe, smooth, explainable replica plan}
\]

It bridges raw metrics and actual autoscaling in a way that’s both *scientifically grounded* and *operationally sane*.
