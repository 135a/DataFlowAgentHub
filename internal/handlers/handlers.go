package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dataflowagenthub/hub/internal/async"
	"github.com/dataflowagenthub/hub/internal/auth"
	"github.com/dataflowagenthub/hub/internal/config"
	hubcrypto "github.com/dataflowagenthub/hub/internal/crypto"
	"github.com/dataflowagenthub/hub/internal/llm"
	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/dataflowagenthub/hub/internal/mysqlmgr"
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
	Bus        ssebus.Bus
	NATS       *nats.Conn
	AsyncTask  *async.Client
	NL2SQLExec *nl2sqlexec.Executor
	LlmClient  *llm.Client
	MySQLMgr   *mysqlmgr.Manager
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// unrecoverable: headers already sent; connection likely broken if this fails
	_ = json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

// sessionBelongsToWorkspace 检查会话是否属于指定工作区
func (a *App) sessionBelongsToWorkspace(ctx context.Context, sessionID, workspaceID string) bool {
	var ws string
	err := a.DB.QueryRow(ctx, `SELECT workspace_id::text FROM sessions WHERE id = $1::uuid`, sessionID).Scan(&ws)
	return err == nil && ws == workspaceID
}

// Login 接受种子用户凭证并返回 JWT
func (a *App) Login(w http.ResponseWriter, r *http.Request) {
	// 限流：每 IP 每分钟 20 次
	if ok, _ := ratelimit.Allow(r.Context(), a.Redis, "login:"+r.RemoteAddr, 20, time.Minute, a.Cfg.RateLimitFailClosed); !ok {
		errJSON(w, http.StatusTooManyRequests, "rate limit")
		return
	}
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
	if err := rows.Err(); err != nil {
		a.Log.Error("list sessions rows iteration", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (a *App) SSEToken(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	sid := chi.URLParam(r, "sessionID")
	if !a.sessionBelongsToWorkspace(r.Context(), sid, c.WorkspaceID) {
		errJSON(w, http.StatusNotFound, "session not found")
		return
	}
	tok, err := auth.SignSSEToken(a.Cfg.JWTSecret, c.UserID, c.WorkspaceID, sid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "sse token")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"sse_token": tok, "expires_in": 3600})
}

func (a *App) ListMessages(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	sid := chi.URLParam(r, "sessionID")
	if !a.sessionBelongsToWorkspace(r.Context(), sid, c.WorkspaceID) {
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
		if err := json.Unmarshal(content, &obj); err != nil {
			a.Log.Warn("unmarshal message content", zap.Error(err))
		}
		items = append(items, map[string]any{
			"id": id, "role": role, "content": obj, "created_at": created.UTC().Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		a.Log.Warn("list messages rows iteration", zap.Error(err))
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
			if err := stepRows.Scan(&idx, &agent, &stat, &inSum, &outSum, &errMsg, &created); err != nil {
				a.Log.Warn("scan agent_run_step", zap.Error(err))
				continue
			}
			step := map[string]any{
				"step_index": idx, "agent_name": agent, "status": stat,
				"input_summary": inSum, "output_summary": outSum, "error_message": errMsg,
				"created_at": created.UTC().Format(time.RFC3339),
			}
			steps = append(steps, step)
		}
		if err := stepRows.Err(); err != nil {
			a.Log.Warn("agent run steps rows iteration", zap.Error(err))
		}
	}

	JSON(w, http.StatusOK, map[string]any{"messages": items, "run_steps": steps})
}

func (a *App) CreateSession(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	var body struct {
		Title          string `json:"title"`
		DataSourceID   string `json:"data_source_id"`
		DatasetID      string `json:"dataset_id"`
		DatasetTableID string `json:"dataset_table_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.Log.Warn("decode create session body", zap.Error(err))
	}
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

	// 验证 dataset 和 table 关系
	var dsIDPtr, dtIDPtr *string
	if did := strings.TrimSpace(body.DatasetID); did != "" {
		var exists int
		if err := a.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM datasets WHERE id = $1::uuid AND status != 'deleted'`, did).Scan(&exists); err != nil || exists == 0 {
			errJSON(w, http.StatusBadRequest, "dataset not found")
			return
		}
		dsIDPtr = &did
	}
	if tid := strings.TrimSpace(body.DatasetTableID); tid != "" {
		if dsIDPtr == nil {
			errJSON(w, http.StatusBadRequest, "dataset_table_id requires dataset_id")
			return
		}
		var exists int
		if err := a.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM dataset_tables WHERE id = $1::uuid AND dataset_id = $2::uuid AND status = 'active'`, tid, *dsIDPtr).Scan(&exists); err != nil || exists == 0 {
			errJSON(w, http.StatusBadRequest, "table not found in dataset")
			return
		}
		dtIDPtr = &tid
	}

	id := uuid.NewString()
	var err error
	if dsID != nil {
		_, err = a.DB.Exec(r.Context(), `
			INSERT INTO sessions (id, workspace_id, user_id, data_source_id, title, dataset_id, dataset_table_id)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6::uuid, $7::uuid)`,
			id, c.WorkspaceID, c.UserID, *dsID, title, dsIDPtr, dtIDPtr)
	} else {
		_, err = a.DB.Exec(r.Context(), `
			INSERT INTO sessions (id, workspace_id, user_id, title, dataset_id, dataset_table_id)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, $6::uuid)`,
			id, c.WorkspaceID, c.UserID, title, dsIDPtr, dtIDPtr)
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
	var datasetID, datasetTableID *string
	if err := a.DB.QueryRow(ctx, `
		SELECT dataset_id::text, dataset_table_id::text FROM sessions WHERE id = $1::uuid`, sid).Scan(&datasetID, &datasetTableID); err != nil {
		// sessions 没有 dataset 关联，继续使用旧流程
	}

	// 插入用户消息
	userContent, err := json.Marshal(map[string]string{"text": body.Text})
	if err != nil {
		a.Log.Error("marshal user message", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "internal error")
		return
	}
	if _, err := a.DB.Exec(ctx, `INSERT INTO messages (session_id, role, content) VALUES ($1::uuid, 'user', $2)`, sid, userContent); err != nil {
		a.Log.Error("insert user message", zap.Error(err))
	}
	a.Bus.Publish(sid, ssebus.Event{Type: "user_message", Data: map[string]string{"text": body.Text}})

	// 创建 run
	rid := uuid.NewString()
	if _, err := a.DB.Exec(ctx, `INSERT INTO runs (id, session_id, status) VALUES ($1::uuid, $2::uuid, 'running')`, rid, sid); err != nil {
		a.Log.Error("insert run", zap.Error(err))
	}
	a.Bus.Publish(sid, ssebus.Event{Type: "run_started", Data: map[string]string{
		"run_id":     rid,
		"started_at": startTime.UTC().Format(time.RFC3339),
	}})

	// 如果会话关联了数据集表，走 MySQL 路径
	if datasetID != nil && datasetTableID != nil && *datasetID != "" && *datasetTableID != "" {
		a.postMessageToDataset(ctx, w, r, c, sid, rid, body, trace, startTime, *datasetID, *datasetTableID)
		return
	}

	// 旧流程：PostgreSQL / 外部数据源
	var dsID, dsKind *string
	if err := a.DB.QueryRow(ctx, `
		SELECT ds.id::text, ds.kind FROM data_sources ds
		JOIN sessions s ON s.data_source_id = ds.id
		WHERE s.id = $1::uuid`, sid).Scan(&dsID, &dsKind); err != nil {
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
}, trace string, startTime time.Time, datasetID, datasetTableID string) {
	// 构建 schema 从 table_fields
	schemaJSON, err := a.resolveDatasetSchema(ctx, datasetTableID)
	if err != nil {
		a.finishRunFailed(ctx, rid, sid, "schema error: "+err.Error(), codes.Internal)
		errJSON(w, http.StatusBadGateway, err.Error())
		return
	}

	// 获取 MySQL 连接池
	var mysqlDB string
	if err := a.DB.QueryRow(ctx, `SELECT mysql_database FROM datasets WHERE id = $1::uuid`, datasetID).Scan(&mysqlDB); err != nil {
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

	result, err := a.NL2SQLExec.ExecuteMySQL(ctx, nl2sqlexec.Input{
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

// resolveDatasetSchema 从 table_fields 构建数据集表的 schema JSON。
func (a *App) resolveDatasetSchema(ctx context.Context, tableID string) (string, error) {
	rows, err := a.DB.Query(ctx, `
		SELECT dt.name, tf.name, tf.field_type, tf.is_nullable
		FROM table_fields tf
		JOIN dataset_tables dt ON dt.id = tf.table_id
		WHERE tf.table_id = $1::uuid
		ORDER BY tf.ordinal_position ASC`, tableID)
	if err != nil {
		return "", fmt.Errorf("query dataset schema: %w", err)
	}
	defer rows.Close()

	var tableName string
	var columns []schema.ColumnSchema
	for rows.Next() {
		var colName, colType string
		var nullable bool
		if err := rows.Scan(&tableName, &colName, &colType, &nullable); err != nil {
			return "", fmt.Errorf("scan dataset schema: %w", err)
		}
		columns = append(columns, schema.ColumnSchema{
			Name:     colName,
			Type:     colType,
			Nullable: nullable,
		})
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate dataset schema: %w", err)
	}

	sr := &schema.SchemaResult{
		Tables: []schema.TableSchema{
			{Name: tableName, Columns: columns},
		},
	}
	return sr.ToJSON()
}

// resolveSchema 为会话解析数据库 schema，返回 JSON 字符串
func (a *App) resolveSchema(ctx context.Context, dsID *string, workspaceID string) (string, error) {
	sourceKey := "hub"
	var discoverPool *pgxpool.Pool = a.DB
	if dsID != nil {
		sourceKey = *dsID
		var host, db, user, pwd, ssl string
		var port int
		if err := a.DB.QueryRow(ctx,
			`SELECT host, port, database, username, password, sslmode FROM data_sources WHERE id = $1::uuid`, *dsID,
		).Scan(&host, &port, &db, &user, &pwd, &ssl); err != nil {
			return "", fmt.Errorf("data source not found: %w", err)
		}
		decryptedPwd, decErr := hubcrypto.Decrypt(pwd, a.Cfg.DBEncryptionKey)
		if decErr != nil {
			a.Log.Error("decrypt datasource password", zap.Error(decErr))
			return "", fmt.Errorf("failed to decrypt datasource password")
		}
		extPool, connErr := schema.ConnectToExternalDataSource(ctx, host, port, db, user, decryptedPwd, ssl)
		if connErr != nil {
			return "", fmt.Errorf("schema discovery: cannot connect to data source: %w", connErr)
		}
		defer extPool.Close()
		discoverPool = extPool
	}

	schemaResult, schemaErr := schema.CachedSchema(ctx, discoverPool, a.Redis, a.Cfg, a.Log, workspaceID, sourceKey)
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

	if _, err := a.DB.Exec(ctx, `INSERT INTO messages (session_id, role, content) VALUES ($1::uuid, 'assistant', $2)`, sid, assist); err != nil {
		a.Log.Error("insert assistant message", zap.Error(err))
	}
	if _, err := a.DB.Exec(ctx, `UPDATE runs SET status = 'completed', updated_at = now() WHERE id = $1::uuid`, rid); err != nil {
		a.Log.Error("update run completed", zap.Error(err))
	}
	a.Bus.Publish(sid, ssebus.Event{Type: "result", Data: json.RawMessage(assist)})
	return resp, nil
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
	assist, err := json.Marshal(map[string]any{"error": msg, "code": c.String()})
	if err != nil {
		a.Log.Error("marshal failed run", zap.Error(err))
	}
	if _, err := a.DB.Exec(ctx, `INSERT INTO messages (session_id, role, content) VALUES ($1::uuid, 'assistant', $2)`, sessionID, assist); err != nil {
		a.Log.Error("insert failed assistant message", zap.Error(err))
	}
	if _, err := a.DB.Exec(ctx, `UPDATE runs SET status = 'failed', pending_reason = $2, updated_at = now() WHERE id = $1::uuid`, runID, msg); err != nil {
		a.Log.Error("update run failed", zap.Error(err))
	}
	a.Bus.Publish(sessionID, ssebus.Event{Type: "error", Data: map[string]string{"message": msg}})
}

// SessionStream 为会话提供 SSE 事件流（MVP 内存总线）
func (a *App) SessionStream(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "sessionID")
	c := middleware.ClaimsFromContext(r.Context())
	if !a.sessionBelongsToWorkspace(r.Context(), sid, c.WorkspaceID) {
		errJSON(w, http.StatusNotFound, "session not found")
		return
	}
	ch, cancel := a.Bus.Subscribe(sid)
	defer cancel()

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
			if _, err := w.Write([]byte("event: " + ev.Type + "\n")); err != nil {
				return
			}
			if _, err := w.Write([]byte("data: ")); err != nil {
				return
			}
			if err := enc.Encode(ev.Data); err != nil {
				return
			}
			if _, err := w.Write([]byte("\n")); err != nil {
				return
			}
			fl.Flush()
		}
	}
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

	// 注册公开路由（无需认证）
	r.Post("/v1/auth/self-register", a.SelfRegister) // 用户自注册

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

		// 用户管理路由（super_admin only）
		r.With(middleware.RequireMinRole("super_admin")).Post("/auth/register", a.Register)         // 创建用户
		r.With(middleware.RequireMinRole("super_admin")).Get("/users", a.ListUsers)                  // 用户列表
		r.With(middleware.RequireMinRole("super_admin")).Put("/users/{id}/role", a.ChangeUserRole)   // 修改角色
		r.With(middleware.RequireMinRole("super_admin")).Delete("/users/{id}", a.DeleteUser)         // 删除用户

		// 权限升级申请路由
		r.Post("/auth/upgrade-request", a.CreateUpgradeRequest)                                              // 提交升级申请（normal_user）
		r.With(middleware.RequireMinRole("super_admin")).Get("/auth/upgrade-requests", a.ListUpgradeRequests)  // 列出申请
		r.With(middleware.RequireMinRole("super_admin")).Put("/auth/upgrade-requests/{id}", a.ReviewUpgradeRequest) // 审核申请

		// 数据源相关路由
		r.Get("/data-sources", a.ListDataSources) // 获取数据源列表

		// 以下路由需要data_admin+角色权限
		r.With(middleware.RequireMinRole("data_admin")).Post("/data-sources", a.CreateDataSource)         // 创建数据源
		r.With(middleware.RequireMinRole("normal_user")).Post("/data-sources/{id}/test", a.TestDataSource) // 测试数据源
		r.With(middleware.RequireMinRole("super_admin")).Put("/data-sources/{id}", a.UpdateDataSource)     // 编辑数据源
		r.With(middleware.RequireMinRole("super_admin")).Delete("/data-sources/{id}", a.DeleteDataSource)  // 删除数据源

		// 知识文档相关路由
		r.With(middleware.RequireMinRole("data_admin")).Get("/workspaces/{workspaceID}/knowledge/docs", a.ListKnowledgeDocs)   // 获取知识文档列表
		r.With(middleware.RequireMinRole("data_admin")).Post("/workspaces/{workspaceID}/knowledge/docs", a.UploadKnowledgeDoc) // 上传知识文档

		// 任务和运行相关路由
		r.Get("/runs/{runID}/report", a.DownloadReport) // 下载运行报告
		r.Get("/tasks/{taskID}", a.TaskStatus)          // 获取任务状态

		// 数据管理路由
		r.With(middleware.RequireMinRole("normal_user")).Post("/data/upload", a.UploadData)                // 文件上传导入
		r.Post("/data/suggest-table", a.SuggestTable)        // 已禁用
		r.Post("/data/create-table", a.CreateTable)          // 已禁用

		// Schema 路由
		r.Get("/schema/tables", a.ListTables) // 获取表结构列表

		// ====== 数据集管理路由 ======
		r.Get("/datasets", a.ListDatasets)                                                    // 列出数据集
		r.With(middleware.RequireMinRole("super_admin")).Post("/datasets", a.CreateDataset)    // 创建数据集
		r.Get("/datasets/{id}", a.GetDataset)                                                 // 获取数据集详情
		r.With(middleware.RequireMinRole("super_admin")).Put("/datasets/{id}", a.UpdateDataset) // 更新数据集
		r.With(middleware.RequireMinRole("super_admin")).Delete("/datasets/{id}", a.DeleteDataset) // 删除数据集
		r.With(middleware.RequireMinRole("super_admin")).Post("/datasets/{id}/grant", a.GrantDatasetAccess)   // 授权
		r.With(middleware.RequireMinRole("super_admin")).Post("/datasets/{id}/revoke", a.RevokeDatasetAccess) // 撤销授权

		// ====== 数据表管理路由 ======
		r.Get("/datasets/{did}/tables", a.ListDatasetTables)                                                  // 列出表
		r.With(middleware.RequireMinRole("data_admin")).Post("/datasets/{did}/tables", a.CreateDatasetTable)    // 创建表
		r.Get("/datasets/{did}/tables/{tid}", a.GetDatasetTable)                                              // 获取表详情
		r.With(middleware.RequireMinRole("data_admin")).Delete("/datasets/{did}/tables/{tid}", a.DeleteDatasetTable) // 删除表
		r.Get("/datasets/{did}/tables/{tid}/fields", a.ListFields)                                            // 获取字段列表
	})

	// 返回配置好的路由器
	return r
}
