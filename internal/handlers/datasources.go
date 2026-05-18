package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	hubcrypto "github.com/dataflowagenthub/hub/internal/crypto"
	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/dataflowagenthub/hub/internal/ratelimit"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type createDSBody struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSLMode  string `json:"sslmode"`
}

// ListDataSources 返回已配置的数据源（不包含密码值）
func (a *App) ListDataSources(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT id, name, kind, host, port, database, username,
		       (LENGTH(password) > 0) AS has_password, sslmode, created_at
		FROM data_sources WHERE workspace_id = ? ORDER BY created_at DESC`,
		c.WorkspaceID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "db")
		return
	}
	defer rows.Close()
	var items []map[string]any
	for rows.Next() {
		var id, name, kind, host, db, user, ssl string
		var port int
		var hasPwd bool
		var created time.Time
		if err := rows.Scan(&id, &name, &kind, &host, &port, &db, &user, &hasPwd, &ssl, &created); err != nil {
			errJSON(w, http.StatusInternalServerError, "db")
			return
		}
		items = append(items, map[string]any{
			"id": id, "name": name, "kind": kind, "host": host, "port": port,
			"database": db, "username": user, "has_password": hasPwd, "sslmode": ssl,
			"created_at": created.UTC().Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		a.Log.Warn("list data sources rows iteration", zap.Error(err))
	}
	JSON(w, http.StatusOK, map[string]any{"items": items})
}

// CreateDataSource 在服务端存储凭据（GET 请求不会返回密码）
func (a *App) CreateDataSource(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	// 限流：每用户每分钟 30 次
	if c != nil {
		if ok, _ := ratelimit.Allow(r.Context(), a.Redis, "ds:"+c.UserID, 30, time.Minute, a.Cfg.RateLimitFailClosed); !ok {
			errJSON(w, http.StatusTooManyRequests, "rate limit")
			return
		}
	}
	var body createDSBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Kind = strings.ToLower(strings.TrimSpace(body.Kind))
	if body.Kind != "mysql" {
		errJSON(w, http.StatusBadRequest, "kind must be mysql")
		return
	}
	if body.Name == "" || body.Host == "" || body.Port == 0 || body.Database == "" || body.Username == "" {
		errJSON(w, http.StatusBadRequest, "missing required fields")
		return
	}
	if body.SSLMode == "" {
		body.SSLMode = "disable"
	}

	encryptedPassword, err := hubcrypto.Encrypt(body.Password, a.Cfg.DBEncryptionKey)
	if err != nil {
		a.Log.Error("encrypt password", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to encrypt password")
		return
	}

	id := uuid.NewString()
	_, err = a.DB.ExecContext(r.Context(), `
		INSERT INTO data_sources (id, workspace_id, name, kind, host, port, database, username, password, sslmode)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, c.WorkspaceID, body.Name, body.Kind, body.Host, body.Port, body.Database, body.Username, encryptedPassword, body.SSLMode)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "db")
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "name": body.Name, "has_password": body.Password != ""})
}

// TestDataSource 对已存储的 mysql 数据源执行 ping 测试
func (a *App) TestDataSource(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var host, db, user, pwd, ssl string
	var port int
	err := a.DB.QueryRowContext(r.Context(), `
		SELECT host, port, database, username, password, sslmode FROM data_sources
		WHERE id = ? AND workspace_id = ?`, id, c.WorkspaceID).Scan(&host, &port, &db, &user, &pwd, &ssl)
	if err != nil {
		errJSON(w, http.StatusNotFound, "data source not found")
		return
	}

	decryptedPwd, err := hubcrypto.Decrypt(pwd, a.Cfg.DBEncryptionKey)
	if err != nil {
		a.Log.Error("decrypt password", zap.Error(err))
		JSON(w, http.StatusOK, map[string]any{"ok": false, "error": "failed to decrypt password"})
		return
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local&tls=%s",
		user, decryptedPwd, host, port, db, ssl)
	testDB, err := sql.Open("mysql", dsn)
	if err != nil {
		JSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	defer testDB.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if err := testDB.PingContext(ctx); err != nil {
		JSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// UpdateDataSource admin 编辑数据源连接参数（仅 admin）
func (a *App) UpdateDataSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "missing data source id")
		return
	}

	c := middleware.ClaimsFromContext(r.Context())
	var body createDSBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Kind = strings.ToLower(strings.TrimSpace(body.Kind))
	if body.Kind != "mysql" {
		errJSON(w, http.StatusBadRequest, "kind must be mysql")
		return
	}
	if body.Name == "" || body.Host == "" || body.Port == 0 || body.Database == "" || body.Username == "" {
		errJSON(w, http.StatusBadRequest, "missing required fields")
		return
	}
	if body.SSLMode == "" {
		body.SSLMode = "disable"
	}

	encryptedPassword, err := hubcrypto.Encrypt(body.Password, a.Cfg.DBEncryptionKey)
	if err != nil {
		a.Log.Error("encrypt password", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to encrypt password")
		return
	}

	res, err := a.DB.ExecContext(r.Context(), `
		UPDATE data_sources SET name=?, kind=?, host=?, port=?, database=?, username=?, password=?, sslmode=?
		WHERE id=? AND workspace_id=?`,
		body.Name, body.Kind, body.Host, body.Port, body.Database, body.Username, encryptedPassword, body.SSLMode,
		id, c.WorkspaceID)
	if err != nil {
		a.Log.Error("update data source", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		errJSON(w, http.StatusNotFound, "data source not found")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

// DeleteDataSource admin 删除数据源（仅 admin）
func (a *App) DeleteDataSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "missing data source id")
		return
	}

	c := middleware.ClaimsFromContext(r.Context())
	res, err := a.DB.ExecContext(r.Context(), `DELETE FROM data_sources WHERE id=? AND workspace_id=?`,
		id, c.WorkspaceID)
	if err != nil {
		a.Log.Error("delete data source", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		errJSON(w, http.StatusNotFound, "data source not found")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"message": "ok"})
}
