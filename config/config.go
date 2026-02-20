package config

import (
	"os"
	"gopkg.in/yaml.v3"
)

type Config struct {
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

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
