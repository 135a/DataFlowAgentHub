package seed

import (
	"context"
	"errors"

	"github.com/dataflowagenthub/hub/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const demoWorkspaceID = "00000000-0000-4000-8000-000000000001"

// EnsureAdminUser creates the seed admin user if missing.
func EnsureAdminUser(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) error {
	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE workspace_id = $1 AND email = $2`, demoWorkspaceID, cfg.SeedEmail).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.SeedPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tag, err := pool.Exec(ctx, `
		INSERT INTO users (workspace_id, email, password_hash, role)
		VALUES ($1, $2, $3, 'admin')`,
		demoWorkspaceID, cfg.SeedEmail, string(hash),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("seed user insert affected 0 rows")
	}
	return nil
}

// DemoWorkspaceID returns the fixed demo workspace UUID string.
func DemoWorkspaceID() string { return demoWorkspaceID }

const ServiceAPIUserID = "00000000-0000-4000-8000-000000000099"

// EnsureServiceAPIUser inserts a dedicated admin row for X-Hub-Api-Key auth (not password-loginable).
func EnsureServiceAPIUser(ctx context.Context, pool *pgxpool.Pool) error {
	hash, err := bcrypt.GenerateFromPassword([]byte("no-login-"+demoWorkspaceID), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, workspace_id, email, password_hash, role)
		VALUES ($1::uuid, $2::uuid, 'api-key@internal', $3, 'admin')
		ON CONFLICT (workspace_id, email) DO NOTHING`,
		ServiceAPIUserID, demoWorkspaceID, string(hash),
	)
	return err
}

// GetUserID loads user id by email in demo workspace.
func GetUserID(ctx context.Context, pool *pgxpool.Pool, email string) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `SELECT id::text FROM users WHERE workspace_id = $1 AND email = $2`, demoWorkspaceID, email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}
