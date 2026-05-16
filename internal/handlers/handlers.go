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

// App wires HTTP dependencies.
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

// Login accepts seed user credentials and returns a JWT.
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
	// local import would cycle — use golang.org/x/crypto/bcrypt in same package file bcrypt.go
	return compareBcrypt(pw, hash)
}

// Health reports readiness of Postgres and Redis.
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

	// Fetch run steps
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
	// Validate data_source_id if provided: must be a valid UUID owned by this workspace
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

func (a *App) PostMessage(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	sid := chi.URLParam(r, "sessionID")
	if ok, _ := ratelimit.Allow(r.Context(), a.Redis, "msg:"+c.UserID, 30, time.Minute); !ok {
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

	// Ensure session in workspace, also look up data_source_id
	var ws string
	var dsID, dsKind *string
	err := a.DB.QueryRow(ctx, `SELECT workspace_id::text FROM sessions WHERE id = $1::uuid`, sid).Scan(&ws)
	if err != nil || ws != c.WorkspaceID {
		errJSON(w, http.StatusNotFound, "session not found")
		return
	}
	// Check if session has an associated data source
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

	low := strings.ToLower(body.Text)
	if strings.Contains(low, "export") {
		rid := uuid.NewString()
		_, _ = a.DB.Exec(ctx, `INSERT INTO runs (id, session_id, status, pending_reason) VALUES ($1::uuid, $2::uuid, 'awaiting_approval', $3)`,
			rid, sid, "export_requested")
		_, _ = a.DB.Exec(ctx, `INSERT INTO approval_tasks (workspace_id, run_id, action_type, status, payload)
			VALUES ($1::uuid, $2::uuid, 'export', 'pending', $3::jsonb)`,
			c.WorkspaceID, rid, `{"reason":"export keyword"}`)
		a.Bus.Publish(sid, ssebus.Event{Type: "approval_required", Data: map[string]string{"run_id": rid}})
		JSON(w, http.StatusAccepted, map[string]any{"run_id": rid, "status": "awaiting_approval"})
		return
	}

	// Resolve schema JSON for NL2SQL context
	sourceKey := "hub"
	var discoverPool *pgxpool.Pool = a.DB
	if dsID != nil {
		sourceKey = *dsID
		// Fetch data source credentials
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

	isComplex := strings.Contains(low, "分析") || strings.Contains(low, "报告") ||
		strings.Contains(low, "analyze") || strings.Contains(low, "report")
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
		// SQL execution error — result contains the SQL for debugging
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

// SSE streams run events for a session (MVP in-memory bus).
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

	// Log drop count on stream end for observability
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

func (a *App) ListApprovals(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	_, _ = a.DB.Exec(r.Context(), `
		UPDATE approval_tasks SET status = 'expired', decided_at = now()
		WHERE workspace_id = $1::uuid AND status = 'pending'
		  AND created_at + ($2 * interval '1 second') < now()`,
		c.WorkspaceID, int64(a.Cfg.ApprovalTTL.Seconds()))
	rows, err := a.DB.Query(r.Context(), `
		SELECT a.id::text, a.status, a.action_type, a.created_at::text, a.run_id::text
		FROM approval_tasks a
		WHERE a.workspace_id = $1::uuid AND a.status = 'pending'
		ORDER BY a.created_at ASC`, c.WorkspaceID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "db")
		return
	}
	defer rows.Close()
	var items []map[string]any
	for rows.Next() {
		var id, st, act, created, runID string
		_ = rows.Scan(&id, &st, &act, &created, &runID)
		items = append(items, map[string]any{"id": id, "status": st, "action_type": act, "created_at": created, "run_id": runID})
	}
	JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) DecideApproval(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var body struct {
		Decision string `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	dec := strings.ToLower(strings.TrimSpace(body.Decision))
	if dec != "approve" && dec != "reject" {
		errJSON(w, http.StatusBadRequest, "decision must be approve or reject")
		return
	}
	var ws, runID string
	err := a.DB.QueryRow(r.Context(), `SELECT workspace_id::text, run_id::text FROM approval_tasks WHERE id = $1::uuid`, id).Scan(&ws, &runID)
	if err != nil || ws != c.WorkspaceID {
		errJSON(w, http.StatusNotFound, "not found")
		return
	}
	st := "approved"
	runSt := "completed"
	if dec == "reject" {
		st = "rejected"
		runSt = "cancelled"
	}
	var decided *string
	if _, perr := uuid.Parse(c.UserID); perr == nil {
		s := c.UserID
		decided = &s
	}
	_, err = a.DB.Exec(r.Context(), `
		UPDATE approval_tasks SET status = $2, decided_at = now(), decided_by = $3::uuid WHERE id = $1::uuid AND status = 'pending'`,
		id, st, decided)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "db")
		return
	}
	_, _ = a.DB.Exec(r.Context(), `UPDATE runs SET status = $2, updated_at = now() WHERE id = $1::uuid`, runID, runSt)
	var sid string
	_ = a.DB.QueryRow(r.Context(), `SELECT session_id::text FROM runs WHERE id = $1::uuid`, runID).Scan(&sid)
	a.Bus.Publish(sid, ssebus.Event{Type: "approval_" + st, Data: map[string]string{"approval_id": id}})

	auditPayload, _ := json.Marshal(map[string]string{"approval_id": id, "decision": dec, "run_id": runID})
	if aid, perr := uuid.Parse(c.UserID); perr == nil {
		_, _ = a.DB.Exec(r.Context(), `
			INSERT INTO audit_events (workspace_id, actor_user_id, action, payload)
			VALUES ($1::uuid, $2::uuid, 'approval_decided', $3::jsonb)`,
			c.WorkspaceID, aid.String(), auditPayload)
	} else {
		_, _ = a.DB.Exec(r.Context(), `
			INSERT INTO audit_events (workspace_id, actor_user_id, action, payload)
			VALUES ($1::uuid, NULL, 'approval_decided', $2::jsonb)`,
			c.WorkspaceID, auditPayload)
	}

	JSON(w, http.StatusOK, map[string]string{"status": st})
}

// InternalNL2SQL is called by the Python orchestrator's nl2sql_node to execute NL2SQL
// via Go's secure boundary (gRPC → sqlrun). It receives user_message, schema_json, and trace_id,
// calls the Python worker's GenerateSQL RPC, executes the returned SQL (read-only), and returns results.
// InternalNL2SQL is called by the Python orchestrator's nl2sql_node.
// Authentication is handled by InternalHMACAuth middleware.
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

// Routes builds the chi router.
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
		r.With(middleware.RequireMinRole("operator")).Get("/approvals", a.ListApprovals)
		r.With(middleware.RequireMinRole("operator")).Post("/approvals/{id}/decide", a.DecideApproval)

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
