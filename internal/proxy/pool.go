package proxy

import (
	"errors"
	"net/url"
	"sync"
)

type Pool struct {
	backends []*Backend
	mu       sync.RWMutex
}

func NewPool() *Pool {
	return &Pool{
		backends: make([]*Backend, 0),
	}
}

type BackendInfo struct {
	URL    *url.URL
	Alive  bool
	Active int64
}

func (pool *Pool) GetBackends() []BackendInfo {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	backends := make([]BackendInfo, 0, len(pool.backends))
	for _, x := range pool.backends {
		backends = append(backends, BackendInfo{
			URL:    x.URL,
			Alive:  x.IsAlive(),
			Active: x.GetActiveConnections(),
		})
	}
	return backends
}

func removeElementByBackend(a []*Backend, ele *Backend) []*Backend {
	b := a[:0]
	for _, x := range a {
		if x.URL.String() != ele.URL.String() {
			b = append(b, x)
		}
	}
	return b
}

func removeElementByURL(a []*Backend, ele *url.URL) []*Backend {
	b := a[:0]
	for _, x := range a {
		if x.URL.String() != ele.String() {
			b = append(b, x)
		}
	}
	return b
}

func (pool *Pool) AddBackend(backend *Backend) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.backends = append(pool.backends, backend)
}

func (pool *Pool) RemoveBackend(v any) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	switch val := v.(type) {
	case *Backend:
		pool.backends = removeElementByBackend(pool.backends, val)
	case *url.URL:
		pool.backends = removeElementByURL(pool.backends, val)
	default:
		return errors.New("Parameter type should be *Backend or *url.URL")
	}
	return nil
}
