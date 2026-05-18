package schema

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dataflowagenthub/hub/internal/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// cacheKey 构建用于 schema 缓存的 Redis 键
func cacheKey(workspaceID, sourceKey string) string {
	return fmt.Sprintf("schema:%s:%s", workspaceID, sourceKey)
}

// CachedSchema 返回指定数据源的 schema 发现结果。
// 它首先检查 Redis 缓存；未命中时调用 DiscoverSchema，将结果以配置的 TTL 存入 Redis 并返回。
func CachedSchema(ctx context.Context, db *sql.DB, rdb *redis.Client, cfg *config.Config, log *zap.Logger, workspaceID, sourceKey string) (*SchemaResult, error) {
	key := cacheKey(workspaceID, sourceKey)

	// 1. 尝试缓存
	cached, err := rdb.Get(ctx, key).Result()
	if err == nil {
		var sr SchemaResult
		if err := json.Unmarshal([]byte(cached), &sr); err == nil {
			return &sr, nil
		}
		log.Warn("schema cache unmarshal failed, re-discovering", zap.Error(err))
	}

	// 2. 从数据库发现
	sr, err := DiscoverSchema(ctx, db, cfg, log)
	if err != nil {
		return nil, err
	}

	// 3. 存入缓存
	jsonStr, err := sr.ToJSON()
	if err != nil {
		return sr, nil // still return result even if cache write fails
	}
	ttl := cfg.SchemaCacheTTL
	if ttl <= 0 {
		ttl = 300 * time.Second
	}
	if err := rdb.Set(ctx, key, jsonStr, ttl).Err(); err != nil {
		log.Warn("schema cache write failed", zap.Error(err))
	}

	return sr, nil
}
