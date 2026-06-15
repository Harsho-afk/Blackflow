package registry

import (
	"github.com/Harsho-afk/blackflow/internal/balancer"
)

type Route struct {
	Prefix      string
	StripPrefix bool
	Balancer    balancer.Balancer
}

func NewRoute(prefix string, b balancer.Balancer, stripPrefix bool) *Route {
	return &Route{
		Prefix:      prefix,
		StripPrefix: stripPrefix,
		Balancer:    b,
	}
}
