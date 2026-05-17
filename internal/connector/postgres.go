package connector

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres 持有用于探测和变更的连接池（MVP）。
type Postgres struct {
	Pool *pgxpool.Pool
}

// Ping 检查连接是否正常。
func (p *Postgres) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.Pool.Ping(ctx)
}

// ListPublicTables 返回 public schema 中的表名列表（MVP 元数据）。
func (p *Postgres) ListPublicTables(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	rows, err := p.Pool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// DSN 从各部分构建 PostgreSQL URL（user/password 已 URL 转义）。
func DSN(host string, port int, user, password, database, sslmode string) string {
	if sslmode == "" {
		sslmode = "disable"
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   "/" + database,
	}
	q := u.Query()
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}
