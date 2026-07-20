package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"agent_stock/internal/tools"
)

// ServerStatus is a snapshot for HTTP / logs.
type ServerStatus struct {
	Name      string   `json:"name"`
	Connected bool     `json:"connected"`
	ToolCount int      `json:"tool_count"`
	Tools     []string `json:"tools,omitempty"`
	Error     string   `json:"error,omitempty"`
	Disabled  bool     `json:"disabled,omitempty"`
}

type serverState struct {
	name       string
	client     *mcpclient.Client
	connected  atomic.Bool
	toolNames  []string
	timeoutSec int
	lastErr    string
}

// Manager connects MCP servers from mcp.json and registers BridgeTools.
type Manager struct {
	mu       sync.Mutex
	path     string
	registry *tools.Registry
	configs  map[string]ServerConfig
	servers  map[string]*serverState
}

// NewManager creates a manager bound to a tool registry. Call Start to connect.
func NewManager(registry *tools.Registry, configPath string) *Manager {
	return &Manager{
		path:     configPath,
		registry: registry,
		configs:  map[string]ServerConfig{},
		servers:  map[string]*serverState{},
	}
}

// Path returns the mcp.json path.
func (m *Manager) Path() string { return m.path }

// Start loads mcp.json and connects enabled stdio servers.
// Connection failures are logged; the gateway still boots.
func (m *Manager) Start(ctx context.Context) error {
	cfg, err := LoadFile(m.path)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.configs = cfg.MCPServers
	m.mu.Unlock()

	names := make([]string, 0, len(cfg.MCPServers))
	for name := range cfg.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		sc := cfg.MCPServers[name]
		if sc.Disabled {
			slog.Info("mcp.server.skipped", "server", name, "reason", "disabled")
			continue
		}
		if strings.TrimSpace(sc.Command) == "" {
			slog.Warn("mcp.server.skipped", "server", name, "reason", "empty command (stdio only in Phase 8)")
			continue
		}
		if err := m.connectServer(ctx, name, sc); err != nil {
			slog.Warn("mcp.server.connect_failed", "server", name, "error", err)
			m.mu.Lock()
			m.servers[name] = &serverState{name: name, lastErr: err.Error()}
			m.mu.Unlock()
		}
	}
	return nil
}

// Reload re-reads mcp.json, disconnects all MCP tools, and reconnects.
func (m *Manager) Reload(ctx context.Context) error {
	m.disconnectAll()
	return m.Start(ctx)
}

// Close disconnects all MCP servers.
func (m *Manager) Close() {
	m.disconnectAll()
}

// Status returns per-server connection info.
func (m *Manager) Status() []ServerStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	names := make([]string, 0, len(m.configs))
	for name := range m.configs {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]ServerStatus, 0, len(names))
	for _, name := range names {
		sc := m.configs[name]
		st := ServerStatus{Name: name, Disabled: sc.Disabled}
		if ss, ok := m.servers[name]; ok {
			st.Connected = ss.connected.Load()
			st.ToolCount = len(ss.toolNames)
			st.Tools = append([]string(nil), ss.toolNames...)
			st.Error = ss.lastErr
		} else if sc.Disabled {
			st.Error = "disabled"
		} else if strings.TrimSpace(sc.Command) == "" {
			st.Error = "no stdio command"
		} else {
			st.Error = "not connected"
		}
		out = append(out, st)
	}
	return out
}

// ToolNames returns all registered MCP tool names.
func (m *Manager) ToolNames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for _, ss := range m.servers {
		out = append(out, ss.toolNames...)
	}
	sort.Strings(out)
	return out
}

func (m *Manager) disconnectAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, ss := range m.servers {
		for _, toolName := range ss.toolNames {
			m.registry.Unregister(toolName)
		}
		if ss.client != nil {
			_ = ss.client.Close()
		}
		ss.connected.Store(false)
		delete(m.servers, name)
	}
}

func (m *Manager) connectServer(ctx context.Context, name string, sc ServerConfig) error {
	timeout := sc.TimeoutSec
	if timeout <= 0 {
		timeout = 60
	}
	connectCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	client, err := mcpclient.NewStdioMCPClient(sc.Command, sc.envSlice(), sc.Args...)
	if err != nil {
		return fmt.Errorf("create stdio client: %w", err)
	}

	initReq := mcpgo.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcpgo.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcpgo.Implementation{
		Name:    "agent_stock",
		Version: "0.1.0",
	}
	if _, err := client.Initialize(connectCtx, initReq); err != nil {
		_ = client.Close()
		return fmt.Errorf("initialize: %w", err)
	}

	toolsResult, err := client.ListTools(connectCtx, mcpgo.ListToolsRequest{})
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("list tools: %w", err)
	}

	ss := &serverState{
		name:       name,
		client:     client,
		timeoutSec: timeout,
	}
	ss.connected.Store(true)

	var registered []string
	for _, mcpTool := range toolsResult.Tools {
		if !sc.allowsTool(mcpTool.Name) {
			continue
		}
		bt := newBridgeTool(name, mcpTool, client, timeout, &ss.connected)
		if _, exists := m.registry.Get(bt.Name()); exists {
			slog.Warn("mcp.tool.name_collision", "server", name, "tool", bt.Name())
			continue
		}
		m.registry.Register(bt)
		registered = append(registered, bt.Name())
	}
	ss.toolNames = registered

	m.mu.Lock()
	m.servers[name] = ss
	m.mu.Unlock()

	slog.Info("mcp.server.connected",
		"server", name,
		"transport", "stdio",
		"tools", len(registered),
		"names", registered,
	)
	return nil
}
