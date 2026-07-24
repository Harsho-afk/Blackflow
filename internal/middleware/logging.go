package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Logging logs each request as a structured slog record after it completes.
// Records: method, path, status, duration, remote address, and user-agent.
//
// A debug-level record is also emitted when the request is first received.
// This is useful for tracing requests that never complete (e.g. a hung
// backend or a client that disconnects mid-request), since the completion
// log below would otherwise never appear for those requests.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := wrapResponseWriter(w)

		slog.Debug("request received",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		next.ServeHTTP(rw, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}
