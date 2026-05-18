package schema

import (
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
