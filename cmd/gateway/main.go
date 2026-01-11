package main

import (
    "log"
    "net/http"
    "github.com/redis/go-redis/v9"
    "time"


    "github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/middleware"
    "github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/proxy"
    "github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/ratelimit"
    "github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/tenant"
    "github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/observability"
)

func main() {
    shutdown := observability.InitTracer("api-gateway")
    defer shutdown()

    // Redis client
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    rl := ratelimit.NewRateLimiter(rdb, 5, time.Minute)

    // Backend proxies
    userHandler, _ := proxy.ProxyHandler("http://localhost:9001")
    orderHandler, _ := proxy.ProxyHandler("http://localhost:9002")

    // Secured handlers
    securedUserHandler := tenant.Middleware(
        rl.Middleware(userHandler),
    )
    securedOrderHandler := tenant.Middleware(
        rl.Middleware(orderHandler),
    )

    // Router
    router := proxy.NewRouter()
    router.AddRoute("/users", securedUserHandler)
    router.AddRoute("/orders", securedOrderHandler)

    // Middleware order: Logging → Metrics → Tracing → Router
    http.Handle("/", middleware.Logging(
        middleware.Metrics(
            middleware.Tracing(router),
        ),
    ))

    log.Println("API Gateway running on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

