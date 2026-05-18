package sqlrun

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QueryRows 执行类 SELECT 查询，带超时和最大行数限制（包装后）
func QueryRows(ctx context.Context, pool *pgxpool.Pool, sql string, maxRows int32, timeout time.Duration) ([]map[string]any, error) {
	sql = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sql), ";"))
	if err := IsReadOnlySQL(sql); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	wrapped := fmt.Sprintf("SELECT * FROM (%s) AS _q LIMIT %d", sql, maxRows)
	rows, err := pool.Query(ctx, wrapped)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	desc := rows.FieldDescriptions()
	var out []map[string]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make(map[string]any, len(vals))
		for i, v := range vals {
			row[string(desc[i].Name)] = v
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		if err == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timeout")
		}
		return nil, err
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, nil
}

// ExecuteWrite 执行写操作（INSERT/UPDATE/DELETE/CREATE 等），带超时控制
// 调用方需先通过 ClassifySQL + IsAllowedForRole + IsSystemTable 进行权限检查
func ExecuteWrite(ctx context.Context, pool *pgxpool.Pool, sql string, timeout time.Duration) (int64, error) {
	sql = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sql), ";"))
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tag, err := pool.Exec(ctx, sql)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Ping 检查数据库连接是否正常
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	return pool.Ping(ctx)
}

// ErrNoRows 封装 pgx.ErrNoRows 供 handler 使用
var ErrNoRows = pgx.ErrNoRows

// QueryRowsMySQL 在 MySQL 连接池上执行类 SELECT 查询，带超时和最大行数限制。
func QueryRowsMySQL(ctx context.Context, pool *sql.DB, sqlStr string, maxRows int32, timeout time.Duration) ([]map[string]any, error) {
	sqlStr = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sqlStr), ";"))
	if err := IsReadOnlySQL(sqlStr); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	wrapped := fmt.Sprintf("SELECT * FROM (%s) AS _q LIMIT %d", sqlStr, maxRows)
	rows, err := pool.QueryContext(ctx, wrapped)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var out []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		valPtrs := make([]any, len(cols))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}
		if err := rows.Scan(valPtrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = vals[i]
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, nil
}

// ExecuteWriteMySQL 在 MySQL 连接池上执行写操作（INSERT/UPDATE/DELETE 等）。
func ExecuteWriteMySQL(ctx context.Context, pool *sql.DB, sqlStr string, timeout time.Duration) (int64, error) {
	sqlStr = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sqlStr), ";"))
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := pool.ExecContext(ctx, sqlStr)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
