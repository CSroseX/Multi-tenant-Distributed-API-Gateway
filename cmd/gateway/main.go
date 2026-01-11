package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/middleware"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/proxy"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/ratelimit"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/tenant"
	"github.com/redis/go-redis/v9"
)

func main() {
	// ---- Backends ----
	userBackend := "http://localhost:9001"
	orderBackend := "http://localhost:9002"

	userHandler, err := proxy.ProxyHandler(userBackend)
	if err != nil {
		log.Fatal(err)
	}

	orderHandler, err := proxy.ProxyHandler(orderBackend)
	if err != nil {
		log.Fatal(err)
	}

	// ---- Redis ----
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	rl := ratelimit.NewRateLimiter(redisClient, 5, time.Minute)

	// ---- Secure route handlers (middleware applied AFTER routing) ----
	securedUserHandler := tenant.Middleware(
		rl.Middleware(userHandler),
	)

	securedOrderHandler := tenant.Middleware(
		rl.Middleware(orderHandler),
	)

	// ---- Router ----
	router := proxy.NewRouter()
	router.AddRoute("/users", securedUserHandler)
	router.AddRoute("/orders", securedOrderHandler)

	// ---- Only logging wraps router ----
	http.Handle("/", middleware.Logging(router))

	fmt.Println("API Gateway running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
