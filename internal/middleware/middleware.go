package middleware

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/dataflowagenthub/hub/internal/auth"
	"github.com/dataflowagenthub/hub/internal/config"
	"github.com/dataflowagenthub/hub/internal/seed"
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

// Auth accepts either Bearer JWT or X-Hub-Api-Key matching global key (admin on demo workspace).
// If rdb is non-nil, JWT revocation is checked.
func Auth(cfg *config.Config, log *zap.Logger, rdb *redis.Client) func(http.Handler) http.Handler {
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

			// Support Bearer token from Header or ?token= query parameter (for SSE EventSource)
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

			// Check revocation list
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

// ClaimsFromContext returns JWT claims if present.
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(ctxClaims).(*auth.Claims)
	return c
}

// InternalHMACAuth validates HMAC-SHA256 signatures on internal endpoints.
// The client must send a X-Hub-Signature: sha256=<hex> header where <hex> is
// the HMAC-SHA256 of the request body computed with the shared secret.
func InternalHMACAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sigHeader := r.Header.Get("X-Hub-Signature")
			if sigHeader == "" {
				http.Error(w, `{"error":"missing signature"}`, http.StatusUnauthorized)
				return
			}

			// Parse "sha256=<hex>"
			parts := strings.SplitN(sigHeader, "=", 2)
			if len(parts) != 2 || parts[0] != "sha256" {
				http.Error(w, `{"error":"invalid signature format"}`, http.StatusUnauthorized)
				return
			}
			sigHex := parts[1]

			// Read and buffer body
			bodyBytes, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}

			// Compute expected HMAC
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(bodyBytes)
			expectedHex := hex.EncodeToString(mac.Sum(nil))

			if !hmac.Equal([]byte(sigHex), []byte(expectedHex)) {
				http.Error(w, `{"error":"invalid signature"}`, http.StatusUnauthorized)
				return
			}

			// Restore body for downstream handlers
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			next.ServeHTTP(w, r)
		})
	}
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
