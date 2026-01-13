package chaos

import (
	"sync"
	"time"
)

var (
	mu    sync.RWMutex
	config Config
	stats Stats
)

func Set(cfg Config) {
	mu.Lock()
	defer mu.Unlock()
	config = cfg
	if cfg.Enabled {
		stats.LastInjectionTime = time.Now()
	}
}

func Get() Config {
	mu.RLock()
	defer mu.RUnlock()
	return config
}

func Clear() {
	mu.Lock()
	defer mu.Unlock()
	config = Config{}
	stats.LastRecoveryTime = time.Now()
}

func GetStats() Stats {
	mu.RLock()
	defer mu.RUnlock()
	return stats
}

func RecordRequest() {
	mu.Lock()
	defer mu.Unlock()
	stats.TotalRequests++
}

func RecordDrop() {
	mu.Lock()
	defer mu.Unlock()
	stats.DroppedRequests++
}

func RecordFail() {
	mu.Lock()
	defer mu.Unlock()
	stats.FailedRequests++
}

func RecordDelay() {
	mu.Lock()
	defer mu.Unlock()
	stats.DelayedRequests++
}

func AutoRecover() {
	go func() {
		for {
			time.Sleep(1 * time.Second)
			mu.Lock()
			if config.Enabled && !config.ExpiresAt.IsZero() &&
				time.Now().After(config.ExpiresAt) {
				config = Config{}
				stats.LastRecoveryTime = time.Now()
			}
			mu.Unlock()
		}
	}()
}
