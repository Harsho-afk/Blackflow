package proxy

import (
	"net/url"
	"sync"
	"sync/atomic"
)

type Backend struct {
	url    *url.URL
	alive  bool
	mu     sync.RWMutex
	active int64
}

func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.alive = alive
}

func (b *Backend) IsAlive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.alive
}

func (b *Backend) Increment() {
	atomic.AddInt64(&b.active, 1)
}

func (b *Backend) Decrement() {
	atomic.AddInt64(&b.active, -1)
}

func (b *Backend) GetActiveConnections() int64 {
	return atomic.LoadInt64(&b.active)
}

func (b *Backend) SetURL(new_url *url.URL) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.url = new_url
}

func (b *Backend) GetURL() *url.URL {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.url
}

func (b *Backend) SetActiveConnection(conn int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.active = conn
}
