package registry

import (
	"github.com/Harsho-afk/blackflow/internal/balancer"
)

type Route struct {
	Prefix   string
	Balancer balancer.Balancer
}

func NewRoute(prefix string, b balancer.Balancer) *Route {
	return &Route{
		Prefix:   prefix,
		Balancer: b,
	}
}
