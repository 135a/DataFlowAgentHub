package auth

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 定义各类指标
var (
	tokenRevocationFailures = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "hub_token_revocation_failures_total",
			Help: "Total number of JWT revocation check failures (Redis unavailable etc.)",
		},
	)
)

// revokeLogger 是包级 zap.Logger 变量，通过 SetRevokeLogger 注入。
// 默认使用 zap.NewNop() 避免在未显式配置时产生日志输出。
var revokeLogger *zap.Logger = zap.NewNop()

// SetRevokeLogger 设置 revoke 包内部使用的日志记录器。
// 建议在 main.go 中初始化日志后调用。
func SetRevokeLogger(l *zap.Logger) {
	revokeLogger = l
}

// RevokeJWT 将 JWT ID（jti）加入 Redis 吊销列表。
// 密钥在指定 TTL 后自动过期以清理数据。
func RevokeJWT(ctx context.Context, rdb *redis.Client, jti string, ttl time.Duration) error {
	return rdb.Set(ctx, "jwt:revoked:"+jti, "1", ttl).Err()
}

// IsRevoked 检查 JWT ID（jti）是否在吊销列表中。
// 当 Redis 不可用时返回 false（fail-open）并记录警告日志和 Prometheus 指标。
func IsRevoked(ctx context.Context, rdb *redis.Client, jti string) (bool, error) {
	if rdb == nil {
		revokeLogger.Warn("IsRevoked: redis client is nil, failing open (allowing token)")
		tokenRevocationFailures.Inc()
		return false, nil
	}
	n, err := rdb.Exists(ctx, "jwt:revoked:"+jti).Result()
	if err != nil {
		revokeLogger.Warn("IsRevoked: redis check failed, failing open (allowing token)",
			zap.String("jti", jti),
			zap.Error(err),
		)
		tokenRevocationFailures.Inc()
		return false, err // 返回错误供上层记录，同时 fail-open 放行令牌
	}
	return n > 0, nil
}
