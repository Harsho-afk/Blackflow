package pool

import (
	"sync"

	"github.com/Harsho-afk/blackflow/internal/backend"
)

type Pool struct {
	mu       sync.RWMutex
	backends []*backend.Backend
}

func New() *Pool {
	return &Pool{
		backends: make([]*backend.Backend, 0),
	}
}

func (p *Pool) GetBackends() []*backend.Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*backend.Backend, len(p.backends))
	copy(out, p.backends)
	return out
}

func (p *Pool) AddBackend(b *backend.Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backends = append(p.backends, b)
}

func (p *Pool) Load(urls []string) error {
	for _, u := range urls {
		b, err := backend.New(u)
		if err != nil {
			return err
		}
		p.AddBackend(b)
	}

	return nil
}

func (p *Pool) RemoveBackend(target string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	filtered := p.backends[:0]

	for _, b := range p.backends {
		if b.GetURL().String() != target {
			filtered = append(filtered, b)
		}
	}

	p.backends = filtered
}
