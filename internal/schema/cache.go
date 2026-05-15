package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dataflowagenthub/hub/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// cacheKey builds a Redis key for schema caching.
func cacheKey(workspaceID, sourceKey string) string {
	return fmt.Sprintf("schema:%s:%s", workspaceID, sourceKey)
}

// CachedSchema returns the discovered schema for the given data source.
// It first checks Redis; on miss it calls DiscoverSchema, stores the result
// in Redis with the configured TTL, and returns it.
//
// sourceKey: "hub" when using the platform database, or data_source_id for external sources.
func CachedSchema(ctx context.Context, pool *pgxpool.Pool, rdb *redis.Client, cfg *config.Config, log *zap.Logger, workspaceID, sourceKey string) (*SchemaResult, error) {
	key := cacheKey(workspaceID, sourceKey)

	// 1. Try cache
	cached, err := rdb.Get(ctx, key).Result()
	if err == nil {
		var sr SchemaResult
		if err := json.Unmarshal([]byte(cached), &sr); err == nil {
			return &sr, nil
		}
		log.Warn("schema cache unmarshal failed, re-discovering", zap.Error(err))
	}

	// 2. Discover from database
	sr, err := DiscoverSchema(ctx, pool, cfg, log)
	if err != nil {
		return nil, err
	}

	// 3. Store in cache
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
