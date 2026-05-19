package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dataflowagenthub/hub/internal/auth"
	hubcrypto "github.com/dataflowagenthub/hub/internal/crypto"
	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/dataflowagenthub/hub/internal/nl2sqlexec"
	"github.com/dataflowagenthub/hub/internal/ratelimit"
	"github.com/dataflowagenthub/hub/internal/schema"
	"github.com/dataflowagenthub/hub/internal/ssebus"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PostMessage 处理用户发送的消息请求
func (a *App) PostMessage(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	c := middleware.ClaimsFromContext(r.Context())
	sid := chi.URLParam(r, "sessionID")

	if ok, _ := ratelimit.Allow(r.Context(), a.Redis, "msg:"+c.UserID, 30, time.Minute, a.Cfg.RateLimitFailClosed); !ok {
		errJSON(w, http.StatusTooManyRequests, "rate limit")
		return
	}

	var body struct {
		Text     string `json:"text"`
		Workflow string `json:"workflow"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Text) == "" {
		errJSON(w, http.StatusBadRequest, "text required")
		return
	}

	ctx := r.Context()
	trace := middleware.TraceFromContext(ctx)

	if !a.sessionBelongsToWorkspace(ctx, sid, c.WorkspaceID) {
		errJSON(w, http.StatusNotFound, "session not found")
		return
	}

	// 检查会话是否关联数据集
	var datasetID *string
	if err := a.DB.QueryRowContext(ctx, `
		SELECT dataset_id FROM sessions WHERE id = ?`, sid).Scan(&datasetID); err != nil {
		// sessions 没有 dataset 关联，继续使用旧流程
	}

	// 插入用户消息
	userContent, err := json.Marshal(map[string]string{"text": body.Text})
	if err != nil {
		a.Log.Error("marshal user message", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "internal error")
		return
	}
	if _, err := a.DB.ExecContext(ctx, `INSERT INTO messages (session_id, role, content) VALUES (?, 'user', ?)`, sid, userContent); err != nil {
		a.Log.Error("insert user message", zap.Error(err))
	}
	a.Bus.Publish(sid, ssebus.Event{Type: "user_message", Data: map[string]string{"text": body.Text}})

	// 创建 run
	rid := uuid.NewString()
	if _, err := a.DB.ExecContext(ctx, `INSERT INTO runs (id, session_id, status) VALUES (?, ?, 'running')`, rid, sid); err != nil {
		a.Log.Error("insert run", zap.Error(err))
	}
	a.Bus.Publish(sid, ssebus.Event{Type: "run_started", Data: map[string]string{
		"run_id":     rid,
		"started_at": startTime.UTC().Format(time.RFC3339),
	}})

	// 如果会话关联了数据集，走 MySQL 路径
	if datasetID != nil && *datasetID != "" {
		a.postMessageToDataset(ctx, w, r, c, sid, rid, body, trace, startTime, *datasetID)
		return
	}

	// 知识库模式（无数据集关联）
	if a.LlmClient != nil && a.LlmClient.APIKey != "" {
		a.postMessageToKnowledge(ctx, w, r, c, sid, rid, body, trace, startTime)
		return
	}

	// 旧流程：PostgreSQL / 外部数据源（向后兼容）
	var dsID, dsKind *string
	if err := a.DB.QueryRowContext(ctx, `
		SELECT ds.id, ds.kind FROM data_sources ds
		JOIN sessions s ON s.data_source_id = ds.id
		WHERE s.id = ?`, sid).Scan(&dsID, &dsKind); err != nil {
		a.Log.Debug("session has no data source, using default", zap.String("session_id", sid))
	}
	if dsKind == nil {
		k := "postgres"
		dsKind = &k
	}

	schemaJSON, err := a.resolveSchema(ctx, dsID, c.WorkspaceID)
	if err != nil {
		errJSON(w, http.StatusBadGateway, err.Error())
		return
	}
	dialect := *dsKind

	workflow := strings.ToLower(strings.TrimSpace(body.Workflow))
	if workflow == "" {
		workflow = "auto"
	}
	textLow := strings.ToLower(body.Text)
	isComplex := strings.Contains(textLow, "分析") || strings.Contains(textLow, "报告") ||
		strings.Contains(textLow, "analyze") || strings.Contains(textLow, "report")
	useAgentPipeline := (workflow == "agent_pipeline") || (workflow == "auto" && isComplex)

	if useAgentPipeline {
		taskID, err := a.AsyncTask.EnqueueTask(ctx, c.WorkspaceID, sid, rid, "agent_pipeline", map[string]any{
			"user_message": body.Text,
			"schema_json":  schemaJSON,
		})
		if err != nil {
			a.finishRunFailed(ctx, rid, sid, "failed to enqueue task: "+err.Error(), codes.Internal)
			errJSON(w, http.StatusInternalServerError, "enqueue error")
			return
		}
		JSON(w, http.StatusAccepted, map[string]any{"run_id": rid, "task_id": taskID, "status": "pending_async"})
		return
	}

	result, err := a.NL2SQLExec.Execute(ctx, nl2sqlexec.Input{
		TraceID:     trace,
		SessionID:   sid,
		UserMessage: body.Text,
		SchemaJSON:  schemaJSON,
		Dialect:     dialect,
		Role:        c.Role,
	}, a.DB)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			a.finishRunFailed(ctx, rid, sid, "nl2sql: "+st.Message(), st.Code())
			errJSON(w, http.StatusBadGateway, mapGRPCCode(st.Code()))
			return
		}
		if genErr, ok := err.(*nl2sqlexec.GenerateError); ok {
			a.finishRunFailed(ctx, rid, sid, genErr.Message, codes.Internal)
			errJSON(w, http.StatusBadRequest, genErr.Message)
			return
		}
		a.finishRunFailed(ctx, rid, sid, err.Error(), codes.InvalidArgument)
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := a.publishSyncResult(ctx, rid, sid, result, startTime)
	if err != nil {
		a.finishRunFailed(ctx, rid, sid, "marshal error", codes.Internal)
		errJSON(w, http.StatusInternalServerError, "internal error")
		return
	}
	JSON(w, http.StatusOK, resp)
}

// postMessageToDataset 处理关联到数据集的会话消息（MySQL 路径）
func (a *App) postMessageToDataset(ctx context.Context, w http.ResponseWriter, r *http.Request, c *auth.Claims, sid, rid string, body struct {
	Text     string `json:"text"`
	Workflow string `json:"workflow"`
}, trace string, startTime time.Time, datasetID string) {
	// 构建 schema 从 table_fields（返回数据集下所有表）
	schemaJSON, err := a.resolveDatasetSchema(ctx, datasetID)
	if err != nil {
		a.finishRunFailed(ctx, rid, sid, "schema error: "+err.Error(), codes.Internal)
		errJSON(w, http.StatusBadGateway, err.Error())
		return
	}

	// 获取 MySQL 连接池
	var mysqlDB string
	if err := a.DB.QueryRowContext(ctx, `SELECT mysql_database FROM datasets WHERE id = ?`, datasetID).Scan(&mysqlDB); err != nil {
		a.finishRunFailed(ctx, rid, sid, "dataset not found", codes.NotFound)
		errJSON(w, http.StatusNotFound, "dataset not found")
		return
	}
	pool, ok := a.MySQLMgr.GetPool(datasetID)
	if !ok {
		pool, err = a.MySQLMgr.Connect(datasetID, mysqlDB)
		if err != nil {
			a.Log.Error("connect mysql", zap.Error(err))
			a.finishRunFailed(ctx, rid, sid, "database connection error", codes.Internal)
			errJSON(w, http.StatusInternalServerError, "database connection error")
			return
		}
	}

	workflow := strings.ToLower(strings.TrimSpace(body.Workflow))
	if workflow == "" {
		workflow = "auto"
	}
	textLow := strings.ToLower(body.Text)
	isComplex := strings.Contains(textLow, "分析") || strings.Contains(textLow, "报告") ||
		strings.Contains(textLow, "analyze") || strings.Contains(textLow, "report")
	useAgentPipeline := (workflow == "agent_pipeline") || (workflow == "auto" && isComplex)

	if useAgentPipeline {
		taskID, err := a.AsyncTask.EnqueueTask(ctx, "workspace", sid, rid, "agent_pipeline", map[string]any{
			"user_message": body.Text,
			"schema_json":  schemaJSON,
		})
		if err != nil {
			a.finishRunFailed(ctx, rid, sid, "failed to enqueue task: "+err.Error(), codes.Internal)
			errJSON(w, http.StatusInternalServerError, "enqueue error")
			return
		}
		JSON(w, http.StatusAccepted, map[string]any{"run_id": rid, "task_id": taskID, "status": "pending_async"})
		return
	}

	result, err := a.NL2SQLExec.Execute(ctx, nl2sqlexec.Input{
		TraceID:     trace,
		SessionID:   sid,
		UserMessage: body.Text,
		SchemaJSON:  schemaJSON,
		Dialect:     "mysql",
		Role:        c.Role,
	}, pool)
	if err != nil {
		if genErr, ok := err.(*nl2sqlexec.GenerateError); ok {
			a.finishRunFailed(ctx, rid, sid, genErr.Message, codes.Internal)
			errJSON(w, http.StatusBadRequest, genErr.Message)
			return
		}
		a.finishRunFailed(ctx, rid, sid, err.Error(), codes.InvalidArgument)
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := a.publishSyncResult(ctx, rid, sid, result, startTime)
	if err != nil {
		a.finishRunFailed(ctx, rid, sid, "marshal error", codes.Internal)
		errJSON(w, http.StatusInternalServerError, "internal error")
		return
	}
	JSON(w, http.StatusOK, resp)
}

// resolveDatasetSchema 从 table_fields 构建数据集下所有活跃表的 schema JSON。
func (a *App) resolveDatasetSchema(ctx context.Context, datasetID string) (string, error) {
	rows, err := a.DB.QueryContext(ctx, `
		SELECT dt.name, tf.name, tf.field_type, tf.is_nullable
		FROM table_fields tf
		JOIN dataset_tables dt ON dt.id = tf.table_id
		WHERE tf.table_id IN (
			SELECT id FROM dataset_tables
			WHERE dataset_id = ? AND status = 'active'
		)
		ORDER BY dt.name, tf.ordinal_position ASC`, datasetID)
	if err != nil {
		return "", fmt.Errorf("query dataset schema: %w", err)
	}
	defer rows.Close()

	type tableCols struct {
		name    string
		columns []schema.ColumnSchema
	}
	var tables []tableCols
	var currentTable *tableCols
	for rows.Next() {
		var tblName, colName, colType string
		var nullable bool
		if err := rows.Scan(&tblName, &colName, &colType, &nullable); err != nil {
			return "", fmt.Errorf("scan dataset schema: %w", err)
		}
		if currentTable == nil || currentTable.name != tblName {
			if currentTable != nil {
				tables = append(tables, *currentTable)
			}
			currentTable = &tableCols{name: tblName}
		}
		currentTable.columns = append(currentTable.columns, schema.ColumnSchema{
			Name:     colName,
			Type:     colType,
			Nullable: nullable,
		})
	}
	if currentTable != nil {
		tables = append(tables, *currentTable)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate dataset schema: %w", err)
	}

	var tableSchemas []schema.TableSchema
	for _, t := range tables {
		tableSchemas = append(tableSchemas, schema.TableSchema{Name: t.name, Columns: t.columns})
	}
	sr := &schema.SchemaResult{Tables: tableSchemas}
	return sr.ToJSON()
}

// knowledgeQAResult 知识库问答结果
type knowledgeQAResult struct {
	Answer  string `json:"answer"`
	Sources []struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	} `json:"sources"`
}

// postMessageToKnowledge 处理知识库问答（RAG + LLM）
func (a *App) postMessageToKnowledge(ctx context.Context, w http.ResponseWriter, r *http.Request, c *auth.Claims, sid, rid string, body struct {
	Text     string `json:"text"`
	Workflow string `json:"workflow"`
}, trace string, startTime time.Time) {
	var answer string

	// 1. 尝试通过 gRPC 调用 Worker RAGSearch（ChromaDB 检索 + LLM 问答）
	if a.Nl2sql != nil {
		ragCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		resp, err := a.Nl2sql.RAGSearch(ragCtx, trace, c.WorkspaceID, body.Text, 3)
		if err == nil && resp.Ok {
			answer = resp.Answer
		} else {
			// gRPC 调用失败，记录警告并走 fallback
			a.Log.Warn("gRPC RAGSearch failed, falling back to direct LLM",
				zap.Error(err),
				zap.String("trace", trace))
		}
	}

	// 2. Fallback：直接 LLM 问答
	if answer == "" {
		prompt := fmt.Sprintf(`你是一个知识库问答助手。请基于你的知识回答以下问题。
如果不知道答案，请如实说明，不要编造。

问题：%s

请用中文回答。`, body.Text)

		llmCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		resp, err := a.LlmClient.ChatCompletion(llmCtx, a.Cfg.LLMModel, prompt)
		if err != nil {
			a.finishRunFailed(ctx, rid, sid, "knowledge query failed: "+err.Error(), codes.Internal)
			errJSON(w, http.StatusBadGateway, "知识库查询失败: "+err.Error())
			return
		}
		answer = resp
	}

	result := knowledgeQAResult{
		Answer:  answer,
		Sources: []struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}{},
	}

	nl2Result := &nl2sqlexec.Result{
		SQL:     "",
		Rows:    []map[string]any{{"answer": result.Answer}},
		IsWrite: false,
	}
	finalResp, err := a.publishSyncResult(ctx, rid, sid, nl2Result, startTime)
	if err != nil {
		a.finishRunFailed(ctx, rid, sid, "marshal error", codes.Internal)
		errJSON(w, http.StatusInternalServerError, "internal error")
		return
	}
	JSON(w, http.StatusOK, finalResp)
}

// resolveSchema 为会话解析数据库 schema，返回 JSON 字符串
func (a *App) resolveSchema(ctx context.Context, dsID *string, workspaceID string) (string, error) {
	sourceKey := "hub"
	var discoverDB *sql.DB = a.DB
	if dsID != nil {
		sourceKey = *dsID
		var host, dbName, user, pwd, ssl string
		var port int
		if err := a.DB.QueryRowContext(ctx,
			`SELECT host, port, database, username, password, sslmode FROM data_sources WHERE id = ?`, *dsID,
		).Scan(&host, &port, &dbName, &user, &pwd, &ssl); err != nil {
			return "", fmt.Errorf("data source not found: %w", err)
		}
		decryptedPwd, decErr := hubcrypto.Decrypt(pwd, a.Cfg.DBEncryptionKey)
		if decErr != nil {
			a.Log.Error("decrypt datasource password", zap.Error(decErr))
			return "", fmt.Errorf("failed to decrypt datasource password")
		}
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
			user, decryptedPwd, host, port, dbName)
		extDB, openErr := sql.Open("mysql", dsn)
		if openErr != nil {
			return "", fmt.Errorf("schema discovery: cannot connect to data source: %w", openErr)
		}
		defer extDB.Close()
		discoverDB = extDB
	}

	schemaResult, schemaErr := schema.CachedSchema(ctx, discoverDB, a.Redis, a.Cfg, a.Log, workspaceID, sourceKey)
	if schemaErr != nil {
		return "", fmt.Errorf("schema discovery failed: %w", schemaErr)
	}
	schemaJSON, err := schemaResult.ToJSON()
	if err != nil {
		a.Log.Error("marshal schema json", zap.Error(err))
		return "", fmt.Errorf("schema error")
	}
	return schemaJSON, nil
}

// publishSyncResult 构建同步执行结果，发布 SSE，写入消息和更新 run 状态
func (a *App) publishSyncResult(ctx context.Context, rid, sid string, result *nl2sqlexec.Result, startTime time.Time) (map[string]any, error) {
	totalElapsedMs := time.Since(startTime).Milliseconds()
	startedAt := startTime.UTC().Format(time.RFC3339)

	a.Bus.Publish(sid, ssebus.Event{Type: "sql_generated", Data: map[string]any{
		"sql":        result.SQL,
		"elapsed_ms": time.Since(startTime).Milliseconds(),
		"is_write":   result.IsWrite,
	}})

	var assist []byte
	var resp map[string]any
	if result.IsWrite {
		var marshalErr error
		assist, marshalErr = json.Marshal(map[string]any{
			"sql":           result.SQL,
			"rows_affected": result.RowsAffected,
			"notes":         result.SelfCheckNotes,
			"type":          "write",
			"elapsed_ms":    totalElapsedMs,
			"started_at":    startedAt,
		})
		if marshalErr != nil {
			a.Log.Error("marshal write result", zap.Error(marshalErr))
			return nil, marshalErr
		}
		resp = map[string]any{
			"run_id":        rid,
			"sql":           result.SQL,
			"rows_affected": result.RowsAffected,
			"elapsed_ms":    totalElapsedMs,
			"started_at":    startedAt,
		}
	} else {
		var marshalErr error
		assist, marshalErr = json.Marshal(map[string]any{
			"sql":        result.SQL,
			"rows":       result.Rows,
			"notes":      result.SelfCheckNotes,
			"elapsed_ms": totalElapsedMs,
			"started_at": startedAt,
		})
		if marshalErr != nil {
			a.Log.Error("marshal read result", zap.Error(marshalErr))
			return nil, marshalErr
		}
		resp = map[string]any{
			"run_id":     rid,
			"sql":        result.SQL,
			"rows":       result.Rows,
			"elapsed_ms": totalElapsedMs,
			"started_at": startedAt,
		}
	}

	if _, err := a.DB.ExecContext(ctx, `INSERT INTO messages (session_id, role, content) VALUES (?, 'assistant', ?)`, sid, assist); err != nil {
		a.Log.Error("insert assistant message", zap.Error(err))
	}
	if _, err := a.DB.ExecContext(ctx, `UPDATE runs SET status = 'completed', updated_at = NOW() WHERE id = ?`, rid); err != nil {
		a.Log.Error("update run completed", zap.Error(err))
	}
	a.Bus.Publish(sid, ssebus.Event{Type: "result", Data: json.RawMessage(assist)})
	return resp, nil
}

// mapGRPCCode 将gRPC状态码映射为自定义的错误信息字符串
func mapGRPCCode(c codes.Code) string {
	switch c {
	case codes.Unavailable:
		return "worker unavailable"
	case codes.DeadlineExceeded:
		return "deadline exceeded"
	default:
		return "worker error"
	}
}

// finishRunFailed 标记 run 为失败并发送错误 SSE
func (a *App) finishRunFailed(ctx context.Context, runID, sessionID, msg string, c codes.Code) {
	assist, err := json.Marshal(map[string]any{"error": msg, "code": c.String()})
	if err != nil {
		a.Log.Error("marshal failed run", zap.Error(err))
	}
	if _, err := a.DB.ExecContext(ctx, `INSERT INTO messages (session_id, role, content) VALUES (?, 'assistant', ?)`, sessionID, assist); err != nil {
		a.Log.Error("insert failed assistant message", zap.Error(err))
	}
	if _, err := a.DB.ExecContext(ctx, `UPDATE runs SET status = 'failed', pending_reason = ?, updated_at = NOW() WHERE id = ?`, msg, runID); err != nil {
		a.Log.Error("update run failed", zap.Error(err))
	}
	a.Bus.Publish(sessionID, ssebus.Event{Type: "error", Data: map[string]string{"message": msg}})
}
