# Blackflow Architecture

## Overview

Blackflow is a reverse proxy and load balancer written in Go (1.22+).  
It routes incoming HTTP requests to backend services based on URL path prefixes and distributes load using pluggable balancing strategies. Configuration is driven by a TOML file, and the server supports graceful shutdown.

---

## Request Flow

```
Client -> Recover -> Logging -> Metrics -> Proxy -> Route Matching -> Load Balancer -> Backend -> Response
```

### Step-by-step

1. Client sends an HTTP request to Blackflow
2. `Recover` middleware wraps the request — catches any downstream panic and returns 500
3. `Logging` middleware records the start time and wraps the response writer
4. `Metrics` middleware hook
5. `Proxy.ServeHTTP` looks up the request path in the `Registry` (longest-prefix match)
6. The route's load balancer selects a healthy backend
7. `backend.Increment()` is called to track the active connection
8. `httputil.NewSingleHostReverseProxy` forwards the request, with the Director rewriting scheme, host, path, query, and forwarding headers
9. Response is returned to the client
10. `backend.Decrement()` is deferred, releasing the connection count
11. `Logging` middleware emits a structured slog record (method, path, status, duration_ms, remote_addr, user_agent)

---

## Project Layout

```
blackflow/
├── cmd/blackflow/main.go           # Entry point: logging setup, config load, OS signals
├── internal/
│   ├── app/
│   │   └── app.go                  # App: wires config → pool → balancer → registry → health → middleware → HTTP server
│   ├── backend/
│   │   ├── backend.go              # Backend struct (URL, alive state, active-connection counter)
│   │   └── iface.go                # Instance and Provider interfaces
│   ├── balancer/
│   │   └── balancer.go             # Balancer interface + RoundRobin / LeastConnection / IPHash impls
│   ├── config/
│   │   └── config.go               # TOML config loading, tilde expansion, default-file creation
│   ├── health/
│   │   ├── check.go                # Per-backend HTTP health check logic, state-change logging
│   │   └── health.go               # Manager: runs health checks on a ticker, supports graceful stop
│   ├── middleware/
│   │   ├── middleware.go           # Middleware type + Chain() composer
│   │   ├── recover.go              # Panic recovery → 500, logs stack trace
│   │   ├── logging.go              # Structured per-request logging via slog
│   │   ├── metrics.go              # Metrics hook stub
│   │   ├── ratelimit.go            # Token-bucket per-IP rate limiter
│   │   └── responsewriter.go       # http.ResponseWriter wrapper to capture status code
│   ├── pool/
│   │   └── pool.go                 # Thread-safe backend pool
│   ├── proxy/
│   │   └── proxy.go                # Proxy (ServeHTTP, reverse proxy wiring, forwarding headers)
│   └── registry/
│       ├── registery.go            # Registry: thread-safe route map with longest-prefix Match
│       └── route.go                # Route struct binding a prefix to a Balancer
├── docs/
│   ├── architecture.md
│   └── example-configs/
│       └── example.toml            # Example configuration file
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── go.mod                          # Module: github.com/Harsho-afk/blackflow
```

---

## Core Components

### 1. App (`internal/app/app.go`)

The application bootstrap layer. `New(cfg)` iterates the config routes, constructing a `Pool`, `Balancer`, and `Route` for each, registers them with a `Registry` and `health.Manager`, then wraps the proxy in the middleware chain and creates the `http.Server`.

| Method | Behaviour |
|---|---|
| `New(cfg)` | Wires all components; returns a ready-to-start `*App` |
| `Start()` | Calls `server.ListenAndServe` in a background goroutine |
| `Shutdown(ctx)` | Cancels the health manager context, waits for checkers to stop, then drains the HTTP server |

### 2. Middleware (`internal/middleware/`)

A composable middleware chain wrapping the proxy handler. Applied in `app.New()` via `Chain()`.

```
Recover → Logging → Metrics → Proxy
```

| Middleware | Behaviour |
|---|---|
| `Recover` | Deferred panic catch; logs stack trace via slog; returns 500 |
| `Logging` | Wraps `ResponseWriter` to capture status; logs method, path, status, duration_ms, remote_addr, user_agent after response |
| `Metrics` | Stub — wired into chain now |

`Chain(h, A, B, C)` applies middlewares right-to-left so execution order is left-to-right: A -> B -> C -> h.

`responseWriter` wraps `http.ResponseWriter` to intercept `WriteHeader` and capture the status code, defaulting to 200 if the handler never calls it explicitly.

### 3. Proxy (`internal/proxy/proxy.go`)

The inner HTTP handler. Delegates route lookup to the `Registry` and constructs a fresh `httputil.NewSingleHostReverseProxy` per request, customising its `Director` to preserve the original path and query and inject standard forwarding headers.

**Key behaviour:**
- Returns `404` if no route matches
- Returns `503` if the balancer returns no healthy backend
- Returns `502` (Bad Gateway) via `proxy.ErrorHandler` if the upstream call fails
- Sets `X-Forwarded-Host`, `X-Forwarded-Proto`, and `X-Forwarded-For` headers

### 4. Registry (`internal/registry/`)

Thread-safe map of URL prefix → `*Route`. Used by `Proxy` to resolve incoming paths.

| Method | Notes |
|---|---|
| `Add(*Route)` | Inserts or replaces under write lock |
| `Remove(prefix)` | Deletes under write lock |
| `Match(path) *Route` | Returns the route with the longest matching prefix (read lock) |

### 5. Route (`internal/registry/route.go`)

A plain struct binding a URL prefix to its `Balancer`.

| Field | Type | Purpose |
|---|---|---|
| `Prefix` | `string` | URL path prefix (e.g. `/auth`) |
| `Balancer` | `balancer.Balancer` | Load-balancing strategy for this prefix |

### 6. Pool (`internal/pool/pool.go`)

Thread-safe container for `[]*backend.Backend`.

| Method | Notes |
|---|---|
| `AddBackend(*Backend)` | Appends under write lock |
| `RemoveBackend(string)` | Filters by URL string equality under write lock |
| `GetBackends() []backend.Instance` | Returns a snapshot (read lock); implements `backend.Provider` |
| `Load([]string) error` | Parses URLs and appends backends; health checks run separately via `health.Manager` |

### 7. Backend (`internal/backend/`)

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

### 8. Balancer (`internal/balancer/balancer.go`)

```go
type Balancer interface {
    NextBackend(key string) Backend
    GetAlgorithm() string
}
```

`key` is an algorithm-specific routing key — currently the client IP, computed once per request in `proxy.go`. Round Robin and Least Connection ignore it; IPHash uses it for affinity.

`NewBalancer(pool, algo)` is a factory returning the correct implementation. Unknown algorithm strings fall back to Round Robin.

| Algorithm | Implementation |
|---|---|
| `round_robin` | Atomic `uint64` counter; iterates all backends once, skipping unhealthy |
| `least_connection` | Linear scan; picks the alive backend with the fewest active connections |
| `ip_hash` | FNV-1a hash of the client IP mod pool size selects a start index; probes forward through the pool in fixed order if that backend is unhealthy |

### 9. Health Manager (`internal/health/`)

`Manager` (in `health.go`) runs one goroutine per registered pool. Each goroutine performs an immediate check on startup, then polls on a `time.Ticker`.

`checkProvider` / `checkBackend` (in `check.go`) issue `GET <backend-url>/health` with a 2-second HTTP timeout. State changes are logged — a backend going down logs `WARN`, coming back up logs `INFO`. Stable state produces no log output.

| Aspect | Detail |
|---|---|
| HTTP client timeout | 2 seconds |
| Alive condition | HTTP status 200–499 → alive; 500+ or error → dead |
| Logging | Only on state change (up→down or down→up) |
| Startup check | Runs inside goroutine immediately before first ticker tick |
| Shutdown | `Stop()` cancels the shared context and calls `wg.Wait()` |

---

## Configuration (`internal/config/config.go`)

Config is loaded from a TOML file at a user-supplied path. On any failure (path missing, wrong extension, parse error) it falls back to `~/.config/blackflow/default.toml`, creating the file with a minimal skeleton if it does not exist.

```toml
[server]
port = 8080

[server.routes."/auth"]
interval = "10s"          # Health check interval (< 1s clamped to 5s)
algorithm = "round_robin" # round_robin | least_connection | ip_hash
backends = ["http://localhost:8081", "http://localhost:8082"]

[server.routes."/api"]
interval = "10s"
algorithm = "least_connection"
backends = ["http://localhost:8091"]
```

---

## Logging

All logging goes through `log/slog` to stdout. Configured via environment variables at startup.

| Variable | Values | Default |
|---|---|---|
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` |
| `LOG_FORMAT` | `text`, `json` | `text` |

```bash
# local dev
LOG_LEVEL=debug make run configs/example.toml

# production
LOG_LEVEL=warn LOG_FORMAT=json ./bin/blackflow configs/example.toml
```

**What gets logged:**

| Event | Level |
|---|---|
| Startup, config load, route registration | INFO |
| Incoming request (method, path, status, duration_ms) | INFO |
| Backend came back up | INFO |
| Backend went down | WARN |
| Panic recovered | ERROR |
| Listen / shutdown errors | ERROR |

---

## Startup Sequence

1. Configure slog handler (format + level from env vars)
2. Parse optional config-file path from `os.Args`
3. Load `Config` from TOML (or default fallback)
4. `app.New(cfg)` builds all components:
   - For each route: create `Pool`, load backends, create `Balancer`, register with `Registry` and `health.Manager`
   - Wrap proxy in middleware chain: `Recover → Logging → Metrics → Proxy`
5. `app.Start()` — starts HTTP server in a background goroutine
6. Block on `SIGINT` / `SIGTERM`
7. `app.Shutdown(ctx)` — cancels health manager, waits for checker goroutines, then gracefully drains the HTTP server (5-second timeout)

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
| No healthy backend | `503 Service Unavailable` |
| Upstream call failure | `502 Bad Gateway` |
| Invalid backend URL | `500 Internal Server Error` |
| Panic in handler | `500 Internal Server Error` + stack trace logged |
| Backend health-check failure | Backend marked dead; logged on state change only |
| Config parse failure | Falls back to default config |
| Server listen error | `slog.Error` + process exits |
| Shutdown timeout exceeded | Forced shutdown logged |

---

## Current Capabilities

- Reverse proxying via `httputil.NewSingleHostReverseProxy`
- Multi-route support with longest-prefix matching
- Round Robin load balancing (atomic counter, skips unhealthy backends)
- Least Connections load balancing (adapts to runtime load)
- IP Hash load balancing (sticky routing by client IP, degrades gracefully on backend failure)
- Active health checks (periodic HTTP polling, state-change logging)
- Composable middleware chain (panic recovery, structured request logging, metrics stub)
- Structured logging via `log/slog` (configurable level and format via env vars)
- Graceful shutdown (OS signal handling, context cancellation, `WaitGroup` drain)
- Forwarding headers (`X-Forwarded-Host`, `X-Forwarded-Proto`, `X-Forwarded-For`)
- TOML-driven configuration with tilde expansion and default-file fallback
- Interface-driven design (`backend.Instance`, `backend.Provider`) for testability
