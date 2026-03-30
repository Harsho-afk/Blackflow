package proxy

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

type Route struct {
	Prefix        string
	Pool          *Pool
	Balancer      Balancer
	HealthChecker *HealthChecker
}

type Proxy struct {
	Routes []*Route
	proxy  *httputil.ReverseProxy
}

func NewProxy(routes []*Route) (*Proxy, error) {
	p := &Proxy{
		Routes: routes,
	}
	reverseProxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// handled manually in ServeHTTP
		},

		Transport: &http.Transport{
			ResponseHeaderTimeout: 3 * time.Second,
		},

		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			backend := r.Context().Value("backend")
			if backend != nil {
				if b, ok := backend.(*Backend); ok {
					b.SetAlive(false)
				}
			}
			// differentiate errors
			if errors.Is(err, context.DeadlineExceeded) {
				http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
				return
			}
			// backend unreachable / crashed
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},

		ModifyResponse: func(resp *http.Response) error {
			if resp.StatusCode >= 500 {
				log.Printf("Backend error: %d from %s", resp.StatusCode, resp.Request.URL.Host)
			}
			return nil
		},
	}
	p.proxy = reverseProxy
	return p, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}()
	p.handleRequest(w, req)
}

func (p *Proxy) handleRequest(w http.ResponseWriter, req *http.Request) {
	route := p.matchRoute(req.URL.Path)
	if route == nil {
		http.Error(w, "Route not found", http.StatusNotFound)
		return
	}
	backend := route.Balancer.NextBackend()
	if backend == nil {
		http.Error(w, "No healthy backend", http.StatusServiceUnavailable)
		return
	}
	ctx := context.WithValue(req.Context(), "backend", backend)
	req = req.WithContext(ctx)
	backend.Increment()
	defer backend.Decrement()
	target := backend.GetURL()
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.Host = target.Host
	req.URL.Path = strings.TrimPrefix(req.URL.Path, route.Prefix)
	if req.URL.Path == "" {
		req.URL.Path = "/"
	}
	p.proxy.ServeHTTP(w, req)
}

func (p *Proxy) matchRoute(path string) *Route {
	for _, r := range p.Routes {
		if strings.HasPrefix(path, r.Prefix) {
			return r
		}
	}
	return nil
}
