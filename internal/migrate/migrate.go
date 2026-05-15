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

// Up applies embedded SQL migrations in lexical order.
func Up(ctx context.Context, pool *pgxpool.Pool) error {
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
	}
	return nil
}
