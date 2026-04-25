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
2. `Proxy.ServeHTTP` looks up the request path in the `Registry` (longest-prefix match)
3. The route's load balancer selects a healthy backend
4. `backend.Increment()` is called to track the active connection
5. `httputil.NewSingleHostReverseProxy` forwards the request, with the Director rewriting scheme, host, path, query, and forwarding headers
6. Response is returned to the client
7. `backend.Decrement()` is deferred, releasing the connection count

---

## Project Layout

```
blackflow/
├── cmd/blackflow/main.go           # Entry point: loads config, constructs App, handles OS signals
├── configs/example.yml             # Example configuration file
├── internal/
│   ├── app/
│   │   └── app.go                  # App: wires config → pool → balancer → registry → health → HTTP server
│   ├── backend/
│   │   ├── backend.go              # Backend struct (URL, alive state, active-connection counter)
│   │   └── iface.go                # Instance and Provider interfaces
│   ├── balancer/
│   │   └── balancer.go             # Balancer interface + RoundRobin / LeastConnection impls
│   ├── config/
│   │   └── config.go               # YAML config loading, tilde expansion, default-file creation
│   ├── health/
│   │   ├── check.go                # Per-backend HTTP health check logic
│   │   └── health.go               # Manager: runs health checks on a ticker, supports graceful stop
│   ├── pool/
│   │   └── pool.go                 # Thread-safe backend pool
│   ├── proxy/
│   │   └── proxy.go                # Proxy (ServeHTTP, reverse proxy wiring, forwarding headers)
│   └── registry/
│       ├── registery.go            # Registry: thread-safe route map with longest-prefix Match
│       └── route.go                # Route struct binding a prefix to a Balancer
├── docs/
│   └── architecture.md
├── Makefile
└── go.mod                          # Module: github.com/Harsho-afk/blackflow
```

---

## Core Components

### 1. App (`internal/app/app.go`)

The application bootstrap layer. `New(cfg)` iterates the config routes, constructing a `Pool`, `Balancer`, and `Route` for each, registers them with a `Registry` and `health.Manager`, then creates the `http.Server`. This logic was previously spread across `main.go`.

| Method | Behaviour |
|---|---|
| `New(cfg)` | Wires all components; returns a ready-to-start `*App` |
| `Start()` | Calls `server.ListenAndServe` in a background goroutine |
| `Shutdown(ctx)` | Cancels the health manager context, waits for checkers to stop, then drains the HTTP server |

### 2. Proxy (`internal/proxy/proxy.go`)

The top-level HTTP handler. Delegates route lookup to the `Registry` and constructs a fresh `httputil.NewSingleHostReverseProxy` per request, customising its `Director` to preserve the original path and query and inject standard forwarding headers.

**Key behaviour:**
- Returns `404` if no route matches
- Returns `503` if the balancer returns no healthy backend
- Returns `502` (Bad Gateway) via `proxy.ErrorHandler` if the upstream call fails
- Sets `X-Forwarded-Host`, `X-Forwarded-Proto`, and `X-Forwarded-For` headers

### 3. Registry (`internal/registry/`)

Thread-safe map of URL prefix → `*Route`. Used by `Proxy` to resolve incoming paths.

| Method | Notes |
|---|---|
| `Add(*Route)` | Inserts or replaces under write lock |
| `Remove(prefix)` | Deletes under write lock |
| `Match(path) *Route` | Returns the route with the longest matching prefix (read lock) |

### 4. Route (`internal/registry/route.go`)

A plain struct binding a URL prefix to its `Balancer`.

| Field | Type | Purpose |
|---|---|---|
| `Prefix` | `string` | URL path prefix (e.g. `/auth`) |
| `Balancer` | `balancer.Balancer` | Load-balancing strategy for this prefix |

### 5. Pool (`internal/pool/pool.go`)

Thread-safe container for `[]*backend.Backend`.

| Method | Notes |
|---|---|
| `AddBackend(*Backend)` | Appends under write lock |
| `RemoveBackend(string)` | Filters by URL string equality under write lock |
| `GetBackends() []backend.Instance` | Returns a snapshot (read lock); implements `backend.Provider` |
| `Load([]string) error` | Parses URLs and appends backends; health checks run separately via `health.Manager` |

### 6. Backend (`internal/backend/`)

Represents a single upstream service instance. The `Instance` interface in `iface.go` is what the rest of the system depends on, keeping the pool, balancer, and health packages decoupled from the concrete type.

| Concern | Mechanism |
|---|---|
| URL | Protected by `sync.RWMutex` via `GetURL` / `SetURL` |
| Alive flag | Protected by `sync.RWMutex` via `IsAlive` / `SetAlive` |
| Active connections | `int64` field manipulated with `sync/atomic` (`Increment`, `Decrement`, `GetActiveConnections`) |

**Interfaces (`iface.go`):**

```go
type Instance interface {
    IsAlive() bool
    SetAlive(bool)
    GetURL() *url.URL
    GetActiveConnections() int64
    Increment()
    Decrement()
}

type Provider interface {
    GetBackends() []Instance
}
```

### 7. Balancer (`internal/balancer/balancer.go`)

```go
type Balancer interface {
    NextBackend() Backend
    GetAlgorithm() string
}
```

`NewBalancer(pool, algo)` is a factory returning the correct implementation. Unknown algorithm strings fall back to Round Robin.

| Algorithm | Implementation |
|---|---|
| `round_robin` | Atomic `uint64` counter; iterates all backends once, skipping unhealthy |
| `least_connection` | Linear scan; picks the alive backend with the fewest active connections |

### 8. Health Manager (`internal/health/`)

`Manager` (in `health.go`) runs one goroutine per registered pool. Each goroutine performs an immediate check on startup, then polls on a `time.Ticker`.

`checkProvider` / `checkBackend` (in `check.go`) issue `GET <backend-url>/health` with a 2-second HTTP timeout. A `2xx–4xx` response marks the backend alive; a `5xx` or network error marks it dead.

| Aspect | Detail |
|---|---|
| HTTP client timeout | 2 seconds |
| Alive condition | HTTP status 200–499 → alive; 500+ or error → not alive |
| Startup check | `Register` calls `checkProvider` synchronously before starting the ticker |
| Shutdown | `Stop()` cancels the shared context and calls `wg.Wait()` — all checker goroutines exit cleanly |

---

## Configuration (`internal/config/config.go`)

Config is loaded from a YAML file at a user-supplied path. On any failure (path missing, wrong extension, parse error) it falls back to `~/.config/blackflow/default.yml`, creating the file with a minimal skeleton if it does not exist.

```yaml
server:
  port: 8080
  routes:
    /auth:
      interval: 10s           # Health check interval (minimum 1s enforced at load time)
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
| `interval` | `time.Duration` | Health-check polling interval (< 1s is clamped to 5s) |
| `algorithm` | `string` | Load-balancing algorithm name |
| `backends` | `[]string` | List of backend URLs |

---

## Startup Sequence (`cmd/blackflow/main.go` + `internal/app/app.go`)

1. Parse optional config-file path from `os.Args`
2. Load `Config` from YAML (or default fallback)
3. `app.New(cfg)` builds all components:
   - For each route: create `Pool`, load backends, create `Balancer`, register with `Registry` and `health.Manager`
4. `app.Start()` — starts HTTP server in a background goroutine
5. Block on `SIGINT` / `SIGTERM`
6. `app.Shutdown(ctx)` — cancels health manager, waits for checker goroutines, then gracefully drains the HTTP server (5-second timeout)

---

## Concurrency Model

| Concern | Mechanism |
|---|---|
| Backend pool access | `sync.RWMutex` (read-heavy workloads) |
| Active connection count | `sync/atomic` (`int64`) |
| Alive flag | `sync.RWMutex` |
| Round-robin counter | `sync/atomic` (`uint64`) |
| Registry route map | `sync.RWMutex` |
| Health checks | One goroutine per pool via `health.Manager`; stopped via context cancellation + `sync.WaitGroup` |
| HTTP requests | Each handled in its own goroutine by `net/http` |

---

## Error Handling

| Condition | Response |
|---|---|
| No matching route | `404 Not Found` |
| No healthy backend | `503 Service Unavailable` ("No healthy backend") |
| Upstream call failure | `502 Bad Gateway` ("Bad Gateway") |
| Invalid backend URL | `500 Internal Server Error` ("Invalid backend URL") |
| Backend health-check failure | Backend marked not alive; logged |
| Config parse failure | Falls back to default config |
| Server listen error | `log.Fatalf` (fatal) |
| Shutdown timeout exceeded | Forced shutdown logged |

---

## Current Capabilities

- Reverse proxying via `httputil.NewSingleHostReverseProxy`
- Multi-route support with longest-prefix matching
- Round Robin load balancing (atomic counter, skips unhealthy backends)
- Least Connections load balancing (adapts to runtime load)
- Active health checks (periodic HTTP polling, configurable interval)
- Startup health checks (backends probed before traffic is accepted)
- Graceful shutdown (OS signal handling, context cancellation, `WaitGroup` drain)
- Forwarding headers (`X-Forwarded-Host`, `X-Forwarded-Proto`, `X-Forwarded-For`)
- YAML-driven configuration with tilde expansion and default-file fallback
- Interface-driven design (`backend.Instance`, `backend.Provider`) for testability
