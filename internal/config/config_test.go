package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTemp writes content to a temp file with the given extension and returns
// its path. The file is removed automatically when the test ends.
func writeTemp(t *testing.T, ext, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "blackflow-*"+ext)
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	return f.Name()
}

// validYAML is a minimal but complete config used across tests.
const validYAML = `
server:
  port: "9090"
  routes:
    /api:
      interval: 10s
      algorithm: round_robin
      backends:
        - http://localhost:8091
        - http://localhost:8092
    /auth:
      interval: 5s
      algorithm: least_connection
      backends:
        - http://localhost:8081
`

// --- Happy path ---

func TestLoadServerConfig_ValidYMLFile(t *testing.T) {
	path := writeTemp(t, ".yml", validYAML)
	cfg, returnedPath := LoadServerConfig(path)

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if returnedPath != path {
		t.Errorf("returnedPath = %q, want %q", returnedPath, path)
	}
	if cfg.Server.Port != "9090" {
		t.Errorf("Port = %q, want 9090", cfg.Server.Port)
	}
}

func TestLoadServerConfig_ValidYAMLExtension(t *testing.T) {
	path := writeTemp(t, ".yaml", validYAML)
	cfg, _ := LoadServerConfig(path)
	if cfg == nil {
		t.Fatal("expected non-nil config for .yaml extension")
	}
}

func TestLoadServerConfig_ParsesRoutes(t *testing.T) {
	path := writeTemp(t, ".yml", validYAML)
	cfg, _ := LoadServerConfig(path)

	if len(cfg.Server.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(cfg.Server.Routes))
	}

	api, ok := cfg.Server.Routes["/api"]
	if !ok {
		t.Fatal("missing /api route")
	}
	if api.Interval != "10s" {
		t.Errorf("/api interval = %q, want 10s", api.Interval)
	}
	if api.Algorithm != "round_robin" {
		t.Errorf("/api algorithm = %q, want round_robin", api.Algorithm)
	}
	if len(api.Backends) != 2 {
		t.Errorf("/api backends count = %d, want 2", len(api.Backends))
	}

	auth, ok := cfg.Server.Routes["/auth"]
	if !ok {
		t.Fatal("missing /auth route")
	}
	if auth.Algorithm != "least_connection" {
		t.Errorf("/auth algorithm = %q, want least_connection", auth.Algorithm)
	}
	if len(auth.Backends) != 1 {
		t.Errorf("/auth backends count = %d, want 1", len(auth.Backends))
	}
}

func TestLoadServerConfig_BackendURLsPreserved(t *testing.T) {
	path := writeTemp(t, ".yml", validYAML)
	cfg, _ := LoadServerConfig(path)

	api := cfg.Server.Routes["/api"]
	want := []string{"http://localhost:8091", "http://localhost:8092"}
	for i, got := range api.Backends {
		if got != want[i] {
			t.Errorf("backend[%d] = %q, want %q", i, got, want[i])
		}
	}
}

// --- Fallback triggers ---

func TestLoadServerConfig_EmptyPathFallsBack(t *testing.T) {
	// The default file may or may not exist on the test runner; we just verify
	// the function returns without panicking and gives a non-nil config.
	cfg, _ := LoadServerConfig("")
	if cfg == nil {
		t.Error("expected non-nil config even when falling back")
	}
}

func TestLoadServerConfig_NonExistentPathFallsBack(t *testing.T) {
	cfg, _ := LoadServerConfig("/tmp/does-not-exist-blackflow-test.yml")
	if cfg == nil {
		t.Error("expected non-nil config after fallback")
	}
}

func TestLoadServerConfig_DirectoryPathFallsBack(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := LoadServerConfig(dir)
	if cfg == nil {
		t.Error("expected non-nil config when given a directory path")
	}
}

func TestLoadServerConfig_WrongExtensionFallsBack(t *testing.T) {
	path := writeTemp(t, ".json", `{"server":{"port":"8080"}}`)
	cfg, _ := LoadServerConfig(path)
	if cfg == nil {
		t.Error("expected non-nil config after falling back from .json file")
	}
}

func TestLoadServerConfig_TxtExtensionFallsBack(t *testing.T) {
	path := writeTemp(t, ".txt", validYAML)
	cfg, _ := LoadServerConfig(path)
	if cfg == nil {
		t.Error("expected non-nil config after falling back from .txt file")
	}
}

func TestLoadServerConfig_InvalidYAMLFallsBack(t *testing.T) {
	path := writeTemp(t, ".yml", "not: valid: yaml: [[[")
	cfg, _ := LoadServerConfig(path)
	if cfg == nil {
		t.Error("expected non-nil config after YAML parse failure fallback")
	}
}

// --- Returned path ---

func TestLoadServerConfig_ReturnsExpandedPath(t *testing.T) {
	path := writeTemp(t, ".yml", validYAML)
	_, returnedPath := LoadServerConfig(path)
	if returnedPath != path {
		t.Errorf("returnedPath = %q, want %q", returnedPath, path)
	}
}

// --- Tilde expansion (expandTilde) ---

func TestExpandTilde_NoTilde(t *testing.T) {
	input := "/absolute/path"
	got, err := expandTilde(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != input {
		t.Errorf("expandTilde(%q) = %q, want %q", input, got, input)
	}
}

func TestExpandTilde_EmptyString(t *testing.T) {
	got, err := expandTilde("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expandTilde(%q) = %q, want %q", "", got, "")
	}
}

func TestExpandTilde_TildeExpands(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir, skipping tilde test")
	}
	got, err := expandTilde("~/.config/blackflow/default.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(home, ".config/blackflow/default.yml")
	if got != want {
		t.Errorf("expandTilde = %q, want %q", got, want)
	}
}

func TestExpandTilde_TildeOnlyExpands(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir, skipping tilde test")
	}
	got, err := expandTilde("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != home {
		t.Errorf("expandTilde(~) = %q, want %q", got, home)
	}
}

// --- RouteConfig struct fields ---

func TestRouteConfig_Fields(t *testing.T) {
	yaml := `
server:
  port: "8080"
  routes:
    /svc:
      interval: 30s
      algorithm: least_connection
      backends:
        - http://svc1:9000
        - http://svc2:9000
        - http://svc3:9000
`
	path := writeTemp(t, ".yml", yaml)
	cfg, _ := LoadServerConfig(path)

	svc := cfg.Server.Routes["/svc"]
	if svc.Interval != "30s" {
		t.Errorf("Interval = %q, want 30s", svc.Interval)
	}
	if svc.Algorithm != "least_connection" {
		t.Errorf("Algorithm = %q, want least_connection", svc.Algorithm)
	}
	if len(svc.Backends) != 3 {
		t.Errorf("len(Backends) = %d, want 3", len(svc.Backends))
	}
}

// --- Edge cases ---

func TestLoadServerConfig_EmptyRoutes(t *testing.T) {
	yaml := "server:\n  port: \"8080\"\n  routes:\n"
	path := writeTemp(t, ".yml", yaml)
	cfg, _ := LoadServerConfig(path)
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Server.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Server.Port)
	}
}

func TestLoadServerConfig_SingleBackend(t *testing.T) {
	yaml := `
server:
  port: "8080"
  routes:
    /only:
      interval: 10s
      algorithm: round_robin
      backends:
        - http://localhost:9000
`
	path := writeTemp(t, ".yml", yaml)
	cfg, _ := LoadServerConfig(path)
	route := cfg.Server.Routes["/only"]
	if len(route.Backends) != 1 {
		t.Errorf("expected 1 backend, got %d", len(route.Backends))
	}
	if route.Backends[0] != "http://localhost:9000" {
		t.Errorf("backend = %q, want http://localhost:9000", route.Backends[0])
	}
}
