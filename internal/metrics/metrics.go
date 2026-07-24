// Package metrics defines and registers the Prometheus collectors used
// throughout Blackflow. Collectors are package-level singletons so any
// package (middleware, proxy) can record metrics without needing a
// shared registry threaded through constructors.
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// RequestsTotal counts every HTTP request handled by Blackflow,
	// labeled by method and final response status code.
	//
	// NOTE: intentionally NOT labeled by path. Labeling by raw
	// request path would create unbounded label cardinality on a
	// reverse proxy fronting arbitrary backend paths, which is a
	// classic Prometheus foot-gun. Per-route breakdown is available
	// indirectly via BackendRequestsTotal.
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "blackflow_requests_total",
			Help: "Total number of HTTP requests processed, labeled by method and status code.",
		},
		[]string{"method", "status"},
	)

	// RequestDuration tracks end-to-end request latency in seconds,
	// labeled by method.
	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "blackflow_request_duration_seconds",
			Help:    "HTTP request latency in seconds, labeled by method.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	// ActiveConnections is the number of requests currently in flight,
	// across all routes and backends.
	ActiveConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "blackflow_active_connections",
			Help: "Number of in-flight HTTP requests currently being served by Blackflow.",
		},
	)

	// BackendRequestsTotal counts requests forwarded to each backend,
	// labeled by backend URL.
	BackendRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "blackflow_backend_requests_total",
			Help: "Total number of requests forwarded to each backend, labeled by backend URL.",
		},
		[]string{"backend"},
	)

	// BackendActiveConnections tracks in-flight connections per backend.
	BackendActiveConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "blackflow_backend_active_connections",
			Help: "Number of active connections currently open to each backend.",
		},
		[]string{"backend"},
	)

	// FailedRequestsTotal counts requests that could not be proxied
	// successfully, labeled by failure reason.
	FailedRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "blackflow_failed_requests_total",
			Help: "Total number of failed requests, labeled by reason.",
		},
		[]string{"reason"},
	)
)

func init() {
	prometheus.MustRegister(
		RequestsTotal,
		RequestDuration,
		ActiveConnections,
		BackendRequestsTotal,
		BackendActiveConnections,
		FailedRequestsTotal,
	)
}
