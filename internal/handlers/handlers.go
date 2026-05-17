package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dataflowagenthub/hub/internal/async"
	"github.com/dataflowagenthub/hub/internal/auth"
	"github.com/dataflowagenthub/hub/internal/config"
	hubcrypto "github.com/dataflowagenthub/hub/internal/crypto"
	"github.com/dataflowagenthub/hub/internal/llm"
	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/dataflowagenthub/hub/internal/nl2sqlexec"
	"github.com/dataflowagenthub/hub/internal/ratelimit"
	"github.com/dataflowagenthub/hub/internal/schema"
	"github.com/dataflowagenthub/hub/internal/seed"
	"github.com/dataflowagenthub/hub/internal/sqlrun"
	"github.com/dataflowagenthub/hub/internal/ssebus"
	"github.com/dataflowagenthub/hub/internal/telemetry"
	"github.com/dataflowagenthub/hub/internal/worker"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const version = "0.1.0-dev"

// App 封装 HTTP 依赖项
type App struct {
	Cfg        *config.Config
	Log        *zap.Logger
	DB         *pgxpool.Pool
	Redis      *redis.Client
	Nl2sql     *worker.NL2SQLClient
	Bus        *ssebus.Bus
	NATS       *nats.Conn
	AsyncTask  *async.Client
	NL2SQLExec *nl2sqlexec.Executor
	LlmClient  *llm.Client
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

// Login 接受种子用户凭证并返回 JWT
func (a *App) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	var hash, role string
	var uid string
	err := a.DB.QueryRow(r.Context(), `
		SELECT id::text, password_hash, role FROM users WHERE workspace_id = $1 AND email = $2`,
		seed.DemoWorkspaceID(), strings.TrimSpace(body.Email),
	).Scan(&uid, &hash, &role)
	if err != nil {
		errJSON(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err := bcryptCompare(body.Password, hash); err != nil {
		errJSON(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	tok, err := auth.Sign(a.Cfg.JWTSecret, uid, seed.DemoWorkspaceID(), role, 24*time.Hour)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "token")
		return
	}
	JSON(w, http.StatusOK, map[string]any{
		"access_token": tok,
		"token_type":   "Bearer",
		"expires_in":   int((24 * time.Hour).Seconds()),
		"workspace_id": seed.DemoWorkspaceID(),
		"role":         role,
	})
}

func bcryptCompare(pw, hash string) error {
	// 本地导入会导致循环引用 — 改用同包文件 bcrypt.go 中的 golang.org/x/crypto/bcrypt
	return compareBcrypt(pw, hash)
}

// Health 报告 Postgres 和 Redis 的就绪状态
func (a *App) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	pg := "ok"
	if err := sqlrun.Ping(ctx, a.DB); err != nil {
		pg = "down"
	}
	rd := "ok"
	if a.Redis != nil {
		if err := a.Redis.Ping(ctx).Err(); err != nil {
			rd = "down"
		}
	}
	code := http.StatusOK
	if pg != "ok" || rd != "ok" {
		code = http.StatusServiceUnavailable
	}
	JSON(w, code, map[string]any{"postgres": pg, "redis": rd})
}

func (a *App) Version(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]string{"version": version})
}

func (a *App) ListSessions(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	rows, err := a.DB.Query(r.Context(), `
		SELECT id::text, title, created_at FROM sessions WHERE workspace_id = $1::uuid ORDER BY created_at DESC LIMIT 50`,
		c.WorkspaceID)
	if err != nil {
		a.Log.Error("list sessions", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db")
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, title string
		var created time.Time
		if err := rows.Scan(&id, &title, &created); err != nil {
			errJSON(w, http.StatusInternalServerError, "db")
			return
		}
		out = append(out, map[string]any{"id": id, "title": title, "created_at": created.UTC().Format(time.RFC3339)})
	}
	JSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (a *App) SSEToken(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	sid := chi.URLParam(r, "sessionID")
	var ws string
	err := a.DB.QueryRow(r.Context(), `SELECT workspace_id::text FROM sessions WHERE id = $1::uuid`, sid).Scan(&ws)
	if err != nil || ws != c.WorkspaceID {
		errJSON(w, http.StatusNotFound, "session not found")
		return
	}
	tok, err := auth.SignSSEToken(a.Cfg.JWTSecret, c.UserID, ws, sid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "sse token")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"sse_token": tok, "expires_in": 3600})
}

func (a *App) ListMessages(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	sid := chi.URLParam(r, "sessionID")
	var ws string
	err := a.DB.QueryRow(r.Context(), `SELECT workspace_id::text FROM sessions WHERE id = $1::uuid`, sid).Scan(&ws)
	if err != nil || ws != c.WorkspaceID {
		errJSON(w, http.StatusNotFound, "session not found")
		return
	}
	rows, err := a.DB.Query(r.Context(), `
		SELECT id::text, role, content, created_at FROM messages WHERE session_id = $1::uuid ORDER BY created_at ASC`,
		sid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "db")
		return
	}
	defer rows.Close()
	var items []map[string]any
	for rows.Next() {
		var id, role string
		var content []byte
		var created time.Time
		if err := rows.Scan(&id, &role, &content, &created); err != nil {
			errJSON(w, http.StatusInternalServerError, "db")
			return
		}
		var obj any
		_ = json.Unmarshal(content, &obj)
		items = append(items, map[string]any{
			"id": id, "role": role, "content": obj, "created_at": created.UTC().Format(time.RFC3339),
		})
	}

	// 查询运行步骤
	stepRows, err := a.DB.Query(r.Context(), `
		SELECT step_index, agent_name, status, input_summary, output_summary, error_message, created_at 
		FROM agent_run_steps 
		WHERE run_id IN (SELECT id FROM runs WHERE session_id = $1::uuid)
		ORDER BY created_at ASC`, sid)
	var steps []map[string]any
	if err == nil {
		defer stepRows.Close()
		for stepRows.Next() {
			var idx int
			var agent, stat, inSum, outSum, errMsg string
			var created time.Time
			_ = stepRows.Scan(&idx, &agent, &stat, &inSum, &outSum, &errMsg, &created)
			step := map[string]any{
				"step_index": idx, "agent_name": agent, "status": stat,
				"input_summary": inSum, "output_summary": outSum, "error_message": errMsg,
				"created_at": created.UTC().Format(time.RFC3339),
			}
			steps = append(steps, step)
		}
	}

	JSON(w, http.StatusOK, map[string]any{"messages": items, "run_steps": steps})
}

func (a *App) CreateSession(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	var body struct {
		Title        string `json:"title"`
		DataSourceID string `json:"data_source_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	title := strings.TrimSpace(body.Title)
	if title == "" {
		title = "New session"
	}
	// 如果提供了 data_source_id，验证其必须是属于此工作区的有效 UUID
	var dsID *string
	if ds := strings.TrimSpace(body.DataSourceID); ds != "" {
		var existing string
		err := a.DB.QueryRow(r.Context(),
			`SELECT id::text FROM data_sources WHERE id = $1::uuid AND workspace_id = $2::uuid`,
			ds, c.WorkspaceID).Scan(&existing)
		if err != nil {
			errJSON(w, http.StatusBadRequest, "data_source_id not found or not in workspace")
			return
		}
		dsID = &ds
	}
	id := uuid.NewString()
	var err error
	if dsID != nil {
		_, err = a.DB.Exec(r.Context(), `
			INSERT INTO sessions (id, workspace_id, user_id, data_source_id, title)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5)`,
			id, c.WorkspaceID, c.UserID, *dsID, title)
	} else {
		_, err = a.DB.Exec(r.Context(), `
			INSERT INTO sessions (id, workspace_id, user_id, title)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4)`,
			id, c.WorkspaceID, c.UserID, title)
	}
	if err != nil {
		a.Log.Error("create session", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db")
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "title": title})
}

// PostMessage 处理用户发送的消息请求
func (a *App) PostMessage(w http.ResponseWriter, r *http.Request) {
	// 记录请求开始时间，用于 SSE 进度事件中计算耗时
	startTime := time.Now()
	// 从请求上下文中获取用户认证信息
	c := middleware.ClaimsFromContext(r.Context())
	// 从URL参数中获取会话ID
	sid := chi.URLParam(r, "sessionID")
	// 检查消息发送频率限制/
	if ok, _ := ratelimit.Allow(r.Context(), a.Redis, "msg:"+c.UserID, 30, time.Minute); !ok {
		errJSON(w, http.StatusTooManyRequests, "rate limit")
		return
	}
	// 定义请求体结构
	var body struct {
		Text     string `json:"text"`     // 消息内容
		Workflow string `json:"workflow"` // 工作流类型
	}
	// 解析请求体并验证消息内容
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Text) == "" {
		errJSON(w, http.StatusBadRequest, "text required")
		return
	}
	// 获取请求上下文和追踪信息
	ctx := r.Context()
	trace := middleware.TraceFromContext(ctx)

	// 确保会话属于该工作区，同时查找 data_source_id
	var ws string                                                                                           // 声明一个字符串变量ws，用于存储workspace_id
	var dsID, dsKind *string                                                                                // 声明两个字符串指针变量dsID和dsKind，用于存储数据源ID和数据源类型
	err := a.DB.QueryRow(ctx, `SELECT workspace_id::text FROM sessions WHERE id = $1::uuid`, sid).Scan(&ws) // 执行SQL查询，从sessions表中查询指定ID的session的workspace_id，并将其转换为字符串类型
	if err != nil || ws != c.WorkspaceID {                                                                  // 检查查询是否出错，或者查询到的workspace_id与请求中的workspace_id是否不匹配
		errJSON(w, http.StatusNotFound, "session not found") // 如果出错或不匹配，返回404错误，提示session未找到
		return                                               // 终止函数执行
	}
	// 检查会话是否关联了数据源
	_ = a.DB.QueryRow(ctx, `
		SELECT ds.id::text, ds.kind FROM data_sources ds
		JOIN sessions s ON s.data_source_id = ds.id
		WHERE s.id = $1::uuid`, sid).Scan(&dsID, &dsKind)
	if dsKind == nil {
		k := "postgres"
		dsKind = &k
	}

	userContent, _ := json.Marshal(map[string]string{"text": body.Text})
	_, _ = a.DB.Exec(ctx, `INSERT INTO messages (session_id, role, content) VALUES ($1::uuid, 'user', $2)`, sid, userContent)

	a.Bus.Publish(sid, ssebus.Event{Type: "user_message", Data: map[string]string{"text": body.Text}})

	// 解析 schema JSON 以供 NL2SQL 上下文使用
	sourceKey := "hub"
	var discoverPool *pgxpool.Pool = a.DB
	if dsID != nil {
		sourceKey = *dsID
		// 获取数据源凭证
		var host, db, user, pwd, ssl string
		var port int
		err := a.DB.QueryRow(ctx, `SELECT host, port, database, username, password, sslmode FROM data_sources WHERE id = $1::uuid`, *dsID).Scan(&host, &port, &db, &user, &pwd, &ssl)
		if err != nil {
			errJSON(w, http.StatusInternalServerError, "data source not found")
			return
		}
		decryptedPwd, decErr := hubcrypto.Decrypt(pwd, a.Cfg.DBEncryptionKey)
		if decErr != nil {
			a.Log.Error("decrypt datasource password", zap.Error(decErr))
			errJSON(w, http.StatusInternalServerError, "failed to decrypt datasource password")
			return
		}
		extPool, connErr := schema.ConnectToExternalDataSource(ctx, host, port, db, user, decryptedPwd, ssl)
		if connErr != nil {
			errJSON(w, http.StatusBadGateway, "schema discovery: cannot connect to data source: "+connErr.Error())
			return
		}
		defer extPool.Close()
		discoverPool = extPool
	}
	schemaResult, schemaErr := schema.CachedSchema(ctx, discoverPool, a.Redis, a.Cfg, a.Log, c.WorkspaceID, sourceKey)
	if schemaErr != nil {
		errJSON(w, http.StatusBadGateway, "schema discovery failed: "+schemaErr.Error())
		return
	}
	schemaJSON, _ := schemaResult.ToJSON()
	dialect := *dsKind

	rid := uuid.NewString()
	_, _ = a.DB.Exec(ctx, `INSERT INTO runs (id, session_id, status) VALUES ($1::uuid, $2::uuid, 'running')`, rid, sid)

	a.Bus.Publish(sid, ssebus.Event{Type: "run_started", Data: map[string]string{
		"run_id":     rid,
		"started_at": startTime.UTC().Format(time.RFC3339),
	}})

	// 9.2 Route to async if complex (keyword detection or explicit workflow parameter)
	// 将工作流参数转换为小写并去除前后空格
	workflow := strings.ToLower(strings.TrimSpace(body.Workflow))
	// 如果工作流参数为空，则设置为默认值"auto"
	if workflow == "" {
		workflow = "auto"
	}

	// 将文本转换为小写，以便进行不区分大小写的匹配
	textLow := strings.ToLower(body.Text)
	// 检查文本是否包含复杂任务的关键词，如"分析"、"报告"、"analyze"、"report"
	isComplex := strings.Contains(textLow, "分析") || strings.Contains(textLow, "报告") ||
		strings.Contains(textLow, "analyze") || strings.Contains(textLow, "report")
	// 判断是否使用代理流水线：当工作流为"agent_pipeline"时，或者工作流为"auto"且任务被判定为复杂任务时
	useAgentPipeline := (workflow == "agent_pipeline") || (workflow == "auto" && isComplex)

	// 如果使用代理流水线
	if useAgentPipeline {
		// 异步执行任务，将任务加入队列
		// 参数包括：上下文、工作空间ID、会话ID、请求ID、任务类型和任务数据
		taskID, err := a.AsyncTask.EnqueueTask(ctx, c.WorkspaceID, sid, rid, "agent_pipeline", map[string]any{
			"user_message": body.Text,  // 用户消息内容
			"schema_json":  schemaJSON, // JSON格式的schema
		})
		// 如果任务入队失败
		if err != nil {
			// 标记运行为失败状态，并记录错误信息
			a.finishRunFailed(ctx, rid, sid, "failed to enqueue task: "+err.Error(), codes.Internal)
			// 返回错误响应
			errJSON(w, http.StatusInternalServerError, "enqueue error")
			return
		}
		// 返回成功响应，包含运行ID、任务ID和状态
		JSON(w, http.StatusAccepted, map[string]any{"run_id": rid, "task_id": taskID, "status": "pending_async"})
		return
	}

	// 执行自然语言到SQL的转换操作
	// 使用NL2SQLExec执行器处理用户输入的自然语言查询
	result, err := a.NL2SQLExec.Execute(ctx, nl2sqlexec.Input{
		TraceID:     trace,      // 追踪ID，用于请求追踪和日志记录
		SessionID:   sid,        // 会话ID，用于维护用户会话状态
		UserMessage: body.Text,  // 用户输入的文本内容，即自然语言查询
		SchemaJSON:  schemaJSON, // 数据库模式的JSON表示，描述表结构
		Dialect:     dialect,    // SQL方言，指定要使用的SQL类型(如MySQL, PostgreSQL等)
		Role:        c.Role,     // 用户角色，用于写操作权限检查
	}, a.DB) // 数据库连接对象，用于执行生成的SQL查询
	// 检查错误是否为 nil
	if err != nil {
		// 尝试从错误中提取 gRPC 状态信息
		if st, ok := status.FromError(err); ok {
			// 处理 gRPC 状态错误：记录失败运行，返回 HTTP 502 BadGateway 状态码
			a.finishRunFailed(ctx, rid, sid, "nl2sql: "+st.Message(), st.Code())
			errJSON(w, http.StatusBadGateway, mapGRPCCode(st.Code()))
			return
		}
		// 检查是否为 nl2sql 生成错误
		if genErr, ok := err.(*nl2sqlexec.GenerateError); ok {
			// 处理 nl2sql 生成错误：记录失败运行，返回 HTTP 400 BadRequest 状态码
			a.finishRunFailed(ctx, rid, sid, genErr.Message, codes.Internal)
			errJSON(w, http.StatusBadRequest, genErr.Message)
			return
		}
		// SQL 执行错误 — result 中保留了生成的 SQL 供调试
		a.finishRunFailed(ctx, rid, sid, err.Error(), codes.InvalidArgument)
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// 从结果中获取SQL语句和行数据
	sql := result.SQL
	_elapsedMs := time.Since(startTime).Milliseconds()
	// 通过总线发布SQL生成事件，包含会话ID和事件数据
	// 事件类型为"sql_generated"，数据为包含SQL语句和耗时的map
	a.Bus.Publish(sid, ssebus.Event{Type: "sql_generated", Data: map[string]any{
		"sql":        sql,
		"elapsed_ms": _elapsedMs,
		"is_write":   result.IsWrite,
	}})

	// 根据读写类型组装响应
	totalElapsedMs := time.Since(startTime).Milliseconds()
	startedAt := startTime.UTC().Format(time.RFC3339)
	var assist []byte
	var resp map[string]any
	if result.IsWrite {
		assist, _ = json.Marshal(map[string]any{
			"sql":           sql,
			"rows_affected": result.RowsAffected,
			"notes":         result.SelfCheckNotes,
			"type":          "write",
			"elapsed_ms":    totalElapsedMs,
			"started_at":    startedAt,
		})
		resp = map[string]any{
			"run_id":        rid,
			"sql":           sql,
			"rows_affected": result.RowsAffected,
			"elapsed_ms":    totalElapsedMs,
			"started_at":    startedAt,
		}
	} else {
		rows := result.Rows
		assist, _ = json.Marshal(map[string]any{
			"sql":        sql,
			"rows":       rows,
			"notes":      result.SelfCheckNotes,
			"elapsed_ms": totalElapsedMs,
			"started_at": startedAt,
		})
		resp = map[string]any{
			"run_id":     rid,
			"sql":        sql,
			"rows":       rows,
			"elapsed_ms": totalElapsedMs,
			"started_at": startedAt,
		}
	}
	// 将助手的响应（包含SQL查询结果等信息）插入到messages表中
	_, _ = a.DB.Exec(ctx, `INSERT INTO messages (session_id, role, content) VALUES ($1::uuid, 'assistant', $2)`, sid, assist)
	// 更新runs表中的状态为'completed'，并更新updated_at时间戳
	_, _ = a.DB.Exec(ctx, `UPDATE runs SET status = 'completed', updated_at = now() WHERE id = $1::uuid`, rid)
	// 通过事件总线发布结果事件，将JSON格式的响应数据作为事件数据发送
	a.Bus.Publish(sid, ssebus.Event{Type: "result", Data: json.RawMessage(assist)})
	// 返回HTTP响应，包含运行ID、SQL查询语句和查询结果行数
	JSON(w, http.StatusOK, resp)
}

// mapGRPCCode 将gRPC状态码映射为自定义的错误信息字符串
// 参数:
//
//	c - gRPC状态码，使用codes.Code类型
//
// 返回值:
//
//	string - 根据不同的gRPC状态码返回对应的错误信息描述
func mapGRPCCode(c codes.Code) string {
	switch c {
	case codes.Unavailable: // 当gRPC状态码为不可用(Unavailable)时
		return "worker unavailable" // 返回"worker unavailable"字符串
	case codes.DeadlineExceeded: // 当gRPC状态码为超时(DeadlineExceeded)时
		return "deadline exceeded" // 返回"deadline exceeded"字符串
	default: // 处理其他未明确指定的状态码
		return "worker error" // 默认返回"worker error"字符串
	}
}

func (a *App) finishRunFailed(ctx context.Context, runID, sessionID, msg string, c codes.Code) {
	assist, _ := json.Marshal(map[string]any{"error": msg, "code": c.String()})
	_, _ = a.DB.Exec(ctx, `INSERT INTO messages (session_id, role, content) VALUES ($1::uuid, 'assistant', $2)`, sessionID, assist)
	_, _ = a.DB.Exec(ctx, `UPDATE runs SET status = 'failed', pending_reason = $2, updated_at = now() WHERE id = $1::uuid`, runID, msg)
	a.Bus.Publish(sessionID, ssebus.Event{Type: "error", Data: map[string]string{"message": msg}})
}

// SessionStream 为会话提供 SSE 事件流（MVP 内存总线）
func (a *App) SessionStream(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "sessionID")
	c := middleware.ClaimsFromContext(r.Context())
	var ws string
	err := a.DB.QueryRow(r.Context(), `SELECT workspace_id::text FROM sessions WHERE id = $1::uuid`, sid).Scan(&ws)
	if err != nil || ws != c.WorkspaceID {
		errJSON(w, http.StatusNotFound, "session not found")
		return
	}
	ch := a.Bus.Subscribe(sid)
	defer a.Bus.Unsubscribe(sid, ch)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	fl, ok := w.(http.Flusher)
	if !ok {
		errJSON(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	enc := json.NewEncoder(w)

	// 流结束时记录丢弃计数以供可观测性
	defer func() {
		if d := a.Bus.TotalDrops(); d > 0 {
			a.Log.Warn("sse stream ended with drops",
				zap.String("session_id", sid),
				zap.Int64("total_drops", d),
			)
		}
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write([]byte("event: " + ev.Type + "\n"))
			_, _ = w.Write([]byte("data: "))
			_ = enc.Encode(ev.Data)
			_, _ = w.Write([]byte("\n"))
			fl.Flush()
		}
	}
}

// InternalNL2SQL 由 Python 编排器的 nl2sql_node 调用，用于通过 Go 的安全边界（gRPC → sqlrun）执行 NL2SQL。
// 它接收 user_message、schema_json 和 trace_id，调用 Python 工作节点的 GenerateSQL RPC，
// 执行返回的 SQL（只读），并将结果返回。
// 认证由 InternalHMACAuth 中间件处理。
func (a *App) InternalNL2SQL(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TraceID     string `json:"trace_id"`
		UserMessage string `json:"user_message"`
		SchemaJSON  string `json:"schema_json"`
		Dialect     string `json:"dialect"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.UserMessage == "" {
		errJSON(w, http.StatusBadRequest, "user_message is required")
		return
	}
	if body.Dialect == "" {
		body.Dialect = "postgres"
	}

	traceID := body.TraceID
	if traceID == "" {
		traceID = middleware.TraceFromContext(r.Context())
	}

	result, err := a.NL2SQLExec.Execute(r.Context(), nl2sqlexec.Input{
		TraceID:     traceID,
		SessionID:   "",
		UserMessage: body.UserMessage,
		SchemaJSON:  body.SchemaJSON,
		Dialect:     body.Dialect,
	}, a.DB)
	if err != nil {
		if _, ok := status.FromError(err); ok {
			a.Log.Error("internal nl2sql: gRPC GenerateSQL failed", zap.Error(err))
			errJSON(w, http.StatusBadGateway, "nl2sql worker error: "+err.Error())
		} else if genErr, ok := err.(*nl2sqlexec.GenerateError); ok {
			errJSON(w, http.StatusBadRequest, genErr.Message)
		} else {
			errJSON(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"sql":   result.SQL,
		"rows":  result.Rows,
		"notes": result.SelfCheckNotes,
	})
}

// Routes 构建 chi 路由器
func Routes(a *App) http.Handler {
	// 创建一个新的Chi路由器实例
	r := chi.NewRouter()
	// 添加中间件：恢复器，用于捕获和处理panic
	r.Use(chimw.Recoverer)
	// 添加中间件：超时处理器，设置60秒的超时时间
	r.Use(chimw.Timeout(60 * time.Second))
	// 添加中间件：请求追踪ID生成器
	r.Use(middleware.TraceID)
	// 添加中间件：请求日志记录器，使用a.Log记录请求信息
	r.Use(middleware.RequestLog(a.Log))
	// 添加中间件：Prometheus指标收集
	r.Use(telemetry.PrometheusMiddleware)
	// 注册Prometheus指标端点
	r.Handle("/metrics", telemetry.Handler())
	// 注册健康检查端点
	r.Get("/health", a.Health)
	// 注册版本信息端点
	r.Get("/version", a.Version)
	// 注册登录端点
	r.Post("/v1/auth/login", a.Login)

	// 注册v1版本API路由组
	r.Route("/v1", func(r chi.Router) {
		// 为v1路由组添加认证中间件
		r.Use(middleware.Auth(a.Cfg, a.Log, a.Redis))

		// 会话相关路由
		r.Get("/sessions", a.ListSessions)                      // 获取会话列表
		r.Post("/sessions", a.CreateSession)                    // 创建新会话
		r.Get("/sessions/{sessionID}/messages", a.ListMessages) // 获取会话消息
		r.Post("/sessions/{sessionID}/messages", a.PostMessage) // 发送消息
		r.Get("/sessions/{sessionID}/stream", a.SessionStream)  // 会话流
		r.Post("/sessions/{sessionID}/sse-token", a.SSEToken)   // SSE令牌

		// 用户管理路由（admin only）
		r.With(middleware.RequireMinRole("admin")).Post("/auth/register", a.Register)         // 创建用户
		r.With(middleware.RequireMinRole("admin")).Get("/users", a.ListUsers)                  // 用户列表
		r.With(middleware.RequireMinRole("admin")).Put("/users/{id}/role", a.ChangeUserRole)   // 修改角色
		r.With(middleware.RequireMinRole("admin")).Delete("/users/{id}", a.DeleteUser)         // 删除用户

		// 数据源相关路由
		r.Get("/data-sources", a.ListDataSources) // 获取数据源列表

		// 以下路由需要operator角色权限
		r.With(middleware.RequireMinRole("operator")).Post("/data-sources", a.CreateDataSource)         // 创建数据源
		r.With(middleware.RequireMinRole("operator")).Post("/data-sources/{id}/test", a.TestDataSource) // 测试数据源
		r.With(middleware.RequireMinRole("admin")).Put("/data-sources/{id}", a.UpdateDataSource)        // 编辑数据源
		r.With(middleware.RequireMinRole("admin")).Delete("/data-sources/{id}", a.DeleteDataSource)     // 删除数据源

		// 知识文档相关路由
		// 以下路由需要operator角色权限
		r.With(middleware.RequireMinRole("operator")).Get("/workspaces/{workspaceID}/knowledge/docs", a.ListKnowledgeDocs)   // 获取知识文档列表
		r.With(middleware.RequireMinRole("operator")).Post("/workspaces/{workspaceID}/knowledge/docs", a.UploadKnowledgeDoc) // 上传知识文档

		// 任务和运行相关路由
		r.Get("/runs/{runID}/report", a.DownloadReport) // 下载运行报告
		r.Get("/tasks/{taskID}", a.TaskStatus)          // 获取任务状态

		// 数据管理路由（operator+）
		r.With(middleware.RequireMinRole("operator")).Post("/data/upload", a.UploadData)                // 文件上传导入
		r.With(middleware.RequireMinRole("operator")).Post("/data/suggest-table", a.SuggestTable)        // AI 建表建议
		r.With(middleware.RequireMinRole("operator")).Post("/data/create-table", a.CreateTable)          // 确认建表

		// Schema 路由
		r.Get("/schema/tables", a.ListTables) // 获取表结构列表
	})

	// 注册内部API路由组
	r.Route("/internal", func(r chi.Router) {
		// 为内部路由组添加HMAC认证中间件
		r.Use(middleware.InternalHMACAuth(a.Cfg.InternalHMACSecret))

		// 内部任务和回调路由
		r.Post("/tasks/{taskID}/callback", a.TaskCallback)                // 任务回调
		r.Post("/runs/{runID}/steps", a.RunStepCallback)                  // 运行步骤回调
		r.Post("/nl2sql", a.InternalNL2SQL)                               // 内部NL2SQL接口
		r.Patch("/knowledge-docs/{docID}/status", a.KnowledgeDocCallback) // 知识文档状态回调
	})

	// 返回配置好的路由器
	return r
}
