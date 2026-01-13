package chaos

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/decisionlog"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := Get()
		RecordRequest()

		if !cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		if cfg.Route != "" && cfg.Route != r.URL.Path {
			next.ServeHTTP(w, r)
			return
		}

		// Inject delay
		if cfg.Delay > 0 {
			RecordDelay()
			decisionlog.LogDecision(r, decisionlog.DecisionChaos, "Injected latency", map[string]any{
				"delay_ms":   cfg.Delay.Milliseconds(),
				"chaos_type": "SLOW_MODE",
			})
			time.Sleep(cfg.Delay)
		}

		// Inject errors
		if cfg.ErrorRate > 0 && rand.Intn(100) < cfg.ErrorRate {
			RecordFail()
			decisionlog.LogDecision(r, decisionlog.DecisionChaos, "Injected backend failure", map[string]any{
				"error_code": http.StatusServiceUnavailable,
				"chaos_type": "FAIL_BACKEND",
			})
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"Service Unavailable (chaos injection)"}`))
			return
		}

		// Drop requests
		if cfg.DropRate > 0 && rand.Intn(100) < cfg.DropRate {
			RecordDrop()
			decisionlog.LogDecision(r, decisionlog.DecisionChaos, "Dropped request", map[string]any{
				"chaos_type": "DROP_PERCENT",
			})
			w.WriteHeader(http.StatusGatewayTimeout)
			w.Write([]byte(`{"error":"Request dropped (chaos injection)"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
