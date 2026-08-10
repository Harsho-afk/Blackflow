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
