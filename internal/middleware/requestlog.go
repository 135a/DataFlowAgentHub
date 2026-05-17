package middleware

import (
	"net/http"
	"time"

	"github.com/dataflowagenthub/hub/internal/llm"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// RequestLog 记录每个请求的日志行，包含 trace id；Authorization 头部会被脱敏
func RequestLog(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			auth := r.Header.Get("Authorization")
			if auth != "" {
				auth = llm.RedactAuthHeader(auth)
			}
			apiKey := r.Header.Get("X-Hub-Api-Key")
			if apiKey != "" {
				apiKey = llm.RedactAPIKey(apiKey)
			}
			log.Info("http_request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.String("trace_id", TraceFromContext(r.Context())),
				zap.Duration("duration", time.Since(start)),
				zap.String("authorization", auth),
				zap.String("x_hub_api_key", apiKey),
			)
		})
	}
}
