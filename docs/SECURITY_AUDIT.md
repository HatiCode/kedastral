# Security Audit Report - Kedastral v0.1.2

**Date:** 2025-12-16
**Auditor:** Security review of codebase before v0.1.2 release
**Scope:** Complete codebase analysis focusing on input validation, resource exhaustion, credentials, network security, dependencies, runtime safety, and data flow

---

## Executive Summary

This security audit identified **18 vulnerabilities** across the Kedastral codebase:
- **3 Critical** severity issues requiring immediate attention
- **4 High** severity issues
- **6 Medium** severity issues
- **5 Low** severity issues

All findings have been documented with specific file locations, impact analysis, and recommended fixes.

---

## Critical Findings (Priority 1)

### 1. URL Query Injection in Scaler
**Severity:** CRITICAL
**File:** `cmd/scaler/scaler.go:216`
**Status:** 🔴 OPEN

**Description:**
```go
url := fmt.Sprintf("%s/forecast/current?workload=%s", s.forecasterURL, workload)
```
The workload parameter is directly interpolated into the URL query string without proper encoding.

**Impact:**
- Query parameter injection attacks
- Potential SSRF (Server-Side Request Forgery)
- URL parsing bypass
- Cache poisoning

**Recommended Fix:**
```go
u, _ := url.Parse(s.forecasterURL)
u.Path = "/forecast/current"
q := u.Query()
q.Set("workload", workload)
u.RawQuery = q.Encode()
finalURL := u.String()
```

---

### 2. Metrics Endpoint Exposed on All Interfaces
**Severity:** CRITICAL
**File:** `cmd/scaler/main.go:82`
**Status:** 🔴 OPEN

**Description:**
```go
httpServer := httpx.NewServer(":8082", httpMux, log)
```
The metrics endpoint binds to `:8082` (all interfaces) exposing Prometheus metrics.

**Impact:**
- Information disclosure via metrics endpoint
- Reconnaissance for attackers
- Exposure of internal network details
- Potential DoS vector

**Recommended Fix:**
- Make bind address configurable
- Default to `127.0.0.1:8082` for local-only access
- Add authentication/authorization to metrics endpoints
- Document security implications in deployment guides

---

### 3. gRPC Reflection Enabled Without Authentication
**Severity:** CRITICAL
**File:** `cmd/scaler/main.go:65`
**Status:** 🔴 OPEN

**Description:**
```go
reflection.Register(grpcServer)
```
gRPC reflection is enabled unconditionally, allowing any client to enumerate services and methods.

**Impact:**
- Complete API enumeration by attackers
- Information disclosure about internal service structure
- Enables targeted attacks once structure is known
- Violates principle of least privilege

**Recommended Fix:**
```go
if os.Getenv("ENABLE_GRPC_REFLECTION") == "true" {
    reflection.Register(grpcServer)
    log.Warn("gRPC reflection enabled - should only be used in development")
}
```

---

## High Severity Findings (Priority 2)

### 4. Missing HTTP Method Restrictions
**Severity:** HIGH
**File:** `cmd/forecaster/router/router.go:35-41`
**Status:** 🔴 OPEN

**Description:**
The `/forecast/current` endpoint accepts all HTTP methods (GET, POST, PUT, DELETE, etc.).

**Impact:**
- HTTP method confusion attacks
- Cache poisoning
- Potential for unintended side effects

**Recommended Fix:**
```go
if r.Method != http.MethodGet {
    httpx.WriteErrorMessage(w, http.StatusMethodNotAllowed, "method not allowed")
    return
}
```

---

### 5. Redis Password Logging Risk
**Severity:** HIGH
**File:** `cmd/forecaster/store/store.go:71-75`
**Status:** 🔴 OPEN

**Description:**
Redis configuration is logged. If passwords are embedded in connection URLs, they could be exposed.

**Impact:**
- Credential exposure in application logs
- Log aggregation systems may capture sensitive data
- Compliance violations (PCI-DSS, HIPAA)

**Recommended Fix:**
- Never log connection strings with embedded credentials
- Sanitize sensitive configuration before logging
- Use separate fields for structured logging with redaction

---

### 6. Inconsistent Workload Name Validation
**Severity:** HIGH
**File:** `pkg/storage/redis.go:76-81`
**Status:** 🔴 OPEN

**Description:**
Redis storage validates workload names, but memory storage and HTTP handlers don't. This creates security inconsistencies.

**Impact:**
- Different behavior between storage backends
- Potential key injection if memory store is exposed
- Logic bugs when switching storage backends

**Recommended Fix:**
- Implement centralized workload validation in shared package
- Apply validation consistently across all storage implementations
- Document allowed workload name format in API docs

---

### 7. Division by Zero Risk in Lead Time Calculation
**Severity:** HIGH
**File:** `cmd/scaler/scaler.go:260-261`
**Status:** 🔴 OPEN

**Description:**
```go
stepDuration := time.Duration(snapshot.StepSeconds) * time.Second
leadSteps := int(s.leadTime / stepDuration)
```
If `snapshot.StepSeconds` is 0, this causes a panic.

**Impact:**
- Service crash (DoS)
- Incorrect scaling decisions
- Goroutine leaks if panic occurs

**Recommended Fix:**
```go
if snapshot.StepSeconds <= 0 {
    s.logger.Warn("invalid step seconds, using default", "value", snapshot.StepSeconds)
    snapshot.StepSeconds = 60
}
stepDuration := time.Duration(snapshot.StepSeconds) * time.Second
```

---

## Medium Severity Findings (Priority 3)

### 8. Unbounded Array Allocation in Feature Building
**Severity:** MEDIUM
**File:** `pkg/features/builder.go:35`
**Status:** 🔴 OPEN

**Description:**
No upper bound on DataFrame size from Prometheus adapter.

**Impact:**
- Memory exhaustion DoS
- OOM kills of forecaster pod

**Recommended Fix:**
```go
const maxRows = 100000
if len(df.Rows) > maxRows {
    return models.FeatureFrame{}, fmt.Errorf("dataframe exceeds maximum size: %d > %d", len(df.Rows), maxRows)
}
```

---

### 9. Missing Rate Limiting on HTTP Endpoints
**Severity:** MEDIUM
**File:** `cmd/forecaster/router/router.go`
**Status:** 🔴 OPEN

**Description:**
No rate limiting on `/forecast/current` and `/metrics` endpoints.

**Impact:**
- Denial of Service attacks
- Resource exhaustion (CPU, memory)

**Recommended Fix:**
- Implement rate limiting middleware
- Consider using `golang.org/x/time/rate`

---

### 10. No Timeout on Prometheus Query Context
**Severity:** MEDIUM
**File:** `pkg/adapters/prometheus.go:65-71`
**Status:** 🔴 OPEN

**Description:**
Context passed to `Collect()` might not have a deadline.

**Impact:**
- Prometheus adapter could hang indefinitely
- Forecast loop blocked

**Recommended Fix:**
```go
if _, ok := ctx.Deadline(); !ok {
    var cancel context.CancelFunc
    ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
    defer cancel()
}
```

---

### 11. No Validation of Numeric Bounds in ARIMA
**Severity:** MEDIUM
**File:** `pkg/models/arima.go:249`
**Status:** 🔴 OPEN

**Description:**
ARIMA caps predictions at 1e9 but doesn't validate reasonableness.

**Impact:**
- Extreme scaling decisions
- Potential integer overflow

**Recommended Fix:**
```go
const maxPrediction = 1e6 // configurable
if pred > maxPrediction {
    logger.Warn("prediction exceeds maximum", "value", pred)
    pred = maxPrediction
}
```

---

### 12. Hardcoded HTTP Timeout Values
**Severity:** MEDIUM
**Files:** Multiple (`prometheus.go:62`, `scaler.go:48`, `redis.go:50-52`)
**Status:** 🔴 OPEN

**Description:**
Timeouts are hardcoded and not configurable.

**Impact:**
- No way to tune for different network conditions

**Recommended Fix:**
```go
httpTimeout := getEnvDuration("HTTP_CLIENT_TIMEOUT", 10*time.Second)
cli = &http.Client{Timeout: httpTimeout}
```

---

### 13. No Validation of Snapshot Values
**Severity:** MEDIUM
**File:** `cmd/forecaster/forecaster.go:254-263`
**Status:** 🔴 OPEN

**Description:**
Snapshot stores raw forecast values without validation.

**Impact:**
- Invalid data stored in backend
- Scaling decisions based on bad data

**Recommended Fix:**
```go
for _, v := range snapshot.Values {
    if math.IsNaN(v) || math.IsInf(v, 0) {
        return fmt.Errorf("invalid forecast value: %v", v)
    }
}
```

---

## Low Severity Findings (Priority 4)

### 14. Prometheus Query Not Validated
**Severity:** LOW
**File:** `cmd/forecaster/config/config.go:93`
**Status:** 🔴 OPEN

**Description:**
PromQL query is required but not validated for syntax.

**Recommended Fix:**
Add basic PromQL syntax validation during config parsing.

---

### 15. Missing Input Validation on Config Parameters
**Severity:** LOW
**File:** `cmd/forecaster/config/config.go:68-131`
**Status:** 🔴 OPEN

**Description:**
Configuration parsing doesn't validate parameter ranges.

**Recommended Fix:**
```go
if cfg.TargetPerPod <= 0 {
    fmt.Fprintf(os.Stderr, "Error: target-per-pod must be > 0\n")
    os.Exit(1)
}
if cfg.MaxReplicas > 0 && cfg.MaxReplicas < cfg.MinReplicas {
    fmt.Fprintf(os.Stderr, "Error: max-replicas must be >= min-replicas\n")
    os.Exit(1)
}
```

---

### 16. gRPC Server Listening on 0.0.0.0
**Severity:** LOW
**File:** `cmd/scaler/config/config.go:37`
**Status:** 🔴 OPEN

**Description:**
Default gRPC listen address binds to all interfaces.

**Recommended Fix:**
```go
flag.StringVar(&cfg.Listen, "listen", getEnv("SCALER_LISTEN", "127.0.0.1:50051"), "gRPC listen address")
```

---

### 17. No Security Event Logging
**Severity:** LOW
**File:** `cmd/scaler/scaler.go:74-96`
**Status:** 🔴 OPEN

**Description:**
No dedicated security audit log for rejected operations.

**Recommended Fix:**
Add structured security event logging for audit purposes.

---

### 18. No Absolute Max Replica Limit
**Severity:** LOW
**File:** `pkg/capacity/planner.go:56-57`
**Status:** 🔴 OPEN

**Description:**
No sanity check for extremely large MaxReplicas values.

**Recommended Fix:**
```go
const absMaxReplicas = 100000
if p.MaxReplicas == 0 || p.MaxReplicas > absMaxReplicas {
    p.MaxReplicas = absMaxReplicas
}
```

---

## Summary Statistics

| Priority | Count | Status |
|----------|-------|--------|
| Critical | 3 | 🔴 All Open |
| High | 4 | 🔴 All Open |
| Medium | 6 | 🔴 All Open |
| Low | 5 | 🔴 All Open |
| **Total** | **18** | **0 Fixed** |

---

## Dependency Security

**go.mod Analysis (2025-12-16):**
- Go version: 1.25.5 ✅ (current)
- `google.golang.org/grpc v1.77.0` ✅ (recent)
- `github.com/redis/go-redis/v9 v9.17.2` ✅ (latest)
- `github.com/prometheus/client_golang v1.23.2` ✅ (current)

**No known vulnerable dependencies detected.**

Recommended tools:
- `go list -json -m all | nancy sleuth`
- GitHub Dependabot
- `govulncheck`

---

## Remediation Plan

### Phase 1: Critical Fixes (Before v0.1.2 Release)
- [ ] Fix URL query injection in scaler (Finding #1)
- [ ] Restrict metrics endpoint to localhost (Finding #2)
- [ ] Make gRPC reflection conditional (Finding #3)

### Phase 2: High Priority (v0.1.3 or v0.2.0)
- [ ] Add HTTP method restrictions (Finding #4)
- [ ] Sanitize Redis config logging (Finding #5)
- [ ] Centralized workload validation (Finding #6)
- [ ] Add StepSeconds bounds checking (Finding #7)

### Phase 3: Hardening (v0.2.x)
- [ ] Implement rate limiting (Finding #9)
- [ ] Add DataFrame size limits (Finding #8)
- [ ] Add timeout enforcement (Finding #10)
- [ ] Validate snapshot values (Finding #13)
- [ ] Make timeouts configurable (Finding #12)
- [ ] Add ARIMA prediction bounds (Finding #11)

### Phase 4: Polish (Future)
- [ ] Config parameter validation (Finding #15)
- [ ] PromQL query validation (Finding #14)
- [ ] Security event logging (Finding #17)
- [ ] Default to localhost for gRPC (Finding #16)
- [ ] Absolute max replica limit (Finding #18)

---

## Notes

This audit was conducted as part of the v0.1.2 release cycle after implementing the ARIMA forecasting model. The findings represent typical security issues in production systems that weren't specifically hardened. All issues are remediable with the provided fixes.

**Audit Methodology:**
- Static code analysis of all Go source files
- Review of network-facing components
- Configuration security review
- Dependency vulnerability check
- Data flow analysis

**Out of Scope:**
- Kubernetes deployment security (RBAC, network policies)
- Infrastructure security (TLS certificates, secrets management)
- Runtime security (container scanning, runtime protection)
- Third-party service security (Prometheus, Redis, KEDA)

---

**Last Updated:** 2025-12-16
**Next Review:** Before v0.2.0 release
