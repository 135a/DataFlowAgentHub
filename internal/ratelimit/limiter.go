package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Allow 使用 Redis 中的有序集合检查滑动窗口计数器。如果 Redis 故障则放行流量（MVP 阶段 fail-open）。
func Allow(ctx context.Context, rdb *redis.Client, key string, limit int64, window time.Duration) (bool, error) {
	if rdb == nil {
		return true, nil
	}

	k := "rl:" + key
	now := time.Now().UnixNano()
	edge := time.Now().Add(-window).UnixNano()
	member := fmt.Sprintf("%d-%d", now, now%1000) // unique-ish member

	pipe := rdb.Pipeline()
	// 添加当前请求时间戳
	pipe.ZAdd(ctx, k, redis.Z{Score: float64(now), Member: member})
	// 移除窗口外的条目
	pipe.ZRemRangeByScore(ctx, k, "0", fmt.Sprintf("%d", edge))
	// 统计窗口内剩余条目数
	countCmd := pipe.ZCard(ctx, k)
	// 设置过期时间以自动清理
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
