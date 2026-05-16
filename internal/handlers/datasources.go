package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dataflowagenthub/hub/internal/connector"
	hubcrypto "github.com/dataflowagenthub/hub/internal/crypto"
	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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

// ListDataSources returns configured sources without password values.
func (a *App) ListDataSources(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	rows, err := a.DB.Query(r.Context(), `
		SELECT id::text, name, kind, host, port, database, username,
		       (length(password) > 0) AS has_password, sslmode, created_at
		FROM data_sources WHERE workspace_id = $1::uuid ORDER BY created_at DESC`,
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
	JSON(w, http.StatusOK, map[string]any{"items": items})
}

// CreateDataSource stores credentials server-side only (never returned on GET).
func (a *App) CreateDataSource(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	var body createDSBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Kind = strings.ToLower(strings.TrimSpace(body.Kind))
	if body.Kind != "postgres" {
		errJSON(w, http.StatusBadRequest, "kind must be postgres")
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
	_, err = a.DB.Exec(r.Context(), `
		INSERT INTO data_sources (id, workspace_id, name, kind, host, port, database, username, password, sslmode)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10)`,
		id, c.WorkspaceID, body.Name, body.Kind, body.Host, body.Port, body.Database, body.Username, encryptedPassword, body.SSLMode)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "db")
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "name": body.Name, "has_password": body.Password != ""})
}

// TestDataSource pings the stored postgres data source.
func (a *App) TestDataSource(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var host, db, user, pwd, ssl string
	var port int
	err := a.DB.QueryRow(r.Context(), `
		SELECT host, port, database, username, password, sslmode FROM data_sources
		WHERE id = $1::uuid AND workspace_id = $2::uuid`, id, c.WorkspaceID).Scan(&host, &port, &db, &user, &pwd, &ssl)
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

	dsn := connector.DSN(host, port, user, decryptedPwd, db, ssl)
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		JSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		JSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, map[string]any{"ok": true})
}
