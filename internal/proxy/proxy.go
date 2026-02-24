package proxy

import (
	"net/http"
	"net/http/httputil"
)

type Proxy struct {
	pool  *Pool
	proxy *httputil.ReverseProxy
}

func NewProxy(pool *Pool) (*Proxy, error) {
	p := &Proxy{
		pool: pool,
	}

	reverse_proxy := &httputil.ReverseProxy{}

	reverse_proxy.Director = func(req *http.Request) {
		// ...
	}

	p.proxy = reverse_proxy
	return p, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	p.proxy.ServeHTTP(w, req)
}
