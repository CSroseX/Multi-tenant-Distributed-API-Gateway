# Grafana Integration Guide

## Quick Start

### Option 1: Automated Setup (PowerShell - Windows)

```powershell
cd grafana
.\setup.ps1
```

Then open: http://localhost:3000

### Option 2: Automated Setup (Bash - Linux/Mac)

```bash
cd grafana
chmod +x setup.sh
./setup.sh
```

### Option 3: Manual Setup

1. **Open Grafana**
   ```
   http://localhost:3000
   ```
   - Username: `admin`
   - Password: `admin`

2. **Add Data Source**
   - Left sidebar → **Data Sources**
   - Click **Add new data source**
   - Type: **JSON API**
   - Name: `API Gateway Metrics`
   - URL: `http://localhost:8080`
   - Click **Save & Test**

3. **Import Dashboard**
   - Left sidebar → **Dashboards**
   - Click **New** → **Import**
   - Upload `dashboard.json` from `grafana/` folder
   - Select data source: `API Gateway Metrics`
   - Click **Import**

---

## Dashboard Overview

The dashboard shows real-time metrics across 6 panels:

### 📊 Request Rate
- Total requests by route and tenant
- Spikes when load test is running
- Shows baseline traffic patterns

### ⚡ Chaos Status
- Live indicator of whether chaos is active
- Shows when you click buttons in the demo UI
- Red = chaos active, Green = system normal

### 🔴 Error Rate
- 4xx and 5xx errors by route
- Spikes dramatically when:
  - Failure injection is active (100% 503s)
  - Invalid API keys are sent
  - Rate limits are exceeded

### 🚫 Dropped Requests
- Shows requests dropped by chaos injection
- Spikes when "Drop 30%" button is clicked
- Normalizes when recovery is triggered

### ⏱️ Latency (P95, P99)
- Response time percentiles
- Spikes when latency injection is active (2000ms)
- Shows as clear line uplift

### 🚫 Rate Limit Blocks
- Requests blocked by rate limiter per tenant
- Spikes when rate limit burst is triggered
- Resets after time window

---

## Live Demo Flow

**Recommended setup:**
1. Open `/demo` in one window
2. Open Grafana dashboard in another (side-by-side)
3. Set Grafana refresh to **5s**

**Demo sequence:**

```
Click in Demo          →  Observe in Grafana
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Send 100 req/sec       →  Request rate jumps
Simulate Backend       →  Error rate spikes to 100%
  Failure              →  All responses = 503
                       →  Chaos status = ACTIVE

                       →  Wait 30 seconds (auto-recovery)
                       →  Error rate normalizes
                       →  Chaos status = INACTIVE

Inject 2s Latency      →  P95/P99 latency jumps
                       →  Request rate drops (slower)

Drop 30% Requests      →  Dropped requests line
                       →  Request rate = 70% of baseline

Invalid Key Storm      →  Error rate spikes (401s)
                       →  Different spike pattern

Recover System         →  All metrics normalize
                       →  All indicators green
```

---

## Grafana Best Practices

### Theme
- Dashboard uses **Dark** theme (optimized for recruiter demos)
- Metrics are highly visible on dark background

### Refresh Rate
- Set to **5s** for live updates
- Auto-refresh enabled by default

### Time Range
- Default: Last 1 hour
- Can zoom in for detail
- Real-time mode: "Last 5 minutes"

### Annotations
- Decision logs can be added to Grafana Loki
- Not required for this demo but enhances narrative

---

## Metrics Exposed

The `/admin/metrics` endpoint returns JSON with these keys:

```json
{
  "requests_total": {
    "route:tenant:status": count,
    "/users:tenantA:200": 1450,
    "/users:tenantA:503": 89
  },
  "errors_total": {
    "route:tenant": count,
    "/users:tenantA": 89
  },
  "requests_dropped": {
    "route:tenant": count,
    "/users:tenantA": 23
  },
  "rate_limit_blocks": {
    "tenant": count,
    "tenantA": 12
  },
  "latency_percentiles": {
    "route:tenant": {
      "p50": 45.2,
      "p95": 234.8,
      "p99": 1203.5
    }
  }
}
```

---

## Troubleshooting

### Dashboard shows "No Data"

1. Verify datasource is working:
   - Click datasource → **Test**
   - Should show success

2. Verify gateway is running:
   ```powershell
   curl http://localhost:8080/admin/metrics
   ```
   Should return JSON

3. Check Grafana logs:
   ```powershell
   docker logs grafana
   ```

### Metrics not updating

- Refresh interval set too high (change to 5s)
- Gateway may have crashed (restart: `go run cmd/gateway/main.go`)
- Redis connection issue (check Redis is running)

### Cannot import dashboard

- Make sure you're in the `grafana/` folder
- Datasource must exist first
- Dashboard format must be valid JSON

---

## Advanced: Custom Queries

To add custom panels:

1. Create new panel in Grafana
2. Query type: **JSON API**
3. URL path examples:
   - `/admin/metrics` (get all metrics)
   - `/admin/chaos/status` (get chaos state)

Example: Create a gauge for "% Error Rate"
- Query: `/admin/metrics`
- Transform: `errors_total / requests_total * 100`
- Threshold: 0 (green) → 5 (yellow) → 10 (red)

---

## Production Readiness

This dashboard is designed to:
- ✅ Work on Render/Fly.io (HTTP JSON API)
- ✅ Function with no external dependencies
- ✅ Update without restarting gateway
- ✅ Demonstrate SRE-grade observability
- ✅ Show clear cause-and-effect between chaos and metrics

All metrics are generated in-memory; no persistence needed.
