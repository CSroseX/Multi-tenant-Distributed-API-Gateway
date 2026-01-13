package analytics

import (
	"net/http"
	"time"

	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/tenant"
)

// Custom ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wrote {
		rw.status = code
		rw.wrote = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wrote {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Middleware wraps a handler and records request metrics for each tenant
// It captures:
// - Request count per endpoint per tenant
// - Latency in milliseconds
// - Error count (4xx and 5xx status codes)
//
// Important: This middleware runs BEFORE rate limiting, so it captures all requests
// including those that are rate-limited (429).
func Middleware(a *Analytics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		ww := &responseWriter{w, http.StatusOK, false}

		// Call next handler
		next.ServeHTTP(ww, r)

		// Record analytics only if tenant is available
		t, ok := tenant.FromContext(r.Context())
		if ok {
			duration := time.Since(start)
			a.RecordRequest(t.ID, r.URL.Path, duration, ww.status)
		}
	})
}
