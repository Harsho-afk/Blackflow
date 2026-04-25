package config

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server Server `yaml:"server"`
}

type Server struct {
	Port   string                 `yaml:"port"`
	Routes map[string]RouteConfig `yaml:"routes"`
}

type RouteConfig struct {
	Interval  time.Duration `yaml:"interval"`
	Algorithm string        `yaml:"algorithm"`
	Backends  []string      `yaml:"backends"`
}

type rawConfig struct {
	Server struct {
		Port   string                    `yaml:"port"`
		Routes map[string]rawRouteConfig `yaml:"routes"`
	} `yaml:"server"`
}

type rawRouteConfig struct {
	Interval  string   `yaml:"interval"`
	Algorithm string   `yaml:"algorithm"`
	Backends  []string `yaml:"backends"`
}

func Load(path string) (*Config, string, error) {
	if path == "" {
		return loadDefault()
	}

	filePath, err := expandTilde(path)
	if err != nil {
		return loadDefault()
	}

	cfg, err := loadFromFile(filePath)
	if err != nil {
		return loadDefault()
	}

	return cfg, filePath, nil
}

func loadFromFile(path string) (*Config, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, fmt.Errorf("invalid config path")
	}

	ext := strings.TrimSpace(filepath.Ext(path))
	if ext != ".yml" && ext != ".yaml" {
		return nil, fmt.Errorf("invalid file type")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
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
			Interval:  interval,
			Algorithm: algo,
			Backends:  r.Backends,
		}
	}

	return cfg, nil
}

func loadDefault() (*Config, string, error) {
	path, err := expandTilde("~/.config/blackflow/default.yml")
	if err != nil {
		return nil, "", err
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := createDefaultFile(path); err != nil {
			return nil, "", err
		}
	}

	cfg, err := loadFromFile(path)
	return cfg, path, err
}

func createDefaultFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	content := []byte(`server:
  port: "8080"
  routes:
    /api:
      interval: "5s"
      algorithm: "round_robin"
      backends:
        - "http://localhost:8081"
`)

	return os.WriteFile(path, content, 0644)
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
