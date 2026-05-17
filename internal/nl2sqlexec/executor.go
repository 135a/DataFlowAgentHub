package nl2sqlexec

import (
	"context"
	"fmt"
	"strings"
	"time"

	nlv1 "github.com/dataflowagenthub/hub/internal/gen/nl2sql/v1"
	"github.com/dataflowagenthub/hub/internal/sqlrun"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NL2SQLClient 是从自然语言生成 SQL 的接口。现有的 *worker.NL2SQLClient 自动满足此接口。
type NL2SQLClient interface {
	GenerateSQL(ctx context.Context, traceID, sessionID, userMessage, schemaJSON, dialect string) (*nlv1.GenerateSQLResponse, error)
}

// Input 包含 NL2SQL 执行的参数
type Input struct {
	TraceID     string
	SessionID   string
	UserMessage string
	SchemaJSON  string
	Dialect     string
	Role        string // 当前用户角色，用于写操作权限检查
}

// Result 包含 NL2SQL 执行的输出
type Result struct {
	SQL            string
	Rows           []map[string]any
	RowsAffected   int64         // 写操作影响的行数
	IsWrite        bool          // 是否为写操作
	SelfCheckNotes string
}

// Executor 封装了 NL2SQL 执行管道：gRPC GenerateSQL → 分类 → 权限检查 → 执行 → 结果
type Executor struct {
	client  NL2SQLClient
	maxRows int32
	timeout time.Duration
}

// NewExecutor 创建具有给定依赖项的 Executor
func NewExecutor(client NL2SQLClient, maxRows int32, timeout time.Duration) *Executor {
	return &Executor{
		client:  client,
		maxRows: maxRows,
		timeout: timeout,
	}
}

// Execute 运行完整的 NL2SQL 管道并返回结果。
// SELECT/WITH → QueryRows（只读）；INSERT/UPDATE/DELETE/CREATE → ExecuteWrite（分类+权限+系统表检查）。
func (e *Executor) Execute(ctx context.Context, input Input, pool *pgxpool.Pool) (*Result, error) {
	gen, err := e.client.GenerateSQL(ctx, input.TraceID, input.SessionID, input.UserMessage, input.SchemaJSON, input.Dialect)
	if err != nil {
		return nil, err
	}
	if !gen.GetOk() {
		return nil, &GenerateError{Message: gen.GetErrorMessage()}
	}

	sql := strings.TrimSpace(gen.GetSql())
	sqlType := sqlrun.ClassifySQL(sql)

	switch sqlType {
	case sqlrun.SQLTypeSelect:
		// 只读路径
		rows, err := sqlrun.QueryRows(ctx, pool, sql, e.maxRows, e.timeout)
		if err != nil {
			return &Result{SQL: sql}, err
		}
		return &Result{
			SQL:            sql,
			Rows:           rows,
			SelfCheckNotes: gen.GetSelfCheckNotes(),
		}, nil

	default:
		// 写操作路径：权限检查 + 系统表拦截
		if err := sqlrun.IsAllowedForRole(sqlType, input.Role); err != nil {
			return &Result{SQL: sql}, err
		}
		if tbl, ok := sqlrun.CheckSystemTableInSQL(sql); ok {
			return &Result{SQL: sql}, fmt.Errorf("无权操作系统表 %s", tbl)
		}
		affected, err := sqlrun.ExecuteWrite(ctx, pool, sql, e.timeout)
		if err != nil {
			return &Result{SQL: sql}, err
		}
		return &Result{
			SQL:            sql,
			RowsAffected:   affected,
			IsWrite:        true,
			SelfCheckNotes: gen.GetSelfCheckNotes(),
		}, nil
	}
}

// GenerateError 表示 NL2SQL 生成步骤返回的错误
type GenerateError struct {
	Message string
}

func (e *GenerateError) Error() string {
	return fmt.Sprintf("nl2sql generation failed: %s", e.Message)
}
