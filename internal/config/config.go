package config

import "fmt"

// Config is the root configuration object.
type Config struct {
	Host         string
	Port         int
	DatabasePath string
}

// Addr returns host:port for net/http.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
