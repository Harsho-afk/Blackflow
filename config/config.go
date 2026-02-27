package config

import (
	"errors"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"

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

func LoadServerConfig(path string) (*ServerConfig, string) {
	if len(path) == 0 {
		log.Println("Falling back to default config.")
		return createDefaultConfig()
	}
	file_path, err := expandTilde(path)
	if err != nil {
		log.Printf("Failed to parse path %s: %v\n", path, err)
		log.Println("Falling back to default config.")
		return createDefaultConfig()
	}
	file_info, err := os.Stat(file_path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Println("The given config path does not exist.")
		} else {
			log.Printf("Error checking path %s: %v\n", file_path, err)
		}
		log.Println("Falling back to default config.")
		return createDefaultConfig()
	}
	if file_info.IsDir() || !file_info.Mode().IsRegular() {
		log.Printf("%s is not a Config file. Accepted files are .yml and .yaml.\n", file_path)
		log.Println("Falling back to default config.")
		return createDefaultConfig()
	}
	file_ext := filepath.Ext(file_path)
	file_ext = strings.TrimSpace(file_ext)
	if file_ext != ".yml" && file_ext != ".yaml" {
		log.Printf("%s is not a Config file. Accepted files are '.yml' and '.yaml'.\n", file_path)
		log.Println("Falling back to default config.")
		return createDefaultConfig()

	}
	data, err := os.ReadFile(file_path)
	if errors.Is(err, os.ErrNotExist) {
		log.Println("The given config path does not exist.")
		log.Println("Falling back to default config.")
		return createDefaultConfig()
	} else if err != nil {
		log.Printf("Error checking path %s: %v\n", file_path, err)
		log.Println("Falling back to default config.")
		return createDefaultConfig()
	}
	var config ServerConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Printf("Failed to load the config file: %v", err)
		log.Printf("Falling back to default config.")
		return createDefaultConfig()
	}
	return &config, file_path
}

func createDefaultConfig() (*ServerConfig, string) {
	file_path := "~/.config/blackflow/default.yml"
	file_path, err := expandTilde(file_path)
	if err != nil {
		log.Fatalf("Failed to create default config file at path %s: %v\n", file_path, err)
	}
	data, err := os.ReadFile(file_path)
	if err != nil {
		log.Printf("Error finding default config at %s: %v", file_path, err)
		dir := filepath.Dir(file_path)
		err = os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			log.Fatalf("Failed to create default config file at path %s: %v\n", file_path, err)
		}
		file, err := os.Create(file_path)
		if err != nil {
			log.Fatalf("Failed to create default config file at path %s: %v\n", file_path, err)
		}
		defer file.Close()
		data := []byte("server:\n  port: 8080\n  routes:\n")
		written_len, err := file.Write(data)
		if written_len != len(data) {
			file.Close()
			err = os.Remove(file_path)
			log.Fatalf("Failed to create default config file at path %s: %v\n", file_path, err)
		}
		log.Printf("Created default config file at %s\n", file_path)
	}
	var config ServerConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Failed to load default config file at path %s: %v\n", file_path, err)
	}
	return &config, file_path
}

func expandTilde(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, path[1:]), nil
}
