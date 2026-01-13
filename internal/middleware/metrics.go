package middleware

import (
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// MetricsCollector holds Prometheus-style metrics
type MetricsCollector struct {
	mu sync.RWMutex

	// Counters
	requestCount   map[string]int64 // route:tenant:status
	errorCount     map[string]int64 // route:tenant
	droppedCount   map[string]int64 // chaos dropped requests
	rateLimitCount map[string]int64 // tenant blocked by rate limit

	// Histograms (simplified: track P50, P95, P99)
	latencies map[string][]time.Duration // route:tenant -> durations
}

var metricsCollector = &MetricsCollector{
	requestCount:   make(map[string]int64),
	errorCount:     make(map[string]int64),
	droppedCount:   make(map[string]int64),
	rateLimitCount: make(map[string]int64),
	latencies:      make(map[string][]time.Duration),
}

// RecordRequest records a request with labels
func RecordRequest(route, tenant, status string) {
	metricsCollector.mu.Lock()
	defer metricsCollector.mu.Unlock()
	key := route + ":" + tenant + ":" + status
	metricsCollector.requestCount[key]++
}

// RecordLatency records request latency with labels
func RecordLatency(route, tenant string, duration time.Duration) {
	metricsCollector.mu.Lock()
	defer metricsCollector.mu.Unlock()
	key := route + ":" + tenant
	metricsCollector.latencies[key] = append(metricsCollector.latencies[key], duration)
	// Keep only last 1000 samples per route:tenant
	if len(metricsCollector.latencies[key]) > 1000 {
		metricsCollector.latencies[key] = metricsCollector.latencies[key][1:]
	}
}

// RecordError records an error
func RecordError(route, tenant string) {
	metricsCollector.mu.Lock()
	defer metricsCollector.mu.Unlock()
	key := route + ":" + tenant
	metricsCollector.errorCount[key]++
}

// RecordDropped records a chaos-dropped request
func RecordDropped(route, tenant string) {
	metricsCollector.mu.Lock()
	defer metricsCollector.mu.Unlock()
	key := route + ":" + tenant
	metricsCollector.droppedCount[key]++
}

// RecordRateLimit records a rate limit block
func RecordRateLimit(tenant string) {
	metricsCollector.mu.Lock()
	defer metricsCollector.mu.Unlock()
	metricsCollector.rateLimitCount[tenant]++
}

// GetMetrics returns current metrics for Grafana scraping
func GetMetrics() map[string]interface{} {
	metricsCollector.mu.RLock()
	defer metricsCollector.mu.RUnlock()

	// Build percentiles
	percentiles := make(map[string]map[string]float64)
	for key, durations := range metricsCollector.latencies {
		if len(durations) == 0 {
			continue
		}
		// Simplified percentile calculation
		p50, p95, p99 := calculatePercentiles(durations)
		percentiles[key] = map[string]float64{
			"p50": p50,
			"p95": p95,
			"p99": p99,
		}
	}

	return map[string]interface{}{
		"requests_total":      metricsCollector.requestCount,
		"errors_total":        metricsCollector.errorCount,
		"requests_dropped":    metricsCollector.droppedCount,
		"rate_limit_blocks":   metricsCollector.rateLimitCount,
		"latency_percentiles": percentiles,
	}
}

func calculatePercentiles(durations []time.Duration) (float64, float64, float64) {
	if len(durations) == 0 {
		return 0, 0, 0
	}

	// Bubble sort for simplicity (not production-grade for large datasets)
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	p50 := float64(sorted[len(sorted)*50/100].Milliseconds())
	p95 := float64(sorted[len(sorted)*95/100].Milliseconds())
	p99 := float64(sorted[len(sorted)*99/100].Milliseconds())

	return p50, p95, p99
}

// ResponseWriter wrapper to capture status code
type statusCapture struct {
	http.ResponseWriter
	statusCode int
}

func (sc *statusCapture) WriteHeader(code int) {
	sc.statusCode = code
	sc.ResponseWriter.WriteHeader(code)
}

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sc := &statusCapture{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(sc, r)

		duration := time.Since(start)
		route := r.URL.Path
		tenant := r.Header.Get("X-Tenant-ID")
		if tenant == "" {
			tenant = "unknown"
		}
		status := strconv.Itoa(sc.statusCode)

		RecordRequest(route, tenant, status)
		RecordLatency(route, tenant, duration)

		if sc.statusCode >= 400 {
			RecordError(route, tenant)
		}

		log.Printf("[METRIC] path=%s tenant=%s status=%d duration_ms=%d",
			route, tenant, sc.statusCode, duration.Milliseconds())
	})
}
