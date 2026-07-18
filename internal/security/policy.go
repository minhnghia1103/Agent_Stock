package security

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Policy is the loaded POLICY.yaml object.
type Policy struct {
	PromptInjection PromptInjectionPolicy `yaml:"prompt_injection"`
	ToolPermissions ToolPermissionsPolicy `yaml:"tool_permissions"`
	Network         NetworkPolicy         `yaml:"network"`
	OutputRedaction OutputRedactionPolicy `yaml:"output_redaction"`
	WebSocket       WebSocketPolicy       `yaml:"websocket"`
}

type PromptInjectionPolicy struct {
	Enabled      bool   `yaml:"enabled"`
	Action       string `yaml:"action"` // off|log|warn|block
	MaxScanChars int    `yaml:"max_scan_chars"`
}

type ToolPermissionsPolicy struct {
	Enabled   bool     `yaml:"enabled"`
	Allowlist []string `yaml:"allowlist"`
	Denylist  []string `yaml:"denylist"`
}

type NetworkPolicy struct {
	Enabled                bool     `yaml:"enabled"`
	AllowlistHosts         []string `yaml:"allowlist_hosts"`
	DenyPrivateIPs         bool     `yaml:"deny_private_ips"`
	DenyLocalhostHostnames []string `yaml:"deny_localhost_hostnames"`
}

type OutputRedactionPolicy struct {
	Enabled bool `yaml:"enabled"`
}

type WebSocketPolicy struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// Defaults returns a safe built-in policy if file is missing.
func Defaults() Policy {
	return Policy{
		PromptInjection: PromptInjectionPolicy{
			Enabled:      true,
			Action:       "warn",
			MaxScanChars: 50000,
		},
		ToolPermissions: ToolPermissionsPolicy{
			Enabled: false,
		},
		Network: NetworkPolicy{
			DenyPrivateIPs: true,
			DenyLocalhostHostnames: []string{
				"localhost",
				"localhost.localdomain",
				"metadata.google.internal",
			},
		},
		OutputRedaction: OutputRedactionPolicy{Enabled: true},
	}
}

// Load reads POLICY.yaml from path. Missing file → Defaults().
func Load(path string) (*Policy, error) {
	p := Defaults()
	if strings.TrimSpace(path) == "" {
		return &p, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &p, nil
		}
		return nil, fmt.Errorf("read policy %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse policy %s: %w", path, err)
	}
	normalize(&p)
	return &p, nil
}

func normalize(p *Policy) {
	p.PromptInjection.Action = strings.ToLower(strings.TrimSpace(p.PromptInjection.Action))
	if p.PromptInjection.Action == "" {
		p.PromptInjection.Action = "warn"
	}
	if p.PromptInjection.MaxScanChars <= 0 {
		p.PromptInjection.MaxScanChars = 50000
	}
	for i, h := range p.Network.AllowlistHosts {
		p.Network.AllowlistHosts[i] = strings.ToLower(strings.TrimSpace(h))
	}
	for i, h := range p.Network.DenyLocalhostHostnames {
		p.Network.DenyLocalhostHostnames[i] = strings.ToLower(strings.TrimSpace(h))
	}
}

// AllowTool returns an error if the tool is not permitted.
func (p *Policy) AllowTool(name string) error {
	name = strings.TrimSpace(name)
	for _, d := range p.ToolPermissions.Denylist {
		if matchName(d, name) {
			return fmt.Errorf("tool %q denied by policy denylist", name)
		}
	}
	if !p.ToolPermissions.Enabled {
		return nil
	}
	for _, a := range p.ToolPermissions.Allowlist {
		if matchName(a, name) {
			return nil
		}
	}
	return fmt.Errorf("tool %q not in policy allowlist", name)
}

func matchName(pattern, name string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == name
}
