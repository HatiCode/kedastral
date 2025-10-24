# SPEC.md — Kedastral v0.1 (Spec‑Driven Development)

> **Audience:** AI coding assistants (Claude) and engineers implementing Kedastral.  
> **Goal:** Provide an *executable* specification: precise behaviors, APIs, flags, data shapes, performance budgets, and acceptance tests so code can be generated and validated without ambiguity.

---

## 0. Scope & Success Criteria

**Product:** Kedastral — a predictive autoscaling companion for **KEDA**.  
**Version:** v0.1 (MVP).  
**Language:** Go ≥ 1.25.  
**Outcomes:**

1. A **Forecaster** service that:
   - Collects metrics via adapters (MVP: **Prometheus**).
   - Predicts near‑term load using a **baseline model** (moving average + simple seasonality).
   - Computes **desired replicas** using a **capacity policy**.
   - Exposes the **latest forecast snapshot** via HTTP `GET /forecast/current`.
2. An **External Scaler** service that:
   - Implements KEDA’s **External Scaler gRPC** methods.
   - Returns **absolute desired replicas**.
3. A minimal **Helm** chart and **examples** to wire with KEDA/HPA.
4. **Observability**: Prometheus metrics for key actions.
5. **Tests** and acceptance criteria outlined below pass locally.

Out of scope for v0.1: Operator/controller, real CRD reconciliation, ML training, persistence beyond forecast window (Redis optional).

---

## 1. Architecture (Authoritative)

Components and dependencies:

- **Forecaster (HTTP 8081)**  
  Inputs: Prometheus HTTP API.  
  Outputs: in‑memory snapshot; HTTP endpoint; Prometheus metrics.

- **External Scaler (gRPC 8080)**  
  Inputs: Forecaster HTTP endpoint.  
  Outputs: KEDA gRPC responses.

- **KEDA/HPA** (existing in cluster): consumes scaler output.

### 1.1 State Model

- Forecaster maintains **one snapshot per workload** in memory: `Snapshot{generatedAt, stepSec, horizonSec, values[], desired[]}`.
- Snapshot **expires** after `staleAfter` (default 120s). Scaler must treat expired as **inactive** and/or return `defaultMin` replicas.

### 1.2 Sequence (Happy Path)

1. Forecaster tick fires (every `interval`, default 30s).
2. Forecaster pulls recent metrics (`window`).
3. Forecaster builds features and predicts `values[]` (length = `horizon/step`).
4. Forecaster computes `desired[]` via capacity policy and clamps.
5. Scaler polled by KEDA: reads snapshot and returns **current desired** (index at `leadTime`).

---

## 2. Configuration (Flags & Env)

### 2.1 Forecaster flags

| Flag | Env | Type | Default | Description |
|-----|-----|------|---------|-------------|
| `--listen` | `FORECASTER_LISTEN` | string | `:8081` | HTTP bind address |
| `--workload` | `WORKLOAD` | string | **required** | Workload key for snapshot |
| `--metric` | `METRIC` | string | `http_rps` | Metric name being forecast |
| `--horizon` | `HORIZON` | duration | `30m` | Forecast horizon |
| `--step` | `STEP` | duration | `1m` | Forecast step |
| `--lead-time` | `LEAD_TIME` | duration | `5m` | Pre‑scaling lead time |
| `--interval` | `INTERVAL` | duration | `30s` | Forecast loop tick |
| `--window` | `WINDOW` | duration | `30m` | Metrics lookback window |
| `--target-per-pod` | `TARGET_PER_POD` | float | `200` | Capacity model target |
| `--headroom` | `HEADROOM` | float | `1.2` | Safety multiplier |
| `--min` | `MIN_REPLICAS` | int | `2` | Lower bound |
| `--max` | `MAX_REPLICAS` | int | `50` | Upper bound |
| `--up-max-factor` | `UP_MAX_FACTOR` | float | `2.0` | Max upward change per step (×) |
| `--down-max-pct` | `DOWN_MAX_PCT` | int | `50` | Max downward change per step (%) |
| `--prom-url` | `PROM_URL` | string | **required** | Prometheus base URL |
| `--prom-query` | `PROM_QUERY` | string | **required** | PromQL (range/instant supported) |
| `--stale-after` | `STALE_AFTER` | duration | `2m` | Snapshot staleness threshold |
| `--log-level` | `LOG_LEVEL` | string | `info` | `debug|info|warn|error` |

### 2.2 Scaler flags

| Flag | Env | Type | Default | Description |
|-----|-----|------|---------|-------------|
| `--listen` | `SCALER_LISTEN` | string | `:8080` | gRPC bind |
| `--forecaster-url` | `FORECASTER_URL` | string | **required** | `http://<svc>:8081` |
| `--workload` | `WORKLOAD` | string | **required** | Workload key to query |
| `--default-min` | `DEFAULT_MIN` | int | `2` | Fallback replicas |
| `--stale-after` | `STALE_AFTER` | duration | `2m` | Treat older snapshot as stale |
| `--log-level` | `LOG_LEVEL` | string | `info` | `debug|info|warn|error` |

**Note:** Flags override env when both provided.

---

## 3. External Contracts

### 3.1 Snapshot HTTP API (Forecaster)

- **Path:** `GET /forecast/current?workload=<string>`
- **Success (200):**
```json
{
  "workload": "my-api",
  "metric": "http_rps",
  "generatedAt": "2025-10-16T17:35:00Z",
  "stepSeconds": 60,
  "horizonSeconds": 1800,
  "values": [420.0, 415.2, 430.9],
  "desiredReplicas": [8, 8, 9]
}
```
- **Not Found (404):**
```json
{"error":"snapshot not found"}
```
- **Stale (200 + header):** Add header `X-Kedastral-Stale: true` if now - generatedAt > `staleAfter`.

### 3.2 KEDA External Scaler (gRPC)

Implement KEDA v2 External Scaler interface:
- `IsActive(ScaledObjectRef) -> bool`
- `GetMetricSpec(ScaledObjectRef) -> MetricSpec{MetricName:"desired_replicas",TargetSize:1}`
- `GetMetrics(GetMetricsRequest{MetricName:"desired_replicas"}) -> MetricValue{MetricValue:int64}`

**Behavioral rules:**
- If snapshot **missing or stale** → `IsActive=false`.  
- `GetMetrics`: return `default-min` if stale/missing; else return **current desired** (see §4.4).

---

## 4. Algorithms & Determinism

### 4.1 Adapter: Prometheus

- Use HTTP Range Query:
  - Endpoint: `/api/v1/query_range`
  - Params: `query=<PROM_QUERY>`, `start=now-WINDOW`, `end=now`, `step=<STEP>`
- Accept also instant `/api/v1/query` if range not supported; in that case synthesize a short series for EMA.
- Map to `DataFrame{Rows:[{"ts":RFC3339,"value":float64}, ...]}`.

### 4.2 Baseline Model

- Compute **EMA5m** and **EMA30m** over the most recent window.  
- Base forecast value = `0.7*EMA5m + 0.3*EMA30m`.  
- Optional seasonality: if enough historic points aligned by hour‑of‑day `h` exist, compute `Mean_h` and blend:  
  `yhat = 0.8*Base + 0.2*Mean_h`.  
- Output `Values[K]` where `K = horizon/step`. All values are **non‑negative**.

### 4.3 Capacity Planner

Given `Values[]`, `stepSec`, and `Policy`:
1. `raw[i] = Values[i] / TargetPerPod`
2. `adj[i] = raw[i] * Headroom`
3. Let `i0 = ceil(LeadTimeSeconds/stepSec)`.  
   Define **pre‑scale window** `W = [i0, min(i0+2, len(adj)-1)]`.  
   `pre[i] = max(adj[W])` for the **current return value**.
4. **Clamps** from previous output `prev`:
   - Upward: `maxUp = ceil(prev * UpMaxFactorPerStep)`
   - Downward: `maxDown = floor(prev * (1 - DownMaxPercentPerStep/100))`
   - `clamped = min(max(pre, maxDown), maxUp)`
5. `desired = ceil(clamped)` then bound to `[Min, Max]`.
6. For the exported `desired[]` series, apply steps 1–5 for all i cumulatively using previous element as `prev`.

### 4.4 Scaler “current desired” value

- Compute **current index** `i0` as in 4.3.  
- Return `desired[i0]` from the latest snapshot.  
- If `i0 >= len(desired)` return last element.  
- If snapshot stale/missing, return `default-min`.

Determinism requirement: same inputs → same outputs. No random sources.

---

## 5. Telemetry (Prometheus)

Forecaster MUST expose on `/metrics`:
- `kedastral_adapter_collect_seconds{adapter="prometheus"}` (histogram)
- `kedastral_model_predict_seconds{model="baseline"}` (histogram)
- `kedastral_capacity_compute_seconds` (histogram)
- `kedastral_forecast_age_seconds{workload="..."}` (gauge)
- `kedastral_desired_replicas{workload="..."}` (gauge)
- `kedastral_errors_total{component="...",reason="..."}` (counter)

Scaler MUST expose on `/metrics`:
- `kedastral_scaler_requests_total{method="IsActive|GetMetricSpec|GetMetrics",status="ok|error"}`
- `kedastral_scaler_snapshot_stale_total`
- `kedastral_scaler_returned_replicas` (gauge)

---

## 6. Logging

- Use `log/slog`. JSON format selectable via env `LOG_FORMAT=json`.
- Fields: `ts`, `level`, `component`, `workload`, `msg`, `err`.
- No secrets in logs. Truncate PromQL strings at 200 chars.

---

## 7. HTTP & gRPC Servers

- Graceful shutdown: SIGTERM → stop accepting; drain ≤ 10s; exit 0.
- Timeouts: HTTP client to Prometheus: connect 2s, overall 10s; 2 retries with backoff 100ms, 300ms.
- Health endpoints:
  - Forecaster: `GET /healthz` returns 200 always; add `X-Kedastral-Stale: true` if last snapshot > `staleAfter`.
  - Scaler: `GET /healthz` 200.

---

## 8. Security

- All servers bind to specified interfaces only.
- Optional TLS: out of scope v0.1; code structure should enable it later.
- Validate all inputs: durations > 0, min ≤ max, non-empty workload name.

---

## 9. Example YAML (KEDA wiring)

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: my-api-kedastral
spec:
  scaleTargetRef:
    name: my-api
  minReplicaCount: 2
  maxReplicaCount: 50
  triggers:
  - type: external
    metadata:
      scalerAddress: kedastral-scaler.default.svc.cluster.local:8080
```

Helm values should expose env/flags to both pods to align with this spec.

---

## 10. Directory & Files (Must Exist)

```
cmd/forecaster/main.go
cmd/scaler/main.go
pkg/adapters/adapter.go
pkg/adapters/prometheus.go
pkg/models/model.go
pkg/models/baseline.go
pkg/capacity/planner.go
pkg/storage/store.go
pkg/storage/memory.go
pkg/client/forecaster.go
deploy/helm/Chart.yaml
deploy/examples/prometheus-source.yaml
deploy/examples/api-forecast-policy.yaml
docs/README.md (or root README.md)
docs/architecture.md
SPEC.md (this file)
```

---

## 11. Acceptance Tests (Executable Requirements)

Implement unit tests such that **all** pass:

### 11.1 Capacity Planner
- GIVEN `prev=5`, `TargetPerPod=100`, `Headroom=1.2`, `LeadTimeSeconds=120`, `step=60` and `Values=[400,500,1000]`  
  EXPECT returned current desired = `ceil(max(Values[2:4]/100*1.2)) = ceil(1000/100*1.2)=12`, then clamped/bounded.

### 11.2 Baseline Model
- GIVEN monotonically increasing series 100..200 over 30m THEN predictions must be non-decreasing and within `[last, last*1.5]`.

### 11.3 Prometheus Adapter
- GIVEN fake HTTP server returns range vector of 10 points; Collect must return `len(Rows)=10`, increasing `ts`, numeric `value`.

### 11.4 Forecaster HTTP
- AFTER one tick, `GET /forecast/current?workload=...` RETURNS 200 with non-empty `values` and `desiredReplicas` lengths = `horizon/step`.

### 11.5 Scaler gRPC
- GIVEN fresh snapshot with `desired[i0]=8`, `GetMetrics` RETURNS `MetricValue=8`.
- GIVEN stale snapshot (> `staleAfter`), RETURNS `default-min`.

Document tests in `*_test.go`; aim for 80% coverage in `capacity`, `adapters`, `storage` packages.

---

## 12. Performance Budgets (v0.1)

- Forecaster tick end‑to‑end (collect → publish): **p95 < 500ms** for 30 points series.
- Prometheus call: **p95 < 300ms** (with local cluster Prometheus).
- Scaler `GetMetrics` latency: **p95 < 20ms** (excluding network).

---

## 13. Error Codes / Handling

HTTP 4xx for bad inputs, 5xx for internal errors. JSON body: `{"error":"<msg>"}`.  
gRPC errors: use canonical codes (`Unavailable`, `InvalidArgument`) when appropriate, but prefer successful responses with fallback values to keep KEDA stable.

---

## 14. Future Compatibility Notes

- All public types should be in versioned packages or be backwards‑compatible.  
- Avoid breaking fields in snapshot JSON; add new fields only.

---

## 15. Implementation Order (Recommended)

1. `pkg/capacity/planner.go` + tests.
2. `pkg/adapters/adapter.go` and `prometheus.go` + tests (fake server).
3. `pkg/models/baseline.go` + tests.
4. `pkg/storage/memory.go` + tests.
5. `cmd/forecaster/main.go` (wire everything; health, metrics, snapshot API).
6. `cmd/scaler/main.go` (gRPC; call forecaster; serve metrics).
7. Helm skeleton + example YAMLs.
8. Smoke test locally with synthetic Prometheus.

---

## 16. Prompts (For Claude)

**General system prompt:**  
“You are a senior Go engineer. Implement Kedastral v0.1 exactly per SPEC.md. Use Go 1.25, stdlib, table‑driven tests, slog logging, context for I/O, and no third‑party deps unless required.”

**Task prompts:** (split as PRs matching §15)

- *Capacity Planner:* “Create `pkg/capacity/planner.go` and tests implementing §4.3 exactly.”  
- *Prometheus Adapter:* “Create `pkg/adapters/prometheus.go` and tests implementing §4.1; use an httptest server.”  
- *Baseline Model:* “Create `pkg/models/baseline.go` and tests implementing §4.2.”  
- *Forecaster Main:* “Create `cmd/forecaster/main.go` per §2.1, §3.1, §7, §5.”  
- *Scaler Main:* “Create `cmd/scaler/main.go` per §2.2, §3.2, §7, §5.”

---

**End of SPEC.md** — This file is the single source of truth for Kedastral v0.1 behavior.
