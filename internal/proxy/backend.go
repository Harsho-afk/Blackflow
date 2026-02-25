package proxy

import (
	"net/url"
	"sync"
	"sync/atomic"
)

type Backend struct {
	Prefix string
	URL    *url.URL
	Alive  bool
	mu     sync.RWMutex
	Active int64
}

func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Alive = alive
}

func (b *Backend) IsAlive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Alive
}

func (b *Backend) Increment() {
	atomic.AddInt64(&b.Active, 1)
}

func (b *Backend) Decrement() {
	atomic.AddInt64(&b.Active, -1)
}

func (b *Backend) GetActiveConnections() int64 {
	return atomic.LoadInt64(&b.Active)
}

func (b *Backend) SetPrefix(new_prefix string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Prefix = new_prefix
}

func (b *Backend) GetPrefix() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Prefix
}

func (b *Backend) SetURL(new_url *url.URL) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.URL = new_url
}

func (b *Backend) GetURL() *url.URL {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.URL
}

func (b *Backend) SetActiveConnection(conn int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Active = conn
}
