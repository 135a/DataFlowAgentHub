package handlers

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// docTypeFromExt 根据文件扩展名返回 doc_type
func docTypeFromExt(filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt":
		return "text", nil
	case ".md":
		return "markdown", nil
	case ".pdf":
		return "pdf", nil
	case ".doc", ".docx":
		return "doc", nil
	default:
		return "", fmt.Errorf("unsupported file type: %s, only .txt/.md/.pdf/.doc/.docx are supported", ext)
	}
}

// ListKnowledgeDocs 列出工作区中的知识文档
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
	if err := rows.Err(); err != nil {
		a.Log.Warn("list knowledge docs rows iteration", zap.Error(err))
	}

	JSON(w, http.StatusOK, map[string]any{"docs": docs})
}

// UploadKnowledgeDoc 接收文本/标记内容并将其加入队列进行 Chroma 索引
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

	// 1. 插入数据库
	_, err := a.DB.Exec(r.Context(), `
		INSERT INTO knowledge_docs (id, workspace_id, title, content_hash, created_by, status)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid, 'pending')`,
		docID, wsID, body.Title, hashStr, c.UserID)
	if err != nil {
		a.Log.Error("insert knowledge doc", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db insert")
		return
	}

	// 2. 通过 async.Client 加入异步任务队列（写入数据库并发布到 NATS）
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

// UploadKnowledgeDocFromFile 接收 multipart 文件上传（.txt/.pdf/.doc/.docx）并加入 Chroma 索引队列
// POST /v1/workspaces/{workspaceID}/knowledge/docs/upload
// multipart/form-data: file (必填), title (可选，默认文件名)
func (a *App) UploadKnowledgeDocFromFile(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	wsID := chi.URLParam(r, "workspaceID")
	if wsID != c.WorkspaceID {
		errJSON(w, http.StatusForbidden, "workspace mismatch")
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil { // 64MB
		errJSON(w, http.StatusBadRequest, "file too large or parse failed")
		return
	}

	f, header, err := r.FormFile("file")
	if err != nil {
		errJSON(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer f.Close()

	// 检测文件类型
	docType, err := docTypeFromExt(header.Filename)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// 读取文件二进制
	fileData, err := io.ReadAll(f)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "failed to read file")
		return
	}
	if len(fileData) == 0 {
		errJSON(w, http.StatusBadRequest, "empty file")
		return
	}

	// 标题：优先取表单字段，否则使用文件名（不含扩展名）
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	hashBytes := sha256.Sum256(fileData)
	hashStr := hex.EncodeToString(hashBytes[:])

	docID := uuid.NewString()

	// 1a. 保存文件到磁盘持久化存储
	if err := saveKnowledgeFile(a.Cfg.KnowledgeFilesDir, wsID, docID, docType, fileData); err != nil {
		a.Log.Error("save knowledge file", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	// 1b. 插入数据库
	_, dbErr := a.DB.Exec(r.Context(), `
		INSERT INTO knowledge_docs (id, workspace_id, title, doc_type, content_hash, created_by, status)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::uuid, 'pending')`,
		docID, wsID, title, docType, hashStr, c.UserID)
	if dbErr != nil {
		a.Log.Error("insert knowledge doc", zap.Error(dbErr))
		errJSON(w, http.StatusInternalServerError, "db insert")
		return
	}

	// 2. 将文件二进制编码为 base64 加入 NATS 消息
	fileB64 := base64.StdEncoding.EncodeToString(fileData)

	taskID, err := a.AsyncTask.EnqueueTask(r.Context(), wsID, "", "", "knowledge_index", map[string]any{
		"action":     "index_document",
		"doc_id":     docID,
		"title":      title,
		"doc_type":   docType,
		"file_bytes": fileB64,
		"file_name":  header.Filename,
	})
	if err != nil {
		a.Log.Error("enqueue knowledge task", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "failed to enqueue indexing task")
		return
	}

	JSON(w, http.StatusAccepted, map[string]any{"id": docID, "task_id": taskID, "status": "pending", "doc_type": docType})
}

// saveKnowledgeFile 保存知识库上传文件到磁盘持久化存储
func saveKnowledgeFile(baseDir, workspaceID, docID, docType string, data []byte) error {
	ext := extFromDocType(docType)
	dir := filepath.Join(baseDir, workspaceID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create knowledge dir: %w", err)
	}
	path := filepath.Join(dir, docID+ext)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write knowledge file: %w", err)
	}
	return nil
}

// extFromDocType 返回 doc_type 对应的文件扩展名
func extFromDocType(docType string) string {
	switch docType {
	case "text":
		return ".txt"
	case "markdown":
		return ".md"
	case "pdf":
		return ".pdf"
	case "doc":
		return ".docx"
	default:
		return ""
	}
}

// DownloadKnowledgeDoc 下载知识库原始文件
// GET /v1/knowledge/docs/{docID}/download
func (a *App) DownloadKnowledgeDoc(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docID")

	// 查询数据库获取文档信息
	var workspaceID, title, docType string
	err := a.DB.QueryRow(r.Context(), `
		SELECT workspace_id::text, title, doc_type FROM knowledge_docs WHERE id = $1::uuid`, docID).Scan(&workspaceID, &title, &docType)
	if err != nil {
		errJSON(w, http.StatusNotFound, "document not found")
		return
	}

	// 构建文件路径
	ext := extFromDocType(docType)
	path := filepath.Join(a.Cfg.KnowledgeFilesDir, workspaceID, docID+ext)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		a.Log.Warn("knowledge file not found on disk", zap.String("path", path))
		errJSON(w, http.StatusNotFound, "file not available")
		return
	}

	// 猜测原始文件名
	originalName := title + ext
	w.Header().Set("Content-Disposition", "attachment; filename=\""+originalName+"\"")
	http.ServeFile(w, r, path)
}
