# CLAUDE.md — Kedastral Code Generation Guide

> This file is written **for AI coding assistants (Claude)** to implement Kedastral.  
> Humans can read it too. It explains **what to build, how to structure the repo, coding standards, acceptance tests, and task breakdowns**. If you (Claude) are asked to write code, **follow this document precisely**.

---

## 0) TL;DR

- **Project**: Kedastral — predictive autoscaling companion for **KEDA**.
- **Language**: **Go** (>= 1.23).
- **Module**: `github.com/kedastral/kedastral`.
- **Deliverables**: 
  - `cmd/forecaster`: Forecast Engine (collect → predict → replicas → snapshot API).
  - `cmd/scaler`: KEDA External Scaler (gRPC) returning **desired replicas**.
  - CRDs: `ForecastPolicy`, `DataSource` with a controller-free “config-driven” v0.1.
  - Adapters: Prometheus (MVP).
  - Model: `baseline` (no training), optional BYOM HTTP contract.
  - Storage: in-memory (MVP) + Redis (optional).
  - Helm chart + examples + Grafana dashboard JSON (MVP stubs ok).

**Scope for v0.1**: end-to-end predictive scaling working with Prometheus input, baseline model, capacity planner, REST snapshot, and External Scaler feeding KEDA/HPA.

---

## 1) Non‑Goals (v0.1)

- No operator/controller. Use CRDs as static config read by Kedastral pods (or ConfigMaps).
- No heavy ML. Baseline + optional BYOM HTTP endpoint only.
- No persistence requirements beyond forecast window (Redis optional).
- No cross-namespace multi-tenancy guarantees (best effort).
- No RBAC beyond what’s needed for pods/services/endpoints.

---

## 2) Repository Layout

```
kedastral/
├─ cmd/
│  ├─ forecaster/           # main.go (HTTP server, forecast loop)
│  └─ scaler/               # main.go (gRPC External Scaler)
├─ pkg/
│  ├─ adapters/
│  │  ├─ adapter.go         # interfaces
│  │  └─ prometheus.go      # PrometheusAdapter (MVP)
│  ├─ models/
│  │  ├─ model.go           # interfaces
│  │  └─ baseline.go        # moving-average + seasonal baseline
│  ├─ capacity/
│  │  ├─ planner.go         # Policy + ToReplicas()
│  ├─ storage/
│  │  ├─ store.go           # interfaces
│  │  ├─ memory.go          # in-memory store
│  │  └─ redis.go           # optional
│  ├─ client/
│  │  └─ forecaster.go      # client to read /forecast/current
│  ├─ api/
│  │  ├─ crd/               # YAML schemas (v0.1: used as config docs)
│  │  └─ proto/             # (optional) External types
│  ├─ features/
│  │  └─ builder.go         # simple feature engineering
│  └─ httpx/
│     └─ server.go          # shared HTTP helpers
├─ deploy/
│  ├─ helm/                 # chart skeleton
│  ├─ examples/             # example ForecastPolicy/DataSource
│  └─ grafana/              # dashboard JSON (MVP stub)
├─ docs/
│  ├─ README.md             # generated (repo root README is symlink or copy)
│  ├─ architecture.md
│  └─ quickstart.md         # TBD
├─ .github/
│  └─ workflows/ci.yml      # lint + test
├─ go.mod
└─ Makefile
```

> If any directory is missing, create it exactly as above.

---

## 3) Coding Standards (Go)

- Go 1.23. Use modules. Module path is `github.com/kedastral/kedastral`.
- `context.Context` as **first arg** for all I/O or long ops.
- Errors: `errors.Is`, `%w` wrapping. No global panics in libraries.
- Logging: structured; use `log/slog` (stdlib) with levels, request IDs.
- Concurrency: prefer `time.Ticker`, `select`, clean shutdown with `context`.
- Config: `env` + flags; default sane values. Support config file (YAML) later.
- Lint: `golangci-lint` with defaults.
- Tests: `go test ./...`, use table-driven tests, fast unit tests (<2s).

---

## 4) Protocols & Contracts

### 4.1 Snapshot REST (Forecaster → Scaler)

- Endpoint: `GET /forecast/current?workload=<name>`
- Response:
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

### 4.2 BYOM HTTP (optional)

- Endpoint: `POST /predict`
- Request:
```json
{ "now":"<RFC3339>", "horizonSeconds":1800, "stepSeconds":60, "features":[ { "ts":"...", "...":1.0 } ] }
```
- Response:
```json
{ "metric":"http_rps", "values":[420.0, 415.2, 430.9] }
```

### 4.3 KEDA External Scaler (gRPC)

- Use KEDA v2 proto. Implement:
  - `IsActive`
  - `GetMetricSpec`
  - `GetMetrics`
- Return **metricName** = `"desired_replicas"`, **MetricValue** = absolute replicas.

---

## 5) Interfaces (Copy from docs/architecture.md)

### 5.1 Adapters

```go
package adapters

import "context"

type Row map[string]any

type DataFrame struct {
    Rows []Row
}

type Adapter interface {
    Collect(ctx context.Context, windowSeconds int) (DataFrame, error)
    Name() string
}
```

#### Prometheus Adapter (MVP)
- Inputs: `serverURL`, `query`.
- Behavior: execute range query for last N minutes and aggregate to step size to produce rows with timestamps and values.
- Output columns: `ts (RFC3339)`, `value (float64)`; additional labels allowed.

### 5.2 Model

```go
package models

import "context"

type FeatureFrame struct { Rows []map[string]float64 }

type Forecast struct {
    Metric   string
    Values   []float64
    StepSec  int
    Horizon  int
}

type Model interface {
    Train(ctx context.Context, history FeatureFrame) error
    Predict(ctx context.Context, features FeatureFrame) (Forecast, error)
    Name() string
}
```

#### Baseline Model (MVP)
- Moving average of recent metric (EMA 5m + 30m), combine with hour-of-day/day-of-week seasonal means if available. 
- Output length = `horizon/step`.

### 5.3 Capacity Planner

```go
package capacity

type Policy struct {
    TargetPerPod          float64
    Headroom              float64
    LeadTimeSeconds       int
    MinReplicas           int
    MaxReplicas           int
    UpMaxFactorPerStep    float64
    DownMaxPercentPerStep int
}

func ToReplicas(prev int, forecast []float64, stepSec int, p Policy) []int
```

**Algorithm (MVP):**
1. `raw = v / TargetPerPod`
2. `adj = raw * Headroom`
3. Shift by `LeadTimeSeconds`: choose index `i0 = ceil(LeadTimeSeconds/stepSec)`; use max over `[i0, i0+2]` to pre-scale.
4. Clamp changes from `prev` by `UpMaxFactorPerStep` and `DownMaxPercentPerStep`.
5. `ceil(adj)`, bound to `[Min, Max]`.

### 5.4 Store

```go
package storage

import "time"

type Snapshot struct {
    Workload        string
    Metric          string
    GeneratedAt     time.Time
    StepSeconds     int
    HorizonSeconds  int
    Values          []float64
    DesiredReplicas []int
}

type Store interface {
    Put(s Snapshot) error
    GetLatest(workload string) (Snapshot, bool, error)
}
```

---

## 6) Executables

### 6.1 `cmd/forecaster/main.go` (MVP behavior)

- Flags/env:
  - `--listen=:8081`
  - `--workload=my-api`
  - `--metric=http_rps`
  - `--horizon=30m`, `--step=1m`, `--lead-time=5m`
  - `--target-per-pod=200`, `--headroom=1.2`, `--min=2`, `--max=50`
  - `--prom-url=http://prometheus:9090`, `--prom-query=...`
  - `--interval=30s`, `--window=30m`
- Loop:
  - Collect → Build features → Predict → ToReplicas → Store.Put → serve `/forecast/current`.
- HTTP:
  - `GET /healthz`
  - `GET /forecast/current?workload=<name>`

### 6.2 `cmd/scaler/main.go` (MVP behavior)

- Flags/env:
  - `--listen=:8080`
  - `--forecaster-url=http://kedastral-forecaster:8081`
  - `--default-min=2`
  - `--stale-after=120s`
- Implements KEDA gRPC:
  - `IsActive` → true if snapshot exists and not stale.
  - `GetMetricSpec` → `desired_replicas`, target 1.
  - `GetMetrics` → read `/forecast/current`, return **single integer** (current desired).

---

## 7) Testing Guidance

- **Unit tests**:
  - `capacity.ToReplicas`: test clamps, lead-time, bounds.
  - Baseline model: shapes and monotonicity over simple sequences.
  - Prometheus adapter: parse sample JSON into DataFrame.
  - Memory store: Put/GetLatest concurrency.
- **E2E (local)**:
  - Use a small Prometheus dev server (or stub) + synthetic series.
  - Run forecaster and scaler locally; print gRPC responses.
- **Backtest Skeleton**:
  - Command `cmd/backtest` optional; accepts CSV of actuals and replays policy.

---

## 8) Tooling, Build, CI

- `Makefile` targets:
  - `make build` → both binaries
  - `make test` → unit tests
  - `make lint` → golangci-lint
  - `make docker` → images `kedastral/forecaster`, `kedastral/scaler`
- GitHub Actions: run `go test`, `golangci-lint`, build docker on tag.

---

## 9) Security

- Don’t log secrets/queries verbatim.
- Enforce timeouts/retries for Prometheus HTTP calls.
- Optional mTLS between scaler and forecaster (future).
- Validate all inputs from config/CRDs.

---

## 10) Helm (MVP Skeleton)

- 2 Deployments + 1 Service each (forecaster: HTTP 8081, scaler: gRPC 8080).
- Config via values.yaml mapped to flags/env.
- Example `ScaledObject` pointing to scaler’s service.

---

## 11) Example Prompts for Claude (use exactly)

### 11.1 Implement Prometheus Adapter
**System**: “You are a senior Go engineer. Follow CLAUDE.md exactly.”  
**User**:
```
Create file pkg/adapters/prometheus.go implementing Adapter.
- Use http.Client with context timeouts.
- Input env: PROM_URL, PROM_QUERY.
- Support Collect(ctx, windowSeconds) by issuing a range query and returning DataFrame with columns ts, value.
- Include unit tests with a fake HTTP server.
- No third-party deps beyond standard lib.
```

### 11.2 Implement Capacity Planner
**User**:
```
Create file pkg/capacity/planner.go with Policy and ToReplicas as specified.
- Include edge-case tests for lead-time index, clamps, bounds.
```

### 11.3 Forecaster Main
**User**:
```
Create cmd/forecaster/main.go.
- Parse flags/env listed in CLAUDE.md.
- Wire PrometheusAdapter -> baseline model -> ToReplicas -> memory store.
- Expose /healthz and /forecast/current.
- Add graceful shutdown.
```

### 11.4 Scaler Main
**User**:
```
Create cmd/scaler/main.go.
- Implement KEDA External Scaler gRPC server returning desired_replicas.
- Read forecaster URL from env/flag.
- Return default-min if snapshot stale > stale-after.
```

---

## 12) Definition of Done (v0.1)

- `go build ./cmd/...` succeeds; binaries run locally.
- `go test ./...` passes with >80% coverage in `capacity`, `adapters`, `storage`.
- `cmd/forecaster` serves `/forecast/current` with plausible data from Prometheus.
- `cmd/scaler` responds to KEDA gRPC with integer replicas.
- Example YAMLs in `deploy/examples/` are aligned with flags/env.
- README and docs/architecture.md consistent with code.

---

## 13) Guardrails for AI

- **Do not invent APIs or CRDs** not specified here.
- **Do not add third-party deps** without instruction (MVP: stdlib; Redis may use `github.com/redis/go-redis/v9` if asked).
- **Keep functions small and testable.**
- **Always handle context timeouts and errors.**
- **Prefer pure functions in capacity/model code.**

---

## 14) Future Work (not in v0.1)

- Real CRDs + controller/operator (client-go).
- Multi-model ensembles; quantile forecasts.
- Canary policy and hybrid `max(predicted, reactive)` wiring.
- Authn/z and mTLS between services.
- Rich Grafana dashboards and OTEL tracing.

---

*End — CLAUDE.md is the single source of truth for code generation of Kedastral v0.1.*
