package main

import (
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/analytics"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/chaos"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/middleware"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/observability"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/proxy"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/ratelimit"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/tenant"
)

func main() {
	// ---- Tracing ----
	shutdown := observability.InitTracer("api-gateway")
	defer shutdown()

	// ---- Chaos auto-recovery watcher ----
	chaos.AutoRecover()

	// ---- Redis Client ----
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// ---- Analytics Engine ----
	analyticsEngine := analytics.NewAnalytics(rdb)

	// ---- Rate Limiter ----
	rl := ratelimit.NewRateLimiter(rdb, 5, time.Minute)

	// ---- Backend proxies ----
	userHandler, _ := proxy.ProxyHandler("http://localhost:9001")
	orderHandler, _ := proxy.ProxyHandler("http://localhost:9002")

	// ---- Middleware Stack for Secured Endpoints ----
	// Order (from outer to inner):
	// 1. Tenant Resolution  - Extracts tenant from X-API-Key (non-blocking)
	// 2. Analytics          - Records all requests, latency, errors (even if blocked later)
	// 3. Rate Limiter       - Enforces rate limits per tenant
	// 4. Chaos              - Simulates latency/errors if enabled
	// 5. Backend Handler    - Forwards to upstream service

	securedUserHandler := tenant.ResolutionMiddleware(
		analytics.Middleware(
			analyticsEngine,
			rl.Middleware(
				chaos.Middleware(userHandler),
			),
		),
	)
	
	securedOrderHandler := tenant.ResolutionMiddleware(
		analytics.Middleware(
			analyticsEngine,
			rl.Middleware(
				chaos.Middleware(orderHandler),
			),
		),
	)

	// ---- Router ----
	router := proxy.NewRouter()
	router.AddRoute("/users", securedUserHandler)
	router.AddRoute("/orders", securedOrderHandler)

	router.AddRoute("/admin/analytics", analytics.Handler(analyticsEngine))

	finalHandler := middleware.Logging(
		middleware.Metrics(
			middleware.Tracing(router),
		),
	)

	http.Handle("/", finalHandler)

	http.HandleFunc("/admin/chaos/enable", chaos.EnableHandler)
	http.HandleFunc("/admin/chaos/disable", chaos.DisableHandler)

	log.Println("===============================================")
	log.Println("API Gateway running on http://localhost:8080")
	log.Println("===============================================")
	log.Println("")
	log.Println("Endpoints:")
	log.Println("  GET  /users                 → Proxied to localhost:9001")
	log.Println("  GET  /orders                → Proxied to localhost:9002")
	log.Println("  GET  /admin/analytics       → Analytics data (add ?tenant=tenantA)")
	log.Println("  POST /admin/chaos/enable    → Enable chaos testing")
	log.Println("  POST /admin/chaos/disable   → Disable chaos testing")
	log.Println("")
	log.Println("Testing with:")
	log.Println("  curl -H 'X-API-Key: sk_test_123' http://localhost:8080/users")
	log.Println("  curl http://localhost:8080/admin/analytics?tenant=tenantA")
	log.Println("===============================================")
	log.Println("")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
