package config

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	Engine   EngineConfig   `json:"engine"`
	Security SecurityConfig `json:"security"`
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

type SecurityConfig struct {
	AllowPrivateTrackerDestinations bool `json:"allow_private_tracker_destinations"`
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

// LoadDotEnv reads a .env file and sets any unset environment variables from it.
// Lines starting with # or blank are ignored. Existing env vars are NOT overridden.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip inline comments
		if ci := strings.Index(val, " #"); ci >= 0 {
			val = strings.TrimSpace(val[:ci])
		}
		// Strip surrounding quotes
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		// Only set if not already set in the environment
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
	return scanner.Err()
}

// applyEnvOverrides applies TEMPEST_* environment variables on top of the config,
// allowing env vars (and .env file) to override config.json values.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("TEMPEST_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("TEMPEST_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("TEMPEST_DB_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("TEMPEST_MAX_SESSIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Engine.MaxConcurrentSessions = n
		}
	}
	if v := os.Getenv("TEMPEST_ANNOUNCE_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Engine.DefaultAnnouncePort = p
		}
	}
	if v := os.Getenv("TEMPEST_ENABLE_RANDOMIZATION"); v != "" {
		cfg.Engine.EnableRandomization = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("TEMPEST_ALLOW_PRIVATE_TRACKERS"); v != "" {
		cfg.Security.AllowPrivateTrackerDestinations = v == "true" || v == "1" || v == "yes"
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
		} else {
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}

	// Env vars win over config.json
	applyEnvOverrides(cfg)

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
	once.Do(func() {})
	instance = cfg
}
