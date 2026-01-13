package chaos

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/CSroseX/Multi-tenant-Distributed-API-Gateway/internal/decisionlog"
)

// ChaosRequest represents a request to configure chaos
type ChaosRequest struct {
	FailBackend bool   `json:"fail_backend"`
	SlowMs      int    `json:"slow_ms"`
	DropPercent int    `json:"drop_percent"`
	DurationSec int    `json:"duration_sec"` // 0 = manual recovery only
	Route       string `json:"route"`        // empty = all routes
}

// ChaosResponse represents the current chaos state
type ChaosResponse struct {
	Enabled     bool   `json:"enabled"`
	Config      Config `json:"config"`
	Stats       Stats  `json:"stats"`
	IsRecovered bool   `json:"is_recovered"`
}

// ChaosConfigHandler handles POST /admin/chaos for setting chaos parameters
func ChaosConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChaosRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	cfg := Config{
		Enabled:   true,
		Route:     req.Route,
		ErrorRate: 0,
		DropRate:  0,
		Delay:     0,
	}

	// Build chaos configuration from request
	if req.FailBackend {
		cfg.ErrorRate = 100 // Force all requests to fail
	}
	if req.SlowMs > 0 {
		cfg.Delay = time.Duration(req.SlowMs) * time.Millisecond
	}
	if req.DropPercent > 0 {
		cfg.DropRate = req.DropPercent
	}

	// Set auto-recovery timer if duration specified
	if req.DurationSec > 0 {
		cfg.ExpiresAt = time.Now().Add(time.Duration(req.DurationSec) * time.Second)
	}

	Set(cfg)

	// Emit decision log
	decisionlog.LogDecision(r, decisionlog.DecisionChaos, "Chaos configuration applied", map[string]any{
		"fail_backend": req.FailBackend,
		"slow_ms":      req.SlowMs,
		"drop_percent": req.DropPercent,
		"duration_sec": req.DurationSec,
		"route":        req.Route,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Chaos enabled"})
}

// ChaosRecoverHandler handles POST /admin/chaos/recover to disable all chaos
func ChaosRecoverHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	Clear()

	// Emit decision log
	decisionlog.LogDecision(r, decisionlog.DecisionChaos, "Chaos recovery initiated", map[string]any{
		"action": "RECOVERY",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Chaos disabled - system recovered"})
}

// ChaosStatusHandler handles GET /admin/chaos/status to inspect current state
func ChaosStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg := Get()
	stats := GetStats()

	response := ChaosResponse{
		Enabled:     cfg.Enabled,
		Config:      cfg,
		Stats:       stats,
		IsRecovered: !cfg.Enabled && !stats.LastRecoveryTime.IsZero(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Legacy handlers for backward compatibility
func EnableHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Route    string `json:"route"`
		DelayMs  int    `json:"delay_ms"`
		ErrorPct int    `json:"error_rate"`
		DropPct  int    `json:"drop_rate"`
		Duration int    `json:"duration_sec"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	cfg := Config{
		Enabled:   true,
		Route:     req.Route,
		Delay:     time.Duration(req.DelayMs) * time.Millisecond,
		ErrorRate: req.ErrorPct,
		DropRate:  req.DropPct,
	}

	if req.Duration > 0 {
		cfg.ExpiresAt = time.Now().Add(time.Duration(req.Duration) * time.Second)
	}

	Set(cfg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Chaos enabled"})
}

func DisableHandler(w http.ResponseWriter, r *http.Request) {
	Clear()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Chaos disabled"})
}
