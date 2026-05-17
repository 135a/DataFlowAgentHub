package migrate

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var fs embed.FS

// Up 按字典序应用嵌入的 SQL 迁移文件，通过 schema_migrations 表追踪已应用的版本以避免重复执行旧的迁移
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	// 创建追踪表（如果不存在）
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// 加载已应用的版本
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("query schema_migrations: %w", err)
	}
	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	// 读取嵌入的 SQL 文件
	entries, err := fs.ReadDir(".")
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if applied[name] {
			continue
		}
		b, err := fs.ReadFile(name)
		if err != nil {
			return err
		}
		sql := strings.TrimSpace(string(b))
		if sql == "" {
			continue
		}
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}
	return nil
}
