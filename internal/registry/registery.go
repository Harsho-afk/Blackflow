package registry

import (
	"strings"
	"sync"
)

type Registry struct {
	mu     sync.RWMutex
	routes map[string]*Route
}

func New() *Registry {
	return &Registry{
		routes: make(map[string]*Route),
	}
}

func (r *Registry) Add(route *Route) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[route.Prefix] = route
}

func (r *Registry) Remove(prefix string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, prefix)
}

func (r *Registry) Match(path string) *Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var best *Route
	longest := 0

	for prefix, route := range r.routes {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		if len(path) > len(prefix) && path[len(prefix)] != '/' {
			continue
		}
		if len(prefix) > longest {
			best = route
			longest = len(prefix)
		}
	}

	return best
}
