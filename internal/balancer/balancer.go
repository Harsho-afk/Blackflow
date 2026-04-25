package balancer

import (
	"net/url"
	"sync/atomic"
)

type Backend interface {
	IsAlive() bool
	GetActiveConnections() int64
	Increment()
	Decrement()
	GetURL() *url.URL
}

type BackendProvider interface {
	GetBackends() []Backend
}

type Balancer interface {
	NextBackend() Backend
	GetAlgorithm() string
}

func NewBalancer(p BackendProvider, algo string) Balancer {
	switch algo {
	case "least_connection":
		return NewLeastConnection(p)
	default:
		return NewRoundRobin(p)
	}
}

type RoundRobin struct {
	provider BackendProvider
	current  uint64
}

func NewRoundRobin(p BackendProvider) *RoundRobin {
	return &RoundRobin{
		provider: p,
	}
}

func (rr *RoundRobin) NextBackend() Backend {
	backends := rr.provider.GetBackends()
	n := len(backends)
	if n == 0 {
		return nil
	}

	for range n {
		idx := atomic.AddUint64(&rr.current, 1)
		b := backends[idx%uint64(n)]

		if b.IsAlive() {
			return b
		}
	}

	return nil
}

func (rr *RoundRobin) GetAlgorithm() string {
	return "round_robin"
}

type LeastConnection struct {
	provider BackendProvider
}

func NewLeastConnection(p BackendProvider) *LeastConnection {
	return &LeastConnection{
		provider: p,
	}
}

func (lc *LeastConnection) NextBackend() Backend {
	backends := lc.provider.GetBackends()
	var best Backend

	for _, b := range backends {
		if !b.IsAlive() {
			continue
		}
		if best == nil || b.GetActiveConnections() < best.GetActiveConnections() {
			best = b
		}
	}

	return best
}

func (lc *LeastConnection) GetAlgorithm() string {
	return "least_connection"
}
