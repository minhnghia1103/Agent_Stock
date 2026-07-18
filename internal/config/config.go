package config

import "fmt"

// Config is the root configuration object (Phase 0: host/port only).
type Config struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Addr returns host:port for net/http.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
