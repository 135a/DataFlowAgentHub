package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
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

// UploadKnowledgeDoc receives text/markdown and enqueues it for chroma indexing.
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

	// 2. Enqueue async task via async.Client (writes to DB + publishes to NATS)
	taskID, err := a.AsyncTask.EnqueueTask(r.Context(), wsID, "", "", "knowledge_index", map[string]any{
		"action":   "index_document",
		"doc_id":   docID,
		"title":    body.Title,
		"content":  body.Content,
		"doc_type": "markdown",
	})
	if err != nil {
		a.Log.Error("enqueue knowledge task", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to enqueue indexing task")
		return
	}

	JSON(w, http.StatusAccepted, map[string]any{"id": docID, "task_id": taskID, "status": "pending"})
}

// KnowledgeDocCallback is an internal endpoint for the Python worker to update document indexing status.
// Authentication is handled by InternalHMACAuth middleware.
func (a *App) KnowledgeDocCallback(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docID")
	var body struct {
		Status       string `json:"status"` // 'completed' or 'failed'
		ChromaDocID  string `json:"chroma_doc_id"`
		ChunkCount   int    `json:"chunk_count"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	status := strings.ToLower(body.Status)
	if status != "completed" && status != "failed" {
		errJSON(w, http.StatusBadRequest, "invalid status")
		return
	}

	_, err := a.DB.Exec(r.Context(), `
		UPDATE knowledge_docs
		SET status = $2, chroma_doc_id = $3, chunk_count = $4, updated_at = now()
		WHERE id = $1::uuid`,
		docID, status, body.ChromaDocID, body.ChunkCount)
	if err != nil {
		a.Log.Error("update knowledge doc", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "ok"})
}
