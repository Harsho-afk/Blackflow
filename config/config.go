package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type RouteConfig struct {
	Algorithm string   `yaml:"algorithm"`
	Backends  []string `yaml:"backends"`
}

type ServerConfig struct {
	Server struct {
		Port   string                 `yaml:"port"`
		Routes map[string]RouteConfig `yaml:"routes"`
	} `yaml:"server"`
}

func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config ServerConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
