# Kedastral Examples

This directory contains example configurations for deploying and using Kedastral with KEDA.

## Files

- **`deployment.yaml`** - Complete Kubernetes deployment for forecaster and scaler
- **`scaled-object.yaml`** - KEDA ScaledObject configuration example

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.20+)
- KEDA installed ([installation guide](https://keda.sh/docs/latest/deploy/))
- Prometheus running in the cluster

### 1. Deploy Kedastral

```bash
# Deploy forecaster and scaler
kubectl apply -f deployment.yaml

# Verify pods are running
kubectl get pods -l app=kedastral
```

### 2. Configure KEDA ScaledObject

```bash
# Apply the ScaledObject for your workload
kubectl apply -f scaled-object.yaml

# Verify ScaledObject is active
kubectl get scaledobject my-api-scaledobject
```

### 3. Monitor

```bash
# Check forecaster logs
kubectl logs -l component=forecaster -f

# Check scaler logs
kubectl logs -l component=scaler -f

# View current forecast
kubectl port-forward svc/kedastral-forecaster 8081:8081
curl "http://localhost:8081/forecast/current?workload=my-api"

# View metrics
kubectl port-forward svc/kedastral-forecaster 8081:8081
curl http://localhost:8081/metrics

kubectl port-forward svc/kedastral-scaler 8082:8082
curl http://localhost:8082/metrics
```

## Configuration

### Forecaster Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-workload` | Workload name (required) | - |
| `-metric` | Metric name (required) | - |
| `-prom-url` | Prometheus URL | `http://localhost:9090` |
| `-prom-query` | Prometheus query (required) | - |
| `-target-per-pod` | Target metric value per pod | `100` |
| `-headroom` | Headroom multiplier | `1.2` |
| `-min` | Minimum replicas | `1` |
| `-max` | Maximum replicas | `100` |
| `-horizon` | Forecast horizon | `30m` |
| `-step` | Forecast step size | `1m` |
| `-lead-time` | Lead time for pre-scaling | `5m` |
| `-interval` | Forecast interval | `30s` |
| `-window` | Historical window | `30m` |
| `-listen` | HTTP listen address | `:8081` |
| `-log-level` | Log level (debug, info, warn, error) | `info` |
| `-log-format` | Log format (text, json) | `text` |

### Scaler Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-forecaster-url` | Forecaster HTTP endpoint | `http://localhost:8081` |
| `-lead-time` | Lead time for forecast selection | `5m` |
| `-listen` | gRPC listen address | `:50051` |
| `-log-level` | Log level (debug, info, warn, error) | `info` |
| `-log-format` | Log format (text, json) | `text` |

### ScaledObject Metadata

The KEDA ScaledObject supports the following metadata:

| Key | Description | Required |
|-----|-------------|----------|
| `scalerAddress` | Address of Kedastral scaler | Yes |
| `workload` | Workload name (must match forecaster) | Yes |
| `metricName` | Custom metric name | No |

## Example Scenarios

### Scenario 1: HTTP API with RPS-based scaling

```yaml
# Forecaster configuration
args:
  - -workload=my-api
  - -metric=http_rps
  - -prom-query=sum(rate(http_requests_total{service="my-api"}[1m]))
  - -target-per-pod=200
  - -headroom=1.3
  - -lead-time=5m
```

### Scenario 2: Queue-based workload

```yaml
# Forecaster configuration
args:
  - -workload=worker
  - -metric=queue_depth
  - -prom-query=rabbitmq_queue_messages{queue="tasks"}
  - -target-per-pod=50
  - -headroom=1.5
  - -lead-time=10m
```

### Scenario 3: Database connections

```yaml
# Forecaster configuration
args:
  - -workload=db-proxy
  - -metric=active_connections
  - -prom-query=sum(pg_stat_database_numbackends{datname="mydb"})
  - -target-per-pod=100
  - -headroom=1.2
  - -lead-time=3m
```

## Troubleshooting

### Forecast not updating

1. Check forecaster logs for errors:
   ```bash
   kubectl logs -l component=forecaster
   ```

2. Verify Prometheus connectivity:
   ```bash
   kubectl exec -it deployment/kedastral-forecaster -- curl http://prometheus.monitoring.svc:9090/api/v1/query?query=up
   ```

### Scaler not scaling

1. Check scaler can reach forecaster:
   ```bash
   kubectl exec -it deployment/kedastral-scaler -- curl http://kedastral-forecaster:8081/healthz
   ```

2. Check KEDA is calling the scaler:
   ```bash
   kubectl logs -l component=scaler | grep GetMetrics
   ```

3. Verify ScaledObject is active:
   ```bash
   kubectl describe scaledobject my-api-scaledobject
   ```

### Forecast is stale

The scaler marks forecasts as stale if they're older than 2x the lead time. Check:

1. Forecaster is generating forecasts:
   ```bash
   kubectl logs -l component=forecaster | grep "stored forecast"
   ```

2. Interval is set correctly (should be less than lead time)

## Next Steps

- Set up Grafana dashboards for Kedastral metrics
- Configure alerts for stale forecasts
- Tune capacity policy parameters based on your workload
- Integrate with your CI/CD pipeline
