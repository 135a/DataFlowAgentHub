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

// NL2SQLClient is the interface for generating SQL from natural language.
// The existing *worker.NL2SQLClient satisfies this interface automatically.
type NL2SQLClient interface {
	GenerateSQL(ctx context.Context, traceID, sessionID, userMessage, schemaJSON, dialect string) (*nlv1.GenerateSQLResponse, error)
}

// Input contains the parameters for an NL2SQL execution.
type Input struct {
	TraceID     string
	SessionID   string
	UserMessage string
	SchemaJSON  string
	Dialect     string
}

// Result contains the output of an NL2SQL execution.
type Result struct {
	SQL            string
	Rows           []map[string]any
	SelfCheckNotes string
}

// Executor encapsulates the NL2SQL execution pipeline:
// gRPC GenerateSQL → readonly check → SQL execution → result.
type Executor struct {
	client  NL2SQLClient
	maxRows int32
	timeout time.Duration
}

// NewExecutor creates a new Executor with the given dependencies.
func NewExecutor(client NL2SQLClient, maxRows int32, timeout time.Duration) *Executor {
	return &Executor{
		client:  client,
		maxRows: maxRows,
		timeout: timeout,
	}
}

// Execute runs the full NL2SQL pipeline and returns the result.
// On gRPC or generation error, Result is nil.
// On SQL execution error, Result contains the generated SQL for debugging.
func (e *Executor) Execute(ctx context.Context, input Input, pool *pgxpool.Pool) (*Result, error) {
	gen, err := e.client.GenerateSQL(ctx, input.TraceID, input.SessionID, input.UserMessage, input.SchemaJSON, input.Dialect)
	if err != nil {
		return nil, err
	}
	if !gen.GetOk() {
		return nil, &GenerateError{Message: gen.GetErrorMessage()}
	}

	sql := strings.TrimSpace(gen.GetSql())
	rows, err := sqlrun.QueryRows(ctx, pool, sql, e.maxRows, e.timeout)
	if err != nil {
		return &Result{SQL: sql}, err
	}

	return &Result{
		SQL:            sql,
		Rows:           rows,
		SelfCheckNotes: gen.GetSelfCheckNotes(),
	}, nil
}

// GenerateError represents an error returned by the NL2SQL generation step.
type GenerateError struct {
	Message string
}

func (e *GenerateError) Error() string {
	return fmt.Sprintf("nl2sql generation failed: %s", e.Message)
}
