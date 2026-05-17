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

// ColumnSchema 表示单个数据库列
type ColumnSchema struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// TableSchema 表示表及其列
type TableSchema struct {
	Name    string         `json:"name"`
	Columns []ColumnSchema `json:"columns"`
}

// SchemaResult 是已发现 schema 的顶层容器
type SchemaResult struct {
	Tables []TableSchema `json:"tables"`
}

// DiscoverSchema 连接到指定连接池并查询 information_schema.columns
// 以发现 public schema 中的表和列，受配置的最大数量限制
func DiscoverSchema(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, log *zap.Logger) (*SchemaResult, error) {
	qCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 为 MVP 安全性只扫描 public schema；隐式保留 catalog_name = current_database()
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

	// 按表名聚合列，维护插入顺序
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
			continue // 静默跳过超出限制的列
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

// ToJSON 将 SchemaResult 序列化为紧凑的 JSON 字符串，适合作为 schema_json 传递给 NL2SQL 工作节点
func (sr *SchemaResult) ToJSON() (string, error) {
	b, err := json.Marshal(sr)
	if err != nil {
		return "", fmt.Errorf("marshal schema: %w", err)
	}
	return string(b), nil
}

// ConnectToExternalDataSource 为 schema 发现创建临时连接池。调用者必须关闭返回的连接池。
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
