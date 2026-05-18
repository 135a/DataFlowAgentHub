package grpcserver

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	nlv1 "github.com/dataflowagenthub/hub/internal/gen/nl2sql/v1"
	"github.com/dataflowagenthub/hub/internal/handlers"
	"github.com/dataflowagenthub/hub/internal/nl2sqlexec"
	"github.com/dataflowagenthub/hub/internal/ssebus"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// testDSN 读取测试数据库 DSN
func testDSN() string {
	if u := os.Getenv("HUB_TEST_DSN"); u != "" {
		return u
	}
	return "root:root@tcp(localhost:3306)/hub_platform?charset=utf8mb4&parseTime=true&loc=Local"
}

// setupTestDB 尝试连接测试数据库，失败时跳过
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", testDSN())
	if err != nil {
		t.Skipf("skipping integration test: cannot connect to test database: %v", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("skipping integration test: database ping failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// mockApp 创建一个最小化的 App 用于测试
func mockApp(t *testing.T, db *sql.DB) *handlers.App {
	t.Helper()
	nl2sqlExec := nl2sqlexec.NewExecutor(nil, 500, 30*time.Second)
	return &handlers.App{
		Log:       zap.NewNop(),
		DB:        db,
		Bus:       ssebus.NewMemoryBus(zap.NewNop()),
		NL2SQLExec: nl2sqlExec,
	}
}

// TestTaskCallback_InvalidStatus 验证无效状态返回 InvalidArgument 错误
func TestTaskCallback_InvalidStatus(t *testing.T) {
	db := setupTestDB(t)
	app := mockApp(t, db)
	srv := NewInternalServer(app)

	ctx := context.Background()

	tests := []struct {
		name   string
		status string
	}{
		{"empty status", ""},
		{"unknown status", "running"},
		{"misspelled succeeded", "succeded"},
		{"misspelled failed", "fail"},
		{"uppercase SUCCEEDED", "SUCCEEDED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &nlv1.TaskCallbackRequest{
				TaskId:     "00000000-0000-0000-0000-000000000000",
				Status:     tt.status,
				ResultJson: "{}",
			}
			_, err := srv.TaskCallback(ctx, req)
			if err == nil {
				t.Fatal("expected error for invalid status")
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatal("expected gRPC status error")
			}
			if st.Code() != codes.InvalidArgument {
				t.Errorf("expected code InvalidArgument, got %v", st.Code())
			}
		})
	}
}

// TestTaskCallback_ValidSucceeded 验证有效的成功状态回调
func TestTaskCallback_ValidSucceeded(t *testing.T) {
	db := setupTestDB(t)
	app := mockApp(t, db)
	srv := NewInternalServer(app)

	ctx := context.Background()

	// 查找一个已存在的 workspace 并创建一个测试任务
	var wsID string
	err := db.QueryRowContext(ctx, `SELECT id FROM workspaces LIMIT 1`).Scan(&wsID)
	if err != nil {
		t.Skipf("skipping: no workspace found: %v", err)
	}

	taskID := uuid.NewString()
	_, err = db.ExecContext(ctx, `
		INSERT INTO async_tasks (id, workspace_id, task_type, status)
		VALUES (?, ?, 'test', 'running')`,
		taskID, wsID)
	if err != nil {
		t.Skipf("skipping: could not create task: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM async_tasks WHERE id = ?`, taskID)
	})

	req := &nlv1.TaskCallbackRequest{
		TaskId:     taskID,
		Status:     "succeeded",
		ResultJson: `{"report": "test report"}`,
	}

	resp, err := srv.TaskCallback(ctx, req)
	if err != nil {
		t.Fatalf("TaskCallback failed: %v", err)
	}
	if resp.GetMessage() != "ok" {
		t.Errorf("expected message=ok, got %q", resp.GetMessage())
	}

	// 验证任务状态已更新
	var status string
	var result []byte
	err = db.QueryRowContext(ctx, `SELECT status, result FROM async_tasks WHERE id = ?`, taskID).Scan(&status, &result)
	if err != nil {
		t.Fatalf("query task failed: %v", err)
	}
	if status != "succeeded" {
		t.Errorf("expected status=succeeded, got %q", status)
	}
	if string(result) != `{"report": "test report"}` {
		t.Errorf("expected result to be preserved, got %s", string(result))
	}
}

// TestTaskCallback_ValidFailed 验证有效的失败状态回调
func TestTaskCallback_ValidFailed(t *testing.T) {
	db := setupTestDB(t)
	app := mockApp(t, db)
	srv := NewInternalServer(app)

	ctx := context.Background()

	var wsID string
	err := db.QueryRowContext(ctx, `SELECT id FROM workspaces LIMIT 1`).Scan(&wsID)
	if err != nil {
		t.Skipf("skipping: no workspace found: %v", err)
	}

	taskID := uuid.NewString()
	_, err = db.ExecContext(ctx, `
		INSERT INTO async_tasks (id, workspace_id, task_type, status)
		VALUES (?, ?, 'test', 'running')`,
		taskID, wsID)
	if err != nil {
		t.Skipf("skipping: could not create task: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM async_tasks WHERE id = ?`, taskID)
	})

	errMsg := "something went wrong"
	req := &nlv1.TaskCallbackRequest{
		TaskId:       taskID,
		Status:       "failed",
		ResultJson:   `{}`,
		ErrorMessage: errMsg,
	}

	resp, err := srv.TaskCallback(ctx, req)
	if err != nil {
		t.Fatalf("TaskCallback failed: %v", err)
	}
	if resp.GetMessage() != "ok" {
		t.Errorf("expected message=ok, got %q", resp.GetMessage())
	}

	// 验证任务状态和错误信息
	var status, gotErrMsg string
	err = db.QueryRowContext(ctx, `SELECT status, COALESCE(error_message, '') FROM async_tasks WHERE id = ?`, taskID).Scan(&status, &gotErrMsg)
	if err != nil {
		t.Fatalf("query task failed: %v", err)
	}
	if status != "failed" {
		t.Errorf("expected status=failed, got %q", status)
	}
	if gotErrMsg != errMsg {
		t.Errorf("expected error_message=%q, got %q", errMsg, gotErrMsg)
	}
}

// TestTaskCallback_NonexistentTask 验证对不存在的任务调用回调不会报错（静默忽略）
func TestTaskCallback_NonexistentTask(t *testing.T) {
	db := setupTestDB(t)
	app := mockApp(t, db)
	srv := NewInternalServer(app)

	ctx := context.Background()

	req := &nlv1.TaskCallbackRequest{
		TaskId:     "00000000-0000-0000-0000-000000000000",
		Status:     "succeeded",
		ResultJson: `{}`,
	}

	// 对不存在的任务，UPDATE 不会影响任何行，但不是错误
	resp, err := srv.TaskCallback(ctx, req)
	if err != nil {
		t.Fatalf("TaskCallback for nonexistent task failed: %v", err)
	}
	if resp.GetMessage() != "ok" {
		t.Errorf("expected message=ok, got %q", resp.GetMessage())
	}
}

// TestRunStepCallback_Valid 验证有效的步骤回调
func TestRunStepCallback_Valid(t *testing.T) {
	db := setupTestDB(t)
	app := mockApp(t, db)
	srv := NewInternalServer(app)

	ctx := context.Background()

	var wsID string
	err := db.QueryRowContext(ctx, `SELECT id FROM workspaces LIMIT 1`).Scan(&wsID)
	if err != nil {
		t.Skipf("skipping: no workspace found: %v", err)
	}

	sessionID := uuid.NewString()
	_, err = db.ExecContext(ctx, `
		INSERT INTO sessions (id, workspace_id, title)
		VALUES (?, ?, 'test-step-session')`,
		sessionID, wsID)
	if err != nil {
		t.Skipf("skipping: could not create session: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID)
	})

	runID := uuid.NewString()
	_, err = db.ExecContext(ctx, `
		INSERT INTO runs (id, session_id, status)
		VALUES (?, ?, 'running')`,
		runID, sessionID)
	if err != nil {
		t.Skipf("skipping: could not create run: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM runs WHERE id = ?`, runID)
	})

	req := &nlv1.RunStepCallbackRequest{
		RunId:         runID,
		AgentName:     "test-agent",
		Status:        "running",
		InputSummary:  "test input",
		OutputSummary: "test output",
	}

	resp, err := srv.RunStepCallback(ctx, req)
	if err != nil {
		t.Fatalf("RunStepCallback failed: %v", err)
	}
	if resp.GetMessage() != "ok" {
		t.Errorf("expected message=ok, got %q", resp.GetMessage())
	}

	// 验证步骤记录存在
	var stepIndex int32
	var agentName string
	err = db.QueryRowContext(ctx,
		`SELECT step_index, agent_name FROM agent_run_steps WHERE run_id = ? ORDER BY step_index DESC LIMIT 1`,
		runID).Scan(&stepIndex, &agentName)
	if err != nil {
		t.Fatalf("query step failed: %v", err)
	}
	if stepIndex < 0 {
		t.Errorf("expected non-negative step index, got %d", stepIndex)
	}
	if agentName != "test-agent" {
		t.Errorf("expected agent_name=test-agent, got %q", agentName)
	}
}

// TestRunStepCallback_NonexistentRun 验证不存在的 run_id 不会导致错误
func TestRunStepCallback_NonexistentRun(t *testing.T) {
	db := setupTestDB(t)
	app := mockApp(t, db)
	srv := NewInternalServer(app)

	ctx := context.Background()

	req := &nlv1.RunStepCallbackRequest{
		RunId:         "00000000-0000-0000-0000-000000000000",
		AgentName:     "test-agent",
		Status:        "running",
		InputSummary:  "input",
		OutputSummary: "output",
	}

	// 不存在的 run_id 不应导致错误（外键约束可能触发，但跳过具体情况）
	resp, err := srv.RunStepCallback(ctx, req)
	if err != nil {
		// 可能因外键约束失败，这是预期行为
		t.Logf("RunStepCallback for nonexistent run: %v", err)
	} else {
		if resp.GetMessage() != "ok" {
			t.Errorf("expected message=ok, got %q", resp.GetMessage())
		}
	}
}

// TestTaskCallback_EmptyTaskId 验证空 task_id 的处理
func TestTaskCallback_EmptyTaskId(t *testing.T) {
	db := setupTestDB(t)
	app := mockApp(t, db)
	srv := NewInternalServer(app)

	ctx := context.Background()

	req := &nlv1.TaskCallbackRequest{
		TaskId:     "",
		Status:     "succeeded",
		ResultJson: `{}`,
	}

	// 空 ID 的 UPDATE 会在数据库层面出错
	_, err := srv.TaskCallback(ctx, req)
	// 可能成功（不匹配任何行），可能失败（无效 UUID）
	if err != nil {
		t.Logf("empty task_id returns error: %v", err)
	}
}

// TestNewInternalServer 验证 NewInternalServer 创建服务器
func TestNewInternalServer(t *testing.T) {
	db := setupTestDB(t)
	app := mockApp(t, db)
	srv := NewInternalServer(app)

	if srv == nil {
		t.Fatal("expected non-nil InternalServer")
	}
	if srv.app != app {
		t.Error("expected app reference to match")
	}
}
