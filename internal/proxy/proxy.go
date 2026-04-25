package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"

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
		http.NotFound(w, r)
		return
	}

	backend := route.Balancer.NextBackend()
	if backend == nil {
		http.Error(w, "No healthy backend", http.StatusServiceUnavailable)
		return
	}

	backend.Increment()
	defer backend.Decrement()

	var target *url.URL = backend.GetURL()
	if target == nil {
		http.Error(w, "Invalid backend URL", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.URL.Path = r.URL.Path
		req.URL.RawQuery = r.URL.RawQuery

		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Proto", "http")

		if ip := r.RemoteAddr; ip != "" {
			req.Header.Set("X-Forwarded-For", ip)
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	proxy.ServeHTTP(w, r)
}
