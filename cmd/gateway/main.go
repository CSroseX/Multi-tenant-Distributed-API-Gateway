package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/analytics"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/chaos"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/middleware"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/observability"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/proxy"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/ratelimit"
	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/tenant"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// ---- Tracing ----
	shutdown := observability.InitTracer("api-gateway")
	defer shutdown()
	http.Handle("/metrics", promhttp.Handler())
	// ---- Chaos auto-recovery watcher ----
	chaos.AutoRecover()

	// ---- Redis Client ----
	redisAddr := getEnv("REDIS_URL", "localhost:6379")
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// ---- Analytics Engine ----
	analyticsEngine := analytics.NewAnalytics(rdb)

	// ---- Rate Limiter ----
	rl := ratelimit.NewRateLimiter(rdb, 5, time.Minute)

	// ---- Backend proxies ----
	userServiceURL := getEnv("USER_SERVICE_URL", "http://localhost:9001")
	orderServiceURL := getEnv("ORDER_SERVICE_URL", "http://localhost:9002")
	userHandler, _ := proxy.ProxyHandler(userServiceURL)
	orderHandler, _ := proxy.ProxyHandler(orderServiceURL)

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
		tenant.ResolutionMiddleware(
			middleware.Metrics(
				middleware.Tracing(router),
			),
		),
	)

	http.Handle("/", finalHandler)

	// ---- CHAOS ADMIN API ----
	http.HandleFunc("/admin/chaos", chaos.ChaosConfigHandler)
	http.HandleFunc("/admin/chaos/recover", chaos.ChaosRecoverHandler)
	http.HandleFunc("/admin/chaos/status", chaos.ChaosStatusHandler)

	// Legacy endpoints for backward compatibility
	http.HandleFunc("/admin/chaos/enable", chaos.EnableHandler)
	http.HandleFunc("/admin/chaos/disable", chaos.DisableHandler)

	// ---- METRICS ENDPOINT (for Grafana scraping) ----
	http.HandleFunc("/admin/metrics", middleware.MetricsHandler)

	// ---- DEMO HTML PAGE ----
	http.HandleFunc("/demo", serveDemoHTML)

	log.Println("===============================================")
	log.Println("API Gateway running on http://localhost:8080")
	log.Println("===============================================")
	log.Println("")
	log.Println("📊 ENDPOINTS:")
	log.Println("  GET  /users                    → Proxied to localhost:9001")
	log.Println("  GET  /orders                   → Proxied to localhost:9002")
	log.Println("  GET  /admin/analytics          → Analytics data")
	log.Println("  GET  /admin/metrics            → Prometheus metrics (Grafana)")
	log.Println("")
	log.Println("⚡ CHAOS CONTROL:")
	log.Println("  POST /admin/chaos              → Enable chaos (fail_backend, slow_ms, drop_percent)")
	log.Println("  POST /admin/chaos/recover      → Disable all chaos")
	log.Println("  GET  /admin/chaos/status       → Current chaos state + stats")
	log.Println("")
	log.Println("🚀 DEMO:")
	log.Println("  GET  /demo                     → Interactive chaos demo UI")
	log.Println("")
	log.Println("🧪 QUICK TEST:")
	log.Println("  curl -H 'X-API-Key: sk_test_123' http://localhost:8080/users")
	log.Println("  curl http://localhost:8080/demo")
	log.Println("  curl http://localhost:8080/admin/chaos/status")
	log.Println("===============================================")

	port := getEnv("PORT", "8080")
	log.Printf("Starting server on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// getEnv retrieves environment variable or returns default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func serveDemoHTML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	// Inline HTML to avoid file dependencies
	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>API Gateway - Chaos Demo</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
            background: #0f1419;
            color: #e0e0e0;
            padding: 20px;
            line-height: 1.6;
        }
        .container { max-width: 1000px; margin: 0 auto; }
        h1 { 
            font-size: 28px;
            margin-bottom: 10px;
            color: #fff;
        }
        .subtitle { 
            font-size: 14px;
            color: #888;
            margin-bottom: 30px;
        }
        .section { 
            margin-bottom: 40px;
            padding: 0;
        }
        .section-title { 
            font-size: 14px;
            font-weight: 600;
            color: #888;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            margin-bottom: 15px;
        }
        .buttons { 
            display: grid;
            gap: 10px;
        }
        button {
            padding: 12px 16px;
            border: 1px solid #444;
            background: #1a1f2e;
            color: #e0e0e0;
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
            font-weight: 500;
            transition: all 0.2s;
            text-align: left;
        }
        button:hover { 
            background: #252d3d;
            border-color: #555;
        }
        button.primary:hover {
            background: #2d5016;
            border-color: #4caf50;
        }
        button.danger:hover {
            background: #5d1f1f;
            border-color: #f44336;
        }
        button.primary { 
            border-color: #4caf50;
            color: #4caf50;
        }
        button.danger { 
            border-color: #f44336;
            color: #f44336;
        }
        button:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }
        .status { 
            margin-top: 20px;
            padding: 15px;
            background: #1a1f2e;
            border: 1px solid #333;
            border-radius: 4px;
            font-size: 13px;
            font-family: "Monaco", "Courier New", monospace;
            word-break: break-word;
            white-space: pre-wrap;
        }
        .status.success { 
            border-color: #4caf50;
            color: #4caf50;
        }
        .status.error { 
            border-color: #f44336;
            color: #f44336;
        }
        .status.info { 
            border-color: #2196F3;
            color: #2196F3;
        }
        .metrics {
            margin-top: 20px;
            padding: 15px;
            background: #1a1f2e;
            border: 1px solid #333;
            border-radius: 4px;
            font-size: 12px;
            font-family: "Monaco", "Courier New", monospace;
        }
        .metric-row {
            display: flex;
            justify-content: space-between;
            padding: 4px 0;
            border-bottom: 1px solid #333;
        }
        .metric-row:last-child {
            border-bottom: none;
        }
        .metric-label { color: #888; }
        .metric-value { 
            font-weight: 600;
            color: #4caf50;
        }
        .load-test-grid {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 10px;
        }
        @media (max-width: 600px) {
            .load-test-grid {
                grid-template-columns: 1fr;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 API Gateway - Chaos Demo</h1>
        <p class="subtitle">Click buttons to inject failures • Observe metrics in Grafana • Recovery is automatic</p>

        <div class="section">
            <div class="section-title">📊 Load Generation</div>
            <div class="load-test-grid">
                <button onclick="loadTest(100)">Send 100 Requests/sec (10s)</button>
                <button onclick="loadTest(500)">Send 500 Requests/sec (5s)</button>
            </div>
        </div>

        <div class="section">
            <div class="section-title">⚡ Chaos Injection (30 sec auto-recovery)</div>
            <button class="danger" onclick="injectBackendFailure()">💥 Simulate Backend Failure (100%)</button>
            <button class="danger" onclick="injectLatency()">🐌 Inject 2s Latency</button>
            <button class="danger" onclick="injectDropRate()">🚫 Drop 30% of Requests</button>
            <button class="danger" onclick="injectCombined()">🌪️ All Three Combined</button>
        </div>

        <div class="section">
            <div class="section-title">🔓 Attack Scenarios</div>
            <button onclick="invalidKeyAttack()">Invalid API Key Storm</button>
            <button onclick="ratelimitTest()">Rate Limit Burst (500 req/sec)</button>
        </div>

        <div class="section">
            <div class="section-title">✅ Recovery</div>
            <button class="primary" onclick="recover()">Recover System</button>
            <button class="primary" onclick="checkStatus()">Check Chaos Status</button>
        </div>

        <div id="status"></div>

        <div class="metrics" id="metrics">
            <strong>Live Metrics</strong>
            <div class="metric-row">
                <span class="metric-label">Status:</span>
                <span class="metric-value" id="status-val">Loading...</span>
            </div>
            <div class="metric-row">
                <span class="metric-label">Total Requests:</span>
                <span class="metric-value" id="total-req">--</span>
            </div>
            <div class="metric-row">
                <span class="metric-label">Dropped Requests:</span>
                <span class="metric-value" id="dropped-req">--</span>
            </div>
            <div class="metric-row">
                <span class="metric-label">Failed Requests:</span>
                <span class="metric-value" id="failed-req">--</span>
            </div>
        </div>
    </div>

    <script>
        const API_URL = "http://localhost:8080";
        const VALID_KEY = "sk_test_123";

        function showStatus(message, type = "info") {
            const el = document.getElementById("status");
            el.textContent = message;
            el.className = "status " + type;
        }

        async function apiCall(endpoint, method = "GET", body = null, headers = {}) {
            try {
                const opts = {
                    method,
                    headers: {
                        "Content-Type": "application/json",
                        ...headers
                    }
                };
                if (body) opts.body = JSON.stringify(body);

                const res = await fetch(API_URL + endpoint, opts);
                const data = await res.json();
                return { status: res.status, data };
            } catch (err) {
                return { error: err.message };
            }
        }

        async function loadTest(rps) {
            showStatus('🔄 Sending ' + rps + ' req/sec for 10 seconds...', "info");
            const duration = 10000;
            const startTime = Date.now();
            let sent = 0;

            const timer = setInterval(async () => {
                if (Date.now() - startTime > duration) {
                    clearInterval(timer);
                    showStatus('✅ Load test complete: ' + sent + ' requests sent', "success");
                    return;
                }

                for (let i = 0; i < rps / 10; i++) {
                    sent++;
                    apiCall("/users", "GET", null, { "X-API-Key": VALID_KEY }).catch(() => {});
                }
            }, 100);
        }

        async function injectBackendFailure() {
            await apiCall("/admin/chaos", "POST", {
                fail_backend: true,
                slow_ms: 0,
                drop_percent: 0,
                duration_sec: 30
            });
            showStatus("💥 Backend failure injected - all requests return 503 for 30s", "error");
        }

        async function injectLatency() {
            await apiCall("/admin/chaos", "POST", {
                fail_backend: false,
                slow_ms: 2000,
                drop_percent: 0,
                duration_sec: 30
            });
            showStatus("🐌 2-second latency injected for 30s", "error");
        }

        async function injectDropRate() {
            await apiCall("/admin/chaos", "POST", {
                fail_backend: false,
                slow_ms: 0,
                drop_percent: 30,
                duration_sec: 30
            });
            showStatus("🚫 30% drop rate injected for 30s", "error");
        }

        async function injectCombined() {
            await apiCall("/admin/chaos", "POST", {
                fail_backend: false,
                slow_ms: 1000,
                drop_percent: 20,
                duration_sec: 30
            });
            showStatus("🌪️ Combined chaos: 1s latency + 20% drops for 30s", "error");
        }

        async function invalidKeyAttack() {
            showStatus("🔓 Sending 50 requests with invalid key...", "info");
            for (let i = 0; i < 50; i++) {
                apiCall("/users", "GET", null, { "X-API-Key": "invalid_key_" + i }).catch(() => {});
            }
            showStatus("✅ Invalid key storm complete - check Grafana for 401 spike", "success");
        }

        async function ratelimitTest() {
            showStatus("⚡ Rate limit burst (500 req/sec for 5s)...", "info");
            await loadTest(500);
        }

        async function recover() {
            await apiCall("/admin/chaos/recover", "POST");
            showStatus("✅ System recovered - all chaos disabled", "success");
            setTimeout(checkStatus, 1000);
        }

        async function checkStatus() {
            const res = await apiCall("/admin/chaos/status", "GET");
            if (res.data) {
                const cfg = res.data.config || {};
                const stats = res.data.stats || {};
                let msg = "CHAOS STATUS:\n";
                msg += "Enabled: " + (res.data.enabled ? "YES" : "NO") + "\n";
                msg += "Failures: " + (cfg.ErrorRate || 0) + "%\n";
                msg += "Drops: " + (cfg.DropRate || 0) + "%\n";
                msg += "Latency: " + (cfg.Delay || 0) + "ns\n";
                msg += "\nSTATS:\n";
                msg += "Total Requests: " + (stats.TotalRequests || 0) + "\n";
                msg += "Dropped: " + (stats.DroppedRequests || 0) + "\n";
                msg += "Failed: " + (stats.FailedRequests || 0);
                showStatus(msg, "info");
            }
        }

        async function refreshMetrics() {
            const res = await apiCall("/admin/metrics", "GET");
            if (res.data) {
                const stats = res.data.stats || {};
                document.getElementById("status-val").textContent = res.data.enabled ? "CHAOS ACTIVE" : "NORMAL";
                document.getElementById("total-req").textContent = stats.TotalRequests || 0;
                document.getElementById("dropped-req").textContent = stats.DroppedRequests || 0;
                document.getElementById("failed-req").textContent = stats.FailedRequests || 0;
            }
        }

        setInterval(refreshMetrics, 2000);
        checkStatus();
    </script>
</body>
</html>`

	w.Write([]byte(html))
}
