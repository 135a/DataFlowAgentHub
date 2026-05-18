package seed

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dataflowagenthub/hub/internal/config"
	"golang.org/x/crypto/bcrypt"
)

const demoWorkspaceID = "00000000-0000-4000-8000-000000000001"

// EnsureAdminUser 在缺少种子管理员用户时创建之
func EnsureAdminUser(ctx context.Context, db *sql.DB, cfg *config.Config) error {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE workspace_id = ? AND email = ?`, demoWorkspaceID, cfg.SeedEmail).Scan(&count)
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
	res, err := db.ExecContext(ctx, `
		INSERT INTO users (workspace_id, email, password_hash, role, name)
		VALUES (?, ?, ?, 'super_admin', '管理员')`,
		demoWorkspaceID, cfg.SeedEmail, string(hash),
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("seed user insert affected 0 rows")
	}
	return nil
}

// DemoWorkspaceID 返回固定的演示工作区 UUID 字符串
func DemoWorkspaceID() string { return demoWorkspaceID }

// GetUserID 在演示工作区中通过邮箱加载用户 ID
func GetUserID(ctx context.Context, db *sql.DB, email string) (string, error) {
	var id string
	err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE workspace_id = ? AND email = ?`, demoWorkspaceID, email).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}
