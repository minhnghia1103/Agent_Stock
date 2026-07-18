package config

// Defaults returns a Config with built-in defaults.
func Defaults() Config {
	return Config{
		Host:              "127.0.0.1",
		Port:              8080,
		DatabasePath:      "data/agent.db",
		LLMProvider:       "llmgate",
		LLMTimeoutSec:     60,
		LLMMaxRetries:     3,
		SystemPrompt:      "You are a helpful stock research assistant. Use tools when they improve accuracy (time, files, web_fetch, get_stock_quote).",
		WorkspacePath:     "workspace",
		MaxToolIterations: 8,
	}
}
