package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// FileConfig is the root of mcp.json (Claude Desktop / ews-style).
type FileConfig struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}

// ServerConfig describes one MCP server. Phase 8: stdio only.
type ServerConfig struct {
	Command      string            `json:"command"`
	Args         []string          `json:"args"`
	Env          map[string]string `json:"env"`
	AllowedTools []string          `json:"allowed_tools"`
	TimeoutSec   int               `json:"timeout_sec"`
	Disabled     bool              `json:"disabled"`
}

// LoadFile reads mcp.json. Missing file → empty config (nil error).
func LoadFile(path string) (*FileConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return &FileConfig{MCPServers: map[string]ServerConfig{}}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileConfig{MCPServers: map[string]ServerConfig{}}, nil
		}
		return nil, fmt.Errorf("read mcp config %s: %w", path, err)
	}
	var cfg FileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse mcp config %s: %w", path, err)
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = map[string]ServerConfig{}
	}
	return &cfg, nil
}

func (c ServerConfig) envSlice() []string {
	if len(c.Env) == 0 {
		return nil
	}
	out := make([]string, 0, len(c.Env))
	for k, v := range c.Env {
		out = append(out, k+"="+v)
	}
	return out
}

func (c ServerConfig) allowsTool(name string) bool {
	if len(c.AllowedTools) == 0 {
		return true
	}
	for _, a := range c.AllowedTools {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if strings.HasSuffix(a, "*") {
			if strings.HasPrefix(name, strings.TrimSuffix(a, "*")) {
				return true
			}
			continue
		}
		if a == name {
			return true
		}
	}
	return false
}
