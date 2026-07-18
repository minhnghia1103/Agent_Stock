package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// Load reads config from path (optional file), then applies env overrides.
// Missing config file is OK — defaults are used.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	if path != "" {
		if err := loadFile(path, &cfg); err != nil {
			return nil, err
		}
	}

	applyEnv(&cfg)
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	return nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("AGENT_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("AGENT_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
}

func (c *Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	return nil
}
