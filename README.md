![Architecture](assets/architecture.png)

<p>
<img src="https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white&style=for-the-badge"/>
<img src="https://img.shields.io/badge/Redis-DC382D?logo=redis&logoColor=white&style=for-the-badge"/>
<img src="https://img.shields.io/badge/Prometheus-E6522C?logo=prometheus&logoColor=white&style=for-the-badge"/>
<img src="https://img.shields.io/badge/Grafana-F46800?logo=grafana&logoColor=white&style=for-the-badge"/>
<img src="https://img.shields.io/badge/OpenTelemetry-000000?logo=opentelemetry&logoColor=white&style=for-the-badge"/>
<img src="https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white&style=for-the-badge"/>
<img src="https://img.shields.io/badge/Render-46E3B7?logo=render&logoColor=white&style=for-the-badge"/>
</p>

## 1) Overview
- Live gateway: https://centralized-api-orchestration-engine.onrender.com/demo
- Live Grafana dashboard: https://csrosex.grafana.net/public-dashboards/3c2996c1426f4726bbd53f729b5c2a2c
- Go-based API gateway with layered middleware (logging, tracing, metrics, tenant resolution) fronting user and order services via reverse proxy.
- Built for resilience and insight: rate limiting per tenant (Redis token bucket), chaos injection, and analytics capture for every attempt.

## 2) Highlights
- Every request walks the same, intentional path: logging → metrics → tracing → tenant lookup → analytics → rate limit → chaos → reverse proxy.
- Resilience is baked in: Redis keeps tenant limits and analytics steady across restarts, and chaos controls let you rehearse failure without surprises.
- Observability is default, not optional: Prometheus, JSON metrics, and OpenTelemetry keep tenants isolated via `X-API-Key` while showing you what is really happening.

## 3) Live Observability
- Public views are on so you can watch the system breathe without setting anything up.

| Signal | Where to view | Notes |
| --- | --- | --- |
| Dashboard | https://csrosex.grafana.net/public-dashboards/3c2996c1426f4726bbd53f729b5c2a2c | Refreshes roughly every 30s. |
| Prometheus scrape | https://centralized-api-orchestration-engine.onrender.com/metrics | Text exposition format for scrapers. |
| JSON metrics | https://centralized-api-orchestration-engine.onrender.com/admin/metrics | Easy to curl, diff, and script. |
| Traces | internal/observability/otel.go | Point to your collector endpoint to receive spans. |

## 4) Chaos and Traffic Demo (5 minutes)
- Baseline: check status at /admin/chaos/status then hit /users with `X-API-Key: sk_test_123`.
- Induce failure: POST to /admin/chaos with `{ "fail_backend": true, "duration_sec": 30 }`, then hit /users to see 503s.
- Latency or drop tests: use `slow_ms` or `drop_percent` in the chaos payload; observe p95/p99 jump in Grafana within 30 seconds.
- Recovery: POST /admin/chaos/recover, send a few normal requests, and confirm metrics normalize.

## 5) Run Locally
- Prereqs: Go 1.22+, Redis (localhost:6379), ports 8080 (gateway), 9001/9002 (mock services).
- Run gateway: `go run cmd/gateway/main.go`.
- Hit it: `curl -H "X-API-Key: sk_test_123" http://localhost:8080/users` or visit http://localhost:8080/demo.
