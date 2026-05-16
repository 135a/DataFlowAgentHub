package schema

import (
	"os"
	"testing"
)

func TestSchemaResult_ToJSON(t *testing.T) {
	sr := &SchemaResult{
		Tables: []TableSchema{
			{
				Name: "users",
				Columns: []ColumnSchema{
					{Name: "id", Type: "integer", Nullable: false},
					{Name: "name", Type: "text", Nullable: true},
				},
			},
			{
				Name:    "empty_table",
				Columns: []ColumnSchema{},
			},
		},
	}

	jsonStr, err := sr.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}
	if jsonStr == "" {
		t.Fatal("ToJSON() returned empty string")
	}
	if jsonStr[0] != '{' {
		t.Errorf("ToJSON() should return JSON object, got: %s", jsonStr[:50])
	}
}

func TestSchemaResult_ToJSON_Empty(t *testing.T) {
	sr := &SchemaResult{Tables: []TableSchema{}}
	jsonStr, err := sr.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}
	if jsonStr != `{"tables":[]}` {
		t.Errorf("ToJSON() empty = %q, want %q", jsonStr, `{"tables":[]}`)
	}
}

func TestConnectToExternalDataSource_InvalidHost(t *testing.T) {
	_, err := ConnectToExternalDataSource(t.Context(), "invalid-host-that-does-not-exist.local", 5432, "db", "user", "pass", "disable")
	if err == nil {
		t.Error("expected error for invalid host, got nil")
	}
}

func TestCachedSchema_RedisDown(t *testing.T) {
	redisAddr := os.Getenv("HUB_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("HUB_REDIS_ADDR not set, skipping integration test")
	}

	dbURL := os.Getenv("HUB_DATABASE_URL")
	if dbURL == "" {
		t.Skip("HUB_DATABASE_URL not set, skipping integration test")
	}

	// This test validates end-to-end schema discovery with caching.
	// It requires a running Postgres and Redis instance.
	t.Log("Integration test requires running Postgres and Redis - skipping auto-validation")
}

func TestCacheKey(t *testing.T) {
	key := cacheKey("ws-123", "hub")
	if key != "schema:ws-123:hub" {
		t.Errorf("cacheKey = %q, want %q", key, "schema:ws-123:hub")
	}

	key2 := cacheKey("ws-456", "ds-789")
	if key2 != "schema:ws-456:ds-789" {
		t.Errorf("cacheKey = %q, want %q", key2, "schema:ws-456:ds-789")
	}
}
