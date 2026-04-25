package health

import (
	"context"
	"sync"
	"time"

	"github.com/Harsho-afk/blackflow/internal/backend"
)

type Backend = backend.Instance
type BackendProvider = backend.Provider

type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewManager(parent context.Context) *Manager {
	ctx, cancel := context.WithCancel(parent)
	return &Manager{ctx: ctx, cancel: cancel}
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
