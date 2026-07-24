package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/Harsho-afk/blackflow/internal/metrics"
	"github.com/Harsho-afk/blackflow/internal/registry"
)

type Proxy struct {
	registry *registry.Registry
}

func New(r *registry.Registry) *Proxy {
	return &Proxy{
		registry: r,
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := p.registry.Match(r.URL.Path)
	if route == nil {
		metrics.FailedRequestsTotal.WithLabelValues("route_not_found").Inc()
		http.NotFound(w, r)
		return
	}

	backend := route.Balancer.NextBackend()
	if backend == nil {
		metrics.FailedRequestsTotal.WithLabelValues("no_backend_available").Inc()
		slog.Warn("no healthy backend available", "prefix", route.Prefix)
		http.Error(w, "No healthy backend", http.StatusServiceUnavailable)
		return
	}

	var target *url.URL = backend.GetURL()
	if target == nil {
		metrics.FailedRequestsTotal.WithLabelValues("invalid_backend_url").Inc()
		slog.Error("selected backend has no URL", "prefix", route.Prefix)
		http.Error(w, "Invalid backend URL", http.StatusInternalServerError)
		return
	}

	backendLabel := target.String()

	slog.Debug("backend selected",
		"prefix", route.Prefix,
		"backend", backendLabel,
		"algorithm", route.Balancer.GetAlgorithm(),
	)

	backend.Increment()
	metrics.BackendActiveConnections.WithLabelValues(backendLabel).Inc()
	metrics.BackendRequestsTotal.WithLabelValues(backendLabel).Inc()
	defer func() {
		backend.Decrement()
		metrics.BackendActiveConnections.WithLabelValues(backendLabel).Dec()
	}()

	proxy := httputil.NewSingleHostReverseProxy(target)

	forwardPath := r.URL.Path
	if route.StripPrefix {
		forwardPath = strings.TrimPrefix(forwardPath, route.Prefix)
		if forwardPath == "" || !strings.HasPrefix(forwardPath, "/") {
			forwardPath = "/" + forwardPath
		}
	}

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.URL.Path = forwardPath
		req.URL.RawQuery = r.URL.RawQuery

		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Proto", "http")

		if ip := r.RemoteAddr; ip != "" {
			req.Header.Set("X-Forwarded-For", ip)
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		metrics.FailedRequestsTotal.WithLabelValues("bad_gateway").Inc()
		slog.Error("backend request failed",
			"backend", backendLabel,
			"prefix", route.Prefix,
			"error", err,
		)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	proxy.ServeHTTP(w, r)
}
