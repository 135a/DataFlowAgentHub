package middleware

import (
	"context"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/dataflowagenthub/hub/internal/auth"
	"github.com/dataflowagenthub/hub/internal/config"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type ctxKey string

const (
	ctxTraceID ctxKey = "trace_id"
	ctxClaims  ctxKey = "claims"
)

func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := r.Header.Get("X-Trace-Id")
		if tid == "" {
			if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
				tid = sc.TraceID().String()
			}
		}
		if tid == "" {
			tid = uuid.NewString()
		}
		w.Header().Set("X-Trace-Id", tid)
		ctx := context.WithValue(r.Context(), ctxTraceID, tid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TraceFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxTraceID).(string)
	return v
}

// Auth 接受 Bearer JWT（支持从 Header 或 ?token= 查询参数获取，用于 SSE EventSource）。
// 如果 rdb 不为空，则检查 JWT 吊销状态。
func Auth(cfg *config.Config, log *zap.Logger, rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := ""
			h := r.Header.Get("Authorization")
			if strings.HasPrefix(strings.ToLower(h), "bearer ") {
				raw = strings.TrimSpace(h[7:])
			}
			if raw == "" {
				raw = r.URL.Query().Get("token")
			}
			if raw == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			c, err := auth.Parse(cfg.JWTSecret, raw)
			if err != nil {
				log.Debug("jwt parse failed", zap.Error(err))
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// 检查吊销列表
			if rdb != nil && c.ID != "" {
				revoked, revErr := auth.IsRevoked(r.Context(), rdb, c.ID)
				if revErr != nil {
					log.Warn("jwt revocation check failed", zap.Error(revErr))
				} else if revoked {
					http.Error(w, `{"error":"token revoked"}`, http.StatusUnauthorized)
					return
				}
			}

			ctx := context.WithValue(r.Context(), ctxClaims, c)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext 返回上下文中的 JWT 声明（如果存在）
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(ctxClaims).(*auth.Claims)
	return c
}

// RequireMinRole 如果角色权限低于要求则返回 403（admin > operator > viewer）
func RequireMinRole(min string) func(http.Handler) http.Handler {
	order := map[string]int{"viewer": 1, "operator": 2, "admin": 3}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := ClaimsFromContext(r.Context())
			if c == nil {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			if order[c.Role] < order[min] {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
