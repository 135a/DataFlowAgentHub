package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/dataflowagenthub/hub/internal/ratelimit"
	"github.com/dataflowagenthub/hub/internal/seed"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// RegisterRequest 创建用户请求体
type RegisterRequest struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// Register admin 创建新用户（仅 admin 可调用）
func (a *App) Register(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	// 限流：每用户每分钟 10 次
	if c != nil {
		if ok, _ := ratelimit.Allow(r.Context(), a.Redis, "register:"+c.UserID, 10, time.Minute, a.Cfg.RateLimitFailClosed); !ok {
			errJSON(w, http.StatusTooManyRequests, "rate limit")
			return
		}
	}
	if c == nil || c.Role != "admin" {
		errJSON(w, http.StatusForbidden, "仅 admin 可创建用户")
		return
	}

	var body RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Phone = strings.TrimSpace(body.Phone)
	body.Name = strings.TrimSpace(body.Name)
	body.Role = strings.ToLower(strings.TrimSpace(body.Role))

	if body.Phone == "" || body.Name == "" || body.Password == "" {
		errJSON(w, http.StatusBadRequest, "姓名、手机号、密码为必填项")
		return
	}
	if body.Role != "operator" && body.Role != "viewer" {
		errJSON(w, http.StatusBadRequest, "角色必须为 operator 或 viewer")
		return
	}

	// 检查手机号是否已存在
	var exists int
	if err := a.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM users WHERE workspace_id = $1 AND phone = $2`,
		seed.DemoWorkspaceID(), body.Phone).Scan(&exists); err != nil {
		a.Log.Warn("check phone exists", zap.Error(err))
	}
	if exists > 0 {
		errJSON(w, http.StatusConflict, "手机号已存在")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		a.Log.Error("bcrypt hash", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "internal error")
		return
	}

	id := uuid.NewString()
	// email 列不可为 NULL，为手机号用户生成占位邮箱
	placeholderEmail := body.Phone + "@phone.local"
	_, err = a.DB.Exec(r.Context(), `
		INSERT INTO users (id, workspace_id, name, phone, email, password_hash, role)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)`,
		id, seed.DemoWorkspaceID(), body.Name, body.Phone, placeholderEmail, string(hash), body.Role)
	if err != nil {
		a.Log.Error("insert user", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "创建用户失败")
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"id":   id,
		"name": body.Name,
		"role": body.Role,
	})
}

// ListUsers 返回用户列表（仅 admin）
func (a *App) ListUsers(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || c.Role != "admin" {
		errJSON(w, http.StatusForbidden, "仅 admin 可查看用户列表")
		return
	}

	rows, err := a.DB.Query(r.Context(), `
		SELECT id::text, name, phone, role, created_at
		FROM users WHERE workspace_id = $1::uuid
		ORDER BY created_at ASC`, seed.DemoWorkspaceID())
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	var users []map[string]any
	for rows.Next() {
		var id, name, phone, role string
		var createdAt interface{}

		// phone 可能为 NULL
		var phonePtr *string
		if err := rows.Scan(&id, &name, &phonePtr, &role, &createdAt); err != nil {
			errJSON(w, http.StatusInternalServerError, "db scan error")
			return
		}
		if phonePtr != nil {
			phone = *phonePtr
		}

		users = append(users, map[string]any{
			"id":         id,
			"name":       name,
			"phone":      phone,
			"role":       role,
			"created_at": createdAt,
		})
	}

	JSON(w, http.StatusOK, map[string]any{"users": users})
}

// ChangeUserRole admin 修改用户角色（仅 admin）
func (a *App) ChangeUserRole(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || c.Role != "admin" {
		errJSON(w, http.StatusForbidden, "仅 admin 可修改用户角色")
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == "" {
		errJSON(w, http.StatusBadRequest, "missing user id")
		return
	}
	if userID == c.UserID {
		errJSON(w, http.StatusBadRequest, "不能修改自己的角色")
		return
	}

	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Role = strings.ToLower(strings.TrimSpace(body.Role))
	if body.Role != "operator" && body.Role != "viewer" && body.Role != "admin" {
		errJSON(w, http.StatusBadRequest, "角色必须为 admin/operator/viewer")
		return
	}

	// 检查目标用户是否为 admin（不能修改其他 admin）
	var targetRole string
	err := a.DB.QueryRow(r.Context(), `SELECT role FROM users WHERE id = $1::uuid AND workspace_id = $2::uuid`,
		userID, seed.DemoWorkspaceID()).Scan(&targetRole)
	if err != nil {
		errJSON(w, http.StatusNotFound, "用户不存在")
		return
	}
	if targetRole == "admin" {
		errJSON(w, http.StatusBadRequest, "不能修改 admin 用户的角色")
		return
	}

	_, err = a.DB.Exec(r.Context(), `UPDATE users SET role = $1 WHERE id = $2::uuid`, body.Role, userID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	JSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

// DeleteUser admin 删除用户（仅 admin，不能删自己，不能删 admin）
func (a *App) DeleteUser(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || c.Role != "admin" {
		errJSON(w, http.StatusForbidden, "仅 admin 可删除用户")
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == "" {
		errJSON(w, http.StatusBadRequest, "missing user id")
		return
	}
	if userID == c.UserID {
		errJSON(w, http.StatusBadRequest, "不能删除自己")
		return
	}

	// 检查目标用户是否为 admin
	var targetRole string
	err := a.DB.QueryRow(r.Context(), `SELECT role FROM users WHERE id = $1::uuid AND workspace_id = $2::uuid`,
		userID, seed.DemoWorkspaceID()).Scan(&targetRole)
	if err != nil {
		errJSON(w, http.StatusNotFound, "用户不存在")
		return
	}
	if targetRole == "admin" {
		errJSON(w, http.StatusBadRequest, "不能删除 admin 用户")
		return
	}

	_, err = a.DB.Exec(r.Context(), `DELETE FROM users WHERE id = $1::uuid AND workspace_id = $2::uuid`,
		userID, seed.DemoWorkspaceID())
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	JSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

