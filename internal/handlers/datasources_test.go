package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dataflowagenthub/hub/internal/auth"
	"github.com/dataflowagenthub/hub/internal/seed"
)

// testJWTForRole 生成指定角色的测试 JWT
func testJWTForRole(t *testing.T, secret []byte, role string) string {
	t.Helper()
	tok, err := auth.Sign(secret, "00000000-0000-4000-8000-000000000099", seed.DemoWorkspaceID(), role, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to sign test JWT for role %s: %v", role, err)
	}
	return tok
}

// authedRequest 创建带 JWT 认证的 HTTP 请求
func authedRequest(t *testing.T, method, url, body, token string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// decodeJSON 解析 JSON 响应体
func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return result
}

// --- DataSource Tests ---

func TestListDataSources(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWT(t, app.Cfg.JWTSecret)
	req := authedRequest(t, http.MethodGet, srv.URL+"/v1/data-sources", "", tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/data-sources failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	result := decodeJSON[map[string]any](t, resp)
	if _, ok := result["items"]; !ok {
		t.Error("expected 'items' key in response")
	}
}

func TestCreateDataSourceMissingFields(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "operator")

	tests := []struct {
		name string
		body string
	}{
		{"empty body", `{}`},
		{"missing host", `{"name":"test","kind":"postgres","port":5432,"database":"db","username":"u","password":"p"}`},
		{"invalid kind", `{"name":"test","kind":"mysql","host":"h","port":3306,"database":"db","username":"u","password":"p"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := authedRequest(t, http.MethodPost, srv.URL+"/v1/data-sources", tc.body, tok)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("POST /v1/data-sources failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected 400, got %d for %s", resp.StatusCode, tc.name)
			}
		})
	}
}

func TestCreateDataSourceViewerForbidden(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "viewer")
	body := `{"name":"test-ds","kind":"postgres","host":"localhost","port":5432,"database":"test","username":"u","password":"p"}`

	req := authedRequest(t, http.MethodPost, srv.URL+"/v1/data-sources", body, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/data-sources failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for viewer, got %d", resp.StatusCode)
	}
}

func TestDeleteDataSourceNotFound(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "admin")
	req := authedRequest(t, http.MethodDelete, srv.URL+"/v1/data-sources/00000000-0000-4000-8000-000000000001", "", tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /v1/data-sources/{id} failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateDataSourceNotFound(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "admin")
	body := `{"name":"upd","kind":"postgres","host":"h","port":5432,"database":"d","username":"u","password":"p"}`

	req := authedRequest(t, http.MethodPut, srv.URL+"/v1/data-sources/00000000-0000-4000-8000-000000000001", body, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /v1/data-sources/{id} failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- User Management Tests ---

func TestListUsersAsAdmin(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "admin")
	req := authedRequest(t, http.MethodGet, srv.URL+"/v1/users", "", tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	result := decodeJSON[map[string]any](t, resp)
	users, ok := result["users"].([]any)
	if !ok {
		t.Fatal("expected 'users' array in response")
	}
	if len(users) == 0 {
		t.Error("expected at least one user (admin seed)")
	}
}

func TestListUsersAsOperatorForbidden(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "operator")
	req := authedRequest(t, http.MethodGet, srv.URL+"/v1/users", "", tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for operator, got %d", resp.StatusCode)
	}
}

func TestRegisterUserMissingFields(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "admin")

	tests := []struct {
		name string
		body string
	}{
		{"empty", `{}`},
		{"missing phone", `{"name":"t","password":"p","role":"viewer"}`},
		{"invalid role", `{"name":"t","phone":"123","password":"p","role":"superadmin"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := authedRequest(t, http.MethodPost, srv.URL+"/v1/auth/register", tc.body, tok)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("POST /v1/auth/register failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected 400 for %s, got %d", tc.name, resp.StatusCode)
			}
		})
	}
}

func TestRegisterAndDeleteUserLifecycle(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	adminTok := testJWTForRole(t, app.Cfg.JWTSecret, "admin")

	// 1. 创建新用户
	createBody := fmt.Sprintf(`{"name":"test-lifecycle","phone":"%d","password":"secret123","role":"viewer"}`, time.Now().UnixNano())
	req := authedRequest(t, http.MethodPost, srv.URL+"/v1/auth/register", createBody, adminTok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	result := decodeJSON[map[string]any](t, resp)
	userID, ok := result["id"].(string)
	if !ok || userID == "" {
		t.Fatal("expected user id in response")
	}

	// 2. 删除用户
	req = authedRequest(t, http.MethodDelete, fmt.Sprintf("%s/v1/users/%s", srv.URL, userID), "", adminTok)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete user failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on delete, got %d", resp.StatusCode)
	}

	// 3. 再次删除应返回 404
	req = authedRequest(t, http.MethodDelete, fmt.Sprintf("%s/v1/users/%s", srv.URL, userID), "", adminTok)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("double delete user failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 on double delete, got %d", resp.StatusCode)
	}
}

func TestRegisterUserAsViewerForbidden(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "viewer")
	body := `{"name":"test","phone":"999","password":"p","role":"viewer"}`

	req := authedRequest(t, http.MethodPost, srv.URL+"/v1/auth/register", body, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/auth/register failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestChangeUserRoleNotFound(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "admin")
	body := `{"role":"operator"}`

	req := authedRequest(t, http.MethodPut, srv.URL+"/v1/users/00000000-0000-4000-8000-000000000001/role", body, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /v1/users/{id}/role failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Knowledge Doc Tests ---

func TestListKnowledgeDocsAsOperator(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "operator")
	wsID := seed.DemoWorkspaceID()

	req := authedRequest(t, http.MethodGet, fmt.Sprintf("%s/v1/workspaces/%s/knowledge/docs", srv.URL, wsID), "", tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/workspaces/{id}/knowledge/docs failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestListKnowledgeDocsAsViewerForbidden(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "viewer")
	wsID := seed.DemoWorkspaceID()

	req := authedRequest(t, http.MethodGet, fmt.Sprintf("%s/v1/workspaces/%s/knowledge/docs", srv.URL, wsID), "", tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/workspaces/{id}/knowledge/docs failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for viewer, got %d", resp.StatusCode)
	}
}

// --- Data Upload Tests ---

func TestUploadDataNoFile(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "operator")
	req := authedRequest(t, http.MethodPost, srv.URL+"/v1/data/upload", "", tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/data/upload failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing file, got %d", resp.StatusCode)
	}
}

func TestUploadDataAsViewerForbidden(t *testing.T) {
	pool := setupTestDB(t)
	app := setupTestApp(t, pool, nil)
	srv := newTestServer(app)
	defer srv.Close()

	tok := testJWTForRole(t, app.Cfg.JWTSecret, "viewer")
	req := authedRequest(t, http.MethodPost, srv.URL+"/v1/data/upload", "", tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/data/upload failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for viewer, got %d", resp.StatusCode)
	}
}
