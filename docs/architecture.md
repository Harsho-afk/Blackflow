# Blackflow Architecture

## Overview

Blackflow is a reverse proxy and load balancer written in Go (1.22+).  
It routes incoming HTTP requests to backend services based on URL path prefixes and distributes load using pluggable balancing strategies. Configuration is driven by a YAML file, and the server supports graceful shutdown.

---

## Request Flow

```
Client -> Blackflow (ServeHTTP) -> Route Matching -> Load Balancer -> Backend -> Response
```

### Step-by-step

1. Client sends an HTTP request to Blackflow
2. `Proxy.ServeHTTP` matches the request path to a configured route (longest-prefix match, in iteration order)
3. The route's load balancer selects a healthy backend
4. `backend.Increment()` is called to track the active connection
5. `httputil.ReverseProxy` forwards the request (scheme and host rewritten to the selected backend)
6. Response is returned to the client
7. `backend.Decrement()` is deferred, releasing the connection count

---

## Project Layout

```
blackflow/
├── cmd/blackflow/main.go       # Entry point: wires config → routes → proxy → HTTP server
├── config/
│   ├── config.go               # YAML config loading, tilde expansion, default-file creation
│   └── config.yml              # Example config
├── internal/proxy/
│   ├── backend.go              # Backend struct (URL, alive state, active-connection counter)
│   ├── balancer.go             # Balancer interface + RoundRobin / LeastConnection impls
│   ├── health.go               # HealthChecker (periodic HTTP GET /health polling)
│   ├── pool.go                 # Thread-safe backend pool
│   └── proxy.go                # Proxy (ServeHTTP, route matching, ReverseProxy wiring)
├── Makefile
└── go.mod                      # Module: github.com/Harsho-afk/blackflow
```

---

## Core Components

### 1. Proxy (`internal/proxy/proxy.go`)

The top-level HTTP handler. Holds a slice of `*Route` values and a single shared `httputil.ReverseProxy` instance whose `Director` is a no-op (URL rewriting happens in `ServeHTTP` before the call is delegated).

**Key behaviour:**
- `matchRoute(path)` iterates routes and returns the first whose `Prefix` is a prefix of the request path
- Returns `404` if no route matches; `503` if the selected balancer returns no healthy backend
- Rewrites `req.URL.Scheme` and `req.URL.Host` in-place before forwarding

### 2. Route (`internal/proxy/proxy.go`)

A plain struct that binds a URL prefix to its runtime dependencies:

| Field | Type | Purpose |
|---|---|---|
| `Prefix` | `string` | URL path prefix (e.g. `/auth`) |
| `Pool` | `*Pool` | Set of backend instances |
| `Balancer` | `Balancer` | Load-balancing strategy |
| `HealthChecker` | `*HealthChecker` | Periodic health polling for this pool |

Routes are constructed in `main.go` from the YAML config and passed to `NewProxy`.

### 3. Pool (`internal/proxy/pool.go`)

Thread-safe container for `[]*Backend`.

| Method | Notes |
|---|---|
| `AddBackend(*Backend)` | Appends under write lock |
| `RemoveBackend(any)` | Accepts `*Backend` or `*url.URL`; filters by URL string equality |
| `GetBackends() []BackendInfo` | Returns a snapshot (read lock); safe for external consumers |
| `getBackends() []*Backend` | Internal; returns the live slice (read lock) |
| `LoadBackends([]string)` | Parses URLs, runs an immediate health check on each new backend, then appends under write lock |

A `BackendInfo` value-type snapshot is exposed publicly to avoid leaking mutable `*Backend` pointers.

### 4. Backend (`internal/proxy/backend.go`)

Represents a single upstream service instance.

| Concern | Mechanism |
|---|---|
| URL | Protected by `sync.RWMutex` via `GetURL` / `SetURL` |
| Alive flag | Protected by `sync.RWMutex` via `IsAlive` / `SetAlive` |
| Active connections | `int64` field manipulated with `sync/atomic` (`Increment`, `Decrement`, `GetActiveConnections`) |


### 5. Balancer (`internal/proxy/balancer.go`)

```go
type Balancer interface {
    NextBackend() *Backend
    GetAlgorithm() string
}
```

`NewBalancer(pool, algo)` is a factory that returns the correct implementation based on the `algorithm` string from config. Unknown values fall back to Round Robin.

### 6. Health Checker (`internal/proxy/health.go`)

Periodically polls each backend's `/health` endpoint.

| Aspect | Detail |
|---|---|
| HTTP client timeout | 2 seconds |
| Check trigger | `time.NewTicker(interval)` in a background goroutine |
| Alive condition | HTTP status 200–499 → alive; 500+ or error → not alive |
| Startup check | `LoadBackends` calls `checkBackend` synchronously for each new backend before adding it to the pool |
| Minimum interval | Enforced in `main.go` (≥ 1 s) and via `SetInterval` (returns error if < 1 s) |

> The ticker goroutine started by `Start()` has no stop mechanism. Adding a `Stop()` method with a done channel would allow clean shutdown of health-check goroutines.

---

## Configuration (`config/config.go`)

Config is loaded from a YAML file at a user-supplied path. On any failure (path missing, wrong extension, parse error) it falls back to `~/.config/blackflow/default.yml`, creating the file with a minimal skeleton if it does not exist.

```yaml
# config/config.yml (example)
server:
  port: 8080
  routes:
    /auth:
      interval: 10s          # Health check interval (minimum 1s enforced at runtime)
      algorithm: round_robin  # round_robin | least_connection
      backends:
        - http://localhost:8081
        - http://localhost:8082
    /api:
      interval: 10s
      algorithm: least_connection
      backends:
        - http://localhost:8091
```

`RouteConfig` fields:

| Field | Type | Description |
|---|---|---|
| `interval` | `string` | `time.Duration` string for health-check polling |
| `algorithm` | `string` | Load-balancing algorithm name |
| `backends` | `[]string` | List of backend URLs |

---

## Startup Sequence (`cmd/blackflow/main.go`)

1. Parse optional config-file path from `os.Args`
2. Load `ServerConfig` from YAML (or default)
3. For each route in `config.Server.Routes`:
   - Create a `Pool` and load backends (with immediate health checks)
   - Create a `Balancer` for the configured algorithm
   - Parse the health-check interval; clamp to ≥ 1 s
   - Create a `HealthChecker` and wrap everything in a `*Route`
4. Construct the `Proxy` from the route slice
5. Log the endpoint mapping (prefix, algorithm, interval, backends)
6. Start each route's `HealthChecker` goroutine
7. Start the HTTP server in a goroutine
8. Block on `SIGINT` / `SIGTERM`; graceful shutdown with a 1-second context timeout

---

## Concurrency Model

| Concern | Mechanism |
|---|---|
| Backend pool access | `sync.RWMutex` (read-heavy workloads) |
| Active connection count | `sync/atomic` (`int64`) |
| Alive flag | `sync.RWMutex` |
| Round-robin counter | `sync/atomic` (`uint64`) |
| Health checks | One goroutine per pool, spawns per-backend goroutines on each tick |
| HTTP requests | Each handled in its own goroutine by `net/http` |

---

## Error Handling

| Condition | Response |
|---|---|
| No matching route | `404 Not Found` |
| No healthy backend | `503 Service Unavailable` (message: "No healty backend") |
| Backend health-check failure | Backend marked not alive; logged |
| Config parse failure | Falls back to default config |
| Server listen error | `log.Fatalf` (fatal) |
| Shutdown timeout exceeded | Forced shutdown logged |

---

## Current Capabilities

- Reverse proxying via `httputil.ReverseProxy`
- Multi-route support (prefix-based matching)
- Round Robin load balancing (atomic, skips unhealthy backends)
- Least Connections load balancing
- Active health checks (periodic HTTP polling, configurable interval)
- Startup health checks (backends probed before traffic is accepted)
- Graceful shutdown (OS signal handling, configurable drain timeout)
- YAML-driven configuration with tilde expansion and default-file fallback
