package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RevokeJWT 将 JWT ID（jti）加入 Redis 吊销列表。
// 密钥在指定 TTL 后自动过期以清理数据。
func RevokeJWT(ctx context.Context, rdb *redis.Client, jti string, ttl time.Duration) error {
	return rdb.Set(ctx, "jwt:revoked:"+jti, "1", ttl).Err()
}

// IsRevoked 检查 JWT ID（jti）是否在吊销列表中。
func IsRevoked(ctx context.Context, rdb *redis.Client, jti string) (bool, error) {
	if rdb == nil {
		return false, nil // fail-open：Redis 不可用时放行令牌
	}
	n, err := rdb.Exists(ctx, "jwt:revoked:"+jti).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
