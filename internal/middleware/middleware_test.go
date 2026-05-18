package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dataflowagenthub/hub/internal/auth"
	"github.com/dataflowagenthub/hub/internal/config"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// testConfig 创建最小化测试配置
func testConfig() *config.Config {
	return &config.Config{
		JWTSecret: []byte("test-secret-key-for-unit-tests-1234"),
	}
}

// TestTraceID_Injected 验证 TraceID 中间件向请求上下文中注入 TraceID
func TestTraceID_Injected(t *testing.T) {
	r := chi.NewRouter()
	r.Use(TraceID)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		tid := TraceFromContext(r.Context())
		if tid == "" {
			t.Error("expected non-empty TraceID in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Trace-Id") == "" {
		t.Error("expected X-Trace-Id response header")
	}
}

// TestTraceID_RespectsIncomingHeader 验证从请求头传入的 X-Trace-Id 会被保留
func TestTraceID_RespectsIncomingHeader(t *testing.T) {
	r := chi.NewRouter()
	r.Use(TraceID)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		tid := TraceFromContext(r.Context())
		if tid != "my-trace-123" {
			t.Errorf("expected trace_id=my-trace-123, got %q", tid)
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
	req.Header.Set("X-Trace-Id", "my-trace-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Trace-Id") != "my-trace-123" {
		t.Errorf("expected X-Trace-Id=my-trace-123, got %q", resp.Header.Get("X-Trace-Id"))
	}
}

// TestAuth_ValidToken 验证有效 JWT 放行，并在 context 中注入 Claims
func TestAuth_ValidToken(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	token, err := auth.Sign(cfg.JWTSecret, "user-1", "ws-1", "admin", 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		c := ClaimsFromContext(r.Context())
		if c == nil {
			t.Error("expected Claims in context")
			return
		}
		if c.UserID != "user-1" {
			t.Errorf("expected UserID=user-1, got %q", c.UserID)
		}
		if c.WorkspaceID != "ws-1" {
			t.Errorf("expected WorkspaceID=ws-1, got %q", c.WorkspaceID)
		}
		if c.Role != "admin" {
			t.Errorf("expected Role=admin, got %q", c.Role)
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestAuth_NoToken 验证无 token 返回 401
func TestAuth_NoToken(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/protected")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// TestAuth_InvalidToken 验证无效 token 返回 401
func TestAuth_InvalidToken(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// TestAuth_ExpiredToken 验证过期 token 返回 401
func TestAuth_ExpiredToken(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	token, err := auth.Sign(cfg.JWTSecret, "user-1", "ws-1", "admin", -1*time.Hour)
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// TestAuth_QueryToken 验证通过 ?token= 查询参数传递 token 的工作方式
func TestAuth_QueryToken(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	token, err := auth.Sign(cfg.JWTSecret, "user-1", "ws-1", "viewer", 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	called := false
	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.Get("/sse", func(w http.ResponseWriter, r *http.Request) {
		called = true
		c := ClaimsFromContext(r.Context())
		if c == nil {
			t.Error("expected Claims in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/sse?token=" + token)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if !called {
		t.Error("handler was not called")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestAuth_SSEToken 验证 SSE 短效 token 能正常通过认证
func TestAuth_SSEToken(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	token, err := auth.SignSSEToken(cfg.JWTSecret, "user-1", "ws-1", "session-abc")
	if err != nil {
		t.Fatalf("failed to sign SSE token: %v", err)
	}

	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.Get("/sse", func(w http.ResponseWriter, r *http.Request) {
		c := ClaimsFromContext(r.Context())
		if c == nil {
			t.Error("expected Claims in context")
		}
		if c.Role != "viewer" {
			t.Errorf("expected viewer role, got %q", c.Role)
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/sse", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestAuth_MissingBearerPrefix 验证缺少 Bearer 前缀时返回 401
func TestAuth_MissingBearerPrefix(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	token, err := auth.Sign(cfg.JWTSecret, "user-1", "ws-1", "admin", 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/protected", nil)
	// 没有 Bearer 前缀
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// TestRequireMinRole_AdminAccess 验证 admin 可以访问需要 admin 权限的接口
func TestRequireMinRole_AdminAccess(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	token, err := auth.Sign(cfg.JWTSecret, "user-1", "ws-1", "admin", 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.With(RequireMinRole("admin")).Get("/admin-only", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for admin, got %d", resp.StatusCode)
	}
}

// TestRequireMinRole_ViewerBlocked 验证 viewer 无法访问需要 operator 或 admin 权限的接口
func TestRequireMinRole_ViewerBlocked(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	token, err := auth.Sign(cfg.JWTSecret, "user-1", "ws-1", "viewer", 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.With(RequireMinRole("operator")).Get("/operator-only", func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for viewer")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/operator-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for viewer, got %d", resp.StatusCode)
	}
}

// TestRequireMinRole_OperatorAccess 验证 operator 可以访问需要 operator 权限的接口
func TestRequireMinRole_OperatorAccess(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	token, err := auth.Sign(cfg.JWTSecret, "user-1", "ws-1", "operator", 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.With(RequireMinRole("operator")).Get("/operator-only", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/operator-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for operator, got %d", resp.StatusCode)
	}
}

// TestRequireMinRole_OperatorBlockedFromAdmin 验证 operator 无法访问 admin 接口
func TestRequireMinRole_OperatorBlockedFromAdmin(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	token, err := auth.Sign(cfg.JWTSecret, "user-1", "ws-1", "operator", 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.With(RequireMinRole("admin")).Get("/admin-only", func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for operator")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for operator on admin route, got %d", resp.StatusCode)
	}
}

// TestRequireMinRole_NoClaims 验证无 Claims 时返回 403
func TestRequireMinRole_NoClaims(t *testing.T) {
	r := chi.NewRouter()
	r.With(RequireMinRole("viewer")).Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/protected")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

// TestTraceFromContext_Empty 验证空 context 返回空字符串
func TestTraceFromContext_Empty(t *testing.T) {
	tid := TraceFromContext(context.Background())
	if tid != "" {
		t.Errorf("expected empty string, got %q", tid)
	}
}

// TestClaimsFromContext_Empty 验证空 context 返回 nil
func TestClaimsFromContext_Empty(t *testing.T) {
	c := ClaimsFromContext(context.Background())
	if c != nil {
		t.Errorf("expected nil, got %+v", c)
	}
}

// TestRequestLog 验证 RequestLog 中间件不会 panic 并记录日志
func TestRequestLog(t *testing.T) {
	log := zap.NewNop()

	r := chi.NewRouter()
	r.Use(RequestLog(log))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

// TestRequestLog_RedactsAuth 验证 RequestLog 中间件在有 Authorization 头部时不会 panic
func TestRequestLog_RedactsAuth(t *testing.T) {
	log := zap.NewNop()

	r := chi.NewRouter()
	r.Use(RequestLog(log))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
	req.Header.Set("Authorization", "Bearer some-secret-token-value-here")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestSkippedPaths 验证无需认证的公共路径可以跳过 Auth 中间件
// 注意：这只是 Auth 逻辑单元测试，实际的跳过逻辑在 Routes() 中配置
func TestAuth_AllowMissingTokenOnPublicPath(t *testing.T) {
	// 这个测试验证当我们不为某个路由挂载 Auth 中间件时，请求可以正常通过
	// 这模拟了 /health、/version、/v1/auth/login 等公共路径的配置模式

	r := chi.NewRouter()
	// 不为该路由挂载 Auth 中间件
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	// 无 token 访问公共路径
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for public path, got %d", resp.StatusCode)
	}
}

// TestAuth_EmptyBearer 验证空 Bearer token 返回 401
func TestAuth_EmptyBearer(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/protected", nil)
	req.Header.Set("Authorization", "Bearer ")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// TestAuth_WrongSigningMethod 验证使用非 HMAC 签名方法的 token 会被拒绝
func TestAuth_WrongSigningMethod(t *testing.T) {
	cfg := testConfig()
	log := zap.NewNop()

	r := chi.NewRouter()
	r.Use(Auth(cfg, log, nil))
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	// 发送一个格式正确但使用错误密钥签名的 token
	req, _ := http.NewRequest("GET", srv.URL+"/protected", nil)
	badToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxIn0.signature"
	req.Header.Set("Authorization", "Bearer "+badToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}
