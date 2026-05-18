package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dataflowagenthub/hub/internal/middleware"
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

// 有效的角色列表
var validRoles = map[string]bool{
	"super_admin":        true,
	"data_admin":         true,
	"normal_user":        true,
	"read_only_visitor":  true,
}

// Register super_admin 创建新用户（仅 super_admin 可调用）
func (a *App) Register(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || c.Role != "super_admin" {
		errJSON(w, http.StatusForbidden, "only super_admin can create users")
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
		errJSON(w, http.StatusBadRequest, "name, phone, and password are required")
		return
	}
	if body.Role != "data_admin" && body.Role != "read_only_visitor" {
		errJSON(w, http.StatusBadRequest, "role must be data_admin or read_only_visitor")
		return
	}

	// 检查手机号是否已存在
	var exists int
	if err := a.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users WHERE workspace_id = ? AND phone = ?`,
		seed.DemoWorkspaceID(), body.Phone).Scan(&exists); err != nil {
		a.Log.Warn("check phone exists", zap.Error(err))
	}
	if exists > 0 {
		errJSON(w, http.StatusConflict, "phone number already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		a.Log.Error("bcrypt hash", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "internal error")
		return
	}

	id := uuid.NewString()
	placeholderEmail := body.Phone + "@phone.local"
	_, err = a.DB.ExecContext(r.Context(), `
		INSERT INTO users (id, workspace_id, name, phone, email, password_hash, role)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, seed.DemoWorkspaceID(), body.Name, body.Phone, placeholderEmail, string(hash), body.Role)
	if err != nil {
		a.Log.Error("insert user", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"id":   id,
		"name": body.Name,
		"role": body.Role,
	})
}

// SelfRegister 前端开放注册，仅允许注册为 normal_user
func (a *App) SelfRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Phone = strings.TrimSpace(body.Phone)
	body.Name = strings.TrimSpace(body.Name)
	if body.Phone == "" || body.Name == "" || body.Password == "" {
		errJSON(w, http.StatusBadRequest, "name, phone, and password are required")
		return
	}

	// 检查手机号是否已存在
	var exists int
	if err := a.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users WHERE workspace_id = ? AND phone = ?`,
		seed.DemoWorkspaceID(), body.Phone).Scan(&exists); err != nil {
		a.Log.Warn("check phone exists", zap.Error(err))
	}
	if exists > 0 {
		errJSON(w, http.StatusConflict, "phone number already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		a.Log.Error("bcrypt hash", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "internal error")
		return
	}

	id := uuid.NewString()
	placeholderEmail := body.Phone + "@phone.local"
	_, err = a.DB.ExecContext(r.Context(), `
		INSERT INTO users (id, workspace_id, name, phone, email, password_hash, role)
		VALUES (?, ?, ?, ?, ?, ?, 'normal_user')`,
		id, seed.DemoWorkspaceID(), body.Name, body.Phone, placeholderEmail, string(hash))
	if err != nil {
		a.Log.Error("insert user", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to register")
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"id":   id,
		"name": body.Name,
		"role": "normal_user",
		"message": "registration successful, please login",
	})
}

// ListUsers 返回用户列表（仅 super_admin）
func (a *App) ListUsers(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || c.Role != "super_admin" {
		errJSON(w, http.StatusForbidden, "only super_admin can list users")
		return
	}

	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT id, name, phone, role, created_at
		FROM users WHERE workspace_id = ?
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
	if err := rows.Err(); err != nil {
		a.Log.Warn("list users rows iteration", zap.Error(err))
	}

	JSON(w, http.StatusOK, map[string]any{"users": users})
}

// ChangeUserRole super_admin 修改用户角色（仅 super_admin，不能改 super_admin）
func (a *App) ChangeUserRole(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || c.Role != "super_admin" {
		errJSON(w, http.StatusForbidden, "only super_admin can change user role")
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == "" {
		errJSON(w, http.StatusBadRequest, "missing user id")
		return
	}
	if userID == c.UserID {
		errJSON(w, http.StatusBadRequest, "cannot change your own role")
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
	if !validRoles[body.Role] {
		errJSON(w, http.StatusBadRequest, "role must be one of: super_admin, data_admin, normal_user, read_only_visitor")
		return
	}

	var targetRole string
	err := a.DB.QueryRowContext(r.Context(), `SELECT role FROM users WHERE id = ? AND workspace_id = ?`,
		userID, seed.DemoWorkspaceID()).Scan(&targetRole)
	if err != nil {
		errJSON(w, http.StatusNotFound, "user not found")
		return
	}
	if targetRole == "super_admin" {
		errJSON(w, http.StatusBadRequest, "cannot change role of a super_admin user")
		return
	}

	_, err = a.DB.ExecContext(r.Context(), `UPDATE users SET role = ? WHERE id = ?`, body.Role, userID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	JSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

// DeleteUser super_admin 删除用户（仅 super_admin，不能删自己，不能删 super_admin）
func (a *App) DeleteUser(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || c.Role != "super_admin" {
		errJSON(w, http.StatusForbidden, "only super_admin can delete users")
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == "" {
		errJSON(w, http.StatusBadRequest, "missing user id")
		return
	}
	if userID == c.UserID {
		errJSON(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	var targetRole string
	err := a.DB.QueryRowContext(r.Context(), `SELECT role FROM users WHERE id = ? AND workspace_id = ?`,
		userID, seed.DemoWorkspaceID()).Scan(&targetRole)
	if err != nil {
		errJSON(w, http.StatusNotFound, "user not found")
		return
	}
	if targetRole == "super_admin" {
		errJSON(w, http.StatusBadRequest, "cannot delete a super_admin user")
		return
	}

	_, err = a.DB.ExecContext(r.Context(), `DELETE FROM users WHERE id = ? AND workspace_id = ?`,
		userID, seed.DemoWorkspaceID())
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	JSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

// CreateUpgradeRequest normal_user 提交权限升级申请
func (a *App) CreateUpgradeRequest(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil {
		errJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		RequestedRole string `json:"requested_role"`
		Reason        string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.RequestedRole = strings.ToLower(strings.TrimSpace(body.RequestedRole))
	if body.RequestedRole != "data_admin" && body.RequestedRole != "read_only_visitor" {
		errJSON(w, http.StatusBadRequest, "requested_role must be data_admin or read_only_visitor")
		return
	}

	id := uuid.NewString()
	_, err := a.DB.ExecContext(r.Context(), `
		INSERT INTO permission_requests (id, user_id, requested_role, reason)
		VALUES (?, ?, ?, ?)`,
		id, c.UserID, body.RequestedRole, body.Reason)
	if err != nil {
		a.Log.Error("create upgrade request", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to create request")
		return
	}

	JSON(w, http.StatusCreated, map[string]any{"id": id, "status": "pending"})
}

// ListUpgradeRequests super_admin 列出权限升级申请
func (a *App) ListUpgradeRequests(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || c.Role != "super_admin" {
		errJSON(w, http.StatusForbidden, "only super_admin can list upgrade requests")
		return
	}

	statusFilter := r.URL.Query().Get("status")
	if statusFilter == "" {
		statusFilter = "pending"
	}

	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT pr.id, pr.user_id, u.name, u.phone, pr.requested_role, pr.reason, pr.status, pr.created_at
		FROM permission_requests pr
		JOIN users u ON u.id = pr.user_id
		WHERE pr.status = ?
		ORDER BY pr.created_at ASC`, statusFilter)
	if err != nil {
		a.Log.Error("list upgrade requests", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	var requests []map[string]any
	for rows.Next() {
		var id, userID, name, phone, requestedRole, reason, status string
		var createdAt interface{}
		if err := rows.Scan(&id, &userID, &name, &phone, &requestedRole, &reason, &status, &createdAt); err != nil {
			a.Log.Warn("scan upgrade request", zap.Error(err))
			continue
		}
		requests = append(requests, map[string]any{
			"id":             id,
			"user_id":        userID,
			"user_name":      name,
			"user_phone":     phone,
			"requested_role": requestedRole,
			"reason":         reason,
			"status":         status,
			"created_at":     createdAt,
		})
	}

	JSON(w, http.StatusOK, map[string]any{"requests": requests})
}

// ReviewUpgradeRequest super_admin 审核权限升级申请
func (a *App) ReviewUpgradeRequest(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || c.Role != "super_admin" {
		errJSON(w, http.StatusForbidden, "only super_admin can review upgrade requests")
		return
	}

	requestID := chi.URLParam(r, "id")
	if requestID == "" {
		errJSON(w, http.StatusBadRequest, "missing request id")
		return
	}

	var body struct {
		Action      string `json:"action"` // approve or reject
		ReviewNotes string `json:"review_notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Action = strings.ToLower(strings.TrimSpace(body.Action))
	if body.Action != "approve" && body.Action != "reject" {
		errJSON(w, http.StatusBadRequest, "action must be approve or reject")
		return
	}

	// 获取申请信息
	var userID, requestedRole, currentStatus string
	err := a.DB.QueryRowContext(r.Context(), `
		SELECT user_id, requested_role, status FROM permission_requests WHERE id = ?`,
		requestID).Scan(&userID, &requestedRole, &currentStatus)
	if err != nil {
		errJSON(w, http.StatusNotFound, "request not found")
		return
	}
	if currentStatus != "pending" {
		errJSON(w, http.StatusBadRequest, "request already reviewed")
		return
	}

	newStatus := "approved"
	if body.Action == "reject" {
		newStatus = "rejected"
	}

	_, err = a.DB.ExecContext(r.Context(), `
		UPDATE permission_requests SET status = ?, reviewed_by = ?, review_notes = ?, reviewed_at = NOW()
		WHERE id = ?`, newStatus, c.UserID, body.ReviewNotes, requestID)
	if err != nil {
		a.Log.Error("update upgrade request", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	// 如果批准，更新用户角色
	if body.Action == "approve" {
		_, err = a.DB.ExecContext(r.Context(), `UPDATE users SET role = ? WHERE id = ?`,
			requestedRole, userID)
		if err != nil {
			a.Log.Error("update user role after approval", zap.Error(err))
		}
	}

	JSON(w, http.StatusOK, map[string]any{"message": "ok", "status": newStatus})
}
