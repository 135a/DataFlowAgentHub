package sqlrun

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var writeKeywords = []string{
	"INSERT ", "UPDATE ", "DELETE ", "DROP ", "CREATE ", "ALTER ", "TRUNCATE ",
	"MERGE ", "GRANT ", "REVOKE ", "CALL ",
}

// IsReadOnlySQL returns false if the statement appears to contain write DDL/DML.
func IsReadOnlySQL(sql string) error {
	u := " " + strings.ToUpper(strings.TrimSpace(sql)) + " "
	for _, k := range writeKeywords {
		if strings.Contains(u, k) {
			return fmt.Errorf("only read-only SQL is allowed (matched %s)", strings.TrimSpace(k))
		}
	}
	return nil
}

// QueryRows executes a SELECT-like query with timeout and max rows (wrapped).
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

// Ping checks database connectivity.
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	return pool.Ping(ctx)
}

// ErrNoRows wraps pgx.ErrNoRows for handlers.
var ErrNoRows = pgx.ErrNoRows
