package middleware

import (
	"context"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/dataflowagenthub/hub/internal/auth"
	"github.com/dataflowagenthub/hub/internal/config"
	"github.com/dataflowagenthub/hub/internal/seed"
	"github.com/google/uuid"
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

// Auth accepts either Bearer JWT or X-Hub-Api-Key matching global key (admin on demo workspace).
func Auth(cfg *config.Config, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.GlobalAPIKey != "" {
				if r.Header.Get("X-Hub-Api-Key") == cfg.GlobalAPIKey {
					c := &auth.Claims{
						UserID:      seed.ServiceAPIUserID,
						WorkspaceID: seed.DemoWorkspaceID(),
						Role:        "admin",
					}
					ctx := context.WithValue(r.Context(), ctxClaims, c)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			raw := strings.TrimSpace(h[7:])
			c, err := auth.Parse(cfg.JWTSecret, raw)
			if err != nil {
				log.Debug("jwt parse failed", zap.Error(err))
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxClaims, c)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext returns JWT claims if present.
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(ctxClaims).(*auth.Claims)
	return c
}

// RequireMinRole returns 403 if role is weaker than required (admin > operator > viewer).
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
