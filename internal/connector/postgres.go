package connector

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres holds a connection pool for introspection / ping (MVP).
type Postgres struct {
	Pool *pgxpool.Pool
}

// Ping checks connectivity.
func (p *Postgres) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.Pool.Ping(ctx)
}

// ListPublicTables returns table names in public schema (MVP metadata).
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

// DSN builds a postgres URL from parts (user/password URL-escaped).
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
