package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Harsho-afk/blackflow/internal/balancer"
	"github.com/Harsho-afk/blackflow/internal/config"
	"github.com/Harsho-afk/blackflow/internal/health"
	"github.com/Harsho-afk/blackflow/internal/middleware"
	"github.com/Harsho-afk/blackflow/internal/pool"
	"github.com/Harsho-afk/blackflow/internal/proxy"
	"github.com/Harsho-afk/blackflow/internal/registry"
)

type App struct {
	server *http.Server
	health *health.Manager
	cancel context.CancelFunc
}

func New(cfg *config.Config) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())
	reg := registry.New()
	hm := health.NewManager(ctx)

	for prefix, rc := range cfg.Server.Routes {
		p := pool.New()

		if err := p.Load(rc.Backends); err != nil {
			cancel()
			return nil, err
		}

		b := balancer.NewBalancer(p, rc.Algorithm)
		route := registry.NewRoute(prefix, b)

		reg.Add(route)
		hm.Register(p, rc.Interval)

		slog.Info("route registered",
			"prefix", prefix,
			"algorithm", b.GetAlgorithm(),
			"backends", len(rc.Backends),
		)
	}

	chain := []middleware.Middleware{
		middleware.Recover,
		middleware.Logging,
		middleware.Metrics,
	}

	if cfg.Server.RateLimit.Enabled {
		rl := middleware.NewRateLimiter(
			cfg.Server.RateLimit.RequestsPerSecond,
			cfg.Server.RateLimit.Burst,
		)
		chain = append(chain, rl.Middleware)

		slog.Info("rate limiting enabled",
			"requests_per_second", cfg.Server.RateLimit.RequestsPerSecond,
			"burst", cfg.Server.RateLimit.Burst,
		)
	}

	handler := middleware.Chain(
		proxy.New(reg),
		chain...,
	)

	server := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &App{
		server: server,
		health: hm,
		cancel: cancel,
	}, nil
}

func (a *App) Start() {
	go func() {
		slog.Info("server running", "addr", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen error", "error", err)
		}
	}()
}

func (a *App) Shutdown(ctx context.Context) error {
	a.cancel()
	a.health.Stop()

	return a.server.Shutdown(ctx)
}
