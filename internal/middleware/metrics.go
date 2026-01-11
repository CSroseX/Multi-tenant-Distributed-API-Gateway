package middleware

import (
    "log"
    "net/http"
    "time"
)

func Metrics(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        duration := time.Since(start)

        log.Printf("[METRIC] Path=%s Method=%s Duration=%s", r.URL.Path, r.Method, duration)
    })
}
