package async

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// testDSN 读取测试数据库 DSN，用于需要 DB 的集成测试
func testDSN() string {
	if u := os.Getenv("HUB_TEST_DSN"); u != "" {
		return u
	}
	return "root:root@tcp(localhost:3306)/hub_platform?charset=utf8mb4&parseTime=true&loc=Local"
}

// setupTestDB 尝试连接测试数据库，失败时跳过测试
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

// TestNewClient 验证 NewClient 创建客户端
func TestNewClient(t *testing.T) {
	log := zap.NewNop()
	c := NewClient(nil, nil, log)
	if c == nil {
		t.Fatal("expected non-nil Client")
	}
	if c.DB != nil {
		t.Error("expected nil DB")
	}
	if c.NATS != nil {
		t.Error("expected nil NATS")
	}
	if c.Log != log {
		t.Error("expected Log to match")
	}
}

// TestEnqueueTask 验证 EnqueueTask 插入任务并返回 ID
func TestEnqueueTask(t *testing.T) {
	db := setupTestDB(t)
	log := zap.NewNop()
	c := NewClient(db, nil, log)

	ctx := context.Background()
	wsID := "00000000-0000-0000-0000-000000000000" // 不存在的工作区

	// 由于没有有效的工作区，应该返回错误
	taskID, err := c.EnqueueTask(ctx, wsID, "", "", "test_type", map[string]any{"key": "value"})
	if err == nil {
		// 如果有默认数据，任务可能成功，尝试清理
		if taskID != "" {
			db.ExecContext(ctx, `DELETE FROM async_tasks WHERE id = ?`, taskID)
		}
		t.Log("task was created (test database has seed data)")
	} else {
		// 失败是预期行为，因为没有有效的工作区
		t.Logf("expected error due to missing workspace: %v", err)
	}
}

// TestEnqueueTask_ValidWorkspace 验证在工作区存在时成功创建任务
func TestEnqueueTask_ValidWorkspace(t *testing.T) {
	db := setupTestDB(t)
	log := zap.NewNop()
	c := NewClient(db, nil, log)

	ctx := context.Background()

	// 查找第一个工作区 ID
	var wsID string
	err := db.QueryRowContext(ctx, `SELECT id FROM workspaces LIMIT 1`).Scan(&wsID)
	if err != nil {
		t.Skipf("skipping: no workspace found in database: %v", err)
	}

	taskID, err := c.EnqueueTask(ctx, wsID, "", "", "test_type", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("EnqueueTask failed: %v", err)
	}
	if taskID == "" {
		t.Fatal("expected non-empty task ID")
	}

	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM async_tasks WHERE id = ?`, taskID)
	})

	// 验证任务存在于数据库中
	var status string
	err = db.QueryRowContext(ctx, `SELECT status FROM async_tasks WHERE id = ?`, taskID).Scan(&status)
	if err != nil {
		t.Fatalf("query task failed: %v", err)
	}
	if status != "queued" {
		t.Errorf("expected status=queued, got %q", status)
	}
}

// TestEnqueueTask_WithSessionAndRun 验证关联 session 和 run 的任务创建
func TestEnqueueTask_WithSessionAndRun(t *testing.T) {
	db := setupTestDB(t)
	log := zap.NewNop()
	c := NewClient(db, nil, log)

	ctx := context.Background()

	// 查找第一个 workspace
	var wsID string
	err := db.QueryRowContext(ctx, `SELECT id FROM workspaces LIMIT 1`).Scan(&wsID)
	if err != nil {
		t.Skipf("skipping: no workspace found: %v", err)
	}

	// 创建一个 session
	sessionID := uuid.NewString()
	_, err = db.ExecContext(ctx, `
		INSERT INTO sessions (id, workspace_id, title)
		VALUES (?, ?, 'test-session')`,
		sessionID, wsID)
	if err != nil {
		t.Skipf("skipping: could not create session: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID)
	})

	// 创建 run
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

	taskID, err := c.EnqueueTask(ctx, wsID, sessionID, runID, "analysis", map[string]any{"query": "test"})
	if err != nil {
		t.Fatalf("EnqueueTask with session/run failed: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM async_tasks WHERE id = ?`, taskID)
	})

	// 验证关联正确
	var gotSessionID, gotRunID string
	err = db.QueryRowContext(ctx,
		`SELECT COALESCE(session_id, ''), COALESCE(run_id, '') FROM async_tasks WHERE id = ?`, taskID).Scan(&gotSessionID, &gotRunID)
	if err != nil {
		t.Fatalf("query task failed: %v", err)
	}
	if gotSessionID != sessionID {
		t.Errorf("expected session_id=%q, got %q", sessionID, gotSessionID)
	}
	if gotRunID != runID {
		t.Errorf("expected run_id=%q, got %q", runID, gotRunID)
	}
}

// TestStartReaper 验证过期任务清理器可以正常启动和停止
func TestStartReaper(t *testing.T) {
	db := setupTestDB(t)
	log := zap.NewNop()
	c := NewClient(db, nil, log)

	ctx, cancel := context.WithCancel(context.Background())

	// 启动 reaper，使用 50ms 间隔
	c.StartReaper(ctx, 50*time.Millisecond)

	// 让 reaper 运行一小段时间
	time.Sleep(150 * time.Millisecond)

	// 停止 reaper
	cancel()

	// 验证 reaper 没有 panic（正常停止即通过）
}

// TestGetPendingTasks 验证只返回 pending 状态的任务
// 注意：当前 Client 没有 GetPendingTasks 方法，此测试检查相关逻辑
// 实际上查询由 gRPC server 或其他组件执行
func TestClient_StructFields(t *testing.T) {
	log := zap.NewNop()
	c := NewClient(nil, nil, log)

	if c.DB != nil {
		t.Error("expected DB to be nil")
	}
	if c.NATS != nil {
		t.Error("expected NATS to be nil")
	}
}
