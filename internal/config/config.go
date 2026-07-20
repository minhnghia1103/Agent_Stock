package config

import "fmt"

// Config is the root configuration object.
type Config struct {
	Host         string
	Port         int
	DatabasePath string

	LLMProvider   string
	LLMBaseURL    string
	LLMAPIKey     string
	LLMModel      string
	LLMTimeoutSec int
	LLMMaxRetries int
	SystemPrompt  string

	WorkspacePath     string
	MaxToolIterations int

	PolicyPath  string
	MCPJSONPath string
	SkillsPath  string // optional: direct skills dir override (AGENT_SKILLS_PATH)
}

// Addr returns host:port for net/http.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
