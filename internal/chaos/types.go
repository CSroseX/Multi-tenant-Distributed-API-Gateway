package chaos

import "time"

type Config struct {
	Enabled   bool
	Route     string        // empty = all routes
	Delay     time.Duration // artificial delay
	ErrorRate int           // % chance to return 503
	DropRate  int           // % chance to drop request
	ExpiresAt time.Time     // auto recovery time
}

// Stats tracks chaos injection metrics
type Stats struct {
	TotalRequests     int64
	DroppedRequests   int64
	FailedRequests    int64
	DelayedRequests   int64
	LastRecoveryTime  time.Time
	LastInjectionTime time.Time
}
