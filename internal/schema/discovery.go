package schema

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dataflowagenthub/hub/internal/config"
	"github.com/dataflowagenthub/hub/internal/sqlrun"
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
// 以发现 MySQL 数据库中的表和列，受配置的最大数量限制
func DiscoverSchema(ctx context.Context, db *sql.DB, cfg *config.Config, log *zap.Logger) (*SchemaResult, error) {
	qCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(qCtx, `
		SELECT table_name, column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
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
		// 跳过系统表，不暴露给用户
		if sqlrun.IsSystemTable(t) {
			continue
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

// ToJSON 将 SchemaResult 序列化为紧凑的 JSON 字符串
func (sr *SchemaResult) ToJSON() (string, error) {
	b, err := json.Marshal(sr)
	if err != nil {
		return "", fmt.Errorf("marshal schema: %w", err)
	}
	return string(b), nil
}
