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

// EnsureAdminUser 在缺少种子管理员用户时创建之
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
		INSERT INTO users (workspace_id, email, password_hash, role, name)
		VALUES ($1, $2, $3, 'admin', '管理员')`,
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

// DemoWorkspaceID 返回固定的演示工作区 UUID 字符串
func DemoWorkspaceID() string { return demoWorkspaceID }

const ServiceAPIUserID = "00000000-0000-4000-8000-000000000099"

// EnsureServiceAPIUser 为 X-Hub-Api-Key 认证插入专用的 admin 行（不可密码登录）
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

// GetUserID 在演示工作区中通过邮箱加载用户 ID
func GetUserID(ctx context.Context, pool *pgxpool.Pool, email string) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `SELECT id::text FROM users WHERE workspace_id = $1 AND email = $2`, demoWorkspaceID, email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}
