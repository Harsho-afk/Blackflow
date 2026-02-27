package proxy

import "sync/atomic"

type Balancer interface {
	NextBackend() *Backend
	GetAlgorithm() string
}

type RoundRobin struct {
	pool    *Pool
	current uint64
}

func NewBalancer(pool *Pool, algo string) Balancer {
	switch algo {
	case "round_robin":
		return NewRoundRobin(pool)
	case "least_connection":
		return NewLeastConnection(pool)
	default:
		return NewRoundRobin(pool)
	}
}

func NewRoundRobin(pool *Pool) *RoundRobin {
	return &RoundRobin{
		pool: pool,
	}
}

func (rr *RoundRobin) NextBackend() *Backend {
	backends := rr.pool.getBackends()
	if len(backends) == 0 {
		return nil
	}
	length := len(backends)
	for range length {
		index := atomic.AddUint64(&rr.current, 1)
		backend := backends[index%uint64(length)]
		if backend.IsAlive() {
			return backend
		}
	}
	return nil
}

func (rr *RoundRobin) GetAlgorithm() string {
	return "round_robin"
}

type LeastConnection struct {
	pool *Pool
}

func NewLeastConnection(pool *Pool) *LeastConnection {
	return &LeastConnection{
		pool: pool,
	}
}

func (lc *LeastConnection) NextBackend() *Backend {
	backends := lc.pool.getBackends()
	if len(backends) == 0 {
		return nil
	}
	length := len(backends)
	min := backends[0]
	for index := range length {
		if backends[index%length].IsAlive() && min.GetActiveConnections() > backends[index%length].GetActiveConnections() {
			min = backends[index%length]
		}
	}
	if min.IsAlive() {
		return min
	}
	return nil
}

func (lc *LeastConnection) GetAlgorithm() string {
	return "least_connection"
}
