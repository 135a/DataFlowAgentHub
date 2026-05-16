package nl2sqlexec

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	nlv1 "github.com/dataflowagenthub/hub/internal/gen/nl2sql/v1"
	"github.com/jackc/pgx/v5/pgxpool"
)

// mockNL2SQLClient implements NL2SQLClient for testing.
type mockNL2SQLClient struct {
	generateFunc func(ctx context.Context, traceID, sessionID, userMessage, schemaJSON, dialect string) (*nlv1.GenerateSQLResponse, error)
}

func (m *mockNL2SQLClient) GenerateSQL(ctx context.Context, traceID, sessionID, userMessage, schemaJSON, dialect string) (*nlv1.GenerateSQLResponse, error) {
	return m.generateFunc(ctx, traceID, sessionID, userMessage, schemaJSON, dialect)
}

func TestExecute_GRPCError(t *testing.T) {
	wantErr := errors.New("gRPC unavailable")
	mock := &mockNL2SQLClient{
		generateFunc: func(ctx context.Context, traceID, sessionID, userMessage, schemaJSON, dialect string) (*nlv1.GenerateSQLResponse, error) {
			return nil, wantErr
		},
	}
	exec := NewExecutor(mock, 500, 30*time.Second)
	input := Input{
		TraceID:     "trace-1",
		SessionID:   "session-1",
		UserMessage: "show all users",
		SchemaJSON:  `{"tables":["users"]}`,
		Dialect:     "postgres",
	}

	result, err := exec.Execute(context.Background(), input, nil)
	if err == nil {
		t.Fatal("expected error from gRPC failure, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected error %v, got %v", wantErr, err)
	}
	if result != nil {
		t.Errorf("expected nil result on gRPC error, got %+v", result)
	}
}

func TestExecute_GenerationNotOk(t *testing.T) {
	mock := &mockNL2SQLClient{
		generateFunc: func(ctx context.Context, traceID, sessionID, userMessage, schemaJSON, dialect string) (*nlv1.GenerateSQLResponse, error) {
			return &nlv1.GenerateSQLResponse{
				Ok:           false,
				ErrorMessage: "cannot generate SQL for this request",
			}, nil
		},
	}
	exec := NewExecutor(mock, 500, 30*time.Second)
	input := Input{
		TraceID:     "trace-1",
		UserMessage: "do something impossible",
		SchemaJSON:  `{}`,
		Dialect:     "postgres",
	}

	result, err := exec.Execute(context.Background(), input, nil)
	if err == nil {
		t.Fatal("expected error from generation failure, got nil")
	}
	genErr, ok := err.(*GenerateError)
	if !ok {
		t.Fatalf("expected *GenerateError, got %T: %v", err, err)
	}
	if !strings.Contains(genErr.Message, "cannot generate SQL") {
		t.Errorf("expected error message to contain 'cannot generate SQL', got %q", genErr.Message)
	}
	if result != nil {
		t.Errorf("expected nil result on generation error, got %+v", result)
	}
}

func TestExecute_ReadOnlyCheckFailed(t *testing.T) {
	mock := &mockNL2SQLClient{
		generateFunc: func(ctx context.Context, traceID, sessionID, userMessage, schemaJSON, dialect string) (*nlv1.GenerateSQLResponse, error) {
			return &nlv1.GenerateSQLResponse{
				Ok:  true,
				Sql: "INSERT INTO users (name) VALUES ('hack')",
			}, nil
		},
	}
	exec := NewExecutor(mock, 500, 30*time.Second)
	input := Input{
		TraceID:     "trace-1",
		UserMessage: "add a user",
		SchemaJSON:  `{"tables":["users"]}`,
		Dialect:     "postgres",
	}

	// Passing nil pool is safe because sqlrun.IsReadOnlySQL fails before pool.Query.
	result, err := exec.Execute(context.Background(), input, (*pgxpool.Pool)(nil))
	if err == nil {
		t.Fatal("expected error from read-only check, got nil")
	}
	if result == nil {
		t.Fatal("expected non-nil result (containing SQL) on read-only check failure")
	}
	if result.SQL != "INSERT INTO users (name) VALUES ('hack')" {
		t.Errorf("expected result to contain the rejected SQL, got %q", result.SQL)
	}
	if result.Rows != nil {
		t.Errorf("expected nil rows on read-only check failure")
	}
}

func TestNewExecutor(t *testing.T) {
	mock := &mockNL2SQLClient{}
	exec := NewExecutor(mock, 200, 10*time.Second)
	if exec == nil {
		t.Fatal("NewExecutor returned nil")
	}
	if exec.maxRows != 200 {
		t.Errorf("expected maxRows 200, got %d", exec.maxRows)
	}
	if exec.timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", exec.timeout)
	}
}

func TestGenerateError_Error(t *testing.T) {
	e := &GenerateError{Message: "something went wrong"}
	if !strings.Contains(e.Error(), "something went wrong") {
		t.Errorf("expected Error() to contain the message, got %q", e.Error())
	}
}
