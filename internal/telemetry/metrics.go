package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hub_http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"code", "method", "path"},
	)
	httpDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hub_http_request_duration_seconds",
			Help:    "HTTP request duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

// PrometheusMiddleware records request counts and latency.
func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		route := "unknown"
		if rc := chi.RouteContext(r.Context()); rc != nil {
			if p := rc.RoutePattern(); p != "" {
				route = p
			}
		}
		httpRequests.WithLabelValues(strconv.Itoa(ww.Status()), r.Method, route).Inc()
		httpDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// Handler serves process + custom registry metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}
