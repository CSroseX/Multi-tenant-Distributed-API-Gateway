# Quick Reference: Chaos Demo Commands

## Start Everything

```bash
# Terminal 1: Redis
docker run -d --name redis -p 6379:6379 redis:latest

# Terminal 2: Mock services
go run mock-user.go &
go run mock-order.go &

# Terminal 3: Gateway
go run cmd/gateway/main.go
```

## Access Points

```bash
# Demo UI (open in browser)
http://localhost:8080/demo

# Metrics (raw JSON)
curl http://localhost:8080/admin/metrics | jq

# Chaos status
curl http://localhost:8080/admin/chaos/status | jq

# Analytics
curl http://localhost:8080/admin/analytics?tenant=tenantA | jq
```

## Chaos Commands

### Enable Chaos

```bash
# Backend failure (100% errors)
curl -X POST http://localhost:8080/admin/chaos \
  -H "Content-Type: application/json" \
  -d '{"fail_backend":true,"duration_sec":30}'

# Inject latency (2 seconds)
curl -X POST http://localhost:8080/admin/chaos \
  -H "Content-Type: application/json" \
  -d '{"slow_ms":2000,"duration_sec":30}'

# Drop requests (30%)
curl -X POST http://localhost:8080/admin/chaos \
  -H "Content-Type: application/json" \
  -d '{"drop_percent":30,"duration_sec":30}'

# Combined (1s latency + 20% drops)
curl -X POST http://localhost:8080/admin/chaos \
  -H "Content-Type: application/json" \
  -d '{"slow_ms":1000,"drop_percent":20,"duration_sec":30}'
```

### Disable Chaos

```bash
# Immediate recovery
curl -X POST http://localhost:8080/admin/chaos/recover

# Or wait 30 seconds for auto-recovery
```

## Test Requests

```bash
# Valid key (200 OK)
curl -H "X-API-Key: sk_test_123" http://localhost:8080/users

# Invalid key (401)
curl http://localhost:8080/users

# During backend failure chaos (503)
# (after enabling chaos)
curl -H "X-API-Key: sk_test_123" http://localhost:8080/users
```

## Load Testing

```bash
# Simple: 10 requests
for i in {1..10}; do
  curl -H "X-API-Key: sk_test_123" http://localhost:8080/users &
done

# Moderate: 100 requests/sec for 10 seconds
# (Use the demo page button or ab tool)
ab -n 1000 -c 100 \
  -H "X-API-Key: sk_test_123" \
  http://localhost:8080/users/

# High-frequency: 500 req/sec
# (Use the demo page button)
```

## View Logs

```bash
# In gateway terminal, you'll see decision logs like:
# {"timestamp":"...","decision":"CHAOS","reason":"Injected backend failure",...}

# Filter chaos events (if using jq):
# grep CHAOS | jq
```

## Demo Flow (5 minutes)

```bash
# 1. Check baseline
curl http://localhost:8080/admin/chaos/status | jq

# 2. Send traffic
curl -H "X-API-Key: sk_test_123" http://localhost:8080/users

# 3. Inject failure
curl -X POST http://localhost:8080/admin/chaos \
  -d '{"fail_backend":true,"duration_sec":30}' \
  -H "Content-Type: application/json"

# 4. Try request (should fail)
curl -H "X-API-Key: sk_test_123" http://localhost:8080/users

# 5. Check metrics spike
curl http://localhost:8080/admin/metrics | jq '.requests_total'

# 6. Recover
curl -X POST http://localhost:8080/admin/chaos/recover

# 7. Try request (should succeed)
curl -H "X-API-Key: sk_test_123" http://localhost:8080/users

# 8. Verify metrics normalized
curl http://localhost:8080/admin/metrics | jq '.requests_total'
```

## Grafana Setup (2 minutes)

```bash
# 1. Add data source
# Type: JSON API
# URL: http://localhost:8080/admin/metrics
# Refresh: 2s

# 2. Test connection
# Should see requests_total, errors_total, etc.

# 3. Create chart
# Panel type: Graph
# Query: Select any metric from dropdown
# Legend: Show labels

# 4. Add alert
# Condition: errors_total > 0
# Alert title: Chaos Detected
```

## PowerShell Equivalents (Windows)

```powershell
# Valid request
Invoke-WebRequest -Headers @{"X-API-Key"="sk_test_123"} `
  -Uri http://localhost:8080/users

# Enable chaos
$body = @{"fail_backend"=$true;"duration_sec"=30} | ConvertTo-Json
Invoke-WebRequest -Method Post -Uri http://localhost:8080/admin/chaos `
  -Headers @{"Content-Type"="application/json"} `
  -Body $body

# Check status
Invoke-WebRequest -Uri http://localhost:8080/admin/chaos/status |
  Select-Object -ExpandProperty Content |
  ConvertFrom-Json

# Recover
Invoke-WebRequest -Method Post -Uri http://localhost:8080/admin/chaos/recover
```

## Metrics API Fields

```bash
curl http://localhost:8080/admin/metrics | jq

# Output structure:
{
  "requests_total": {
    "/users:tenantA:200": 123,
    "/users:tenantA:503": 45,
    "/orders:tenantB:200": 89
  },
  "errors_total": {
    "/users:tenantA": 45,
    "/orders:tenantB": 2
  },
  "requests_dropped": {
    "/users:tenantA": 12,
    "/orders:tenantB": 5
  },
  "rate_limit_blocks": {
    "tenantA": 3
  },
  "latency_percentiles": {
    "/users:tenantA": {
      "p50": 45.5,
      "p95": 2100.3,
      "p99": 2200.1
    }
  }
}
```

## Metrics Interpretation

```bash
# Calculate error rate
curl http://localhost:8080/admin/metrics | jq '
  .requests_total["/users:tenantA:503"] / 
  (
    .requests_total["/users:tenantA:200"] + 
    .requests_total["/users:tenantA:503"]
  ) * 100
'
# Output: percentage

# Check if chaos active
curl http://localhost:8080/admin/chaos/status | jq '.enabled'
# Output: true/false

# View dropped request count
curl http://localhost:8080/admin/metrics | jq '.requests_dropped'
```

## Common Issues & Fixes

```bash
# Gateway won't start
# → Check Redis is running: docker ps | grep redis

# Chaos doesn't work
# → Check it's enabled: curl .../admin/chaos/status | jq .enabled

# No metrics data
# → Send a request first: curl -H "X-API-Key: sk_test_123" .../users

# Metrics endpoint 404
# → Gateway may be on different port
# → Check startup logs: Gateway running on http://localhost:8080
```

## Files to Know

- `cmd/gateway/main.go` - Routes, demo page, startup
- `internal/chaos/admin.go` - Chaos API handlers
- `internal/chaos/middleware.go` - Request injection logic
- `internal/middleware/metrics.go` - Metrics collection
- `CHAOS_DEMO.md` - Full documentation
- `IMPLEMENTATION_SUMMARY.md` - Architecture guide

## Key Statistics

| Metric | Without Chaos | With Chaos |
|--------|---------------|-----------|
| Latency P95 | ~50ms | +2000ms (if slow_ms=2000) |
| Error Rate | 0% | Configurable (0-100%) |
| Throughput | Normal | -30% (if drops=30%) |
| Monitoring Impact | Negligible | Obvious spike |

---

**Tip:** Use the demo page UI for interactive testing - it's easier than curl commands!

