package tools

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"agent_stock/internal/provider"
	"agent_stock/internal/security"
)

// Registry holds named tools (Registry pattern).
type Registry struct {
	mu     sync.RWMutex
	tools  map[string]Tool
	policy *security.Policy
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) SetPolicy(p *security.Policy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.policy = p
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Unregister removes a tool by name (used by MCP reload).
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.tools))
	for name := range r.tools {
		out = append(out, name)
	}
	return out
}

// Definitions returns tool schemas allowed by policy.
func (r *Registry) Definitions() []provider.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]provider.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		if r.policy != nil {
			if err := r.policy.AllowTool(t.Name()); err != nil {
				continue
			}
		}
		out = append(out, provider.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return out
}

// Execute checks policy, runs the tool, then redacts output.
func (r *Registry) Execute(ctx context.Context, name, rawArgs string) Result {
	r.mu.RLock()
	pol := r.policy
	r.mu.RUnlock()

	if pol != nil {
		if err := pol.AllowTool(name); err != nil {
			slog.Warn("security.tool_denied", "tool", name, "error", err)
			return Err(err.Error())
		}
	}

	t, ok := r.Get(name)
	if !ok {
		return Err(fmt.Sprintf("unknown tool: %s", name))
	}
	args, err := ParseArgs(rawArgs)
	if err != nil {
		return Err(fmt.Sprintf("invalid tool arguments JSON: %v", err))
	}
	res := t.Execute(ctx, args)
	res.Content = security.Redact(pol, res.Content)
	return res
}
