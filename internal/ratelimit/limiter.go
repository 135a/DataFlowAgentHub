package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Allow checks a simple fixed-window counter in Redis. If Redis fails, allows traffic (fail-open for MVP).
func Allow(ctx context.Context, rdb *redis.Client, key string, limit int64, window time.Duration) (bool, error) {
	if rdb == nil {
		return true, nil
	}
	k := "rl:" + key
	n, err := rdb.Incr(ctx, k).Result()
	if err != nil {
		return true, nil
	}
	if n == 1 {
		_ = rdb.Expire(ctx, k, window).Err()
	}
	if n > limit {
		return false, fmt.Errorf("rate limit exceeded")
	}
	return true, nil
}
