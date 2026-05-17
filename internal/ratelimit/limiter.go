package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Allow 使用 Redis 中的有序集合检查滑动窗口计数器。
// failClosed: true 时 Redis 故障则拒绝请求，false 时放行（fail-open）。
func Allow(ctx context.Context, rdb *redis.Client, key string, limit int64, window time.Duration, failClosed bool) (bool, error) {
	if rdb == nil {
		if failClosed {
			return false, fmt.Errorf("rate limiter unavailable")
		}
		return true, nil
	}

	k := "rl:" + key
	now := time.Now().UnixNano()
	edge := time.Now().Add(-window).UnixNano()
	member := fmt.Sprintf("%d-%d", now, now%1000) // unique-ish member

	pipe := rdb.Pipeline()
	pipe.ZAdd(ctx, k, redis.Z{Score: float64(now), Member: member})
	pipe.ZRemRangeByScore(ctx, k, "0", fmt.Sprintf("%d", edge))
	countCmd := pipe.ZCard(ctx, k)
	pipe.Expire(ctx, k, window*2)

	_, err := pipe.Exec(ctx)
	if err != nil {
		if failClosed {
			return false, fmt.Errorf("rate limiter error: %w", err)
		}
		return true, nil // fail-open
	}

	if countCmd.Val() > limit {
		return false, fmt.Errorf("rate limit exceeded")
	}
	return true, nil
}
