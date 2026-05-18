package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/dataflowagenthub/hub/internal/mysqlmgr"
	"github.com/dataflowagenthub/hub/internal/rbac"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// tableResponse 数据表响应结构
type tableResponse struct {
	ID              string `json:"id"`
	DatasetID       string `json:"dataset_id"`
	Name            string `json:"name"`
	DisplayName     string `json:"display_name,omitempty"`
	MySQLTableName  string `json:"mysql_table_name"`
	Status          string `json:"status"`
	CreatedBy       string `json:"created_by,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// fieldResponse 字段响应结构
type fieldResponse struct {
	ID          string `json:"id"`
	TableID     string `json:"table_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	FieldType   string `json:"field_type"`
	FieldLength int    `json:"field_length,omitempty"`
	IsNullable  bool   `json:"is_nullable"`
	OrdinalPos  int    `json:"ordinal_position"`
}

// generateMySQLTableName 生成唯一的 MySQL 表名
func generateMySQLTableName() string {
	return "tbl_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
}

// ListDatasetTables 列出数据集下的所有数据表
func (a *App) ListDatasetTables(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil {
		errJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	datasetID := chi.URLParam(r, "did")
	if datasetID == "" {
		errJSON(w, http.StatusBadRequest, "missing dataset id")
		return
	}

	if !a.hasDatasetAccess(r.Context(), c.UserID, datasetID, c.Role) {
		errJSON(w, http.StatusForbidden, "no access to this dataset")
		return
	}

	rows, err := a.DB.Query(r.Context(), `
		SELECT id::text, dataset_id::text, name, COALESCE(display_name, ''), mysql_table_name, status, created_by::text, created_at, updated_at
		FROM dataset_tables
		WHERE dataset_id = $1::uuid AND status = 'active'
		ORDER BY created_at ASC`, datasetID)
	if err != nil {
		a.Log.Error("list tables", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	var tables []tableResponse
	for rows.Next() {
		var t tableResponse
		var displayName, createdBy string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&t.ID, &t.DatasetID, &t.Name, &displayName, &t.MySQLTableName,
			&t.Status, &createdBy, &createdAt, &updatedAt); err != nil {
			a.Log.Warn("scan table", zap.Error(err))
			continue
		}
		if displayName != "" {
			t.DisplayName = displayName
		}
		if createdBy != "" {
			t.CreatedBy = createdBy
		}
		t.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		t.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		tables = append(tables, t)
	}

	JSON(w, http.StatusOK, map[string]any{"tables": tables})
}

// CreateDatasetTable 在数据集中创建新表（data_admin+）
func (a *App) CreateDatasetTable(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || !rbac.IsDataAdmin(c.Role) && !rbac.IsSuperAdmin(c.Role) {
		errJSON(w, http.StatusForbidden, "requires data_admin or above")
		return
	}

	datasetID := chi.URLParam(r, "did")
	if datasetID == "" {
		errJSON(w, http.StatusBadRequest, "missing dataset id")
		return
	}

	if !a.hasDatasetAccess(r.Context(), c.UserID, datasetID, c.Role) {
		errJSON(w, http.StatusForbidden, "no access to this dataset")
		return
	}

	var body struct {
		Name        string              `json:"name"`
		DisplayName string              `json:"display_name"`
		Fields      []mysqlmgr.FieldDef `json:"fields"`
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
	if len(body.Fields) == 0 {
		errJSON(w, http.StatusBadRequest, "at least one field is required")
		return
	}

	// 获取数据集的 MySQL 数据库名
	var mysqlDB string
	err := a.DB.QueryRow(r.Context(), `SELECT mysql_database FROM datasets WHERE id = $1::uuid AND status != 'deleted'`,
		datasetID).Scan(&mysqlDB)
	if err != nil {
		errJSON(w, http.StatusNotFound, "dataset not found")
		return
	}

	// 检查数据集内表名唯一性
	var exists int
	if err := a.DB.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM dataset_tables WHERE dataset_id = $1::uuid AND name = $2 AND status = 'active'`,
		datasetID, body.Name).Scan(&exists); err != nil {
		a.Log.Warn("check table name", zap.Error(err))
	}
	if exists > 0 {
		errJSON(w, http.StatusConflict, "table name already exists in this dataset")
		return
	}

	// 校验字段定义
	if err := mysqlmgr.ValidateFields(body.Fields); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	mysqlTableName := generateMySQLTableName()
	tableID := uuid.NewString()

	// 在 MySQL 中创建表
	mgr := a.MySQLMgr
	if err := mgr.CreateTable(datasetID, mysqlDB, mysqlTableName, body.Fields); err != nil {
		a.Log.Error("create mysql table", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to create table in mysql")
		return
	}

	// 写入元数据（dataset_tables + table_fields）
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		a.Log.Error("begin tx", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback(r.Context())

	_, err = tx.Exec(r.Context(), `
		INSERT INTO dataset_tables (id, dataset_id, name, display_name, mysql_table_name, created_by)
		VALUES ($1::uuid, $2::uuid, $3, NULLIF($4, ''), $5, $6::uuid)`,
		tableID, datasetID, body.Name, body.DisplayName, mysqlTableName, c.UserID)
	if err != nil {
		a.Log.Error("insert dataset_table", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to create table metadata")
		return
	}

	for i, f := range body.Fields {
		fid := uuid.NewString()
		flen := interface{}(nil)
		if f.FieldLen > 0 {
			flen = f.FieldLen
		}
		_, err = tx.Exec(r.Context(), `
			INSERT INTO table_fields (id, table_id, name, field_type, field_length, is_nullable, ordinal_position)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)`,
			fid, tableID, f.Name, strings.ToUpper(f.FieldType), flen, f.IsNullable, i+1)
		if err != nil {
			a.Log.Error("insert table_field", zap.Error(err))
			errJSON(w, http.StatusInternalServerError, "failed to create field metadata")
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		a.Log.Error("commit tx", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"id":               tableID,
		"name":             body.Name,
		"mysql_table_name": mysqlTableName,
		"fields_count":     len(body.Fields),
	})
}

// GetDatasetTable 获取数据表详情及字段
func (a *App) GetDatasetTable(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil {
		errJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	datasetID := chi.URLParam(r, "did")
	tableID := chi.URLParam(r, "tid")
	if datasetID == "" || tableID == "" {
		errJSON(w, http.StatusBadRequest, "missing dataset id or table id")
		return
	}

	if !a.hasDatasetAccess(r.Context(), c.UserID, datasetID, c.Role) {
		errJSON(w, http.StatusForbidden, "no access to this dataset")
		return
	}

	var t tableResponse
	var displayName, createdBy string
	var createdAt, updatedAt time.Time
	err := a.DB.QueryRow(r.Context(), `
		SELECT id::text, dataset_id::text, name, COALESCE(display_name, ''), mysql_table_name, status, COALESCE(created_by::text, ''), created_at, updated_at
		FROM dataset_tables
		WHERE id = $1::uuid AND dataset_id = $2::uuid AND status = 'active'`,
		tableID, datasetID).Scan(
		&t.ID, &t.DatasetID, &t.Name, &displayName, &t.MySQLTableName,
		&t.Status, &createdBy, &createdAt, &updatedAt)
	if err != nil {
		errJSON(w, http.StatusNotFound, "table not found")
		return
	}
	if displayName != "" {
		t.DisplayName = displayName
	}
	if createdBy != "" {
		t.CreatedBy = createdBy
	}
	t.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	t.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)

	// 获取字段列表
	fields, err := a.DB.Query(r.Context(), `
		SELECT id::text, table_id::text, name, COALESCE(display_name, ''), field_type, COALESCE(field_length, 0), is_nullable, ordinal_position
		FROM table_fields
		WHERE table_id = $1::uuid
		ORDER BY ordinal_position ASC`, tableID)
	if err != nil {
		a.Log.Error("list fields", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}
	defer fields.Close()

	var fieldList []fieldResponse
	for fields.Next() {
		var f fieldResponse
		var dsp string
		if err := fields.Scan(&f.ID, &f.TableID, &f.Name, &dsp, &f.FieldType, &f.FieldLength, &f.IsNullable, &f.OrdinalPos); err != nil {
			a.Log.Warn("scan field", zap.Error(err))
			continue
		}
		if dsp != "" {
			f.DisplayName = dsp
		}
		fieldList = append(fieldList, f)
	}

	JSON(w, http.StatusOK, map[string]any{
		"table":  t,
		"fields": fieldList,
	})
}

// DeleteDatasetTable 删除数据表（data_admin+）
func (a *App) DeleteDatasetTable(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil || !rbac.IsDataAdmin(c.Role) && !rbac.IsSuperAdmin(c.Role) {
		errJSON(w, http.StatusForbidden, "requires data_admin or above")
		return
	}

	datasetID := chi.URLParam(r, "did")
	tableID := chi.URLParam(r, "tid")
	if datasetID == "" || tableID == "" {
		errJSON(w, http.StatusBadRequest, "missing dataset id or table id")
		return
	}

	// 获取表信息
	var mysqlDB, mysqlTableName string
	err := a.DB.QueryRow(r.Context(), `
		SELECT d.mysql_database, dt.mysql_table_name
		FROM dataset_tables dt
		JOIN datasets d ON d.id = dt.dataset_id
		WHERE dt.id = $1::uuid AND dt.dataset_id = $2::uuid AND dt.status = 'active'`,
		tableID, datasetID).Scan(&mysqlDB, &mysqlTableName)
	if err != nil {
		errJSON(w, http.StatusNotFound, "table not found")
		return
	}

	// 从 MySQL 删除表
	if err := a.MySQLMgr.DropTable(datasetID, mysqlDB, mysqlTableName); err != nil {
		a.Log.Error("drop mysql table", zap.Error(err))
		// 继续执行元数据删除
	}

	// 逻辑删除元数据
	_, err = a.DB.Exec(r.Context(), `
		UPDATE dataset_tables SET status = 'inactive', updated_at = NOW()
		WHERE id = $1::uuid`, tableID)
	if err != nil {
		a.Log.Error("delete table metadata", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	JSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}

// ListFields 获取表的字段列表
func (a *App) ListFields(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil {
		errJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	datasetID := chi.URLParam(r, "did")
	tableID := chi.URLParam(r, "tid")
	if datasetID == "" || tableID == "" {
		errJSON(w, http.StatusBadRequest, "missing dataset id or table id")
		return
	}

	if !a.hasDatasetAccess(r.Context(), c.UserID, datasetID, c.Role) {
		errJSON(w, http.StatusForbidden, "no access to this dataset")
		return
	}

	rows, err := a.DB.Query(r.Context(), `
		SELECT id::text, table_id::text, name, COALESCE(display_name, ''), field_type, COALESCE(field_length, 0), is_nullable, ordinal_position
		FROM table_fields
		WHERE table_id = $1::uuid
		ORDER BY ordinal_position ASC`, tableID)
	if err != nil {
		a.Log.Error("list fields", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	var fields []fieldResponse
	for rows.Next() {
		var f fieldResponse
		var dsp string
		if err := rows.Scan(&f.ID, &f.TableID, &f.Name, &dsp, &f.FieldType, &f.FieldLength, &f.IsNullable, &f.OrdinalPos); err != nil {
			a.Log.Warn("scan field", zap.Error(err))
			continue
		}
		if dsp != "" {
			f.DisplayName = dsp
		}
		fields = append(fields, f)
	}

	JSON(w, http.StatusOK, map[string]any{"fields": fields})
}
