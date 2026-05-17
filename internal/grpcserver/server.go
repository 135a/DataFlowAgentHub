package grpcserver

import (
	"context"
	"encoding/json"
	"strings"

	nlv1 "github.com/dataflowagenthub/hub/internal/gen/nl2sql/v1"
	"github.com/dataflowagenthub/hub/internal/handlers"
	"github.com/dataflowagenthub/hub/internal/nl2sqlexec"
	"github.com/dataflowagenthub/hub/internal/ssebus"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// InternalServer 实现 HubInternalServiceServer，处理 Python Worker 的回调请求。
type InternalServer struct {
	nlv1.UnimplementedHubInternalServiceServer
	app *handlers.App
}

// NewInternalServer 创建一个新的内部 gRPC 服务端实例。
func NewInternalServer(app *handlers.App) *InternalServer {
	return &InternalServer{app: app}
}

// TaskCallback 处理异步任务结果回调。
func (s *InternalServer) TaskCallback(ctx context.Context, req *nlv1.TaskCallbackRequest) (*nlv1.TaskCallbackResponse, error) {
	taskID := req.GetTaskId()
	statusStr := strings.ToLower(req.GetStatus())
	if statusStr != "succeeded" && statusStr != "failed" {
		return nil, status.Error(codes.InvalidArgument, "invalid status, must be 'succeeded' or 'failed'")
	}

	var resultJSON []byte
	if req.GetResultJson() != "" {
		resultJSON = []byte(req.GetResultJson())
	} else {
		resultJSON = []byte("null")
	}

	_, err := s.app.DB.Exec(ctx, `
		UPDATE async_tasks
		SET status = $2, result = $3, error_message = $4, updated_at = now()
		WHERE id = $1::uuid AND status IN ('queued', 'running')`,
		taskID, statusStr, resultJSON, req.GetErrorMessage())
	if err != nil {
		s.app.Log.Error("update async task", zap.Error(err))
		return nil, status.Error(codes.Internal, "db error")
	}

	// 更新关联的 run 状态
	var runID *string
	if err := s.app.DB.QueryRow(ctx, `SELECT run_id::text FROM async_tasks WHERE id = $1::uuid`, taskID).Scan(&runID); err != nil {
		s.app.Log.Debug("task has no run_id", zap.String("task_id", taskID))
	}

	if runID != nil && *runID != "" {
		runStatus := "completed"
		if statusStr == "failed" {
			runStatus = "failed"
		}
		if _, err := s.app.DB.Exec(ctx, `UPDATE runs SET status = $2, updated_at = now() WHERE id = $1::uuid`, *runID, runStatus); err != nil {
			s.app.Log.Error("update run status", zap.Error(err))
		}

		var sid string
		if err := s.app.DB.QueryRow(ctx, `SELECT session_id::text FROM runs WHERE id = $1::uuid`, *runID).Scan(&sid); err == nil {
			var msgContent any
			if statusStr == "succeeded" {
				var result any
				if err := json.Unmarshal(resultJSON, &result); err != nil {
					result = map[string]any{}
				}
				msgContent = map[string]any{"final_report": result, "run_id": *runID}
			} else {
				msgContent = map[string]any{"error": req.GetErrorMessage(), "run_id": *runID}
			}
			msgJSON, _ := json.Marshal(msgContent)
			if _, err := s.app.DB.Exec(ctx, `INSERT INTO messages (session_id, role, content) VALUES ($1::uuid, 'assistant', $2)`, sid, msgJSON); err != nil {
				s.app.Log.Error("insert callback message", zap.Error(err))
			}
			s.app.Bus.Publish(sid, ssebus.Event{Type: "result", Data: msgContent})
		}
	}

	return &nlv1.TaskCallbackResponse{Message: "ok"}, nil
}

// RunStepCallback 处理 LangGraph 步骤追踪回调。
func (s *InternalServer) RunStepCallback(ctx context.Context, req *nlv1.RunStepCallbackRequest) (*nlv1.RunStepCallbackResponse, error) {
	runID := req.GetRunId()

	var stepIndex int
	if err := s.app.DB.QueryRow(ctx, `SELECT COALESCE(MAX(step_index), -1) + 1 FROM agent_run_steps WHERE run_id = $1::uuid`, runID).Scan(&stepIndex); err != nil {
		s.app.Log.Warn("get step index", zap.Error(err))
	}

	_, err := s.app.DB.Exec(ctx, `
		INSERT INTO agent_run_steps (run_id, step_index, agent_name, status, input_summary, output_summary, error_message)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)`,
		runID, stepIndex, req.GetAgentName(), req.GetStatus(), req.GetInputSummary(), req.GetOutputSummary(), req.GetErrorMessage())
	if err != nil {
		s.app.Log.Error("insert run step", zap.Error(err))
	}

	var sid string
	if err := s.app.DB.QueryRow(ctx, `SELECT session_id::text FROM runs WHERE id = $1::uuid`, runID).Scan(&sid); err == nil {
		eventData := map[string]string{
			"agent_name": req.GetAgentName(),
			"status":     req.GetStatus(),
			"summary":    req.GetOutputSummary(),
		}
		if req.GetErrorMessage() != "" {
			eventData["error"] = req.GetErrorMessage()
		}
		s.app.Bus.Publish(sid, ssebus.Event{Type: "agent_step", Data: eventData})
	}

	return &nlv1.RunStepCallbackResponse{Message: "ok"}, nil
}

// InternalNL2SQL 处理 Python 编排器的 NL2SQL 调用，通过 Go 的安全边界执行 SQL。
func (s *InternalServer) InternalNL2SQL(ctx context.Context, req *nlv1.InternalNL2SQLRequest) (*nlv1.InternalNL2SQLResponse, error) {
	userMessage := req.GetUserMessage()
	if userMessage == "" {
		return nil, status.Error(codes.InvalidArgument, "user_message is required")
	}
	dialect := req.GetDialect()
	if dialect == "" {
		dialect = "postgres"
	}

	result, err := s.app.NL2SQLExec.Execute(ctx, nl2sqlexec.Input{
		TraceID:     req.GetTraceId(),
		SessionID:   "",
		UserMessage: userMessage,
		SchemaJSON:  req.GetSchemaJson(),
		Dialect:     dialect,
	}, s.app.DB)
	if err != nil {
		if _, ok := status.FromError(err); ok {
			s.app.Log.Error("internal nl2sql: gRPC GenerateSQL failed", zap.Error(err))
			return nil, status.Error(codes.Unavailable, "nl2sql worker error: "+err.Error())
		}
		if genErr, ok := err.(*nl2sqlexec.GenerateError); ok {
			return &nlv1.InternalNL2SQLResponse{Ok: false, ErrorMessage: genErr.Message}, nil
		}
		return &nlv1.InternalNL2SQLResponse{Ok: false, ErrorMessage: err.Error()}, nil
	}

	rowsJSON, _ := json.Marshal(result.Rows)
	return &nlv1.InternalNL2SQLResponse{
		Sql:     result.SQL,
		RowsJson: string(rowsJSON),
		Notes:   result.SelfCheckNotes,
		Ok:      true,
	}, nil
}

// KnowledgeDocCallback 处理知识文档索引状态回调。
func (s *InternalServer) KnowledgeDocCallback(ctx context.Context, req *nlv1.KnowledgeDocCallbackRequest) (*nlv1.KnowledgeDocCallbackResponse, error) {
	docID := req.GetDocId()
	statusStr := strings.ToLower(req.GetStatus())
	if statusStr != "completed" && statusStr != "failed" {
		return nil, status.Error(codes.InvalidArgument, "invalid status, must be 'completed' or 'failed'")
	}

	_, err := s.app.DB.Exec(ctx, `
		UPDATE knowledge_docs
		SET status = $2, chroma_doc_id = $3, chunk_count = $4, updated_at = now()
		WHERE id = $1::uuid`,
		docID, statusStr, req.GetChromaDocId(), req.GetChunkCount())
	if err != nil {
		s.app.Log.Error("update knowledge doc", zap.Error(err))
		return nil, status.Error(codes.Internal, "db error")
	}

	return &nlv1.KnowledgeDocCallbackResponse{Message: "ok"}, nil
}
