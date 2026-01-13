#!/bin/bash
# Grafana Setup Script for API Gateway Chaos Demo

set -e

GRAFANA_URL="http://localhost:3000"
GRAFANA_ADMIN_USER="admin"
GRAFANA_ADMIN_PASS="admin"

echo "==================================="
echo "Setting up Grafana for Chaos Demo"
echo "==================================="
echo ""

# Wait for Grafana to be ready
echo "⏳ Waiting for Grafana to start..."
for i in {1..30}; do
    if curl -s "$GRAFANA_URL/api/health" > /dev/null 2>&1; then
        echo "✅ Grafana is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ Grafana did not start in time"
        exit 1
    fi
    sleep 1
done

echo ""
echo "1️⃣ Creating datasource..."

# Create datasource
curl -s -X POST "$GRAFANA_URL/api/datasources" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $(curl -s -X POST "$GRAFANA_URL/api/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"user\":\"$GRAFANA_ADMIN_USER\",\"password\":\"$GRAFANA_ADMIN_PASS\"}" | jq -r '.token')" \
  -d '{
    "name": "API Gateway Metrics",
    "type": "jsonapi",
    "url": "http://host.docker.internal:8080",
    "access": "proxy",
    "isDefault": true,
    "jsonData": {
      "timeInterval": "2s"
    }
  }' || echo "⚠️  Datasource may already exist"

echo "✅ Datasource configured"
echo ""

echo "2️⃣ Importing dashboard..."

# Get auth token
AUTH_TOKEN=$(curl -s -X POST "$GRAFANA_URL/api/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"user\":\"$GRAFANA_ADMIN_USER\",\"password\":\"$GRAFANA_ADMIN_PASS\"}" | jq -r '.token')

# Import dashboard
curl -s -X POST "$GRAFANA_URL/api/dashboards/db" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d @dashboard.json > /dev/null

echo "✅ Dashboard imported"
echo ""

echo "==================================="
echo "✨ Setup complete!"
echo "==================================="
echo ""
echo "Open Grafana:"
echo "  http://localhost:3000"
echo ""
echo "Default credentials:"
echo "  Username: admin"
echo "  Password: admin"
echo ""
echo "Dashboard: API Gateway - Chaos Engineering Demo"
echo ""
