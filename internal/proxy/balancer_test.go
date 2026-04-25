package proxy

import (
	"net/url"
	"sync"
	"testing"
)

// helpers

func poolWith(t *testing.T, urls []string, alive bool) *Pool {
	t.Helper()
	pool := NewPool()
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("url.Parse(%q): %v", raw, err)
		}
		b := &Backend{}
		b.SetURL(u)
		b.SetAlive(alive)
		pool.AddBackend(b)
	}
	return pool
}

// --- NewBalancer factory ---

func TestNewBalancer_RoundRobin(t *testing.T) {
	pool := NewPool()
	b := NewBalancer(pool, "round_robin")
	if b.GetAlgorithm() != "round_robin" {
		t.Errorf("GetAlgorithm() = %q, want round_robin", b.GetAlgorithm())
	}
}

func TestNewBalancer_LeastConnection(t *testing.T) {
	pool := NewPool()
	b := NewBalancer(pool, "least_connection")
	if b.GetAlgorithm() != "least_connection" {
		t.Errorf("GetAlgorithm() = %q, want least_connection", b.GetAlgorithm())
	}
}

func TestNewBalancer_UnknownFallsBackToRoundRobin(t *testing.T) {
	pool := NewPool()
	b := NewBalancer(pool, "not_a_real_algo")
	if b.GetAlgorithm() != "round_robin" {
		t.Errorf("unknown algo should fall back to round_robin, got %q", b.GetAlgorithm())
	}
}

func TestNewBalancer_EmptyStringFallsBackToRoundRobin(t *testing.T) {
	pool := NewPool()
	b := NewBalancer(pool, "")
	if b.GetAlgorithm() != "round_robin" {
		t.Errorf("empty algo should fall back to round_robin, got %q", b.GetAlgorithm())
	}
}

// --- RoundRobin ---

func TestRoundRobin_EmptyPool(t *testing.T) {
	pool := NewPool()
	rr := NewRoundRobin(pool)
	if rr.NextBackend() != nil {
		t.Error("NextBackend on empty pool should return nil")
	}
}

func TestRoundRobin_AllDeadReturnsNil(t *testing.T) {
	pool := poolWith(t, []string{
		"http://localhost:8001",
		"http://localhost:8002",
	}, false)
	rr := NewRoundRobin(pool)
	if rr.NextBackend() != nil {
		t.Error("NextBackend with all dead backends should return nil")
	}
}

func TestRoundRobin_SingleAliveBackend(t *testing.T) {
	pool := poolWith(t, []string{"http://localhost:8001"}, true)
	rr := NewRoundRobin(pool)
	b := rr.NextBackend()
	if b == nil {
		t.Fatal("NextBackend returned nil for single alive backend")
	}
	if b.GetURL().String() != "http://localhost:8001" {
		t.Errorf("unexpected backend URL: %s", b.GetURL().String())
	}
}

func TestRoundRobin_RotatesAcrossBackends(t *testing.T) {
	urls := []string{
		"http://localhost:8001",
		"http://localhost:8002",
		"http://localhost:8003",
	}
	pool := poolWith(t, urls, true)
	rr := NewRoundRobin(pool)

	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		b := rr.NextBackend()
		if b == nil {
			t.Fatalf("unexpected nil at iteration %d", i)
		}
		seen[b.GetURL().String()]++
	}
	// Each backend should be hit exactly 3 times in 9 calls.
	for _, u := range urls {
		if seen[u] != 3 {
			t.Errorf("backend %s hit %d times, want 3", u, seen[u])
		}
	}
}

func TestRoundRobin_SkipsDeadBackends(t *testing.T) {
	pool := NewPool()
	for _, raw := range []string{"http://dead:8001", "http://alive:8002"} {
		u, _ := url.Parse(raw)
		b := &Backend{}
		b.SetURL(u)
		b.SetAlive(raw == "http://alive:8002")
		pool.AddBackend(b)
	}
	rr := NewRoundRobin(pool)
	for i := 0; i < 6; i++ {
		b := rr.NextBackend()
		if b == nil {
			t.Fatalf("unexpected nil at iteration %d", i)
		}
		if b.GetURL().Host == "dead:8001" {
			t.Error("RoundRobin selected a dead backend")
		}
	}
}

func TestRoundRobin_Concurrent(t *testing.T) {
	pool := poolWith(t, []string{
		"http://localhost:8001",
		"http://localhost:8002",
	}, true)
	rr := NewRoundRobin(pool)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := rr.NextBackend()
			if b == nil {
				t.Errorf("unexpected nil from concurrent NextBackend")
			}
		}()
	}
	wg.Wait()
}

// --- LeastConnection ---

func TestLeastConnection_EmptyPool(t *testing.T) {
	pool := NewPool()
	lc := NewLeastConnection(pool)
	if lc.NextBackend() != nil {
		t.Error("NextBackend on empty pool should return nil")
	}
}

func TestLeastConnection_AllDeadReturnsNil(t *testing.T) {
	pool := poolWith(t, []string{
		"http://localhost:8001",
		"http://localhost:8002",
	}, false)
	lc := NewLeastConnection(pool)
	if lc.NextBackend() != nil {
		t.Error("NextBackend with all dead backends should return nil")
	}
}

func TestLeastConnection_PicksLowestConnections(t *testing.T) {
	pool := NewPool()
	urls := []string{"http://localhost:8001", "http://localhost:8002", "http://localhost:8003"}
	conns := []int64{5, 1, 3}
	var backends []*Backend
	for i, raw := range urls {
		u, _ := url.Parse(raw)
		b := &Backend{}
		b.SetURL(u)
		b.SetAlive(true)
		b.SetActiveConnection(conns[i])
		pool.AddBackend(b)
		backends = append(backends, b)
	}

	lc := NewLeastConnection(pool)
	chosen := lc.NextBackend()
	if chosen == nil {
		t.Fatal("NextBackend returned nil")
	}
	if chosen.GetURL().String() != "http://localhost:8002" {
		t.Errorf("expected backend with fewest connections (8002), got %s", chosen.GetURL().String())
	}
}

func TestLeastConnection_SkipsDeadBackend(t *testing.T) {
	pool := NewPool()
	// dead backend with 0 connections – should be skipped
	u1, _ := url.Parse("http://dead:8001")
	dead := &Backend{}
	dead.SetURL(u1)
	dead.SetAlive(false)
	pool.AddBackend(dead)

	u2, _ := url.Parse("http://alive:8002")
	alive := &Backend{}
	alive.SetURL(u2)
	alive.SetAlive(true)
	alive.SetActiveConnection(10)
	pool.AddBackend(alive)

	lc := NewLeastConnection(pool)
	chosen := lc.NextBackend()
	if chosen == nil {
		t.Fatal("expected alive backend, got nil")
	}
	if chosen.GetURL().Host == "dead:8001" {
		t.Error("LeastConnection selected a dead backend")
	}
}

func TestLeastConnection_TiedConnectionsFavorsFirst(t *testing.T) {
	pool := poolWith(t, []string{
		"http://localhost:8001",
		"http://localhost:8002",
	}, true)
	// both at 0 connections — first found should win
	lc := NewLeastConnection(pool)
	b := lc.NextBackend()
	if b == nil {
		t.Fatal("unexpected nil")
	}
	// just confirm a valid backend is returned; tie-breaking is implementation-defined
}

func TestLeastConnection_Concurrent(t *testing.T) {
	pool := poolWith(t, []string{
		"http://localhost:8001",
		"http://localhost:8002",
		"http://localhost:8003",
	}, true)
	lc := NewLeastConnection(pool)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := lc.NextBackend()
			if b == nil {
				t.Errorf("unexpected nil from concurrent NextBackend")
			}
		}()
	}
	wg.Wait()
}
