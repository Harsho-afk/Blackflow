package health

import (
	"context"
	"net/url"
	"sync"
	"time"
)

type Backend interface {
	IsAlive() bool
	SetAlive(bool)
	GetURL() *url.URL
}

type BackendProvider interface {
	GetBackends() []Backend
}

type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc

	wg sync.WaitGroup
}

func NewManager(parent context.Context) *Manager {
	ctx, cancel := context.WithCancel(parent)

	return &Manager{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (m *Manager) Register(p BackendProvider, interval time.Duration) {
	m.wg.Add(1)

	go func() {
		defer m.wg.Done()
		checkProvider(p)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				checkProvider(p)
			case <-m.ctx.Done():
				return
			}
		}
	}()
}

func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}
