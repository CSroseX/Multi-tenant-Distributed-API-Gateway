# API Gateway - Test Commands for Render Deployment

All commands are for Windows Command Prompt (cmd).
Run all commands in cmd on your local computer.
Output appears in cmd unless stated otherwise.

Base URL: https://centralized-api-orchestration-engine.onrender.com

## Test Credentials

Tenant A: sk_test_123 (ID: tenantA)
Tenant B: sk_test_456 (ID: tenantB)

---

## Basic Tests

### 1. Request with Valid API Key

Run in cmd:
```
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
```

Expected output in cmd:
```
{"service": "users", "status": "ok"}
```

### 2. Request Without API Key

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/users
```

Expected output in cmd:
```
Tenant not found
```

### 3. Request to Invalid Endpoint

Run in cmd:
```
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/invalid
```

Expected output in cmd:
```
Route not found
```

### 4. Check Analytics for Tenant A

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/analytics?tenant=tenantA
```

Expected output in cmd: JSON data showing request counts and errors

### 5. Check Analytics for Tenant B

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/analytics?tenant=tenantB
```

Expected output in cmd: JSON data showing request counts and errors

---

## Rate Limiting Tests

The gateway allows 5 requests per minute per tenant.

### Test Rate Limiter (6 requests)

Run in cmd (copy and paste all 6 lines at once):
```
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
```

Expected output:
- First 5 requests: {"service": "users", "status": "ok"}
- 6th request: Rate limit exceeded

### Verify Request Count

Run in cmd after the above test:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/analytics?tenant=tenantA
```

Expected: Total request count should be 6 (including the blocked request)

---

## Chaos Testing

### Check Current Chaos Status

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/chaos/status
```

Expected output in cmd: JSON showing chaos configuration

### Enable Backend Failure

Run in cmd:
```
curl -X POST https://centralized-api-orchestration-engine.onrender.com/admin/chaos -H "Content-Type: application/json" -d "{\"fail_backend\":true,\"duration_sec\":60}"
```

Expected output in cmd: Confirmation message

### Enable Slow Responses (2 second delay)

Run in cmd:
```
curl -X POST https://centralized-api-orchestration-engine.onrender.com/admin/chaos -H "Content-Type: application/json" -d "{\"slow_ms\":2000,\"duration_sec\":60}"
```

Expected output in cmd: Confirmation message

### Enable Request Dropping (30% drop rate)

Run in cmd:
```
curl -X POST https://centralized-api-orchestration-engine.onrender.com/admin/chaos -H "Content-Type: application/json" -d "{\"drop_percent\":30,\"duration_sec\":60}"
```

Expected output in cmd: Confirmation message

### Disable All Chaos

Run in cmd:
```
curl -X POST https://centralized-api-orchestration-engine.onrender.com/admin/chaos/recover
```

Expected output in cmd: Confirmation message

---

## Interactive Demo Page

Open in your web browser:
```
https://centralized-api-orchestration-engine.onrender.com/demo
```

What you see: Interactive web page with buttons to send requests and control chaos.
This page is easier than using command line.

---

## View Metrics

### Prometheus Metrics (requires login)

Open in your web browser:
```
https://centralized-api-orchestration-engine.onrender.com/metrics
```

When browser asks for login, enter:
- Username: grafana
- Password: metrics_secure_2026

What you see: Text output showing all Prometheus metrics

### Admin Metrics (JSON format, no login needed)

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/metrics
```

Expected output in cmd: JSON data with requests, errors, and latency

---

## Health Check

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/health
```

Expected output in cmd:
```
ok
```

---

## Multi-Tenant Testing

### Send 2 Requests from Tenant A

Run in cmd:
```
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
curl -H "X-API-Key: sk_test_123" https://centralized-api-orchestration-engine.onrender.com/users
```

### Send 3 Requests from Tenant B

Run in cmd:
```
curl -H "X-API-Key: sk_test_456" https://centralized-api-orchestration-engine.onrender.com/orders
curl -H "X-API-Key: sk_test_456" https://centralized-api-orchestration-engine.onrender.com/orders
curl -H "X-API-Key: sk_test_456" https://centralized-api-orchestration-engine.onrender.com/orders
```

### Check Tenant A Analytics

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/analytics?tenant=tenantA
```

Expected: Should show 2 requests to /users

### Check Tenant B Analytics

Run in cmd:
```
curl https://centralized-api-orchestration-engine.onrender.com/admin/analytics?tenant=tenantB
```

Expected: Should show 3 requests to /orders

---

## View Application Logs

You cannot view logs from cmd.

To see application logs:
1. Go to https://dashboard.render.com in your browser
2. Click on service: Centralized-API-Orchestration-Engine
3. Click on Logs tab
4. View real-time logs there

---

## Grafana Dashboard

Your Grafana Cloud dashboard is at: https://csrosex.grafana.net

Metrics update every 30 seconds automatically.

To view dashboard:
1. Login to Grafana Cloud
2. Go to Dashboards menu
3. Find your API Gateway dashboard
4. View live metrics

To query metrics manually:
1. Go to Explore in Grafana
2. Select: grafanacloud-csrosex-prom
3. Type metric name: api_gateway_requests_total
4. Click Run Query

---

## Common Response Codes

200 - Success, request processed normally
401 - Unauthorized, missing or invalid API key
404 - Not Found, invalid endpoint requested
429 - Too Many Requests, rate limit exceeded
503 - Service Unavailable, backend failure or chaos active

---

## Troubleshooting

### Issue: curl command not found

Solution: Install curl for Windows from https://curl.se/windows/

### Issue: Connection timeout or slow response

Solution: Render free tier sleeps after inactivity.
First visit https://centralized-api-orchestration-engine.onrender.com/health to wake it up.
Wait 30-60 seconds, then try your command again.

### Issue: Analytics showing zero requests

Solution: Send some test requests first using commands above.
Analytics only shows data after requests are made.

### Issue: Rate limit not working

Solution: Rate limits reset after 1 minute automatically.
Wait 65 seconds and try again.

### Issue: Demo page not loading

Solution: Check if service is awake by visiting /health first.
Wait for service to start, then refresh demo page.

---

## Quick Reference Table

| Test | Command | Expected Response |
|------|---------|-------------------|
| Valid request | curl -H "X-API-Key: sk_test_123" URL/users | 200 OK |
| No API key | curl URL/users | 401 Unauthorized |
| Rate limit test | Send 6 requests | 6th gets 429 |
| View analytics | curl URL/admin/analytics?tenant=tenantA | JSON data |
| Enable chaos | curl -X POST URL/admin/chaos -d {...} | Confirmation |
| Disable chaos | curl -X POST URL/admin/chaos/recover | Confirmation |
| Health check | curl URL/health | ok |

Note: Replace URL with https://centralized-api-orchestration-engine.onrender.com

---

## Important Notes

1. The gateway is deployed on Render free tier
2. Service may sleep after 15 minutes of inactivity
3. First request after sleep takes 30-60 seconds to respond
4. All subsequent requests are fast
5. Redis data persists but may reset on redeployment
6. Logs are visible only in Render dashboard, not in cmd