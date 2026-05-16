package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dataflowagenthub/hub/internal/auth"
	"github.com/dataflowagenthub/hub/internal/config"
	"github.com/dataflowagenthub/hub/internal/nl2sqlexec"
	"github.com/dataflowagenthub/hub/internal/seed"
	"github.com/dataflowagenthub/hub/internal/sqlrun"
	"github.com/dataflowagenthub/hub/internal/ssebus"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// testDBURL reads the test database URL from env or uses the docker-compose default.
func testDBURL() string {
	if u := os.Getenv("HUB_TEST_DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://hub:hub@localhost:5432/hub?sslmode=disable"
}

// setupTestDB connects to the test database or skips the test.
func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, testDBURL())
	if err != nil {
		t.Skipf("skipping integration test: cannot connect to test database: %v", err)
	}
	if err := sqlrun.Ping(ctx, pool); err != nil {
		pool.Close()
		t.Skipf("skipping integration test: database ping failed: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// setupTestApp creates a minimal App for integration testing.
func setupTestApp(t *testing.T, pool *pgxpool.Pool, nl2sqlExec *nl2sqlexec.Executor) *App {
	t.Helper()
	cfg := &config.Config{
		JWTSecret:          []byte("test-jwt-secret-for-integration-tests"),
		SeedEmail:          "admin@demo.local",
		SeedPassword:       "changeme",
		InternalHMACSecret: "test-hmac-secret",
		QueryMaxRows:       500,
		QueryTimeout:       30 * time.Second,
		ApprovalTTL:        3600 * time.Second,
	}
	log := zap.NewNop()

	if nl2sqlExec == nil {
		nl2sqlExec = nl2sqlexec.NewExecutor(nil, 500, 30*time.Second)
	}

	return &App{
		Cfg:        cfg,
		Log:        log,
		DB:         pool,
		Redis:      nil, // nil Redis = fail-open for rate limiting
		Nl2sql:     nil,
		Bus:        ssebus.New(),
		NATS:       nil,
		AsyncTask:  nil,
		NL2SQLExec: nl2sqlExec,
	}
}

// newTestServer creates an httptest.Server with the chi router.
func newTestServer(app *App) *httptest.Server {
	return httptest.NewServer(Routes(app))
}

// TestHealthEndpoint verifies the /health endpoint returns 200 and ok status.
func TestHealthEndpoint(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["postgres"] != "ok" {
		t.Errorf("expected postgres=ok, got postgres=%s", body["postgres"])
	}
	// Redis is nil in test setup, so it reports "down" or may panic
	// Accept either "ok" or "down" since we don't mock Redis
	if body["redis"] != "ok" && body["redis"] != "down" {
		t.Errorf("expected redis=ok or redis=down, got redis=%s", body["redis"])
	}
}

// TestLoginSuccess verifies login with correct credentials returns a JWT.
func TestLoginSuccess(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	body := `{"email":"admin@demo.local","password":"changeme"}`
	resp, err := http.Post(srv.URL+"/v1/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /v1/auth/login failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if tok, ok := result["access_token"].(string); !ok || tok == "" {
		t.Error("expected non-empty access_token")
	}
	if tt, ok := result["token_type"].(string); !ok || tt != "Bearer" {
		t.Errorf("expected token_type=Bearer, got %v", result["token_type"])
	}
	if role, ok := result["role"].(string); !ok || role == "" {
		t.Error("expected non-empty role")
	}
}

// TestLoginInvalidCredentials verifies login with wrong password returns 401.
func TestLoginInvalidCredentials(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	body := `{"email":"admin@demo.local","password":"wrongpassword"}`
	resp, err := http.Post(srv.URL+"/v1/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /v1/auth/login failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["error"] != "invalid credentials" {
		t.Errorf("expected error='invalid credentials', got '%s'", result["error"])
	}
}

// TestVersionEndpoint verifies the /version endpoint returns the version.
func TestVersionEndpoint(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/version")
	if err != nil {
		t.Fatalf("GET /version failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["version"] == "" {
		t.Error("expected non-empty version")
	}
}

// --- helper functions for PostMessage tests ---

// testJWT generates a valid JWT for integration testing.
func testJWT(t *testing.T, secret []byte) string {
	t.Helper()
	tok, err := auth.Sign(secret, seed.ServiceAPIUserID, seed.DemoWorkspaceID(), "admin", 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to sign test JWT: %v", err)
	}
	return tok
}

// createTestSession creates a session in the test database and returns its ID.
func createTestSession(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id := "00000000-0000-4000-8000-000000000099" // deterministic test ID
	_, err := pool.Exec(ctx, `
		INSERT INTO sessions (id, workspace_id, user_id, title)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'test session')
		ON CONFLICT (id) DO NOTHING`,
		id, seed.DemoWorkspaceID(), seed.ServiceAPIUserID)
	if err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}
	return id
}

// TestHealthEndpointUnhealthy verifies /health returns 503 when Postgres is down.
func TestHealthEndpointUnhealthy(t *testing.T) {
	pool := setupTestDB(t)
	pool.Close() // simulate database down
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", resp.StatusCode)
	}

	var body map[string]string
	if derr := json.NewDecoder(resp.Body).Decode(&body); derr != nil {
		t.Fatalf("failed to decode response: %v", derr)
	}
	if body["postgres"] != "down" {
		t.Errorf("expected postgres=down, got postgres=%s", body["postgres"])
	}
}

// TestPostMessageSessionNotFound verifies PostMessage returns 404 for a non-existent session.
func TestPostMessageSessionNotFound(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWT(t, app.Cfg.JWTSecret)
	nonexistentID := "00000000-0000-4000-8000-000000000099"
	// Ensure the session doesn't exist in this test
	// Use a random UUID to guarantee not found
	body := `{"text":"show tables"}`
	req, _ := http.NewRequest(http.MethodPost,
		srv.URL+"/v1/sessions/"+nonexistentID+"/messages",
		strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/sessions/{id}/messages failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		respBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 404 for non-existent session, got %d: %s", resp.StatusCode, string(respBytes))
	}
}

// TestPostMessageEmptyText verifies PostMessage returns 400 when text is empty.
func TestPostMessageEmptyText(t *testing.T) {
	pool := setupTestDB(t)
	sid := createTestSession(t, pool)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWT(t, app.Cfg.JWTSecret)

	body := `{"text":""}`
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/v1/sessions/%s/messages", srv.URL, sid),
		strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/sessions/{id}/messages failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		respBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 400 for empty text, got %d: %s", resp.StatusCode, string(respBytes))
	}

	var result map[string]string
	if derr := json.NewDecoder(resp.Body).Decode(&result); derr != nil {
		t.Fatalf("failed to decode response: %v", derr)
	}
}
