package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type Proxy struct {
	routes map[string]*url.URL
	proxy  *httputil.ReverseProxy
}

func NewProxy(routes map[string]string) (*Proxy, error) {
	parsed_routes := make(map[string]*url.URL)

	for prefix, target := range routes {
		url, err := url.Parse(target)
		if err != nil {
			return nil, err
		}
		parsed_routes[prefix] = url
	}

	p := &Proxy{
		routes: parsed_routes,
	}

	reverse_proxy := &httputil.ReverseProxy{}

	reverse_proxy.Director = func(req *http.Request) {
		for prefix, target := range (*p).routes {
			if strings.HasPrefix(req.URL.Path, prefix) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				return
			}
		}
	}

	p.proxy = reverse_proxy
	return p, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	p.proxy.ServeHTTP(w, req)
}
