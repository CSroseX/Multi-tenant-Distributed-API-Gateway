package analytics

import (
	"context"
	"time"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type Analytics struct {
	redis *redis.Client
}

func NewAnalytics(r *redis.Client) *Analytics {
	return &Analytics{redis: r}
}

// Increment a counter for tenant + endpoint
func (a *Analytics) RecordRequest(tenantID, path string, duration time.Duration, statusCode int) error {
	ctx := context.Background()

	// Key for requests count
	reqKey := "analytics:req:" + tenantID + ":" + path
	a.redis.Incr(ctx, reqKey)

	// Key for latency
	latKey := "analytics:lat:" + tenantID + ":" + path
	a.redis.Set(ctx, latKey, int(duration.Milliseconds()), time.Minute*60)

	// Key for errors
	if statusCode >= 400 {
		errKey := "analytics:err:" + tenantID + ":" + path
		a.redis.Incr(ctx, errKey)
	}

	return nil
}

// Fetch analytics data
func (a *Analytics) FetchTenantAnalytics(tenantID string) (map[string]map[string]int, error) {
	ctx := context.Background()
	result := make(map[string]map[string]int)

	pattern := "analytics:req:" + tenantID + ":*"
	keys, _ := a.redis.Keys(ctx, pattern).Result()

	for _, k := range keys {
		parts := len("analytics:req:" + tenantID + ":")
		path := k[parts:]
		val, _ := a.redis.Get(ctx, k).Result()
		count, _ := strconv.Atoi(val)
		if result[path] == nil {
			result[path] = make(map[string]int)
		}
		result[path]["requests"] = count

		// errors
		errKey := "analytics:err:" + tenantID + ":" + path
		errVal, _ := a.redis.Get(ctx, errKey).Result()
		errCount, _ := strconv.Atoi(errVal)
		result[path]["errors"] = errCount
	}

	return result, nil
}
