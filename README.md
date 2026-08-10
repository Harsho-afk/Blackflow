# Blackflow

A lightweight reverse proxy and load balancer written in Go.  
Blackflow routes incoming HTTP requests to backend services based on URL path prefixes and distributes load using pluggable balancing strategies, driven entirely by a TOML config file.

---

## Features

- Reverse proxying via `httputil.NewSingleHostReverseProxy`
- Longest-prefix multi-route matching
- Round Robin load balancing (atomic counter, skips unhealthy backends)
- Least Connections load balancing (adapts to runtime load)
- IP Hash load balancing (sticky routing by client IP, degrades gracefully on backend failure)
- Active health checks — periodic `GET /health` polling per backend pool
- Startup health checks — backends are probed before the server accepts traffic
- Forwarding headers — `X-Forwarded-Host`, `X-Forwarded-Proto`, `X-Forwarded-For`
- Graceful shutdown on `SIGINT` / `SIGTERM`
- TOML-driven configuration with `~` tilde expansion
- Interface-driven design for testability (`backend.Instance`, `backend.Provider`)

---

## Requirements

- Go 1.22+

---

## Installation

```bash
git clone https://github.com/Harsho-afk/blackflow.git
cd blackflow
make build
```

The compiled binary is written to `bin/blackflow`.

---

## Usage

```bash
# Run with a config file
./bin/blackflow /path/to/config.toml

# Run without a path — falls back to ~/.config/blackflow/default.toml
# (this file is never created automatically; it must already exist)
./bin/blackflow

# Run directly with go run
make run /path/to/config.toml
```

If the config path is omitted, invalid, or unreadable, Blackflow falls back to `~/.config/blackflow/default.toml`. Blackflow never creates this file (or any config file) on its own — if it doesn't already exist, startup fails with an error.

---

## Configuration

Config files must use the `.toml` extension.

```toml
[server]
port = 8080

[server.routes."/auth"]
interval = "10s"          # Health check polling interval (values < 1s default to 5s)
algorithm = "round_robin" # round_robin | least_connection | ip_hash
backends = ["http://localhost:8081", "http://localhost:8082"]

[server.routes."/api"]
interval = "30s"
algorithm = "least_connection"
backends = ["http://localhost:8091", "http://localhost:8092"]
```

---

## Architecture

See [architecture.md](docs/architecture.md) for a detailed breakdown of every component and the concurrency model.

---

<!-- PERF:START -->
## Performance

Benchmarked on a 12-core machine with client, backends, and Blackflow pinned to separate cores (`taskset`), using [wrk2](https://github.com/giltene/wrk2) for coordinated-omission-corrected load generation. Each configuration ran 5×20s measured runs at a fixed rate (`-t2 -c50 -R 5000 --latency`) after a discarded 10s warm-up, against two backend instances simulating 5ms of upstream latency. Full methodology and raw output in [docs/bench](docs/bench). Regenerate this section with `docs/bench/scripts/run_bench.sh` + `docs/bench/scripts/gen_report.py`.

### End-to-end throughput and latency

| Config | Requests/sec (mean ± stdev) | p99 latency (mean ± stdev) | Overhead vs. baseline |
|---|---|---|---|
| Baseline (direct, no proxy) | 4984.18 ± 0.141 | 8.498ms ± 0.194ms | — |
| Round Robin | 4984.11 ± 0.148 | 8.880ms ± 0.087ms | +0.382ms |
| Least Connection | 4984.15 ± 0.113 | 8.908ms ± 0.156ms | +0.410ms |
| IP Hash | 4984.22 ± 0.108 | 8.864ms ± 0.161ms | +0.366ms |

Throughput is unaffected by proxying — the run rate is capped by `-R 5000`, so this measures overhead at a fixed load rather than a max-throughput ceiling. Proxying adds a small, consistent amount of p99 latency regardless of balancing algorithm; at this network and backend-latency scale, algorithm choice has no measurable effect on end-to-end performance.

### Isolated algorithm-selection cost

Measured with `go test -bench=. -benchmem -run=^$ ./internal/balancer/...` — pure CPU cost of backend selection, no network or HTTP involved.

| Algorithm | 2 backends | 10 backends | 100 backends |
|---|---|---|---|
| Round Robin | 72.9 ns/op, 2 allocs | 104.9 ns/op, 2 allocs | 348.6 ns/op, 2 allocs |
| Least Connection | 77.4 ns/op, 2 allocs | 214.3 ns/op, 2 allocs | 1727.0 ns/op, 2 allocs |
| IP Hash | 72.8 ns/op, 2 allocs | 106.4 ns/op, 2 allocs | 345.0 ns/op, 2 allocs |

<!-- PERF:END -->
