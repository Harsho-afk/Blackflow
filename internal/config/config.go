package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server Server `toml:"server"`
}

type Server struct {
	Port      string                 `toml:"port"`
	RateLimit RateLimitConfig        `toml:"rate_limit"`
	Routes    map[string]RouteConfig `toml:"routes"`
}

type RateLimitConfig struct {
	Enabled           bool    `toml:"enabled"`
	RequestsPerSecond float64 `toml:"requests_per_second"`
	Burst             int     `toml:"burst"`
}

type RouteConfig struct {
	Interval    time.Duration `toml:"interval"`
	Algorithm   string        `toml:"algorithm"`
	Backends    []string      `toml:"backends"`
	StripPrefix bool          `toml:"strip_prefix"`
}

type rawConfig struct {
	Server struct {
		Port      string                    `toml:"port"`
		RateLimit RateLimitConfig           `toml:"rate_limit"`
		Routes    map[string]rawRouteConfig `toml:"routes"`
	} `toml:"server"`
}

type rawRouteConfig struct {
	Interval    string   `toml:"interval"`
	Algorithm   string   `toml:"algorithm"`
	Backends    []string `toml:"backends"`
	StripPrefix bool     `toml:"strip_prefix"`
}

func Load(path string) (*Config, string, error) {
	if path == "" {
		return nil, "", fmt.Errorf("no config file provided")
	}

	filePath, err := expandTilde(path)
	if err != nil {
		return nil, "", fmt.Errorf("invalid config path: %w", err)
	}

	cfg, err := loadFromFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load config: %w", err)
	}

	return cfg, filePath, nil
}

func loadFromFile(path string) (*Config, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, fmt.Errorf("invalid config path")
	}

	ext := strings.TrimSpace(filepath.Ext(path))
	if ext != ".toml" {
		return nil, fmt.Errorf("invalid file type")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw rawConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	return normalize(raw)
}

func normalize(raw rawConfig) (*Config, error) {
	cfg := &Config{
		Server: Server{
			Port:   raw.Server.Port,
			Routes: make(map[string]RouteConfig),
		},
	}

	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}

	cfg.Server.RateLimit = raw.Server.RateLimit
	if cfg.Server.RateLimit.RequestsPerSecond <= 0 {
		cfg.Server.RateLimit.RequestsPerSecond = 10
	}
	if cfg.Server.RateLimit.Burst <= 0 {
		cfg.Server.RateLimit.Burst = 20
	}

	for prefix, r := range raw.Server.Routes {
		if len(r.Backends) == 0 {
			return nil, fmt.Errorf("route %s has no backends", prefix)
		}

		interval, err := time.ParseDuration(r.Interval)
		if err != nil || interval < time.Second {
			interval = 5 * time.Second
		}

		algo := r.Algorithm
		if algo == "" {
			algo = "round_robin"
		}

		cfg.Server.Routes[prefix] = RouteConfig{
			Interval:    interval,
			Algorithm:   algo,
			Backends:    r.Backends,
			StripPrefix: r.StripPrefix,
		}
	}

	return cfg, nil
}

func expandTilde(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	return filepath.Join(usr.HomeDir, path[1:]), nil
}
