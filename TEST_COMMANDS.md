# API Gateway - Quick Test Commands

**For Windows PowerShell users:** All commands below are PowerShell-compatible. Use the PowerShell section or bash section as needed.

## Prerequisites

### Terminal 1: Start Redis
```powershell
docker run -d --name redis -p 6379:6379 redis:latest
```

### Terminal 2: Start Gateway
```powershell
go run cmd/gateway/main.go
```

### Terminal 3: Start Mock Services (Optional)
```powershell
go run mock-user.go
go run mock-order.go
```

## Test Credentials
```
Tenant A: sk_test_123  (ID: tenantA)
Tenant B: sk_test_456  (ID: tenantB)
```

---

## Quick Test Commands

### 1. **Basic Request with Valid API Key** (Should succeed 200)
```powershell
curl -H "X-API-Key: sk_test_123" http://localhost:8080/users
```

### 2. **Request Without API Key** (Should fail 401)
```powershell
curl http://localhost:8080/users
```

### 3. **Request to Invalid Endpoint** (Should fail 404)
```powershell
curl -H "X-API-Key: sk_test_123" http://localhost:8080/invalid
```

### 4. **Check Analytics for Tenant A** (Pretty formatted)
```powershell
(curl http://localhost:8080/admin/analytics?tenant=tenantA).Content | ConvertFrom-Json | Format-List
```

**Note:** Make sure you have the opening `(` before curl!

### 5. **Check Analytics for Tenant A** (Compact view)
```powershell
curl http://localhost:8080/admin/analytics?tenant=tenantA
```

### 6. **Check Analytics for Tenant B** (Pretty formatted)
```powershell
(curl http://localhost:8080/admin/analytics?tenant=tenantB).Content | ConvertFrom-Json | Format-List
```

---

## Rate Limiting Tests (5 requests/minute)

### Test Rate Limiter (Send 6 requests rapidly)
```powershell
# Send 6 requests - first 5 should be 200, 6th should be 429
for ($i = 1; $i -le 6; $i++) {
    Write-Host "Request $i/6..."
    $response = curl -H "X-API-Key: sk_test_123" http://localhost:8080/users
    Write-Host "Response: $response"
    Start-Sleep -Milliseconds 200
}
```

**Expected:**
- Requests 1-5: `{"status":"ok",...}` (200 OK)
- Request 6: `Rate limit exceeded` (429)
- Analytics shows 6 requests recorded (including blocked one)

### Check Request Count in Analytics
```powershell
$analytics = (curl http://localhost:8080/admin/analytics?tenant=tenantA).Content | ConvertFrom-Json
Write-Host "Total /users requests: $($analytics.'/users'.requests)"
Write-Host "Total /users errors: $($analytics.'/users'.errors)"
```

### Wait for Rate Limit Reset (65 seconds)
```powershell
Write-Host "Waiting 65 seconds for rate limit to reset..."
Start-Sleep -Seconds 65

# Now this should work again
Write-Host "Testing after reset..."
curl -H "X-API-Key: sk_test_123" http://localhost:8080/users
```

---

## Chaos Middleware Tests

### Enable Chaos - 50% Error Rate
```powershell
$chaosConfig = @{
    delay = "0ms"
    errorRate = 50
    dropRate = 0
} | ConvertTo-Json

curl -X POST http://localhost:8080/admin/chaos/enable `
  -H "Content-Type: application/json" `
  -Body $chaosConfig

Write-Host "Chaos enabled: 50% error rate"
```

### Send Requests During Chaos (expect ~50% to fail)
```powershell
for ($i = 1; $i -le 5; $i++) {
    Write-Host "Request $i/5..."
    try {
        $response = curl -H "X-API-Key: sk_test_123" http://localhost:8080/users
        Write-Host "✓ Response: $response"
    } catch {
        Write-Host "✗ Error: $($_.Exception.Message)"
    }
    Start-Sleep -Milliseconds 500
}
```

### Enable Chaos - 1 Second Delay
```powershell
$chaosConfig = @{
    delay = "1000ms"
    errorRate = 0
    dropRate = 0
} | ConvertTo-Json

curl -X POST http://localhost:8080/admin/chaos/enable `
  -H "Content-Type: application/json" `
  -Body $chaosConfig

Write-Host "Chaos enabled: 1 second delay"
Write-Host "Making request (should take ~1 second)..."

$startTime = Get-Date
$response = curl -H "X-API-Key: sk_test_123" http://localhost:8080/users
$duration = (Get-Date) - $startTime

Write-Host "Response: $response"
Write-Host "Total time: $($duration.TotalSeconds) seconds"
```

### Enable Chaos - 30% Drop Rate
```powershell
$chaosConfig = @{
    delay = "0ms"
    errorRate = 0
    dropRate = 30
} | ConvertTo-Json

curl -X POST http://localhost:8080/admin/chaos/enable `
  -H "Content-Type: application/json" `
  -Body $chaosConfig

Write-Host "Chaos enabled: 30% drop rate"
Write-Host "Sending 10 requests (expect ~3 to be dropped)..."

$successCount = 0
$failureCount = 0

for ($i = 1; $i -le 10; $i++) {
    try {
        $response = curl -H "X-API-Key: sk_test_123" http://localhost:8080/users -ErrorAction SilentlyContinue
        if ($response) {
            $successCount++
            Write-Host "Request $i: ✓ Success"
        } else {
            $failureCount++
            Write-Host "Request $i: ✗ Dropped"
        }
    } catch {
        $failureCount++
        Write-Host "Request $i: ✗ Failed"
    }
    Start-Sleep -Milliseconds 300
}

Write-Host ""
Write-Host "Results: $successCount succeeded, $failureCount failed"
```

### Disable Chaos
```powershell
curl -X POST http://localhost:8080/admin/chaos/disable
Write-Host "Chaos disabled"
```

### Verify Chaos is Disabled
```powershell
Write-Host "Testing after disabling chaos (should succeed)..."
curl -H "X-API-Key: sk_test_123" http://localhost:8080/users
```
---

## Multi-Tenant Isolation Test

### Clear Analytics Data
```powershell
docker exec redis redis-cli FLUSHDB
Write-Host "Analytics cleared"
```

### Send Requests from Tenant A
```powershell
Write-Host "Sending 2 requests from Tenant A..."
for ($i = 1; $i -le 2; $i++) {
    Write-Host "Tenant A Request $i..."
    curl -H "X-API-Key: sk_test_123" http://localhost:8080/users
    Start-Sleep -Milliseconds 500
}
```

### Send Requests from Tenant B
```powershell
Write-Host "Sending 3 requests from Tenant B..."
for ($i = 1; $i -le 3; $i++) {
    Write-Host "Tenant B Request $i..."
    curl -H "X-API-Key: sk_test_456" http://localhost:8080/orders
    Start-Sleep -Milliseconds 500
}
```

### Check Tenant A Analytics (should show 2 requests)
```powershell
Write-Host "=== Tenant A Analytics ==="
$analyticsA = (curl http://localhost:8080/admin/analytics?tenant=tenantA).Content | ConvertFrom-Json
$analyticsA | Format-List

Write-Host "Total requests: $($analyticsA.'/users'.requests)"
Write-Host "Total errors: $($analyticsA.'/users'.errors)"
```

### Check Tenant B Analytics (should show 3 requests)
```powershell
Write-Host "=== Tenant B Analytics ==="
$analyticsB = (curl http://localhost:8080/admin/analytics?tenant=tenantB).Content | ConvertFrom-Json
$analyticsB | Format-List

Write-Host "Total requests: $($analyticsB.'/orders'.requests)"
Write-Host "Total errors: $($analyticsB.'/orders'.errors)"
```

---

## Redis Direct Inspection

### Check All Analytics Keys in Redis
```powershell
Write-Host "All analytics keys in Redis:"
docker exec redis redis-cli KEYS "analytics:*"
```

### Check Request Count for Tenant A /users
```powershell
$count = docker exec redis redis-cli GET "analytics:req:tenantA:/users"
Write-Host "Total requests to /users from tenantA: $count"
```

### Check Error Count for Tenant A /users
```powershell
$errors = docker exec redis redis-cli GET "analytics:err:tenantA:/users"
Write-Host "Total errors to /users from tenantA: $errors"
```

### Check Latency for Tenant A /users (in milliseconds)
```powershell
$latency = docker exec redis redis-cli GET "analytics:lat:tenantA:/users"
Write-Host "Last latency for /users from tenantA: $latency ms"
```

### Check Rate Limit Tokens for Tenant A
```powershell
$tokens = docker exec redis redis-cli GET "ratelimit:tenantA"
Write-Host "Remaining tokens for tenantA: $tokens"
```

### Check Rate Limit Tokens for Tenant B
```powershell
$tokens = docker exec redis redis-cli GET "ratelimit:tenantB"
Write-Host "Remaining tokens for tenantB: $tokens"
```

---

## Run Comprehensive Test Suite

### PowerShell (Windows)
```powershell
# Run the full test suite
powershell -ExecutionPolicy Bypass -File test-gateway.ps1
```

### Bash (Linux/Mac)
```bash
bash test-gateway.sh
```

Both scripts will run all tests automatically including:
- ✓ Basic connectivity
- ✓ Analytics recording
- ✓ Rate limiting
- ✓ Chaos injection
- ✓ Multi-tenant isolation
- ✓ Status code recording

---

## Expected Middleware Order (from outer to inner)

```
Request with X-API-Key: sk_test_123
   ↓
[Global] Logging Middleware
   ↓
[Global] Metrics Middleware
   ↓
[Global] Tracing Middleware
   ↓
Router (matches /users, /orders, /admin/*)
   ↓
[Endpoint] Analytics Middleware ← Records EVERYTHING
   ├─ Captures: Request count, latency, status code
   ├─ Stores in Redis: analytics:req, analytics:lat, analytics:err
   │
[Endpoint] Tenant Middleware ← Validates X-API-Key
   ├─ If missing/invalid → 401 (Analytics records it)
   ├─ If valid → Store tenant in context
   │
[Endpoint] Chaos Middleware ← Injects errors/latency
   ├─ If enabled + error injection → 503 (Analytics records it)
   ├─ If disabled → Continue normally
   │
[Endpoint] Rate Limiter ← Per-tenant limits (5/min)
   ├─ If tokens available → Decrement and continue
   ├─ If no tokens → 429 (Analytics records it!)
   │
[Endpoint] Backend Handler → Forward to mock service
   ├─ Returns 200 (or error from backend)
   │
Response (200, 401, 429, or 503)
   ↓
[Analytics] Records latency and status code to Redis
```

**Key Point**: Analytics wraps everything, so it records ALL responses including 401, 429, and 503!

---

## Quick Verification Tests

### Verify Analytics Works
```powershell
# 1. Clear data
docker exec redis redis-cli FLUSHDB

# 2. Make one request
curl -H "X-API-Key: sk_test_123" http://localhost:8080/users

# 3. Check if it was recorded
$data = docker exec redis redis-cli GET "analytics:req:tenantA:/users"
if ($data -eq "1") {
    Write-Host "✓ Analytics working: 1 request recorded"
} else {
    Write-Host "✗ Analytics not working"
}
```

### Verify Rate Limiter Works
```powershell
# 1. Clear data
docker exec redis redis-cli FLUSHDB

# 2. Send 6 requests
$results = @()
for ($i = 1; $i -le 6; $i++) {
    $response = curl -H "X-API-Key: sk_test_123" http://localhost:8080/users -ErrorAction SilentlyContinue
    if ($response -like "*Rate limit*") {
        $results += "BLOCKED"
    } else {
        $results += "OK"
    }
}

# 3. Check results
Write-Host "Results: $($results -join ', ')"
Write-Host "Expected: OK, OK, OK, OK, OK, BLOCKED"

# 4. Verify all 6 were recorded
$total = docker exec redis redis-cli GET "analytics:req:tenantA:/users"
Write-Host "Total requests recorded: $total (should be 6)"
```

### Verify Chaos Works
```powershell
# 1. Enable 100% error injection
$chaos = @{
    delay = "0ms"
    errorRate = 100
    dropRate = 0
} | ConvertTo-Json

curl -X POST http://localhost:8080/admin/chaos/enable `
  -H "Content-Type: application/json" `
  -Body $chaos

# 2. Make request (should fail with 503)
Write-Host "Making request with 100% error injection..."
$response = curl -H "X-API-Key: sk_test_123" http://localhost:8080/users
Write-Host "Response: $response"

if ($response -like "*Service Unavailable*") {
    Write-Host "✓ Chaos working: Got 503 error"
} else {
    Write-Host "✗ Chaos not working properly"
}

# 3. Disable chaos
curl -X POST http://localhost:8080/admin/chaos/disable
```

---

## Troubleshooting Guide

### Issue: Gateway not starting
```powershell
# Check if port 8080 is in use
netstat -ano | findstr :8080

# If port is in use, kill the process
taskkill /PID <PID> /F
```

### Issue: Redis connection refused
```powershell
# Check if Redis is running
docker ps | grep redis

# If not running, start it
docker run -d --name redis -p 6379:6379 redis:latest

# Test Redis connection
docker exec redis redis-cli PING
# Should return: PONG
```

### Issue: Mock backends not responding
```powershell
# Check if gateway started the backends
curl http://localhost:9001/users
curl http://localhost:9002/orders

# If not responding, start them manually
go run mock-user.go    # Terminal
go run mock-order.go   # Terminal
```

### Issue: Analytics not recording
```powershell
# Check 1: Did you use a valid API key?
# Use: sk_test_123 or sk_test_456

# Check 2: Is Redis running?
docker exec redis redis-cli PING

# Check 3: Do you have any analytics keys?
docker exec redis redis-cli KEYS "analytics:*"

# Check 4: View raw analytics data
docker exec redis redis-cli GET "analytics:req:tenantA:/users"
```

### Issue: jq command not found
```powershell
# Don't use jq in PowerShell! Use ConvertFrom-Json instead:
# ✗ Wrong (bash):  curl http://localhost:8080/admin/analytics?tenant=tenantA | jq .
# ✓ Right (PowerShell): (curl http://localhost:8080/admin/analytics?tenant=tenantA).Content | ConvertFrom-Json | Format-List
```

---

## Key Insights

1. **Analytics records BEFORE rate limiting** - All requests are tracked, even blocked ones (429)
2. **Tenant validation first** - Invalid API keys return 401 before rate limiting
3. **Chaos simulation before rate limiting** - Can test rate limiter under fault conditions
4. **Per-tenant isolation** - Each tenant has separate rate limit counter and analytics
5. **Admin endpoints unprotected** - `/admin/analytics`, `/admin/chaos/*` don't require API key
6. **PowerShell JSON parsing** - Use `ConvertFrom-Json` instead of `jq` for formatting

---

## Quick Summary Table

| Test | Command | Expected |
|------|---------|----------|
| Valid request | `curl -H "X-API-Key: sk_test_123" http://localhost:8080/users` | 200 OK |
| No API key | `curl http://localhost:8080/users` | 401 Unauthorized |
| Rate limit (6th req) | Send 6 requests | 6th is 429 |
| View analytics | `(curl http://localhost:8080/admin/analytics?tenant=tenantA).Content \| ConvertFrom-Json` | JSON response |
| Enable chaos | `curl -X POST http://localhost:8080/admin/chaos/enable -Body $chaos` | 200 OK |
| Check Redis | `docker exec redis redis-cli GET "analytics:req:tenantA:/users"` | Number |
| Clear Redis | `docker exec redis redis-cli FLUSHDB` | (empty) |

