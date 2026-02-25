package proxy

import (
	"net/http"
	"net/http/httputil"
)

type Proxy struct {
	Pool     *Pool
	proxy    *httputil.ReverseProxy
	Balancer Balancer
}

func NewProxy(pool *Pool, balancer Balancer) (*Proxy, error) {
	p := &Proxy{
		Pool:     pool,
		Balancer: balancer,
	}

	reverse_proxy := &httputil.ReverseProxy{}

	reverse_proxy.Director = func(req *http.Request) {
		backend := p.Balancer.NextBackend()
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
