package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Allow checks a sliding-window counter in Redis using a sorted set.
// If Redis fails, allows traffic (fail-open for MVP).
func Allow(ctx context.Context, rdb *redis.Client, key string, limit int64, window time.Duration) (bool, error) {
	if rdb == nil {
		return true, nil
	}

	k := "rl:" + key
	now := time.Now().UnixNano()
	edge := time.Now().Add(-window).UnixNano()
	member := fmt.Sprintf("%d-%d", now, now%1000) // unique-ish member

	pipe := rdb.Pipeline()
	// Add current request timestamp
	pipe.ZAdd(ctx, k, redis.Z{Score: float64(now), Member: member})
	// Remove entries outside the window
	pipe.ZRemRangeByScore(ctx, k, "0", fmt.Sprintf("%d", edge))
	// Count remaining entries in window
	countCmd := pipe.ZCard(ctx, k)
	// Set TTL on the key for automatic cleanup
	pipe.Expire(ctx, k, window*2)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, nil // fail-open
	}

	if countCmd.Val() > limit {
		return false, fmt.Errorf("rate limit exceeded")
	}
	return true, nil
}
