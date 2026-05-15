package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// TaskStatus returns the current status and result of an async task
func (a *App) TaskStatus(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	taskID := chi.URLParam(r, "taskID")

	var status, taskType string
	var result, payload []byte
	var errorMsg *string

	err := a.DB.QueryRow(r.Context(), `
		SELECT status, task_type, payload, result, error_message
		FROM async_tasks 
		WHERE id = $1::uuid AND workspace_id = $2::uuid`,
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
		_ = json.Unmarshal(result, &rObj)
		res["result"] = rObj
	}
	if errorMsg != nil {
		res["error"] = *errorMsg
	}

	JSON(w, http.StatusOK, res)
}

// TaskCallback is an internal endpoint for workers to report task results
func (a *App) TaskCallback(w http.ResponseWriter, r *http.Request) {
	// Secret check
	authHeader := r.Header.Get("X-Hub-Internal-Secret")
	if authHeader == "" || authHeader != a.Cfg.InternalHMACSecret {
		errJSON(w, http.StatusUnauthorized, "invalid internal secret")
		return
	}

	taskID := chi.URLParam(r, "taskID")

	var body struct {
		Status       string `json:"status"` // 'succeeded', 'failed'
		Result       any    `json:"result"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	status := strings.ToLower(body.Status)
	if status != "succeeded" && status != "failed" {
		errJSON(w, http.StatusBadRequest, "invalid status")
		return
	}

	resultJSON, _ := json.Marshal(body.Result)

	_, err := a.DB.Exec(r.Context(), `
		UPDATE async_tasks 
		SET status = $2, result = $3, error_message = $4, updated_at = now()
		WHERE id = $1::uuid AND status IN ('queued', 'running')`,
		taskID, status, resultJSON, body.ErrorMessage)

	if err != nil {
		a.Log.Error("update async task", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db error")
		return
	}

	// Update run status if run_id is attached to task
	var runID *string
	_ = a.DB.QueryRow(r.Context(), `SELECT run_id::text FROM async_tasks WHERE id = $1::uuid`, taskID).Scan(&runID)

	if runID != nil && *runID != "" {
		runStatus := "completed"
		if status == "failed" {
			runStatus = "failed"
		}
		_, _ = a.DB.Exec(r.Context(), `UPDATE runs SET status = $2, updated_at = now() WHERE id = $1::uuid`, *runID, runStatus)

		var sid string
		if err := a.DB.QueryRow(r.Context(), `SELECT session_id::text FROM runs WHERE id = $1::uuid`, *runID).Scan(&sid); err == nil {
			var msgContent any
			if status == "succeeded" {
				msgContent = map[string]any{"final_report": body.Result, "run_id": *runID}
			} else {
				msgContent = map[string]any{"error": body.ErrorMessage, "run_id": *runID}
			}
			msgJSON, _ := json.Marshal(msgContent)
			_, _ = a.DB.Exec(r.Context(), `INSERT INTO messages (session_id, role, content) VALUES ($1::uuid, 'assistant', $2)`, sid, msgJSON)
			a.Bus.Publish(sid, ssebus.Event{Type: "result", Data: msgContent})
		}
	}

	JSON(w, http.StatusOK, map[string]string{"message": "ok"})
}

// RunStepCallback is an internal endpoint to track LangGraph steps
func (a *App) RunStepCallback(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("X-Hub-Internal-Secret")
	if authHeader == "" || authHeader != a.Cfg.InternalHMACSecret {
		errJSON(w, http.StatusUnauthorized, "invalid internal secret")
		return
	}

	runID := chi.URLParam(r, "runID")
	var body struct {
		AgentName     string `json:"agent_name"`
		Status        string `json:"status"`
		InputSummary  string `json:"input_summary"`
		OutputSummary string `json:"output_summary"`
		ErrorMessage  string `json:"error_message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}

	// Get step index
	var stepIndex int
	_ = a.DB.QueryRow(r.Context(), `SELECT COALESCE(MAX(step_index), -1) + 1 FROM agent_run_steps WHERE run_id = $1::uuid`, runID).Scan(&stepIndex)

	_, err := a.DB.Exec(r.Context(), `
		INSERT INTO agent_run_steps (run_id, step_index, agent_name, status, input_summary, output_summary, error_message)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)`,
		runID, stepIndex, body.AgentName, body.Status, body.InputSummary, body.OutputSummary, body.ErrorMessage)

	if err != nil {
		a.Log.Error("insert run step", zap.Error(err))
	}

	// Push SSE
	var sid string
	if err := a.DB.QueryRow(r.Context(), `SELECT session_id::text FROM runs WHERE id = $1::uuid`, runID).Scan(&sid); err == nil {
		eventData := map[string]string{
			"agent_name": body.AgentName,
			"status":     body.Status,
			"summary":    body.OutputSummary,
		}
		if body.ErrorMessage != "" {
			eventData["error"] = body.ErrorMessage
		}
		a.Bus.Publish(sid, ssebus.Event{Type: "agent_step", Data: eventData})
	}

	JSON(w, http.StatusOK, map[string]string{"message": "ok"})
}
