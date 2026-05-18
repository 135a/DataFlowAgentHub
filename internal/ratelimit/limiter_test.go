package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// testRedisAddr 从环境变量读取，或使用默认值
func testRedisAddr() string {
	return "localhost:6379"
}

// setupTestRedis 尝试连接本地 Redis，失败时跳过
func setupTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{
		Addr:     testRedisAddr(),
		Password: "",
		DB:       1, // 使用 1 号数据库避免干扰业务数据
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping test: Redis not available at %s: %v", testRedisAddr(), err)
	}
	t.Cleanup(func() { rdb.FlushDB(ctx); rdb.Close() })
	return rdb
}

// randKey 辅助生成测试用的唯一 key
func randKey(t *testing.T) string {
	t.Helper()
	return "test:" + t.Name()
}

// TestAllow_NilRedis_FailOpen 验证 Redis 为 nil 时 fail-open 行为：请求通过
func TestAllow_NilRedis_FailOpen(t *testing.T) {
	ok, err := Allow(context.Background(), nil, "test-key", 5, time.Minute, false)
	if err != nil {
		t.Errorf("expected no error in fail-open mode, got: %v", err)
	}
	if !ok {
		t.Error("expected true in fail-open mode")
	}
}

// TestAllow_NilRedis_FailClosed 验证 Redis 为 nil 时 fail-closed 行为：请求被拒绝
func TestAllow_NilRedis_FailClosed(t *testing.T) {
	ok, err := Allow(context.Background(), nil, "test-key", 5, time.Minute, true)
	if err == nil {
		t.Error("expected error in fail-closed mode")
	}
	if ok {
		t.Error("expected false in fail-closed mode")
	}
}

// TestAllow_WithinLimit 验证窗口内未超出限制时放行请求
func TestAllow_WithinLimit(t *testing.T) {
	rdb := setupTestRedis(t)
	ctx := context.Background()
	key := randKey(t)

	// 发送 3 个请求，限制为 5，应全部放行
	for i := 0; i < 3; i++ {
		ok, err := Allow(ctx, rdb, key, 5, 10*time.Second, false)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if !ok {
			t.Errorf("request %d: expected allowed, got blocked", i)
		}
	}
}

// TestAllow_ExceedsLimit 验证超出限制时返回限流
func TestAllow_ExceedsLimit(t *testing.T) {
	rdb := setupTestRedis(t)
	ctx := context.Background()
	key := randKey(t)
	limit := int64(3)

	// 发送 limit 个请求，应全部放行
	for i := 0; i < int(limit); i++ {
		ok, err := Allow(ctx, rdb, key, limit, 10*time.Second, false)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if !ok {
			t.Errorf("request %d: expected allowed, got blocked", i)
		}
	}

	// 第 limit+1 个请求应被限流
	ok, err := Allow(ctx, rdb, key, limit, 10*time.Second, false)
	if err == nil {
		t.Error("expected error when rate limit exceeded")
	}
	if ok {
		t.Error("expected false when rate limit exceeded")
	}
}

// TestAllow_DifferentKeysIndependent 验证不同 key 的限流计数器互相独立
func TestAllow_DifferentKeysIndependent(t *testing.T) {
	rdb := setupTestRedis(t)
	ctx := context.Background()
	key1 := randKey(t) + ":1"
	key2 := randKey(t) + ":2"
	limit := int64(2)

	// key1 发 3 个请求，达到上限
	for i := 0; i < int(limit); i++ {
		ok, err := Allow(ctx, rdb, key1, limit, 10*time.Second, false)
		if err != nil {
			t.Fatalf("key1 request %d: unexpected error: %v", i, err)
		}
		if !ok {
			t.Errorf("key1 request %d: expected allowed", i)
		}
	}

	// key1 第 3 个请求被限流
	ok, err := Allow(ctx, rdb, key1, limit, 10*time.Second, false)
	if ok {
		t.Error("expected key1 to be rate limited")
	}
	if err == nil {
		t.Error("expected error for key1 rate limit exceeded")
	}

	// key2 不受 key1 的影响，请求仍应通过
	ok, err = Allow(ctx, rdb, key2, limit, 10*time.Second, false)
	if err != nil {
		t.Fatalf("key2 unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected key2 to be allowed independently")
	}
}

// TestAllow_WindowExpiry 验证窗口过期后计数器重置
func TestAllow_WindowExpiry(t *testing.T) {
	rdb := setupTestRedis(t)
	ctx := context.Background()
	key := randKey(t)
	limit := int64(2)
	// 使用极短的窗口
	window := 100 * time.Millisecond

	// 发送 2 个请求，达到上限
	for i := 0; i < int(limit); i++ {
		ok, err := Allow(ctx, rdb, key, limit, window, false)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if !ok {
			t.Errorf("request %d: expected allowed", i)
		}
	}

	// 第 3 个被限流
	ok, err := Allow(ctx, rdb, key, limit, window, false)
	if ok {
		t.Error("expected to be rate limited")
	}
	_ = err

	// 等待窗口过期
	time.Sleep(window + 50*time.Millisecond)

	// 窗口过期后应可以再次请求
	ok, err = Allow(ctx, rdb, key, limit, window, false)
	if err != nil {
		t.Fatalf("unexpected error after window expiry: %v", err)
	}
	if !ok {
		t.Error("expected allowed after window expiry")
	}
}

// TestAllow_LargeWindow 验证大窗口正常工作
func TestAllow_LargeWindow(t *testing.T) {
	rdb := setupTestRedis(t)
	ctx := context.Background()
	key := randKey(t)

	// 使用 1 小时窗口和较高限制
	for i := 0; i < 10; i++ {
		ok, err := Allow(ctx, rdb, key, 100, 1*time.Hour, false)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if !ok {
			t.Errorf("request %d: expected allowed", i)
		}
	}
}

// TestAllow_FailClosedWithRedisError 验证在 fail-closed 模式下 Redis 错误时拒绝请求
func TestAllow_FailClosedWithRedisError(t *testing.T) {
	// 使用一个错误的 Redis 地址
	badRdb := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:1", // 不可能有 Redis 的端口
		Password: "",
		DB:       0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	ok, err := Allow(ctx, badRdb, "test-key", 5, time.Minute, true)
	if ok {
		t.Error("expected false when Redis is unavailable in fail-closed mode")
	}
	if err == nil {
		t.Error("expected error when Redis is unavailable in fail-closed mode")
	}
	badRdb.Close()
}

// TestAllow_FailOpenWithRedisError 验证在 fail-open 模式下 Redis 错误时放行请求
func TestAllow_FailOpenWithRedisError(t *testing.T) {
	// 使用一个错误的 Redis 地址
	badRdb := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:1",
		Password: "",
		DB:       0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	ok, err := Allow(ctx, badRdb, "test-key", 5, time.Minute, false)
	if !ok {
		t.Error("expected true (fail-open) when Redis is unavailable")
	}
	if err != nil {
		t.Errorf("expected no error in fail-open mode, got: %v", err)
	}
	badRdb.Close()
}
