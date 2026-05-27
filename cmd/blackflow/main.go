package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Harsho-afk/blackflow/internal/app"
	"github.com/Harsho-afk/blackflow/internal/config"
)

func main() {
	slog.SetDefault(slog.New(buildHandler()))

	configPath := ""
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, path, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("loaded config", "path", path)

	a, err := app.New(cfg)
	if err != nil {
		slog.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	a.Start()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	slog.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
	} else {
		slog.Info("shutdown complete")
	}
}

func buildHandler() slog.Handler {
	opts := &slog.HandlerOptions{Level: logLevel()}

	if os.Getenv("LOG_FORMAT") == "json" {
		return slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.NewTextHandler(os.Stdout, opts)
}

func logLevel() slog.Level {
	switch os.Getenv("LOG_LEVEL") {
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "warn", "WARN":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
