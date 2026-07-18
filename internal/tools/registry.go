package tools

import (
	"context"
	"fmt"
	"sync"

	"agent_stock/internal/provider"
)

// Registry holds named tools (Registry pattern).
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
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

// Definitions returns OpenAI-style tool schemas for the LLM.
func (r *Registry) Definitions() []provider.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]provider.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, provider.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return out
}

// Execute looks up and runs a tool by name.
func (r *Registry) Execute(ctx context.Context, name, rawArgs string) Result {
	t, ok := r.Get(name)
	if !ok {
		return Err(fmt.Sprintf("unknown tool: %s", name))
	}
	args, err := ParseArgs(rawArgs)
	if err != nil {
		return Err(fmt.Sprintf("invalid tool arguments JSON: %v", err))
	}
	return t.Execute(ctx, args)
}
