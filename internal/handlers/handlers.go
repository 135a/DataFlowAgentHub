package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dataflowagenthub/hub/internal/async"
	"github.com/dataflowagenthub/hub/internal/auth"
	"github.com/dataflowagenthub/hub/internal/config"
	"github.com/dataflowagenthub/hub/internal/llm"
	"github.com/dataflowagenthub/hub/internal/middleware"
	"github.com/dataflowagenthub/hub/internal/mysqlmgr"
	"github.com/dataflowagenthub/hub/internal/nl2sqlexec"
	"github.com/dataflowagenthub/hub/internal/ratelimit"
	"github.com/dataflowagenthub/hub/internal/seed"
	"github.com/dataflowagenthub/hub/internal/sqlrun"
	"github.com/dataflowagenthub/hub/internal/ssebus"
	"github.com/dataflowagenthub/hub/internal/telemetry"
	"github.com/dataflowagenthub/hub/internal/worker"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const version = "0.1.0-dev"

// App 封装 HTTP 依赖项
type App struct {
	Cfg        *config.Config
	Log        *zap.Logger
	DB         *sql.DB
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
	err := a.DB.QueryRowContext(ctx, `SELECT workspace_id FROM sessions WHERE id = ?`, sessionID).Scan(&ws)
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
	err := a.DB.QueryRowContext(r.Context(), `
		SELECT id, password_hash, role FROM users WHERE workspace_id = ? AND email = ?`,
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
	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT id, title, created_at FROM sessions WHERE workspace_id = ? ORDER BY created_at DESC LIMIT 50`,
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
	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT id, role, content, created_at FROM messages WHERE session_id = ? ORDER BY created_at ASC`,
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
	stepRows, err := a.DB.QueryContext(r.Context(), `
		SELECT step_index, agent_name, status, input_summary, output_summary, error_message, created_at
		FROM agent_run_steps
		WHERE run_id IN (SELECT id FROM runs WHERE session_id = ?)
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
		DatasetTableID string `json:"dataset_table_id"` // 保留向后兼容，不再使用
		QuerySource    string `json:"query_source"`     // "knowledge" | "dataset"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.Log.Warn("decode create session body", zap.Error(err))
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		title = "New session"
	}

	// 推断 query_source：有 dataset_id 则是 dataset 模式，否则 knowledge
	querySource := strings.TrimSpace(body.QuerySource)
	if querySource == "" {
		if body.DatasetID != "" {
			querySource = "dataset"
		} else {
			querySource = "knowledge"
		}
	}
	if querySource != "knowledge" && querySource != "dataset" {
		errJSON(w, http.StatusBadRequest, "query_source must be 'knowledge' or 'dataset'")
		return
	}

	// 如果提供了 data_source_id，验证其必须是属于此工作区的有效 UUID
	var dsID *string
	if ds := strings.TrimSpace(body.DataSourceID); ds != "" {
		var existing string
		err := a.DB.QueryRowContext(r.Context(),
			`SELECT id FROM data_sources WHERE id = ? AND workspace_id = ?`,
			ds, c.WorkspaceID).Scan(&existing)
		if err != nil {
			errJSON(w, http.StatusBadRequest, "data_source_id not found or not in workspace")
			return
		}
		dsID = &ds
	}

	// 验证 dataset（dataset 模式下必填）
	var dsIDPtr *string
	if querySource == "dataset" {
		did := strings.TrimSpace(body.DatasetID)
		if did == "" {
			errJSON(w, http.StatusBadRequest, "dataset_id is required when query_source is 'dataset'")
			return
		}
		var exists int
		if err := a.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM datasets WHERE id = ? AND status != 'deleted'`, did).Scan(&exists); err != nil || exists == 0 {
			errJSON(w, http.StatusBadRequest, "dataset not found")
			return
		}
		dsIDPtr = &did
	}

	id := uuid.NewString()
	var err error
	if dsID != nil {
		_, err = a.DB.ExecContext(r.Context(), `
			INSERT INTO sessions (id, workspace_id, user_id, data_source_id, title, dataset_id)
			VALUES (?, ?, ?, ?, ?, ?)`,
			id, c.WorkspaceID, c.UserID, *dsID, title, dsIDPtr)
	} else {
		_, err = a.DB.ExecContext(r.Context(), `
			INSERT INTO sessions (id, workspace_id, user_id, title, dataset_id)
			VALUES (?, ?, ?, ?, ?)`,
			id, c.WorkspaceID, c.UserID, title, dsIDPtr)
	}
	if err != nil {
		a.Log.Error("create session", zap.Error(err))
		errJSON(w, http.StatusInternalServerError, "db")
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "title": title})
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
		r.With(middleware.RequireMinRole("data_admin")).Post("/workspaces/{workspaceID}/knowledge/docs", a.UploadKnowledgeDoc)          // 上传知识文档（JSON body，内部使用）
		r.With(middleware.RequireMinRole("data_admin")).Post("/workspaces/{workspaceID}/knowledge/docs/upload", a.UploadKnowledgeDocFromFile) // 上传知识文档（multipart 文件）
		r.Get("/knowledge/docs/{docID}/download", a.DownloadKnowledgeDoc)      // 下载知识库原始文件

		// 任务和运行相关路由
		r.Get("/runs/{runID}/report", a.DownloadReport) // 下载运行报告
		r.Get("/tasks/{taskID}", a.TaskStatus)          // 获取任务状态

		// 数据管理路由
		r.With(middleware.RequireMinRole("normal_user")).Post("/data/upload", a.UploadData) // 文件上传导入
		r.With(middleware.RequireMinRole("data_admin")).Post("/data/execute", a.ExecuteDataSQL) // SQL 终端执行

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
