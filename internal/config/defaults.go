package config

// Defaults returns a Config with built-in defaults.
func Defaults() Config {
	return Config{
		Host:         "127.0.0.1",
		Port:         8080,
		DatabasePath: "data/agent.db",
	}
}
