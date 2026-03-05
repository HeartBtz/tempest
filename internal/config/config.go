package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	Engine   EngineConfig   `json:"engine"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type DatabaseConfig struct {
	Path string `json:"path"`
}

type EngineConfig struct {
	MaxConcurrentSessions int  `json:"max_concurrent_sessions"`
	DefaultAnnouncePort   int  `json:"default_announce_port"`
	EnableRandomization   bool `json:"enable_randomization"`
}

var (
	instance *Config
	once     sync.Once
)

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8377,
		},
		Database: DatabaseConfig{
			Path: "tempest.db",
		},
		Engine: EngineConfig{
			MaxConcurrentSessions: 500,
			DefaultAnnouncePort:   6881,
			EnableRandomization:   true,
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func Get() *Config {
	once.Do(func() {
		instance = Default()
	})
	return instance
}

func Set(cfg *Config) {
	instance = cfg
}
