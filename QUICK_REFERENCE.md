# Quick Reference - Chaos Demo Commands for Render Deployment


## Important Notes

1. Service runs on Render free tier
2. Service sleeps after 15 minutes of no activity
3. First request after sleep takes 30-60 seconds
4. Chaos auto-recovers after specified duration
5. Metrics persist but may reset on redeployment
6. Grafana updates every 30 seconds
7. Rate limit is 5 requests per minute per tenant

All commands are for Windows Command Prompt (cmd).
Run all commands in cmd on your local computer.
Output appears in cmd unless stated otherwise.

Base URL: https://centralized-api-orchestration-engine.onrender.com

## Access Points

### Demo UI (open in browser)

```
https://centralized-api-orchestration-engine.onrender.com/demo
```

What you see: Interactive web page with buttons

### Metrics (JSON format)

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/metrics
```

Output in cmd: JSON data with all metrics

### Chaos Status

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/chaos/status
```

Output in cmd: JSON showing current chaos configuration

### Analytics

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/analytics?tenant=tenantA
```

Output in cmd: JSON showing tenant analytics data

---

## Chaos Commands

### Enable Backend Failure (100% errors)

Run in cmd:
```
curl -X POST https://centralized-api-orchestration-engine.onrender.com/admin/chaos -H "Content-Type: application/json" -d "{\"fail_backend\":true,\"duration_sec\":30}"
```

What happens: All requests will fail with 503 error for 30 seconds

### Enable Latency Injection (2 second delay)

Run in cmd:
```
curl -X POST https://centralized-api-orchestration-engine.onrender.com/admin/chaos -H "Content-Type: application/json" -d "{\"slow_ms\":2000,\"duration_sec\":30}"
```

What happens: All requests will take 2 seconds longer for 30 seconds

### Enable Request Dropping (30% drop rate)

Run in cmd:
```
curl -X POST https://centralized-api-orchestration-engine.onrender.com/admin/chaos -H "Content-Type: application/json" -d "{\"drop_percent\":30,\"duration_sec\":30}"
```

What happens: 30% of requests will be dropped for 30 seconds

### Enable Combined Chaos (1 second delay + 20% drops)

Run in cmd:
```
curl -X POST https://centralized-api-orchestration-engine.onrender.com/admin/chaos -H "Content-Type: application/json" -d "{\"slow_ms\":1000,\"drop_percent\":20,\"duration_sec\":30}"
```

What happens: Requests delayed by 1 second and 20% dropped for 30 seconds

### Disable Chaos Immediately

Run in cmd:
```
curl -X POST https://centralized-api-orchestration-engine.onrender.com/admin/chaos/recover
```

What happens: All chaos effects stop immediately

---

## Test Requests

### Valid Request (should succeed with 200)

Run in cmd:
```
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
```

Expected output: {"service": "users", "status": "ok"}

### Invalid API Key (should fail with 401)

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/users
```

Expected output: Tenant not found

### Request During Backend Failure Chaos (should fail with 503)

First enable chaos, then run in cmd:
```
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
```

Expected output: Service Unavailable

---

## Demo Flow (5 minutes)

### Step 1: Check Baseline

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/chaos/status
```

Expected: Chaos should be disabled

### Step 2: Send Normal Traffic

Run in cmd:
```
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
```

Expected: Request succeeds

### Step 3: Inject Failure

Run in cmd:
```
curl -X POST https://centralized-api-orchestration-engine.onrender.com/admin/chaos -H "Content-Type: application/json" -d "{\"fail_backend\":true,\"duration_sec\":30}"
```

Expected: Chaos enabled

### Step 4: Try Request (should fail)

Run in cmd:
```
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
```

Expected: Service Unavailable error

### Step 5: Check Metrics Spike

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/metrics
```

Expected: Error count increased in JSON output

### Step 6: Recover

Run in cmd:
```
curl -X POST https://centralized-api-orchestration-engine.onrender.com/admin/chaos/recover
```

Expected: Chaos disabled

### Step 7: Try Request (should succeed)

Run in cmd:
```
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
```

Expected: Request succeeds again

### Step 8: Verify Metrics Normalized

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/metrics
```

Expected: No new errors in JSON output

---

## Grafana Setup

Your Grafana Cloud is at: https://csrosex.grafana.net

### Step 1: Access Grafana

Open in browser: https://csrosex.grafana.net

### Step 2: View Dashboard

1. Click Dashboards menu
2. Find your API Gateway dashboard
3. View live metrics updating every 30 seconds

### Step 3: Query Metrics Manually

1. Click Explore menu
2. Select data source: grafanacloud-csrosex-prom
3. Type metric name: api_gateway_requests_total
4. Click Run Query
5. View graph and data

---

## Metrics Output Structure

When you run:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/metrics
```

You get JSON like this:
```
{
  "requests_total": {
    "/users:tenantA:200": 123,
    "/users:tenantA:503": 45
  },
  "errors_total": {
    "/users:tenantA": 45
  },
  "requests_dropped": {
    "/users:tenantA": 12
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

Each field explained:
- requests_total: Total requests by route, tenant, and status code
- errors_total: Total error count by route and tenant
- requests_dropped: Requests dropped by chaos or issues
- rate_limit_blocks: How many times rate limit blocked requests
- latency_percentiles: Response time statistics (p50, p95, p99)

---

## Common Issues and Fixes

### Gateway is slow to respond

Reason: Render free tier sleeps after inactivity
Fix: Visit https://centralized-api-orchestration-engine.onrender.com/health first
Wait 30-60 seconds, then try again

### Chaos not working

Reason: Chaos may be disabled
Fix: Check status with:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/chaos/status
```

### No metrics data

Reason: No requests sent yet
Fix: Send some test requests first:
```
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
```

### Demo page not loading

Reason: Service may be sleeping
Fix: Wait 30-60 seconds and refresh browser

---

## Key Statistics

What happens when you enable chaos:

| Metric | Normal | With Chaos |
|--------|--------|------------|
| Latency P95 | 50ms | 2000ms+ (if slow_ms=2000) |
| Error Rate | 0% | Up to 100% (if fail_backend=true) |
| Throughput | 100% | 70% (if drop_percent=30) |

---

## Testing with Demo Page

Instead of using cmd, you can use the demo page for easier testing.

### Step 1: Open Demo Page

Open in browser:
```
https://centralized-api-orchestration-engine.onrender.com/demo
```

### Step 2: Send Requests

Click button on page: "Send 100 Requests"
Watch the responses in the page

### Step 3: Enable Chaos

Use buttons on page to enable different chaos modes
Watch how responses change

### Step 4: View Metrics

Metrics update automatically on the demo page
No need to run curl commands

---
