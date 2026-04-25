package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// buildRoute creates a Route with a real backend server, a live pool, and the
// specified balancer algorithm. The caller is responsible for closing the server.
func buildRoute(t *testing.T, prefix string, algo string, srv *httptest.Server) *Route {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", srv.URL, err)
	}
	pool := NewPool()
	b := &Backend{}
	b.SetURL(u)
	b.SetAlive(true)
	pool.AddBackend(b)
	balancer := NewBalancer(pool, algo)
	health := NewHealthChecker(pool, 5*60*1000*1000*1000) // 5-min interval, won't fire
	return &Route{
		Prefix:        prefix,
		Pool:          pool,
		Balancer:      balancer,
		HealthChecker: health,
	}
}

// newEchoServer returns an httptest.Server that writes 200 + "OK:<path>" for
// every request, making it easy to verify path rewriting.
func newEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK:" + r.URL.Path))
	}))
}

// --- NewProxy ---

func TestNewProxy_NoRoutes(t *testing.T) {
	p, err := NewProxy([]*Route{})
	if err != nil {
		t.Fatalf("NewProxy returned error: %v", err)
	}
	if p == nil {
		t.Fatal("NewProxy returned nil proxy")
	}
}

func TestNewProxy_WithRoutes(t *testing.T) {
	srv := newEchoServer(t)
	defer srv.Close()

	route := buildRoute(t, "/api", "round_robin", srv)
	p, err := NewProxy([]*Route{route})
	if err != nil {
		t.Fatalf("NewProxy returned error: %v", err)
	}
	if len(p.Routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(p.Routes))
	}
}

// --- matchRoute ---

func TestProxy_MatchRoute_ExactPrefix(t *testing.T) {
	srv := newEchoServer(t)
	defer srv.Close()

	route := buildRoute(t, "/api", "round_robin", srv)
	p, _ := NewProxy([]*Route{route})

	got := p.matchRoute("/api/users")
	if got == nil {
		t.Fatal("expected route match, got nil")
	}
	if got.Prefix != "/api" {
		t.Errorf("matched route prefix = %q, want /api", got.Prefix)
	}
}

func TestProxy_MatchRoute_NoMatch(t *testing.T) {
	srv := newEchoServer(t)
	defer srv.Close()

	route := buildRoute(t, "/api", "round_robin", srv)
	p, _ := NewProxy([]*Route{route})

	if got := p.matchRoute("/auth/login"); got != nil {
		t.Errorf("expected nil for unmatched path, got route with prefix %q", got.Prefix)
	}
}

func TestProxy_MatchRoute_RootPrefix(t *testing.T) {
	srv := newEchoServer(t)
	defer srv.Close()

	route := buildRoute(t, "/", "round_robin", srv)
	p, _ := NewProxy([]*Route{route})

	if got := p.matchRoute("/anything"); got == nil {
		t.Error("root prefix / should match any path")
	}
}

func TestProxy_MatchRoute_FirstMatchWins(t *testing.T) {
	srv := newEchoServer(t)
	defer srv.Close()

	r1 := buildRoute(t, "/api", "round_robin", srv)
	r2 := buildRoute(t, "/api/v2", "round_robin", srv)
	p, _ := NewProxy([]*Route{r1, r2})

	got := p.matchRoute("/api/v2/items")
	if got == nil {
		t.Fatal("expected match, got nil")
	}
	// iteration order: r1 (/api) is checked first and matches
	if got.Prefix != "/api" {
		t.Errorf("first-registered route should win, got prefix %q", got.Prefix)
	}
}

func TestProxy_MatchRoute_EmptyRoutes(t *testing.T) {
	p, _ := NewProxy([]*Route{})
	if got := p.matchRoute("/api"); got != nil {
		t.Error("matchRoute on empty Routes should return nil")
	}
}

// --- ServeHTTP: 404 ---

func TestProxy_ServeHTTP_404_NoMatchingRoute(t *testing.T) {
	srv := newEchoServer(t)
	defer srv.Close()

	route := buildRoute(t, "/api", "round_robin", srv)
	p, _ := NewProxy([]*Route{route})

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unmatched route, got %d", rec.Code)
	}
}

// --- ServeHTTP: 503 ---

func TestProxy_ServeHTTP_503_NoHealthyBackend(t *testing.T) {
	pool := NewPool()
	u, _ := url.Parse("http://localhost:8001")
	b := &Backend{}
	b.SetURL(u)
	b.SetAlive(false) // all dead
	pool.AddBackend(b)

	route := &Route{
		Prefix:        "/api",
		Pool:          pool,
		Balancer:      NewBalancer(pool, "round_robin"),
		HealthChecker: NewHealthChecker(pool, 5*60*1000*1000*1000),
	}
	p, _ := NewProxy([]*Route{route})

	req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 with no healthy backend, got %d", rec.Code)
	}
}

// --- ServeHTTP: successful proxy ---

func TestProxy_ServeHTTP_200_ForwardsRequest(t *testing.T) {
	srv := newEchoServer(t)
	defer srv.Close()

	route := buildRoute(t, "/api", "round_robin", srv)
	p, _ := NewProxy([]*Route{route})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- Path rewriting ---

func TestProxy_ServeHTTP_StripsPrefixFromPath(t *testing.T) {
	srv := newEchoServer(t)
	defer srv.Close()

	route := buildRoute(t, "/api", "round_robin", srv)
	p, _ := NewProxy([]*Route{route})

	req := httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "/users/42") {
		t.Errorf("backend should receive /users/42 after prefix strip, body = %q", body)
	}
	if strings.Contains(body, "/api") {
		t.Errorf("prefix /api should be stripped before forwarding, body = %q", body)
	}
}

func TestProxy_ServeHTTP_ExactPrefixBecomesSlash(t *testing.T) {
	srv := newEchoServer(t)
	defer srv.Close()

	route := buildRoute(t, "/api", "round_robin", srv)
	p, _ := NewProxy([]*Route{route})

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	// After stripping /api from /api we get "" which should be rewritten to /
	if !strings.Contains(body, "OK:/") {
		t.Errorf("exact prefix path should forward as /, body = %q", body)
	}
}

// --- Active connection counter ---

func TestProxy_ServeHTTP_DecrementsConnectionAfterRequest(t *testing.T) {
	srv := newEchoServer(t)
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	pool := NewPool()
	b := &Backend{}
	b.SetURL(u)
	b.SetAlive(true)
	pool.AddBackend(b)

	route := &Route{
		Prefix:        "/api",
		Pool:          pool,
		Balancer:      NewBalancer(pool, "round_robin"),
		HealthChecker: NewHealthChecker(pool, 5*60*1000*1000*1000),
	}
	p, _ := NewProxy([]*Route{route})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if got := b.GetActiveConnections(); got != 0 {
		t.Errorf("active connections should be 0 after request completes, got %d", got)
	}
}

// --- Multiple routes ---

func TestProxy_ServeHTTP_RoutesMultiplePrefixes(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("api-backend"))
	}))
	defer apiSrv.Close()

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("auth-backend"))
	}))
	defer authSrv.Close()

	p, _ := NewProxy([]*Route{
		buildRoute(t, "/api", "round_robin", apiSrv),
		buildRoute(t, "/auth", "round_robin", authSrv),
	})

	cases := []struct {
		path    string
		wantBody string
	}{
		{"/api/items", "api-backend"},
		{"/auth/login", "auth-backend"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		if !strings.Contains(rec.Body.String(), tc.wantBody) {
			t.Errorf("path %q: body = %q, want %q", tc.path, rec.Body.String(), tc.wantBody)
		}
	}
}

// --- HTTP methods ---

func TestProxy_ServeHTTP_ForwardsPostRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	route := buildRoute(t, "/api", "round_robin", srv)
	p, _ := NewProxy([]*Route{route})

	req := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
}

// --- Panic recovery ---

func TestProxy_ServeHTTP_RecoversPanic(t *testing.T) {
	// Build a balancer that panics on NextBackend.
	pool := NewPool()
	route := &Route{
		Prefix:        "/panic",
		Pool:          pool,
		Balancer:      &panicBalancer{},
		HealthChecker: NewHealthChecker(pool, 5*60*1000*1000*1000),
	}
	p, _ := NewProxy([]*Route{route})

	req := httptest.NewRequest(http.MethodGet, "/panic/test", nil)
	rec := httptest.NewRecorder()

	// Should not panic the test; recover() in ServeHTTP catches it.
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after panic recovery, got %d", rec.Code)
	}
}

// panicBalancer is a test-only Balancer that panics.
type panicBalancer struct{}

func (pb *panicBalancer) NextBackend() *Backend { panic("intentional test panic") }
func (pb *panicBalancer) GetAlgorithm() string  { return "panic" }

// --- Header forwarding ---

func TestProxy_ServeHTTP_ForwardsRequestHeaders(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	route := buildRoute(t, "/api", "round_robin", srv)
	p, _ := NewProxy([]*Route{route})

	req := httptest.NewRequest(http.MethodGet, "/api/secure", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if receivedAuth != "Bearer token123" {
		t.Errorf("backend received Authorization = %q, want %q", receivedAuth, "Bearer token123")
	}
}
