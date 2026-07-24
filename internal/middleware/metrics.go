package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Harsho-afk/blackflow/internal/metrics"
)

// Metrics records request-level Prometheus metrics: total requests,
// request duration, and in-flight connections. Per-backend metrics
// (requests, active connections) and failure-reason metrics are
// recorded separately in the proxy package, which is the only layer
// that knows which backend served (or failed to serve) the request.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics.ActiveConnections.Inc()
		defer metrics.ActiveConnections.Dec()

		start := time.Now()
		rw := wrapResponseWriter(w)

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.status)

		metrics.RequestsTotal.WithLabelValues(r.Method, status).Inc()
		metrics.RequestDuration.WithLabelValues(r.Method).Observe(duration)
	})
}
