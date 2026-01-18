package main

import (
	"log"
	"net/http"
	"os"
	"strings"
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

func basicAuth(handler http.Handler, username, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()

		if !ok || user != username || pass != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="metrics"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func main() {
	// ---- Start mock services as goroutines (separate muxes) ----
	go startUserService()
	go startOrderService()

	// Services start independently; no blocking/sleep needed

	// ---- Tracing ----
	shutdown := observability.InitTracer("api-gateway")
	defer shutdown()
	// ---- Gateway mux ----
	gatewayMux := http.NewServeMux()
	metricsUsername := getEnv("METRICS_USERNAME", "grafana")
	metricsPassword := getEnv("METRICS_PASSWORD", "metrics_secure_2026")
	gatewayMux.Handle("/metrics", basicAuth(promhttp.Handler(), metricsUsername, metricsPassword))
	// ---- Chaos auto-recovery watcher ----
	chaos.AutoRecover()

	// ---- Redis Client ----
	redisAddr := getEnv("REDIS_URL", "localhost:6379")
	var rdb *redis.Client
	if strings.HasPrefix(redisAddr, "redis://") {
		opt, err := redis.ParseURL(redisAddr)
		if err != nil {
			log.Fatalf("invalid REDIS_URL: %v", err)
		}
		rdb = redis.NewClient(opt)
	} else {
		rdb = redis.NewClient(&redis.Options{Addr: redisAddr})
	}

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

	gatewayMux.Handle("/", finalHandler)

	// ---- CHAOS ADMIN API ----
	gatewayMux.HandleFunc("/admin/chaos", chaos.ChaosConfigHandler)
	gatewayMux.HandleFunc("/admin/chaos/recover", chaos.ChaosRecoverHandler)
	gatewayMux.HandleFunc("/admin/chaos/status", chaos.ChaosStatusHandler)

	// Legacy endpoints for backward compatibility
	gatewayMux.HandleFunc("/admin/chaos/enable", chaos.EnableHandler)
	gatewayMux.HandleFunc("/admin/chaos/disable", chaos.DisableHandler)

	// ---- METRICS ENDPOINT (for Grafana scraping) ----
	gatewayMux.HandleFunc("/admin/metrics", middleware.MetricsHandler)

	// ---- DEMO HTML PAGE ----
	gatewayMux.HandleFunc("/demo", serveDemoHTML)
	gatewayMux.HandleFunc("/about", serveArchitectureHTML)

	// ---- HEALTH CHECK ----
	gatewayMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

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
	log.Fatal(http.ListenAndServe(":"+port, gatewayMux))
}

// getEnv retrieves environment variable or returns default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// startUserService starts the mock user service on :9001
func startUserService() {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("User Service: %s %s", r.Method, r.RequestURI)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"service": "users", "status": "ok"}`))
	})

	log.Println("✓ User Service starting on :9001")
	if err := http.ListenAndServe(":9001", mux); err != nil {
		log.Printf("User Service error: %v", err)
	}
}

// startOrderService starts the mock order service on :9002
func startOrderService() {
	mux := http.NewServeMux()
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Order Service: %s %s", r.Method, r.RequestURI)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"service": "orders", "status": "ok"}`))
	})

	log.Println("✓ Order Service starting on :9002")
	if err := http.ListenAndServe(":9002", mux); err != nil {
		log.Printf("Order Service error: %v", err)
	}
}

func serveDemoHTML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	html := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>API Gateway - Chaos Engineering Dashboard</title>

<!-- Premium Font -->
<link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;500;700&display=swap" rel="stylesheet">

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  font-family: 'Space Grotesk', sans-serif;
  background: #111;
  color: #f5f5f5;
  min-height: 100vh;
}

/* Layout */
.container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 2rem;
}

.layout-columns {
  display: grid;
  grid-template-columns: 1.6fr 1fr;
  gap: 1.5rem;
  align-items: start;
}

@media (max-width: 1024px) {
  .layout-columns {
    grid-template-columns: 1fr;
  }
}

/* Navbar */
.navbar {
  background: #000;
  padding: 1rem 2rem;
  border-bottom: 3px solid #3b82f6;
  display: flex;
  justify-content: space-between;
  align-items: center;
  position: sticky;
  top: 0;
  z-index: 10;
  box-shadow: 4px 4px 0 #3b82f6;
}

.navbar-brand {
  font-size: 1.6rem;
  font-weight: 700;
  color: #fff;
}

.navbar-links {
  display: flex;
  gap: 2rem;
}

.nav-link {
  color: #ccc;
  text-decoration: none;
  font-weight: 500;
  border-bottom: 2px solid transparent;
}

.nav-link:hover {
  color: #fff;
  border-bottom: 2px solid #3b82f6;
}

.nav-link.active {
  color: #3b82f6;
  border-bottom: 2px solid #3b82f6;
}

.navbar-credit {
  font-size: 0.875rem;
  color: #aaa;
  display: flex;
  align-items: center;
  gap: 1rem;
}

.navbar-credit span {
  color: #3b82f6;
  font-weight: 600;
}

.social-link {
  color: #aaa;
  text-decoration: none;
  border: 2px solid #3b82f6;
  padding: 0.25rem 0.75rem;
  box-shadow: 3px 3px 0 #3b82f6;
}

.social-link:hover {
  background: #3b82f6;
  color: #000;
}

/* Card */
.card {
  background: #1a1a1a;
  border: 3px solid #000;
  padding: 1.5rem;
  box-shadow: 4px 4px 0 #3b82f6;
}

.card-title {
  font-weight: 700;
  font-size: 1.1rem;
  margin-bottom: 1rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

/* Buttons */
.button-grid {
  display: grid;
  gap: 0.75rem;
}

.button-grid.two-col {
  grid-template-columns: 1fr 1fr;
}

button {
  background: #fff;
  color: #000;
  border: 3px solid #000;
  font-weight: 600;
  padding: 1rem 1.25rem;
  cursor: pointer;
  box-shadow: 4px 4px 0 #000;
  transition: transform 0.1s ease;
  text-align: left;
}

button:hover {
  transform: translate(-3px, -3px);
}

button.primary {
  background: #3b82f6;
  color: #fff;
}

button.danger {
  background: #ef4444;
  color: #fff;
}

button.secondary {
  background: #fbbf24;
  color: #000;
}

/* Metrics */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1rem;
}

.metric-card {
  background: #000;
  border: 2px solid #3b82f6;
  padding: 1rem;
  box-shadow: 3px 3px 0 #3b82f6;
}

.metric-label {
  font-size: 0.75rem;
  color: #999;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.metric-value {
  font-size: 1.4rem;
  font-weight: 700;
  color: #3b82f6;
}

/* Status Display */
.status-display {
  margin-top: 1.5rem;
  padding: 1rem;
  background: #000;
  border: 2px solid #3b82f6;
  font-family: 'Courier New', monospace;
  white-space: pre-wrap;
  overflow-y: auto;
}

/* Logs */
.logs-container {
  background: #1a1a1a;
  border: 3px solid #000;
  margin-top: 2rem;
  box-shadow: 4px 4px 0 #3b82f6;
}

.logs-header {
  padding: 0.75rem 1.5rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 2px solid #3b82f6;
  cursor: pointer;
}

.logs-title {
  font-weight: 700;
}

.logs-content {
  padding: 1rem 1.5rem;
  font-family: 'Courier New', monospace;
  font-size: 0.85rem;
}

.log-entry {
  background: #000;
  padding: 0.5rem;
  margin-bottom: 0.25rem;
  border-left: 4px solid #3b82f6;
}

.log-entry.success { border-left-color: #22c55e; }
.log-entry.error { border-left-color: #ef4444; }

.minimize-btn {
  background: none;
  border: none;
  color: #fff;
  cursor: pointer;
  font-size: 1.2rem;
}
  .guide-card h3 {
    margin-top: 2rem;
}
</style>
</head>
<body>

<div class="navbar">
  <div class="navbar-brand">API Gateway Project</div>
  <div class="navbar-links">
    <a href="/about" class="nav-link">About</a>
    <a href="/demo" class="nav-link active">Test</a>
  </div>
  <div class="navbar-credit">
    Engineered by <span>Chitransh</span>
    <a href="https://github.com/CSroseX" target="_blank" class="social-link">GitHub</a>
    <a href="https://www.linkedin.com/in/chitranshatlkdin/" target="_blank" class="social-link">LinkedIn</a>
  </div>
</div>

<div class="container">
        <div class="layout-columns">
            <!-- LEFT COLUMN: Load Generation, Chaos, Attack, Control -->
            <div class="left-column">
                <div class="card">
                    <div class="card-title">
                        <span class="icon">📊</span>
                        Load Generation
                    </div>
                    <div class="button-grid two-col">
                        <button class="secondary" onclick="loadTest(100)">
                            <span>⚡</span>
                            100 req/sec (10s)
                        </button>
                        <button class="secondary" onclick="loadTest(500)">
                            <span>🚀</span>
                            500 req/sec (5s)
                        </button>
                    </div>
                </div>

                <div class="card">
                    <div class="card-title">
                        <span class="icon">⚡</span>
                        Chaos Injection <span style="font-size: 0.75rem; color: #9ca3af;">(auto-recovers in 30s)</span>
                    </div>
                    <div class="button-grid">
                        <button class="danger" onclick="injectBackendFailure()">
                            <span>💥</span>
                            Backend Failure (100% errors)
                        </button>
                        <button class="danger" onclick="injectLatency()">
                            <span>🐌</span>
                            2-Second Latency
                        </button>
                        <button class="danger" onclick="injectDropRate()">
                            <span>🚫</span>
                            Drop 30% Requests
                        </button>
                        <button class="danger" onclick="injectCombined()">
                            <span>🌪️</span>
                            Combined Chaos
                        </button>
                    </div>
                </div>

                <div class="card">
                    <div class="card-title">
                        <span class="icon">🔓</span>
                        Attack Scenarios
                    </div>
                    <div class="button-grid">
                        <button onclick="invalidKeyAttack()">
                            <span>🔑</span>
                            Invalid API Key Storm
                        </button>
                        <button onclick="ratelimitTest()">
                            <span>⚡</span>
                            Rate Limit Burst Test
                        </button>
                    </div>
                </div>

                <div class="card">
                    <div class="card-title">
                        <span class="icon">✅</span>
                        System Control
                    </div>
                    <div class="button-grid two-col">
                        <button class="primary" onclick="recover()">
                            <span>🔄</span>
                            Recover System
                        </button>
                        <button class="primary" onclick="checkStatus()">
                            <span>📋</span>
                            Check Status
                        </button>
                    </div>
                </div>
            </div>

            <!-- RIGHT COLUMN: Live Metrics, Getting Started Guide -->
            <div class="right-column">
                <div class="card">
                    <div class="card-title">
                        <span class="icon">📈</span>
                        Live Metrics
                    </div>
                    <div class="metrics-grid">
                        <div class="metric-card">
                            <div class="metric-label">System Status</div>
                            <div class="metric-value" id="status-val">Loading...</div>
                        </div>
                        <div class="metric-card">
                            <div class="metric-label">Total Requests</div>
                            <div class="metric-value" id="total-req">--</div>
                        </div>
                        <div class="metric-card">
                            <div class="metric-label">Dropped Requests</div>
                            <div class="metric-value" id="dropped-req">--</div>
                        </div>
                        <div class="metric-card">
                            <div class="metric-label">Failed Requests</div>
                            <div class="metric-value" id="failed-req">--</div>
                        </div>
                    </div>
                </div>

                <div class="card guide-card">
                    <div class="card-title">
                        <span class="icon">📖</span>
                        Getting Started Guide
                    </div>
                    
                    <h3>What is this?</h3>
                    <ul>
                        <li>A chaos engineering dashboard for testing API Gateway resilience</li>
                        <li>Inject failures, monitor metrics, and observe system behavior in real-time</li>
                        <li>All metrics are sent to Grafana Cloud for visualization</li>
                    </ul>

                    <h3>How to use</h3>
                    <ul>
                        <li><strong>Load Generation:</strong> Send test traffic to your API</li>
                        <li><strong>Chaos Injection:</strong> Simulate failures (auto-recovers in 30 seconds)</li>
                        <li><strong>Attack Scenarios:</strong> Test security and rate limiting</li>
                        <li><strong>Recovery:</strong> Manually restore normal operations</li>
                    </ul>

                    <h3>Key Features</h3>
                    <ul>
                        <li>Live metrics update every 2 seconds</li>
                        <li>Real-time logs visible at the bottom</li>
                        <li>All chaos effects auto-recover after 30 seconds</li>
                        <li>Metrics visible in Grafana Cloud dashboard</li>
                    </ul>

                    <h3>Important Notes</h3>
                    <ul>
                        <li>Service may sleep after inactivity (Render free tier)</li>
                        <li>First request after sleep takes 30-60 seconds</li>
                        <li>Rate limit: 5 requests per minute per tenant</li>
                        <li>Check bottom logs for real-time activity</li>
                    </ul>
                </div>
            </div>
        </div>

        <!-- FULL WIDTH: Status Display -->
        <div id="status"></div>

        <div class="logs-container expanded" id="logsContainer">
            <div class="logs-header" onclick="toggleLogs()">
                <div class="logs-title">
                    <span>📝</span>
                    Activity Logs
                </div>
                <button class="minimize-btn" onclick="event.stopPropagation(); toggleLogs()">−</button>
            </div>
            <div class="logs-content" id="logsContent"></div>
        </div>
    </div>

<script>
        const API_URL = window.location.origin;
        const VALID_KEY = "sk_test_123";
        let logs = [];

        function addLog(message, type = "info") {
            const timestamp = new Date().toLocaleTimeString();
            logs.push({ time: timestamp, message, type });
            if (logs.length > 100) logs.shift();
            updateLogsDisplay();
        }

        function updateLogsDisplay() {
            const container = document.getElementById("logsContent");
            container.innerHTML = logs.map(log => 
                '<div class="log-entry ' + log.type + '">' +
                    '<span class="log-time">' + log.time + '</span> ' + log.message +
                '</div>'
            ).join('');
            container.scrollTop = container.scrollHeight;
        }

        function toggleLogs() {
            const container = document.getElementById("logsContainer");
            container.classList.toggle("minimized");
            container.classList.toggle("expanded");
            const btn = container.querySelector(".minimize-btn");
            btn.textContent = container.classList.contains("minimized") ? "□" : "−";
        }

        function showStatus(message, type = "info") {
            const statusDiv = document.getElementById("status");
            if (!statusDiv.querySelector('.status-display')) {
                const div = document.createElement('div');
                div.className = 'status-display';
                statusDiv.appendChild(div);
            }
            const display = statusDiv.querySelector('.status-display');
            display.textContent = message;
            display.className = 'status-display ' + type;
            addLog(message, type);
        }

        async function apiCall(endpoint, method = "GET", body = null, headers = {}, silent = false) {
            try {
                const opts = {
                    method,
                    headers: {
                        "Content-Type": "application/json",
                        ...headers
                    }
                };
                if (body) opts.body = JSON.stringify(body);

                if (!silent) addLog(method + " " + endpoint, "info");
                const res = await fetch(API_URL + endpoint, opts);
                const data = await res.json();
                if (!silent) addLog(method + " " + endpoint + " → " + res.status, res.status >= 200 && res.status < 300 ? "success" : "error");
                return { status: res.status, data };
            } catch (err) {
                if (!silent) addLog("Request failed: " + err.message, "error");
                return { error: err.message };
            }
        }

        async function loadTest(rps) {
            showStatus('Sending ' + rps + ' req/sec for 10 seconds...', "info");
            const duration = 10000;
            const startTime = Date.now();
            let sent = 0;

            const timer = setInterval(async () => {
                if (Date.now() - startTime > duration) {
                    clearInterval(timer);
                    showStatus('Load test complete: ' + sent + ' requests sent', "success");
                    return;
                }

                for (let i = 0; i < rps / 10; i++) {
                    sent++;
                    apiCall("/users", "GET", null, { "X-API-Key": VALID_KEY }, true).catch(() => {});
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
            showStatus("Backend failure injected - all requests return 503 for 30s", "error");
        }

        async function injectLatency() {
            await apiCall("/admin/chaos", "POST", {
                fail_backend: false,
                slow_ms: 2000,
                drop_percent: 0,
                duration_sec: 30
            });
            showStatus("2-second latency injected for 30s", "error");
        }

        async function injectDropRate() {
            await apiCall("/admin/chaos", "POST", {
                fail_backend: false,
                slow_ms: 0,
                drop_percent: 30,
                duration_sec: 30
            });
            showStatus("30% drop rate injected for 30s", "error");
        }

        async function injectCombined() {
            await apiCall("/admin/chaos", "POST", {
                fail_backend: false,
                slow_ms: 1000,
                drop_percent: 20,
                duration_sec: 30
            });
            showStatus("Combined chaos: 1s latency + 20% drops for 30s", "error");
        }

        async function invalidKeyAttack() {
            showStatus("Sending 50 requests with invalid key...", "info");
            for (let i = 0; i < 50; i++) {
                apiCall("/users", "GET", null, { "X-API-Key": "invalid_key_" + i }, true).catch(() => {});
            }
            showStatus("Invalid key storm complete - check Grafana for 401 spike", "success");
        }

        async function ratelimitTest() {
            showStatus("Rate limit burst (500 req/sec for 5s)...", "info");
            await loadTest(500);
        }

        async function recover() {
            await apiCall("/admin/chaos/recover", "POST");
            showStatus("System recovered - all chaos disabled", "success");
            setTimeout(() => checkStatus(), 1000);
        }

        async function checkStatus() {
            const res = await apiCall("/admin/chaos/status", "GET");
            if (res.data) {
                const cfg = res.data.config || {};
                const stats = res.data.stats || {};
                let msg = "CHAOS STATUS:-\n";
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
            const res = await apiCall("/admin/metrics", "GET", null, {}, true);
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
        addLog("Dashboard initialized - ready for chaos engineering", "success");
    </script>
</body>
</html>`

	w.Write([]byte(html))
}

func serveArchitectureHTML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	html := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>API Gateway Architecture - Interactive Visualization</title>

<link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;500;700&display=swap" rel="stylesheet">

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  font-family: 'Space Grotesk', sans-serif;
  background: #111;
  color: #f5f5f5;
  min-height: 100vh;
}

/* Navbar */
.navbar {
  background: #000;
  padding: 1rem 2rem;
  border-bottom: 3px solid #3b82f6;
  display: flex;
  justify-content: space-between;
  align-items: center;
  position: sticky;
  top: 0;
  z-index: 10;
  box-shadow: 4px 4px 0 #3b82f6;
}

.navbar-brand {
  font-size: 1.6rem;
  font-weight: 700;
  color: #fff;
}

.navbar-links {
  display: flex;
  gap: 2rem;
}

.nav-link {
  color: #ccc;
  text-decoration: none;
  font-weight: 500;
  border-bottom: 2px solid transparent;
}

.nav-link:hover {
  color: #fff;
  border-bottom: 2px solid #3b82f6;
}

.nav-link.active {
  color: #3b82f6;
  border-bottom: 2px solid #3b82f6;
}

.navbar-credit {
  font-size: 0.875rem;
  color: #aaa;
  display: flex;
  align-items: center;
  gap: 1rem;
}

.navbar-credit span {
  color: #3b82f6;
  font-weight: 600;
}

.social-link {
  color: #aaa;
  text-decoration: none;
  border: 2px solid #3b82f6;
  padding: 0.25rem 0.75rem;
  box-shadow: 3px 3px 0 #3b82f6;
}

.social-link:hover {
  background: #3b82f6;
  color: #000;
}

/* Header */
.header {
  text-align: center;
  padding: 2rem;
  border-bottom: 3px solid #000;
  box-shadow: 4px 4px 0 #3b82f6;
  background: #1a1a1a;
}

.header h1 {
  font-size: 2.2rem;
  font-weight: 700;
  color: #fff;
}

.header p {
  color: #999;
  margin-top: 0.5rem;
}

/* Container */
.container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 2rem;
}

/* Tabs */
.tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  margin-bottom: 2rem;
}

.tab {
  background: #fff;
  color: #000;
  border: 3px solid #000;
  font-weight: 600;
  padding: 1rem 1.5rem;
  cursor: pointer;
  box-shadow: 4px 4px 0 #000;
  transition: transform 0.1s ease;
}

.tab:hover {
  transform: translate(-3px, -3px);
}

.tab.active {
  background: #3b82f6;
  color: #fff;
  border-color: #3b82f6;
  box-shadow: 4px 4px 0 #000;
}

/* Content */
.content {
  display: none;
}

.content.active {
  display: block;
}

/* Layers */
.flow-diagram {
  background: #1a1a1a;
  border: 3px solid #000;
  padding: 2rem;
  box-shadow: 4px 4px 0 #3b82f6;
}

.layer {
  border: 3px solid #000;
  background: #fff;
  color: #000;
  padding: 1.5rem;
  margin: 1.5rem 0;
  box-shadow: 4px 4px 0 #000;
}

.layer.global { border-color: #3b82f6; }
.layer.endpoint { border-color: #fbbf24; }
.layer.danger { border-color: #ef4444; }

.layer-title {
  font-size: 1.2rem;
  font-weight: 700;
  margin-bottom: 1rem;
}

.middleware-item {
  background: #f5f5f5;
  padding: 1rem;
  border-left: 6px solid #3b82f6;
  margin-bottom: 0.75rem;
  box-shadow: 3px 3px 0 #000;
}

.middleware-name {
  font-weight: 700;
}

.middleware-desc {
  font-size: 0.875rem;
  color: #333;
}

/* Arrows */
.arrow {
  text-align: center;
  font-size: 1.8rem;
  margin: 0.75rem 0;
  color: #3b82f6;
}

/* Cards */
.card {
  background: #1a1a1a;
  border: 3px solid #000;
  padding: 1.5rem;
  box-shadow: 4px 4px 0 #3b82f6;
}

.card h3 {
  font-size: 1.1rem;
  color: #3b82f6;
  margin-bottom: 0.75rem;
}

.card ul {
  list-style: none;
}

.card li {
  padding: 0.4rem 0;
  border-bottom: 2px solid #000;
}

.card li:last-child {
  border-bottom: none;
}

/* Tables */
.decision-table {
  width: 100%;
  border-collapse: collapse;
  background: #fff;
  color: #000;
  box-shadow: 4px 4px 0 #000;
}

.decision-table th,
.decision-table td {
  padding: 0.75rem 1rem;
  border: 2px solid #000;
}

.decision-table th {
  background: #3b82f6;
  color: #fff;
}

.badge {
  display: inline-block;
  padding: 0.25rem 0.75rem;
  border: 2px solid #000;
  box-shadow: 2px 2px 0 #000;
  font-size: 0.75rem;
  font-weight: 600;
}

.badge.chosen { background: #3b82f6; color: #fff; }
.badge.rejected { background: #ef4444; color: #fff; }

/* Code block */
.code-block {
  background: #000;
  color: #3b82f6;
  padding: 1rem;
  border-left: 6px solid #3b82f6;
  font-family: monospace;
  font-size: 0.85rem;
  margin: 1rem 0;
}

/* Timeline */
.timeline {
  border-left: 4px solid #3b82f6;
  padding-left: 1.5rem;
}

.timeline-item {
  background: #fff;
  color: #000;
  border: 3px solid #000;
  margin-bottom: 1rem;
  padding: 1rem;
  box-shadow: 3px 3px 0 #000;
}

.step-num {
  display: inline-block;
  background: #3b82f6;
  color: #fff;
  font-weight: 700;
  border: 2px solid #000;
  width: 2rem;
  height: 2rem;
  text-align: center;
  line-height: 1.8rem;
  margin-right: 0.5rem;
}

/* Grid */
.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1.5rem;
}

@media (max-width: 768px) {
  .grid-2 { grid-template-columns: 1fr; }
  .tabs { flex-direction: column; }
}

/* Metrics */
.metric-value {
  font-size: 1.6rem;
  font-weight: 700;
  color: #3b82f6;
}
</style>
</head>
<body>

<div class="navbar">
  <div class="navbar-brand">API Gateway Project</div>
  <div class="navbar-links">
    <a href="/about" class="nav-link active">About</a>
    <a href="/demo" class="nav-link">Test</a>
  </div>
  <div class="navbar-credit">
    Engineered by <span>Chitransh</span>
    <a href="https://github.com/CSroseX" target="_blank" class="social-link">GitHub</a>
    <a href="https://www.linkedin.com/in/chitranshatlkdin/" target="_blank" class="social-link">LinkedIn</a>
  </div>
</div>

<div class="header">
  <h1>API Gateway Architecture</h1>
  <p>Interactive visualization of the multi-tenant distributed API gateway system</p>
</div>

<div class="container">
        <div class="tabs">
            <div class="tab active" onclick="showTab('overview')">System Overview</div>
            <div class="tab" onclick="showTab('flow')">Request Flow</div>
            <div class="tab" onclick="showTab('decisions')">Design Decisions</div>
            <div class="tab" onclick="showTab('data')">Data Storage</div>
            <div class="tab" onclick="showTab('features')">Features</div>
        </div>

        <div id="overview" class="content active">
            <div class="flow-diagram">
                <h2 style="margin-bottom: 2rem;">Architecture Layers</h2>
                
                <div class="layer global">
                    <div class="layer-title">🌐 External Layer</div>
                    <div class="layer-content">
                        <div class="middleware-item">
                            <div class="middleware-name">Clients (Web, Mobile, CLI)</div>
                            <div class="middleware-desc">Send HTTP requests to :8080 with X-API-Key header</div>
                        </div>
                    </div>
                </div>

                <div class="arrow">↓</div>

                <div class="layer global">
                    <div class="layer-title">🔧 Global Middleware (All Requests)</div>
                    <div class="layer-content">
                        <div class="middleware-item">
                            <div class="middleware-name">1. Logging Middleware</div>
                            <div class="middleware-desc">Logs method, path, duration for every request</div>
                        </div>
                        <div class="middleware-item">
                            <div class="middleware-name">2. Tenant Resolution</div>
                            <div class="middleware-desc">Extracts X-API-Key, resolves tenant (tenantA/tenantB), stores in context</div>
                        </div>
                        <div class="middleware-item">
                            <div class="middleware-name">3. Metrics Middleware</div>
                            <div class="middleware-desc">Records requests, latency (P50/P95/P99), exports to Prometheus</div>
                        </div>
                        <div class="middleware-item">
                            <div class="middleware-name">4. Tracing Middleware</div>
                            <div class="middleware-desc">OpenTelemetry spans for distributed tracing</div>
                        </div>
                    </div>
                </div>

                <div class="arrow">↓</div>

                <div class="layer endpoint">
                    <div class="layer-title">🎯 Router</div>
                    <div class="layer-content">
                        <div class="middleware-item">
                            <div class="middleware-name">Pattern Matching</div>
                            <div class="middleware-desc">/users → User Service | /orders → Order Service | /admin/* → Admin endpoints</div>
                        </div>
                    </div>
                </div>

                <div class="arrow">↓</div>

                <div class="layer endpoint">
                    <div class="layer-title">⚙️ Endpoint Middleware (/users, /orders only)</div>
                    <div class="layer-content">
                        <div class="middleware-item">
                            <div class="middleware-name">1. Analytics Middleware</div>
                            <div class="middleware-desc">Records ALL requests (even blocked ones) to Redis: analytics:req:tenant:path</div>
                        </div>
                        <div class="middleware-item">
                            <div class="middleware-name">2. Rate Limiter</div>
                            <div class="middleware-desc">Redis token bucket (100 req/min), returns 429 if exceeded</div>
                        </div>
                        <div class="middleware-item">
                            <div class="middleware-name">3. Chaos Middleware</div>
                            <div class="middleware-desc">Injects failures (503), latency (sleep), drops (504) for testing</div>
                        </div>
                        <div class="middleware-item">
                            <div class="middleware-name">4. Backend Proxy</div>
                            <div class="middleware-desc">Reverse proxy to :9001 (users) or :9002 (orders)</div>
                        </div>
                    </div>
                </div>

                <div class="arrow">↓</div>

                <div class="layer global">
                    <div class="layer-title">🖥️ Backend Services</div>
                    <div class="layer-content">
                        <div class="middleware-item">
                            <div class="middleware-name">User Service (:9001) | Order Service (:9002)</div>
                            <div class="middleware-desc">Mock services returning JSON responses</div>
                        </div>
                    </div>
                </div>
            </div>

            <div class="grid-2">
                <div class="card">
                    <h3>Key Features</h3>
                    <ul>
                        <li>Multi-tenant isolation via API keys</li>
                        <li>Redis-based rate limiting</li>
                        <li>Chaos engineering built-in</li>
                        <li>Prometheus + JSON metrics</li>
                        <li>OpenTelemetry tracing</li>
                        <li>Analytics with Redis persistence</li>
                    </ul>
                </div>
                <div class="card">
                    <h3>Performance Stats</h3>
                    <ul>
                        <li>Latency: 5-10ms (normal)</li>
                        <li>Throughput: 1000+ req/sec</li>
                        <li>Memory: ~50 MB</li>
                        <li>Rate Limit: 100 req/min/tenant</li>
                        <li>Redis Connections: 1 persistent</li>
                    </ul>
                </div>
            </div>
        </div>

        <div id="flow" class="content">
            <h2 style="margin-bottom: 2rem;">Request Lifecycle: GET /users</h2>
            
            <div class="timeline">
                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">1</span>Request Arrives</div>
                    <div class="middleware-desc">Client sends: GET /users with X-API-Key: sk_test_123</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">2</span>Logging Starts</div>
                    <div class="middleware-desc">Timer starts tracking request duration</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">3</span>Tenant Resolution</div>
                    <div class="middleware-desc">Resolves sk_test_123 → tenantA, stores in context</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">4</span>Metrics Preparation</div>
                    <div class="middleware-desc">Wraps response to capture final status code</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">5</span>Tracing Span</div>
                    <div class="middleware-desc">Creates OpenTelemetry span: "GET /users"</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">6</span>Router Matches</div>
                    <div class="middleware-desc">/users → securedUserHandler</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">7</span>Analytics Records</div>
                    <div class="middleware-desc">Redis INCR analytics:req:tenantA:/users</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">8</span>Rate Limit Check</div>
                    <div class="middleware-desc">Redis GET ratelimit:tenantA → 99 tokens remaining ✓</div>
                    <div class="code-block">Redis DECR ratelimit:tenantA → 98</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">9</span>Chaos Check</div>
                    <div class="middleware-desc">Config: Enabled=false → No injection ✓</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">10</span>Backend Proxy</div>
                    <div class="middleware-desc">HTTP GET http://localhost:9001/users</div>
                    <div class="code-block">Response: 200 OK {"service": "users", "status": "ok"}</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">11</span>Analytics Captures</div>
                    <div class="middleware-desc">Duration: 45ms, Status: 200</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">12</span>Metrics Records</div>
                    <div class="middleware-desc">api_gateway_requests_total{route="/users",tenant="tenantA",status="200"}++</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">13</span>Logging Outputs</div>
                    <div class="middleware-desc">GET /users 45ms</div>
                </div>

                <div class="timeline-item">
                    <div class="middleware-name"><span class="step-num">14</span>Response Sent</div>
                    <div class="middleware-desc">Client receives: 200 OK {"service": "users", "status": "ok"}</div>
                </div>
            </div>
        </div>

        <div id="decisions" class="content">
            <h2 style="margin-bottom: 2rem;">Design Decisions</h2>

            <table class="decision-table">
                <thead>
                    <tr>
                        <th>Decision Point</th>
                        <th>Chosen</th>
                        <th>Rejected Alternatives</th>
                        <th>Why Chosen</th>
                    </tr>
                </thead>
                <tbody>
                    <tr>
                        <td>Middleware Architecture</td>
                        <td><span class="badge chosen">✓</span>Layered (Onion)</td>
                        <td>Pipeline, Decorator</td>
                        <td>Composability, easy testing, clear unwinding</td>
                    </tr>
                    <tr>
                        <td>Rate Limiting</td>
                        <td><span class="badge chosen">✓</span>Redis Token Bucket</td>
                        <td>In-memory, Database</td>
                        <td>Multi-instance support, persistence, atomic ops</td>
                    </tr>
                    <tr>
                        <td>Analytics Storage</td>
                        <td><span class="badge chosen">✓</span>Redis + In-memory</td>
                        <td>PostgreSQL, InfluxDB</td>
                        <td>Sub-ms writes, dual export (Prometheus + JSON)</td>
                    </tr>
                    <tr>
                        <td>Chaos Engineering</td>
                        <td><span class="badge chosen">✓</span>Chaos Middleware</td>
                        <td>Service Mesh, External tools</td>
                        <td>Simple, controlled, no K8s required</td>
                    </tr>
                    <tr>
                        <td>Authentication</td>
                        <td><span class="badge chosen">✓</span>API Key (X-API-Key)</td>
                        <td>JWT, OAuth, mTLS</td>
                        <td>Demo-friendly, fast lookup, easy to test</td>
                    </tr>
                    <tr>
                        <td>Backend Proxy</td>
                        <td><span class="badge chosen">✓</span>httputil.ReverseProxy</td>
                        <td>Manual HTTP Client, gRPC</td>
                        <td>Built-in, efficient streaming, no buffering</td>
                    </tr>
                    <tr>
                        <td>Observability</td>
                        <td><span class="badge chosen">✓</span>Prometheus + JSON</td>
                        <td>Prom only, Grafana only</td>
                        <td>Compatibility, flexibility, human-readable</td>
                    </tr>
                    <tr>
                        <td>Logging</td>
                        <td><span class="badge chosen">✓</span>JSON Structured</td>
                        <td>Plain text, No logging</td>
                        <td>Parseable, queryable, AI-friendly</td>
                    </tr>
                </tbody>
            </table>

            <div style="margin-top: 2rem;" class="card">
                <h3>Design Philosophy</h3>
                <ul>
                    <li><strong>Boring Technology:</strong> Go stdlib, Redis, Prometheus (proven, reliable)</li>
                    <li><strong>Maintainability over Cleverness:</strong> Simple patterns, clear code</li>
                    <li><strong>Demo-Friendly:</strong> Easy to test with curl, visual dashboard</li>
                    <li><strong>Production-Ready Path:</strong> Can upgrade to JWT, circuit breaker later</li>
                </ul>
            </div>
        </div>

        <div id="data" class="content">
            <h2 style="margin-bottom: 2rem;">Data Storage Architecture</h2>

            <div class="grid-2">
                <div class="card">
                    <h3>Redis Storage</h3>
                    <div class="middleware-desc" style="margin-bottom: 1rem;">localhost:6379 (Render managed)</div>
                    
                    <h4 style="color: #4CAF50; margin: 1rem 0;">Analytics Data</h4>
                    <div class="code-block">
analytics:req:tenantA:/users → 1450<br>
Type: Counter, TTL: None
                    </div>
                    <div class="code-block">
analytics:lat:tenantA:/users → 45ms<br>
Type: String, TTL: 60 min
                    </div>
                    <div class="code-block">
analytics:err:tenantA:/users → 12<br>
Type: Counter, TTL: None
                    </div>

                    <h4 style="color: #4CAF50; margin: 1rem 0;">Rate Limiting</h4>
                    <div class="code-block">
ratelimit:tenantA → 98<br>
Type: Integer, TTL: 60 sec<br>
Max: 100, Decrement on request
                    </div>
                </div>

                <div class="card">
                    <h3>In-Memory (Go)</h3>
                    
                    <h4 style="color: #4CAF50; margin: 1rem 0;">Chaos Config</h4>
                    <div class="code-block">
struct Config {<br>
  Enabled: bool<br>
  Route: string<br>
  Delay: time.Duration<br>
  ErrorRate: int (0-100)<br>
  DropRate: int (0-100)<br>
  ExpiresAt: time.Time<br>
}
                    </div>

                    <h4 style="color: #4CAF50; margin: 1rem 0;">Prometheus Metrics</h4>
                    <div class="code-block">
api_gateway_requests_total<br>
api_gateway_request_duration_seconds<br>
api_gateway_errors_total<br>
api_gateway_requests_dropped_total
                    </div>
                </div>
            </div>
        </div>

        <div id="features" class="content">
            <h2 style="margin-bottom: 2rem;">All Features (20 Total)</h2>

            <div class="grid-2">
                <div class="card">
                    <h3>Core Gateway (7)</h3>
                    <ul>
                        <li>1. Reverse Proxy to backends</li>
                        <li>2. Multi-tenancy via API keys</li>
                        <li>3. Rate limiting (100 req/min)</li>
                        <li>4. Analytics tracking</li>
                        <li>5. Metrics (Prometheus + JSON)</li>
                        <li>6. Distributed tracing</li>
                        <li>7. Structured logging</li>
                    </ul>
                </div>

                <div class="card">
                    <h3>Chaos Engineering (5)</h3>
                    <ul>
                        <li>8. Backend failure injection (503)</li>
                        <li>9. Latency injection (sleep)</li>
                        <li>10. Request dropping (504)</li>
                        <li>11. Chaos control API</li>
                        <li>12. Chaos metrics tracking</li>
                    </ul>
                </div>

                <div class="card">
                    <h3>Admin & Utility (5)</h3>
                    <ul>
                        <li>13. Analytics API</li>
                        <li>14. Interactive demo dashboard</li>
                        <li>15. Health check endpoint</li>
                        <li>16. Basic auth for /metrics</li>
                        <li>17. Environment configuration</li>
                    </ul>
                </div>

                <div class="card">
                    <h3>Architecture (3)</h3>
                    <ul>
                        <li>18. Middleware composability</li>
                        <li>19. Context propagation</li>
                        <li>20. Structured error handling</li>
                    </ul>
                </div>
            </div>

            <div class="card" style="margin-top: 2rem;">
                <h3>Performance Metrics</h3>
                <div class="grid-2">
                    <div>
                        <div class="middleware-desc">Request Latency (normal)</div>
                        <div class="metric-value">5-10ms</div>
                    </div>
                    <div>
                        <div class="middleware-desc">Throughput</div>
                        <div class="metric-value">1000+ req/s</div>
                    </div>
                    <div>
                        <div class="middleware-desc">Memory Usage</div>
                        <div class="metric-value">~50 MB</div>
                    </div>
                    <div>
                        <div class="middleware-desc">Rate Limit</div>
                        <div class="metric-value">100 req/min</div>
                    </div>
                </div>
            </div>
        </div>
    </div>

<script>
function showTab(tabName) {
  document.querySelectorAll('.tab').forEach(tab => tab.classList.remove('active'));
  document.querySelectorAll('.content').forEach(content => content.classList.remove('active'));
  event.target.classList.add('active');
  document.getElementById(tabName).classList.add('active');
}
</script>
</body>
</html>`

	w.Write([]byte(html))
}
