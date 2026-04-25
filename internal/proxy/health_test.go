package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// newBackendFromServer creates a Backend pointing at a test server.
func newBackendFromServer(t *testing.T, server *httptest.Server) *Backend {
	t.Helper()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	b := &Backend{}
	b.SetURL(u)
	return b
}

// --- NewHealthChecker ---

func TestNewHealthChecker_DefaultValues(t *testing.T) {
	pool := NewPool()
	h := NewHealthChecker(pool, 5*time.Second)
	if h.GetInterval() != "5s" {
		t.Errorf("GetInterval() = %q, want 5s", h.GetInterval())
	}
}

// --- SetInterval ---

func TestHealthChecker_SetInterval_Valid(t *testing.T) {
	pool := NewPool()
	h := NewHealthChecker(pool, 5*time.Second)
	if err := h.SetInterval(10 * time.Second); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if h.GetInterval() != "10s" {
		t.Errorf("GetInterval() = %q, want 10s", h.GetInterval())
	}
}

func TestHealthChecker_SetInterval_ExactlyOneSecond(t *testing.T) {
	pool := NewPool()
	h := NewHealthChecker(pool, 5*time.Second)
	if err := h.SetInterval(time.Second); err != nil {
		t.Errorf("1s should be a valid interval, got error: %v", err)
	}
}

func TestHealthChecker_SetInterval_TooShort(t *testing.T) {
	pool := NewPool()
	h := NewHealthChecker(pool, 5*time.Second)
	if err := h.SetInterval(500 * time.Millisecond); err == nil {
		t.Error("expected error for interval < 1s")
	}
}

func TestHealthChecker_SetInterval_Zero(t *testing.T) {
	pool := NewPool()
	h := NewHealthChecker(pool, 5*time.Second)
	if err := h.SetInterval(0); err == nil {
		t.Error("expected error for zero interval")
	}
}

// --- checkBackend (via a real test HTTP server) ---

func TestHealthChecker_CheckBackend_200MarksAlive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pool := NewPool()
	h := NewHealthChecker(pool, 5*time.Second)
	b := newBackendFromServer(t, srv)
	h.checkBackend(b)

	if !b.IsAlive() {
		t.Error("backend should be alive after 200 response")
	}
}

func TestHealthChecker_CheckBackend_404MarksAlive(t *testing.T) {
	// 200–499 → alive
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	pool := NewPool()
	h := NewHealthChecker(pool, 5*time.Second)
	b := newBackendFromServer(t, srv)
	h.checkBackend(b)

	if !b.IsAlive() {
		t.Error("backend should be alive for 404 (within 200-499 range)")
	}
}

func TestHealthChecker_CheckBackend_500MarksDead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	pool := NewPool()
	h := NewHealthChecker(pool, 5*time.Second)
	b := newBackendFromServer(t, srv)
	b.SetAlive(true) // start alive
	h.checkBackend(b)

	if b.IsAlive() {
		t.Error("backend should be dead after 500 response")
	}
}

func TestHealthChecker_CheckBackend_UnreachableMarksDead(t *testing.T) {
	u, _ := url.Parse("http://127.0.0.1:19990") // nothing listening here
	b := &Backend{}
	b.SetURL(u)
	b.SetAlive(true)

	pool := NewPool()
	h := NewHealthChecker(pool, 5*time.Second)
	h.checkBackend(b)

	if b.IsAlive() {
		t.Error("backend should be dead when unreachable")
	}
}

func TestHealthChecker_CheckBackend_VerifiesHealthPath(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			called = true
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	pool := NewPool()
	h := NewHealthChecker(pool, 5*time.Second)
	b := newBackendFromServer(t, srv)
	h.checkBackend(b)

	if !called {
		t.Error("health checker should request /health endpoint")
	}
}

// --- checkAll ---

func TestHealthChecker_CheckAll_UpdatesMultipleBackends(t *testing.T) {
	alive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer alive.Close()

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()

	pool := NewPool()

	bAlive := newBackendFromServer(t, alive)
	bAlive.SetAlive(false) // start inverted
	pool.AddBackend(bAlive)

	bDead := newBackendFromServer(t, dead)
	bDead.SetAlive(true) // start inverted
	pool.AddBackend(bDead)

	h := NewHealthChecker(pool, 5*time.Second)
	h.checkAll()

	// checkAll spawns goroutines; give them a moment
	time.Sleep(100 * time.Millisecond)

	if !bAlive.IsAlive() {
		t.Error("backend with 200 response should be alive")
	}
	if bDead.IsAlive() {
		t.Error("backend with 500 response should be dead")
	}
}

// --- Start (smoke test) ---

func TestHealthChecker_Start_DoesNotPanic(t *testing.T) {
	pool := NewPool()
	h := NewHealthChecker(pool, 2*time.Second)
	// Just ensure Start() returns without panicking; we can't easily stop the
	// goroutine without a Stop() method, but the pool is empty so it's a no-op.
	h.Start()
}
