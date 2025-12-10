# Kedastral CLI Design

## Overview

The Kedastral CLI provides real-time feedback and interaction with the Kedastral forecaster and scaler components. It's designed for operators, developers, and SREs who need to monitor, debug, and inspect forecast predictions and scaling decisions.

## Architecture

```
┌─────────────────┐
│  kedastral CLI  │
│   (cmd/cli)     │
└────────┬────────┘
         │
         ├─── HTTP Client ──→ Forecaster HTTP API (:8081)
         │                    • /forecast/current
         │                    • /healthz
         │                    • /metrics
         │
         └─── gRPC Client ──→ Scaler gRPC API (:50051)
                              • GetMetrics
                              • IsActive
```

## Core Commands

### 1. Forecast Commands

#### `kedastral forecast get [workload]`
Get the current forecast for a workload.

**Output:**
```
WORKLOAD       METRIC      GENERATED           HORIZON  STEP  CURRENT  NEXT-5M  NEXT-10M
my-api         http_rps    2025-12-10 17:00    30m      1m    2        3        3
```

**Flags:**
- `--forecaster-url` - Forecaster HTTP endpoint (default: http://localhost:8081)
- `--output, -o` - Output format: table, json, yaml (default: table)
- `--all` - Show all future values, not just summary

**Example:**
```bash
kedastral forecast get my-api --output json
```

#### `kedastral forecast watch [workload]`
Watch forecasts in real-time, updating as new predictions are generated.

**Output:**
```
Watching forecasts for my-api (Ctrl+C to stop)...

17:00:30 │ RPS: 150.2 → Replicas: 2 (current) → 3 (+5m) → 3 (+10m)
17:01:00 │ RPS: 152.1 → Replicas: 2 (current) → 3 (+5m) → 4 (+10m)
17:01:30 │ RPS: 155.8 → Replicas: 2 (current) → 3 (+5m) → 4 (+10m)
```

**Flags:**
- `--forecaster-url` - Forecaster HTTP endpoint
- `--interval` - Poll interval (default: 30s)
- `--follow, -f` - Follow mode (like tail -f)

#### `kedastral forecast status [workload]`
Get forecast status and metadata.

**Output:**
```
Workload:           my-api
Metric:             http_rps
Status:             HEALTHY
Last Generated:     2025-12-10 17:00:15 (15s ago)
Forecast Horizon:   30m
Step Size:          1m
Values:             30 points
Desired Replicas:   30 points
Staleness:          Fresh (< 2m old)
```

**Flags:**
- `--forecaster-url` - Forecaster HTTP endpoint

### 2. Health Commands

#### `kedastral health forecaster`
Check forecaster health status.

**Output:**
```
Forecaster:     http://localhost:8081
Status:         HEALTHY ✓
Response Time:  12ms
Uptime:         2h 15m
```

#### `kedastral health scaler`
Check scaler health status.

**Output:**
```
Scaler:         localhost:50051
Status:         HEALTHY ✓
gRPC Active:    true
Response Time:  5ms
```

### 3. Metrics Commands

#### `kedastral metrics [component]`
Fetch and display Prometheus metrics from a component.

**Output:**
```
METRIC                                     VALUE
kedastral_adapter_collect_seconds_sum      1.234
kedastral_adapter_collect_seconds_count    150
kedastral_forecast_age_seconds             12.5
kedastral_desired_replicas                 3
```

**Flags:**
- `--forecaster-url` - Forecaster endpoint
- `--filter` - Filter metrics by name pattern

### 4. Global Flags

All commands support:
- `--forecaster-url` - Forecaster HTTP endpoint (env: KEDASTRAL_FORECASTER_URL, default: http://localhost:8081)
- `--scaler-url` - Scaler gRPC endpoint (env: KEDASTRAL_SCALER_URL, default: localhost:50051)
- `--output, -o` - Output format: table, json, yaml
- `--no-color` - Disable colored output
- `--debug` - Enable debug logging
- `--timeout` - Request timeout (default: 30s)

## Implementation Structure

```
cmd/cli/
├── main.go                  # CLI entry point
├── forecast/
│   ├── get.go              # forecast get command
│   ├── watch.go            # forecast watch command
│   └── status.go           # forecast status command
├── health/
│   ├── forecaster.go       # health forecaster command
│   └── scaler.go           # health scaler command
├── metrics/
│   └── metrics.go          # metrics command
└── client/
    ├── forecaster.go       # HTTP client for forecaster
    └── scaler.go           # gRPC client for scaler

pkg/cli/
├── client/
│   ├── forecaster.go       # Forecaster HTTP client
│   └── types.go            # Shared types
├── output/
│   ├── table.go           # Table formatter
│   ├── json.go            # JSON formatter
│   └── yaml.go            # YAML formatter
└── watch/
    └── poller.go          # Polling logic for watch command
```

## Dependencies

- **cobra** - CLI framework (`github.com/spf13/cobra`)
- **viper** - Configuration management (`github.com/spf13/viper`)
- **tablewriter** - Table formatting (`github.com/olekukonko/tablewriter`)
- **color** - Colored output (`github.com/fatih/color`)
- **grpc** - gRPC client (`google.golang.org/grpc`)

## User Experience Examples

### Quick status check
```bash
$ kedastral forecast get my-api
WORKLOAD  METRIC    CURRENT  NEXT-5M  NEXT-10M  STATUS
my-api    http_rps  2        3        3         HEALTHY ✓
```

### Detailed JSON output
```bash
$ kedastral forecast get my-api --output json
{
  "workload": "my-api",
  "metric": "http_rps",
  "generatedAt": "2025-12-10T17:00:15Z",
  "stepSeconds": 60,
  "horizonSeconds": 1800,
  "values": [150.2, 152.1, ...],
  "desiredReplicas": [2, 3, 3, ...],
  "staleness": "fresh"
}
```

### Watch mode
```bash
$ kedastral forecast watch my-api --follow
Watching forecasts for my-api...
17:00:00 │ RPS: 150 → 2 pods (now) → 3 pods (+5m)
17:00:30 │ RPS: 152 → 2 pods (now) → 3 pods (+5m)
```

### Health check
```bash
$ kedastral health forecaster
✓ Forecaster is healthy
  URL: http://localhost:8081
  Response time: 12ms
```

## Output Formats

### Table (default)
Human-readable tabular output with colors and symbols.

### JSON
Machine-readable JSON for scripting and automation.

### YAML
Human-readable YAML for configuration files.

## Error Handling

All commands should:
1. Provide clear error messages
2. Exit with appropriate status codes (0 = success, 1 = error)
3. Show connection errors with helpful suggestions
4. Handle timeouts gracefully

Example error:
```bash
$ kedastral forecast get my-api
Error: Failed to connect to forecaster at http://localhost:8081
  → Is the forecaster running?
  → Check --forecaster-url flag or KEDASTRAL_FORECASTER_URL env var
```

## Future Enhancements

### v0.3+
- `kedastral config` - Manage CLI configuration
- `kedastral policies list` - List all forecast policies (requires CRD support)
- `kedastral diagnose` - Comprehensive diagnostics
- `kedastral tail` - Tail forecaster logs
- `kedastral export` - Export forecast data to CSV/JSON

### v0.4+
- Interactive TUI mode with dashboard
- Real-time charts and graphs
- Alert notifications
- Multi-cluster support
