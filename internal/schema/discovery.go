package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dataflowagenthub/hub/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ColumnSchema represents a single database column.
type ColumnSchema struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// TableSchema represents a table with its columns.
type TableSchema struct {
	Name    string         `json:"name"`
	Columns []ColumnSchema `json:"columns"`
}

// SchemaResult is the top-level container for discovered schema.
type SchemaResult struct {
	Tables []TableSchema `json:"tables"`
}

// DiscoverSchema connects to the given pool and queries information_schema.columns
// to discover tables and columns in the public schema, respecting configured max limits.
func DiscoverSchema(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, log *zap.Logger) (*SchemaResult, error) {
	qCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Only scan the public schema for MVP safety; keep catalog_name = current_database() implicitly.
	rows, err := pool.Query(qCtx, `
		SELECT table_name, column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		ORDER BY table_name, ordinal_position
	`)
	if err != nil {
		return nil, fmt.Errorf("information_schema query: %w", err)
	}
	defer rows.Close()

	// Aggregate columns by table name, maintaining insertion order.
	tableMap := make(map[string][]ColumnSchema)
	var tableOrder []string
	maxCols := int(cfg.SchemaMaxColumnsPerTable)
	maxTables := int(cfg.SchemaMaxTables)

	for rows.Next() {
		var t, c, dt, nullable string
		if err := rows.Scan(&t, &c, &dt, &nullable); err != nil {
			return nil, fmt.Errorf("scan schema row: %w", err)
		}
		cols, exists := tableMap[t]
		if !exists {
			if len(tableOrder) >= maxTables {
				log.Warn("schema_max_tables reached, truncating remaining tables",
					zap.Int("max_tables", maxTables))
				break
			}
			tableOrder = append(tableOrder, t)
			cols = make([]ColumnSchema, 0, 10)
		}
		if len(cols) >= maxCols {
			continue // silently skip columns beyond limit for this table
		}
		tableMap[t] = append(cols, ColumnSchema{
			Name:     c,
			Type:     dt,
			Nullable: nullable == "YES",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema rows: %w", err)
	}

	tables := make([]TableSchema, 0, len(tableOrder))
	for _, t := range tableOrder {
		tables = append(tables, TableSchema{Name: t, Columns: tableMap[t]})
	}

	return &SchemaResult{Tables: tables}, nil
}

// ToJSON serializes the SchemaResult to a compact JSON string
// suitable for passing as schema_json to the NL2SQL worker.
func (sr *SchemaResult) ToJSON() (string, error) {
	b, err := json.Marshal(sr)
	if err != nil {
		return "", fmt.Errorf("marshal schema: %w", err)
	}
	return string(b), nil
}

// ConnectToExternalDataSource creates a temporary connection pool for schema discovery.
// The caller MUST close the returned pool.
func ConnectToExternalDataSource(ctx context.Context, host string, port int, database, username, password, sslmode string) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		username, password, host, port, database, sslmode)
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn for external source: %w", err)
	}
	poolCfg.MaxConns = 1
	poolCfg.ConnConfig.ConnectTimeout = 5 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("connect to external source: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping external source: %w", err)
	}
	return pool, nil
}
