package backend

import (
	"net/url"
	"sync"
	"sync/atomic"
)

type Backend struct {
	url    *url.URL
	mu     sync.RWMutex
	alive  bool
	active int64
}

func New(rawURL string) (*Backend, error) {
	parsed, err := url.Parse(rawURL)

	if err != nil {
		return nil, err
	}

	return &Backend{
		url:   parsed,
		alive: false,
	}, nil
}
func (b *Backend) GetURL() *url.URL {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.url
}

func (b *Backend) SetURL(u *url.URL) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.url = u
}

func (b *Backend) IsAlive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.alive
}

func (b *Backend) SetAlive(v bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.alive = v
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
