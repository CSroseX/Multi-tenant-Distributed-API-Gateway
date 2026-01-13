# Grafana Setup Script for API Gateway Chaos Demo (Windows)

$GRAFANA_URL = "http://localhost:3000"
$GRAFANA_ADMIN_USER = "admin"
# Grafana docker default is "admin" but may need to check logs
# Use Invoke-WebRequest with basic auth
$GRAFANA_BASIC_AUTH = "Basic " + [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("admin:admin"))

Write-Host "===================================" -ForegroundColor Cyan
Write-Host "Setting up Grafana for Chaos Demo" -ForegroundColor Cyan
Write-Host "===================================" -ForegroundColor Cyan
Write-Host ""

# Wait for Grafana to be ready
Write-Host "⏳ Waiting for Grafana to start..." -ForegroundColor Yellow
$retries = 30
for ($i = 1; $i -le $retries; $i++) {
    try {
        $response = Invoke-WebRequest -Uri "$GRAFANA_URL/api/health" -UseBasicParsing -ErrorAction Stop
        Write-Host "✅ Grafana is ready" -ForegroundColor Green
        break
    }
    catch {
        if ($i -eq $retries) {
            Write-Host "❌ Grafana did not start in time" -ForegroundColor Red
            exit 1
        }
        Start-Sleep -Seconds 1
    }
}

Write-Host ""
Write-Host "1️⃣ Creating datasource..." -ForegroundColor Cyan

# Create datasource
try {
    $datasourceBody = @{
        name = "API Gateway Metrics"
        type = "jsonapi"
        url = "http://localhost:8080"
        access = "proxy"
        isDefault = $true
        jsonData = @{
            timeInterval = "2s"
        }
    } | ConvertTo-Json

    $response = Invoke-WebRequest -Uri "$GRAFANA_URL/api/datasources" `
        -Method Post `
        -ContentType "application/json" `
        -Headers @{ "Authorization" = $GRAFANA_BASIC_AUTH } `
        -Body $datasourceBody `
        -UseBasicParsing

    Write-Host "✅ Datasource configured" -ForegroundColor Green
}
catch {
    if ($_.Exception.Response.StatusCode -eq 409) {
        Write-Host "⚠️  Datasource already exists" -ForegroundColor Yellow
    }
    else {
        Write-Host "⚠️  Could not create datasource: $_" -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "2️⃣ Importing dashboard..." -ForegroundColor Cyan

# Import dashboard
try {
    $dashboardContent = Get-Content ".\dashboard.json" -Raw
    
    Invoke-WebRequest -Uri "$GRAFANA_URL/api/dashboards/db" `
        -Method Post `
        -ContentType "application/json" `
        -Headers @{ "Authorization" = $GRAFANA_BASIC_AUTH } `
        -Body $dashboardContent `
        -UseBasicParsing | Out-Null

    Write-Host "✅ Dashboard imported" -ForegroundColor Green
}
catch {
    Write-Host "❌ Failed to import dashboard: $_" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "===================================" -ForegroundColor Cyan
Write-Host "✨ Setup complete!" -ForegroundColor Green
Write-Host "===================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Open Grafana:" -ForegroundColor Green
Write-Host "  http://localhost:3000" -ForegroundColor Cyan
Write-Host ""
Write-Host "Default credentials:" -ForegroundColor Green
Write-Host "  Username: admin" -ForegroundColor Cyan
Write-Host "  Password: admin" -ForegroundColor Cyan
Write-Host ""
Write-Host "Dashboard: API Gateway - Chaos Engineering Demo" -ForegroundColor Green
Write-Host ""
