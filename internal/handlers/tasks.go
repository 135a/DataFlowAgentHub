package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// TaskStatus 返回异步任务的当前状态和结果
func (a *App) TaskStatus(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	taskID := chi.URLParam(r, "taskID")

	var status, taskType string
	var result, payload []byte
	var errorMsg *string

	err := a.DB.QueryRowContext(r.Context(), `
		SELECT status, task_type, payload, result, error_message
		FROM async_tasks
		WHERE id = ? AND workspace_id = ?`,
		taskID, c.WorkspaceID).Scan(&status, &taskType, &payload, &result, &errorMsg)

	if err != nil {
		errJSON(w, http.StatusNotFound, "task not found")
		return
	}

	res := map[string]any{
		"id":        taskID,
		"status":    status,
		"task_type": taskType,
	}

	if result != nil {
		var rObj any
		if err := json.Unmarshal(result, &rObj); err != nil {
			a.Log.Warn("unmarshal task result", zap.Error(err))
		}
		res["result"] = rObj
	}
	if errorMsg != nil {
		res["error"] = *errorMsg
	}

	JSON(w, http.StatusOK, res)
}

