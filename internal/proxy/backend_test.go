package proxy

import (
	"net/url"
	"sync"
	"testing"
)

func newTestBackend(t *testing.T, rawURL string, alive bool) *Backend {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("failed to parse URL %q: %v", rawURL, err)
	}
	b := &Backend{}
	b.SetURL(u)
	b.SetAlive(alive)
	return b
}

// --- Alive state ---

func TestBackend_SetAndIsAlive(t *testing.T) {
	b := &Backend{}
	b.SetAlive(true)
	if !b.IsAlive() {
		t.Error("expected backend to be alive after SetAlive(true)")
	}
	b.SetAlive(false)
	if b.IsAlive() {
		t.Error("expected backend to be dead after SetAlive(false)")
	}
}

func TestBackend_DefaultAliveIsFalse(t *testing.T) {
	b := &Backend{}
	if b.IsAlive() {
		t.Error("zero-value Backend should default to not alive")
	}
}

// --- URL ---

func TestBackend_SetAndGetURL(t *testing.T) {
	b := &Backend{}
	u, _ := url.Parse("http://localhost:8080")
	b.SetURL(u)
	got := b.GetURL()
	if got.String() != u.String() {
		t.Errorf("GetURL() = %q, want %q", got.String(), u.String())
	}
}

func TestBackend_GetURLReturnsNilWhenUnset(t *testing.T) {
	b := &Backend{}
	if b.GetURL() != nil {
		t.Error("expected nil URL on zero-value Backend")
	}
}

// --- Active connections ---

func TestBackend_IncrementAndDecrement(t *testing.T) {
	b := &Backend{}
	b.Increment()
	b.Increment()
	if got := b.GetActiveConnections(); got != 2 {
		t.Errorf("GetActiveConnections() = %d, want 2", got)
	}
	b.Decrement()
	if got := b.GetActiveConnections(); got != 1 {
		t.Errorf("GetActiveConnections() = %d after Decrement, want 1", got)
	}
}

func TestBackend_DecrementBelowZero(t *testing.T) {
	b := &Backend{}
	b.Decrement()
	if got := b.GetActiveConnections(); got != -1 {
		t.Errorf("GetActiveConnections() = %d, want -1", got)
	}
}

func TestBackend_SetActiveConnection(t *testing.T) {
	b := &Backend{}
	b.SetActiveConnection(42)
	if got := b.GetActiveConnections(); got != 42 {
		t.Errorf("GetActiveConnections() = %d, want 42", got)
	}
}

// --- Concurrency ---

func TestBackend_ConcurrentAlive(t *testing.T) {
	b := &Backend{}
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b.SetAlive(i%2 == 0)
			_ = b.IsAlive()
		}(i)
	}
	wg.Wait()
}

func TestBackend_ConcurrentIncrementDecrement(t *testing.T) {
	b := &Backend{}
	var wg sync.WaitGroup
	n := 1000
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Increment()
		}()
	}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Decrement()
		}()
	}
	wg.Wait()
	if got := b.GetActiveConnections(); got != 0 {
		t.Errorf("expected net 0 active connections after balanced inc/dec, got %d", got)
	}
}

func TestBackend_ConcurrentURL(t *testing.T) {
	b := &Backend{}
	u1, _ := url.Parse("http://host-a:8080")
	u2, _ := url.Parse("http://host-b:9090")
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); b.SetURL(u1) }()
		go func() { defer wg.Done(); _ = b.GetURL() }()
		wg.Add(2)
		go func() { defer wg.Done(); b.SetURL(u2) }()
		go func() { defer wg.Done(); _ = b.GetURL() }()
	}
	wg.Wait()
}
