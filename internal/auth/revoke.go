package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RevokeJWT adds a JWT ID (jti) to the revocation list in Redis.
// The key expires after the given TTL to auto-cleanup.
func RevokeJWT(ctx context.Context, rdb *redis.Client, jti string, ttl time.Duration) error {
	return rdb.Set(ctx, "jwt:revoked:"+jti, "1", ttl).Err()
}

// IsRevoked checks whether a JWT ID (jti) is in the revocation list.
func IsRevoked(ctx context.Context, rdb *redis.Client, jti string) (bool, error) {
	if rdb == nil {
		return false, nil // fail-open: if Redis is down, allow the token
	}
	n, err := rdb.Exists(ctx, "jwt:revoked:"+jti).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
