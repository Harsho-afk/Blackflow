package balancer

import (
	"hash/fnv"
	"sync/atomic"

	"github.com/Harsho-afk/blackflow/internal/backend"
)

type Backend = backend.Instance
type BackendProvider = backend.Provider

// Balancer selects the next backend for a request. key is an
// algorithm-specific routing key (e.g. client IP) used by
// affinity-based strategies such as IPHash; algorithms that don't
// need one (RoundRobin, LeastConnection) ignore it.
type Balancer interface {
	NextBackend(key string) Backend
	GetAlgorithm() string
}

func NewBalancer(p BackendProvider, algo string) Balancer {
	switch algo {
	case "least_connection":
		return NewLeastConnection(p)
	case "ip_hash":
		return NewIPHash(p)
	default:
		return NewRoundRobin(p)
	}
}

type RoundRobin struct {
	provider BackendProvider
	current  uint64
}

func NewRoundRobin(p BackendProvider) *RoundRobin {
	return &RoundRobin{provider: p}
}

func (rr *RoundRobin) NextBackend(_ string) Backend {
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

func (rr *RoundRobin) GetAlgorithm() string { return "round_robin" }

type LeastConnection struct {
	provider BackendProvider
}

func NewLeastConnection(p BackendProvider) *LeastConnection {
	return &LeastConnection{provider: p}
}

func (lc *LeastConnection) NextBackend(_ string) Backend {
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

func (lc *LeastConnection) GetAlgorithm() string { return "least_connection" }

// IPHash provides sticky routing: the same key (typically client IP)
// consistently maps to the same backend as long as the pool's alive
// membership doesn't change. If the chosen backend is unhealthy, it
// probes forward through the remaining backends in a fixed order
// rather than falling back to round robin, so affinity degrades
// gracefully instead of being abandoned entirely.
type IPHash struct {
	provider BackendProvider
}

func NewIPHash(p BackendProvider) *IPHash {
	return &IPHash{provider: p}
}

func (ih *IPHash) NextBackend(key string) Backend {
	backends := ih.provider.GetBackends()
	n := len(backends)
	if n == 0 {
		return nil
	}

	h := fnv.New32a()
	h.Write([]byte(key))
	start := h.Sum32() % uint32(n)

	for i := uint32(0); i < uint32(n); i++ {
		b := backends[(start+i)%uint32(n)]
		if b.IsAlive() {
			return b
		}
	}

	return nil
}

func (ih *IPHash) GetAlgorithm() string { return "ip_hash" }
