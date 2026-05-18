package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/dataflowagenthub/hub/internal/rbac"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// datasetResponse 数据集响应结构
type datasetResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	MysqlDatabase string `json:"mysql_database"`
	Status        string `json:"status"`
	CreatedBy     string `json:"created_by,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// generateMysqlDBName 生成唯一的 MySQL 数据库名
func generateMysqlDBName() string {
	return "ds_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
}

// hasDatasetAccess 检查用户是否有权访问数据集
func (a *App) hasDatasetAccess(ctx context.Context, userID, datasetID, role string) bool {
	if userID == "" || datasetID == "" {
		return false
	}
	if role == "super_admin" {
		return true
	}
	var count int
	err := a.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM dataset_permissions dp
		 JOIN datasets d ON d.id = dp.dataset_id
		 WHERE dp.dataset_id = $1::uuid AND dp.user_id = $2::uuid AND d.status != 'deleted'`,
		datasetID, userID).Scan(&count)
	return err == nil && count > 0
}

// ListDatasets 列出当前用户可访问的数据集
func (a *App) ListDatasets(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil {
		errJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var rows pgx.Rows
	var err error

	if c.Role == "super_admin" {
		rows, err = a.DB.Query(r.Context(), `
			SELECT d.id::text, d.name, d.mysql_database, d.status, d.created_at, d.updated_at
			FROM datasets d
			WHERE d.status != 'deleted'
			ORDER BY d.created_at DESC`)
	} else {
		rows, err = a.DB.Query(r.Context(), `
			SELECT d.id::text, d.name, d.mysql_database, d.status, d.created_at, d.updated_at
			FROM datasets d
			JOIN dataset_permissions dp ON dp.dataset_id = d.id
			WHERE dp.user_id = $1::uuid AND d.status != 'deleted'
			ORDER BY d.created_at DESC`, c.UserID)
	}

	if err != nil {
		a.Log.Error("list datasets", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	var datasets []datasetResponse
	for rows.Next() {
		var id, name, mysqlDB, status string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &name, &mysqlDB, &status, &createdAt, &updatedAt); err != nil {
			a.Log.Warn("scan dataset", zap.Error(err))
			continue
		}
		datasets = append(datasets, datasetResponse{
			ID:            id,
			Name:          name,
			MysqlDatabase: mysqlDB,
			Status:        status,
			CreatedAt:     createdAt.UTC().Format(time.RFC3339),
			UpdatedAt:     updatedAt.UTC().Format(time.RFC3339),
		})
	}

	JSON(w, http.StatusOK, map[string]any{"datasets": datasets})
}

// CreateDataset 创建新数据集（super_admin only）
func (a *App) CreateDataset(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || !rbac.IsSuperAdmin(c.Role) {
		errJSON(w, http.StatusForbidden, "only super_admin can create datasets")
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		errJSON(w, http.StatusBadRequest, "name is required")
		return
	}

	var exists int
	if err := a.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM datasets WHERE name = $1`, body.Name).Scan(&exists); err != nil {
		a.Log.Warn("check dataset name", zap.Error(err))
	}
	if exists > 0 {
		errJSON(w, http.StatusConflict, "dataset name already exists")
		return
	}

	mysqlDB := generateMysqlDBName()
	if err := a.MySQLMgr.CreateDatabase(mysqlDB); err != nil {
		a.Log.Error("create mysql database", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to create database")
		return
	}

	id := uuid.NewString()
	_, err := a.DB.Exec(r.Context(), `
		INSERT INTO datasets (id, name, mysql_database, created_by)
		VALUES ($1::uuid, $2, $3, $4::uuid)`,
		id, body.Name, mysqlDB, c.UserID)
	if err != nil {
		if dropErr := a.MySQLMgr.DropDatabase(mysqlDB); dropErr != nil {
			a.Log.Error("rollback mysql database", zap.Error(dropErr))
		}
		a.Log.Error("insert dataset", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to create dataset")
		return
	}

	// 自动授予 super_admin admin 权限
	_, err = a.DB.Exec(r.Context(), `
		INSERT INTO dataset_permissions (dataset_id, user_id, permission_level)
		VALUES ($1::uuid, $2::uuid, 'admin')`, id, c.UserID)
	if err != nil {
		a.Log.Warn("grant self permission", zap.Error(err))
	}

	JSON(w, http.StatusCreated, map[string]any{
		"id":             id,
		"name":           body.Name,
		"mysql_database": mysqlDB,
	})
}

// GetDataset 获取单个数据集详情
func (a *App) GetDataset(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil {
		errJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "missing dataset id")
		return
	}

	if !a.hasDatasetAccess(r.Context(), c.UserID, id, c.Role) {
		errJSON(w, http.StatusForbidden, "no access to this dataset")
		return
	}

	var name, mysqlDB, status string
	var createdBy *string
	var createdAt, updatedAt time.Time
	err := a.DB.QueryRow(r.Context(), `
		SELECT name, mysql_database, status, created_by::text, created_at, updated_at
		FROM datasets WHERE id = $1::uuid`, id).
		Scan(&name, &mysqlDB, &status, &createdBy, &createdAt, &updatedAt)
	if err != nil {
		errJSON(w, http.StatusNotFound, "dataset not found")
		return
	}

	resp := datasetResponse{
		ID:            id,
		Name:          name,
		MysqlDatabase: mysqlDB,
		Status:        status,
		CreatedAt:     createdAt.UTC().Format(time.RFC3339),
		UpdatedAt:     updatedAt.UTC().Format(time.RFC3339),
	}
	if createdBy != nil {
		resp.CreatedBy = *createdBy
	}

	JSON(w, http.StatusOK, resp)
}

// UpdateDataset 更新数据集名称（super_admin only）
func (a *App) UpdateDataset(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || !rbac.IsSuperAdmin(c.Role) {
		errJSON(w, http.StatusForbidden, "only super_admin can update datasets")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "missing dataset id")
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		errJSON(w, http.StatusBadRequest, "name is required")
		return
	}

	var exists int
	if err := a.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM datasets WHERE name = $1 AND id != $2::uuid`,
		body.Name, id).Scan(&exists); err != nil {
		a.Log.Warn("check dataset name", zap.Error(err))
	}
	if exists > 0 {
		errJSON(w, http.StatusConflict, "dataset name already exists")
		return
	}

	_, err := a.DB.Exec(r.Context(), `UPDATE datasets SET name = $1, updated_at = NOW() WHERE id = $2::uuid`,
		body.Name, id)
	if err != nil {
		a.Log.Error("update dataset", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	JSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

// DeleteDataset 删除数据集（super_admin only）
func (a *App) DeleteDataset(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || !rbac.IsSuperAdmin(c.Role) {
		errJSON(w, http.StatusForbidden, "only super_admin can delete datasets")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "missing dataset id")
		return
	}

	var mysqlDB string
	err := a.DB.QueryRow(r.Context(), `SELECT mysql_database FROM datasets WHERE id = $1::uuid`, id).Scan(&mysqlDB)
	if err != nil {
		errJSON(w, http.StatusNotFound, "dataset not found")
		return
	}

	if err := a.MySQLMgr.DropDatabase(mysqlDB); err != nil {
		a.Log.Error("drop mysql database", zap.Error(err))
	}

	_, err = a.DB.Exec(r.Context(), `UPDATE datasets SET status = 'deleted', updated_at = NOW() WHERE id = $1::uuid`, id)
	if err != nil {
		a.Log.Error("delete dataset", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	a.MySQLMgr.Close(id)

	JSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}

// GrantDatasetAccess 授予用户数据集访问权限（super_admin only）
func (a *App) GrantDatasetAccess(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || !rbac.IsSuperAdmin(c.Role) {
		errJSON(w, http.StatusForbidden, "only super_admin can grant dataset access")
		return
	}

	datasetID := chi.URLParam(r, "id")
	if datasetID == "" {
		errJSON(w, http.StatusBadRequest, "missing dataset id")
		return
	}

	var body struct {
		UserID          string `json:"user_id"`
		PermissionLevel string `json:"permission_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.PermissionLevel = strings.ToLower(strings.TrimSpace(body.PermissionLevel))
	if body.UserID == "" {
		errJSON(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if body.PermissionLevel != "read" && body.PermissionLevel != "write" && body.PermissionLevel != "admin" {
		errJSON(w, http.StatusBadRequest, "permission_level must be read, write, or admin")
		return
	}

	var exists int
	if err := a.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM datasets WHERE id = $1::uuid AND status != 'deleted'`,
		datasetID).Scan(&exists); err != nil || exists == 0 {
		errJSON(w, http.StatusNotFound, "dataset not found")
		return
	}

	_, err := a.DB.Exec(r.Context(), `
		INSERT INTO dataset_permissions (dataset_id, user_id, permission_level)
		VALUES ($1::uuid, $2::uuid, $3)
		ON CONFLICT (dataset_id, user_id) DO UPDATE SET permission_level = $3`,
		datasetID, body.UserID, body.PermissionLevel)
	if err != nil {
		a.Log.Error("grant dataset access", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	JSON(w, http.StatusOK, map[string]any{"message": "granted"})
}

// RevokeDatasetAccess 撤销用户数据集访问权限（super_admin only）
func (a *App) RevokeDatasetAccess(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || !rbac.IsSuperAdmin(c.Role) {
		errJSON(w, http.StatusForbidden, "only super_admin can revoke dataset access")
		return
	}

	datasetID := chi.URLParam(r, "id")
	if datasetID == "" {
		errJSON(w, http.StatusBadRequest, "missing dataset id")
		return
	}

	var body struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.UserID == "" {
		errJSON(w, http.StatusBadRequest, "user_id is required")
		return
	}

	_, err := a.DB.Exec(r.Context(), `
		DELETE FROM dataset_permissions WHERE dataset_id = $1::uuid AND user_id = $2::uuid`,
		datasetID, body.UserID)
	if err != nil {
		a.Log.Error("revoke dataset access", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	JSON(w, http.StatusOK, map[string]any{"message": "revoked"})
}
