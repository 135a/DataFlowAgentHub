package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ListKnowledgeDocs lists the documents in the workspace
func (a *App) ListKnowledgeDocs(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	wsID := chi.URLParam(r, "workspaceID")
	if wsID != c.WorkspaceID {
		errJSON(w, http.StatusForbidden, "workspace mismatch")
		return
	}

	rows, err := a.DB.Query(r.Context(), `
		SELECT id::text, title, doc_type, status, created_at 
		FROM knowledge_docs 
		WHERE workspace_id = $1::uuid 
		ORDER BY created_at DESC`, wsID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	var docs []map[string]any
	for rows.Next() {
		var id, title, docType, status string
		var createdAt time.Time
		if err := rows.Scan(&id, &title, &docType, &status, &createdAt); err != nil {
			errJSON(w, http.StatusInternalServerError, "db err")
			return
		}
		docs = append(docs, map[string]any{
			"id":         id,
			"title":      title,
			"doc_type":   docType,
			"status":     status,
			"created_at": createdAt.UTC().Format(time.RFC3339),
		})
	}

	JSON(w, http.StatusOK, map[string]any{"docs": docs})
}

// UploadKnowledgeDoc receives text/markdown and enqueues it for chroma indexing (MVP: we'll index it asynchronously or directly here. Actually, to keep it simple, we just save it to DB and schedule an async task to Python to index it).
func (a *App) UploadKnowledgeDoc(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	wsID := chi.URLParam(r, "workspaceID")
	if wsID != c.WorkspaceID {
		errJSON(w, http.StatusForbidden, "workspace mismatch")
		return
	}

	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.Content == "" {
		errJSON(w, http.StatusBadRequest, "invalid payload")
		return
	}

	hashBytes := sha256.Sum256([]byte(body.Content))
	hashStr := hex.EncodeToString(hashBytes[:])

	docID := uuid.NewString()

	// 1. Insert into DB
	_, err := a.DB.Exec(r.Context(), `
		INSERT INTO knowledge_docs (id, workspace_id, title, content_hash, created_by, status) 
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid, 'pending')`,
		docID, wsID, body.Title, hashStr, c.UserID)
	if err != nil {
		a.Log.Error("insert knowledge doc", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db insert")
		return
	}

	// 2. Schedule async task to index (Python worker will process it)
	payload, _ := json.Marshal(map[string]any{
		"action":  "index_document",
		"doc_id":  docID,
		"title":   body.Title,
		"content": body.Content,
	})

	taskID := uuid.NewString()
	_, err = a.DB.Exec(r.Context(), `
		INSERT INTO async_tasks (id, workspace_id, task_type, payload) 
		VALUES ($1::uuid, $2::uuid, 'index_knowledge', $3)`,
		taskID, wsID, payload)

	if err != nil {
		a.Log.Error("insert async task", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db async task")
		return
	}

	// TODO: Publish task to NATS

	JSON(w, http.StatusAccepted, map[string]any{"id": docID, "task_id": taskID, "status": "pending"})
}
