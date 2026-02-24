package config

import (
	"gopkg.in/yaml.v3"
	"os"
)

type ServerConfig struct {
	Server struct {
		Port         string `yaml:"port"`
		LoadBalancer struct {
			Algorithm string `yaml:"algorithm"`
		} `yaml:"load_balancer"`
		Health struct {
			Interval string `yaml:"interval"`
		} `yaml:"health"`
		Routes map[string]string `yaml:"routes"`
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
