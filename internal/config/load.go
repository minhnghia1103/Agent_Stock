package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Load applies defaults, loads .env (if present), then reads process env.
func Load() (*Config, error) {
	cfg := Defaults()

	envFile := os.Getenv("AGENT_ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	// Missing .env is OK — defaults + exported env still work.
	_ = godotenv.Load(envFile)

	applyEnv(&cfg)
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
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
	if v := os.Getenv("AGENT_DATABASE_PATH"); v != "" {
		cfg.DatabasePath = v
	}
}

func (c *Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if strings.TrimSpace(c.DatabasePath) == "" {
		return fmt.Errorf("database_path is required")
	}
	return nil
}
