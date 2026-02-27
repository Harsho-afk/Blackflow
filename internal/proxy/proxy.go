package proxy

import (
	"net/http"
	"net/http/httputil"
	"strings"
)

type Route struct {
	Prefix   string
	Pool     *Pool
	Balancer Balancer
}

type Proxy struct {
	Routes []*Route
	proxy  *httputil.ReverseProxy
}

func NewProxy(routes []*Route) (*Proxy, error) {
	p := &Proxy{
		Routes: routes,
	}

	reverse_proxy := &httputil.ReverseProxy{}

	reverse_proxy.Director = func(req *http.Request) {
		route := p.matchRoute(req.URL.Path)
		backends := route.Pool.getBackends()
		for _, backend := range backends {
			req.URL.Scheme = backend.URL.Scheme
			req.URL.Host = backend.URL.Host
		}
	}

	p.proxy = reverse_proxy
	return p, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	route := p.matchRoute(req.URL.Path)
	if route == nil {
		http.Error(w, "Route not found", http.StatusNotFound)
		return
	}
	backend := route.Balancer.NextBackend()
	if backend == nil {
		http.Error(w, "No healty backend", http.StatusServiceUnavailable)
		return
	}
	backend.Increment()
	defer backend.Decrement()
	req.URL.Scheme = backend.URL.Scheme
	req.URL.Host = backend.URL.Host
	// log.Printf("%s - %d\n", req.URL.String(), backend.Active)
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
