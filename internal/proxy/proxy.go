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
		backend := route.Balancer.NextBackend()
		if backend == nil {
			return
		}
		req.URL.Scheme = backend.URL.Scheme
		req.URL.Host = backend.URL.Host
	}

	p.proxy = reverse_proxy
	return p, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
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
