package config

// Defaults returns a Config with Phase 0 defaults.
func Defaults() Config {
	return Config{
		Host: "127.0.0.1",
		Port: 8080,
	}
}
