package proxy

import (
	"net/url"
	"sync"
	"testing"
)

// helpers

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", raw, err)
	}
	return u
}

func addAliveBackend(t *testing.T, pool *Pool, raw string) *Backend {
	t.Helper()
	b := &Backend{}
	b.SetURL(mustParseURL(t, raw))
	b.SetAlive(true)
	pool.AddBackend(b)
	return b
}

// --- NewPool ---

func TestNewPool_IsEmpty(t *testing.T) {
	pool := NewPool()
	if len(pool.GetBackends()) != 0 {
		t.Error("new pool should be empty")
	}
}

// --- AddBackend ---

func TestPool_AddBackend(t *testing.T) {
	pool := NewPool()
	addAliveBackend(t, pool, "http://localhost:8001")
	addAliveBackend(t, pool, "http://localhost:8002")
	if got := len(pool.GetBackends()); got != 2 {
		t.Errorf("pool size = %d, want 2", got)
	}
}

// --- GetBackends snapshot ---

func TestPool_GetBackends_ReturnsSnapshot(t *testing.T) {
	pool := NewPool()
	addAliveBackend(t, pool, "http://localhost:8001")
	snap1 := pool.GetBackends()
	addAliveBackend(t, pool, "http://localhost:8002")
	snap2 := pool.GetBackends()

	if len(snap1) != 1 {
		t.Errorf("first snapshot should have 1 backend, got %d", len(snap1))
	}
	if len(snap2) != 2 {
		t.Errorf("second snapshot should have 2 backends, got %d", len(snap2))
	}
}

func TestPool_GetBackends_Fields(t *testing.T) {
	pool := NewPool()
	b := addAliveBackend(t, pool, "http://localhost:8001")
	b.Increment()

	backends := pool.GetBackends()
	if len(backends) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(backends))
	}
	info := backends[0]
	if info.URL.String() != "http://localhost:8001" {
		t.Errorf("URL = %q, want http://localhost:8001", info.URL.String())
	}
	if !info.Alive {
		t.Error("Alive should be true")
	}
	if info.Active != 1 {
		t.Errorf("Active = %d, want 1", info.Active)
	}
}

// --- RemoveBackend ---

func TestPool_RemoveBackend_ByBackend(t *testing.T) {
	pool := NewPool()
	b1 := addAliveBackend(t, pool, "http://localhost:8001")
	addAliveBackend(t, pool, "http://localhost:8002")

	err := pool.RemoveBackend(b1)
	if err != nil {
		t.Fatalf("RemoveBackend returned error: %v", err)
	}
	backends := pool.GetBackends()
	if len(backends) != 1 {
		t.Fatalf("expected 1 backend after removal, got %d", len(backends))
	}
	if backends[0].URL.String() != "http://localhost:8002" {
		t.Errorf("remaining backend = %q, want http://localhost:8002", backends[0].URL.String())
	}
}

func TestPool_RemoveBackend_ByURL(t *testing.T) {
	pool := NewPool()
	addAliveBackend(t, pool, "http://localhost:8001")
	addAliveBackend(t, pool, "http://localhost:8002")

	target := mustParseURL(t, "http://localhost:8001")
	err := pool.RemoveBackend(target)
	if err != nil {
		t.Fatalf("RemoveBackend returned error: %v", err)
	}
	if got := len(pool.GetBackends()); got != 1 {
		t.Errorf("expected 1 backend after removal, got %d", got)
	}
}

func TestPool_RemoveBackend_InvalidType(t *testing.T) {
	pool := NewPool()
	err := pool.RemoveBackend("not-a-valid-type")
	if err == nil {
		t.Error("expected error for invalid RemoveBackend argument type")
	}
}

func TestPool_RemoveBackend_NonExistent(t *testing.T) {
	pool := NewPool()
	addAliveBackend(t, pool, "http://localhost:8001")

	ghost := mustParseURL(t, "http://localhost:9999")
	_ = pool.RemoveBackend(ghost)
	if got := len(pool.GetBackends()); got != 1 {
		t.Errorf("pool should still have 1 backend after removing nonexistent URL, got %d", got)
	}
}

func TestPool_RemoveAllBackends(t *testing.T) {
	pool := NewPool()
	b := addAliveBackend(t, pool, "http://localhost:8001")
	_ = pool.RemoveBackend(b)
	if got := len(pool.GetBackends()); got != 0 {
		t.Errorf("pool should be empty, got %d backends", got)
	}
}

// --- LoadBackends ---

func TestPool_LoadBackends_ParsesURLs(t *testing.T) {
	// Use an unreachable URL so health checks mark them dead quickly.
	pool := NewPool()
	pool.LoadBackends([]string{
		"http://127.0.0.1:19991",
		"http://127.0.0.1:19992",
	})
	if got := len(pool.GetBackends()); got != 2 {
		t.Errorf("LoadBackends: pool size = %d, want 2", got)
	}
}

// --- Concurrency ---

func TestPool_ConcurrentAddAndGet(t *testing.T) {
	pool := NewPool()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := &Backend{}
			u, _ := url.Parse("http://localhost:8001")
			b.SetURL(u)
			b.SetAlive(true)
			pool.AddBackend(b)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pool.GetBackends()
		}()
	}
	wg.Wait()
}

func TestPool_ConcurrentRemove(t *testing.T) {
	pool := NewPool()
	backends := make([]*Backend, 20)
	for i := range backends {
		b := addAliveBackend(t, pool, "http://localhost:8001")
		backends[i] = b
	}
	var wg sync.WaitGroup
	for _, b := range backends {
		b := b
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pool.RemoveBackend(b)
		}()
	}
	wg.Wait()
}
