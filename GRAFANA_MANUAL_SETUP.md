# Manual Grafana Setup (Quick)

## Step 1: Open Grafana
```
http://localhost:3000
```

Default login:
- Username: `admin`
- Password: `admin`

## Step 2: Add Datasource

1. Click **Settings** (gear icon) → **Data sources**
2. Click **Add new data source**
3. Select **JSON API**
4. Configure:
   - Name: `API Gateway Metrics`
   - URL: `http://localhost:8080`
   - Click **Save & Test**

You should see "datasource is working"

## Step 3: Create Simple Panels

Create a new dashboard:

### Panel 1: Request Count
- Title: "📊 Requests"
- Type: **Stat**
- Data source: **API Gateway Metrics**
- Query: `/admin/metrics` → select `requests_total`

### Panel 2: Error Rate
- Title: "🔴 Errors"
- Type: **Stat**
- Data source: **API Gateway Metrics**
- Query: `/admin/metrics` → select `errors_total`

### Panel 3: Dropped Requests
- Title: "🚫 Dropped"
- Type: **Stat**
- Data source: **API Gateway Metrics**
- Query: `/admin/metrics` → select `requests_dropped`

### Panel 4: Latency
- Title: "⏱️ Latency"
- Type: **Graph/Timeseries**
- Data source: **API Gateway Metrics**
- Query: `/admin/metrics` → select `latency_percentiles`

## Step 4: Test Connection

While gateway is running, open demo:
```
http://localhost:8080/demo
```

Click "Send 100 Requests/sec" - panels should start updating in Grafana!

## Troubleshooting

**"No data" in panels?**
- Check datasource URL can reach gateway
- Verify gateway is running: `curl http://localhost:8080/admin/metrics`
- Refresh page (F5)

**Can't see JSON API option?**
- Grafana may need plugin: restart Grafana and try again
- Or use **Infinity** plugin from Grafana marketplace

