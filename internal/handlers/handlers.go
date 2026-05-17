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

	a.Bus.Publish(sid, ssebus.Event{Type: "run_started", Data: map[string]string{"run_id": rid}})

	// 9.2 Route to async if complex (keyword detection or explicit workflow parameter)
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
		// SQL 执行错误 — result 中保留了生成的 SQL 供调试
		a.finishRunFailed(ctx, rid, sid, err.Error(), codes.InvalidArgument)
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	sql := result.SQL
	rows := result.Rows
	a.Bus.Publish(sid, ssebus.Event{Type: "sql_generated", Data: map[string]string{"sql": sql}})

	assist, _ := json.Marshal(map[string]any{"sql": sql, "rows": rows, "notes": result.SelfCheckNotes})
	_, _ = a.DB.Exec(ctx, `INSERT INTO messages (session_id, role, content) VALUES ($1::uuid, 'assistant', $2)`, sid, assist)
	_, _ = a.DB.Exec(ctx, `UPDATE runs SET status = 'completed', updated_at = now() WHERE id = $1::uuid`, rid)
	a.Bus.Publish(sid, ssebus.Event{Type: "result", Data: json.RawMessage(assist)})
	JSON(w, http.StatusOK, map[string]any{"run_id": rid, "sql": sql, "rows": rows})
}

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
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(middleware.TraceID)
	r.Use(middleware.RequestLog(a.Log))
	r.Use(telemetry.PrometheusMiddleware)
	r.Handle("/metrics", telemetry.Handler())
	r.Get("/health", a.Health)
	r.Get("/version", a.Version)
	r.Post("/v1/auth/login", a.Login)

	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.Auth(a.Cfg, a.Log, a.Redis))
		r.Get("/sessions", a.ListSessions)
		r.Post("/sessions", a.CreateSession)
		r.Get("/sessions/{sessionID}/messages", a.ListMessages)
		r.Post("/sessions/{sessionID}/messages", a.PostMessage)
		r.Get("/sessions/{sessionID}/stream", a.SessionStream)
		r.Post("/sessions/{sessionID}/sse-token", a.SSEToken)
		r.Get("/data-sources", a.ListDataSources)
		r.With(middleware.RequireMinRole("operator")).Post("/data-sources", a.CreateDataSource)
		r.With(middleware.RequireMinRole("operator")).Post("/data-sources/{id}/test", a.TestDataSource)
		r.With(middleware.RequireMinRole("operator")).Get("/workspaces/{workspaceID}/knowledge/docs", a.ListKnowledgeDocs)
		r.With(middleware.RequireMinRole("operator")).Post("/workspaces/{workspaceID}/knowledge/docs", a.UploadKnowledgeDoc)

		r.Get("/runs/{runID}/report", a.DownloadReport)
		r.Get("/tasks/{taskID}", a.TaskStatus)
	})

	r.Route("/internal", func(r chi.Router) {
		r.Use(middleware.InternalHMACAuth(a.Cfg.InternalHMACSecret))
		r.Post("/tasks/{taskID}/callback", a.TaskCallback)
		r.Post("/runs/{runID}/steps", a.RunStepCallback)
		r.Post("/nl2sql", a.InternalNL2SQL)
		r.Patch("/knowledge-docs/{docID}/status", a.KnowledgeDocCallback)
	})

	return r
}
