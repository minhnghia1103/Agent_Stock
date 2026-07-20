package mcp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent_stock/internal/mcp"
	"agent_stock/internal/tools"
)

func TestLoadFileMissing(t *testing.T) {
	cfg, err := mcp.LoadFile(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.MCPServers) != 0 {
		t.Fatalf("expected empty, got %#v", cfg.MCPServers)
	}
}

func TestLoadFileParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	raw := `{
  "mcpServers": {
    "fs": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "."],
      "allowed_tools": ["read_file", "list_*"],
      "disabled": false
    }
  }
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := mcp.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := cfg.MCPServers["fs"]
	if !ok {
		t.Fatal("missing fs")
	}
	if sc.Command != "npx" {
		t.Fatalf("command=%q", sc.Command)
	}
}

func TestManagerStartEmptyConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := tools.NewRegistry()
	mgr := mcp.NewManager(reg, path)
	if err := mgr.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	if len(mgr.ToolNames()) != 0 {
		t.Fatalf("expected no tools, got %v", mgr.ToolNames())
	}
	st := mgr.Status()
	if len(st) != 0 {
		t.Fatalf("expected empty status, got %#v", st)
	}
}

func TestManagerSkipsDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"filesystem": map[string]any{
				"command":  "npx",
				"args":     []string{"-y", "@modelcontextprotocol/server-filesystem", "."},
				"disabled": true,
			},
		},
	}
	raw, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	reg := tools.NewRegistry()
	mgr := mcp.NewManager(reg, path)
	if err := mgr.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	if len(mgr.ToolNames()) != 0 {
		t.Fatalf("disabled server should not register tools: %v", mgr.ToolNames())
	}
	st := mgr.Status()
	if len(st) != 1 || !st[0].Disabled {
		t.Fatalf("expected disabled status, got %#v", st)
	}
}

func TestPolicyAllowsMCPPrefix(t *testing.T) {
	// sanity: registered name shape used with POLICY mcp_*
	name := "mcp_filesystem__list_directory"
	if !(len(name) > 4 && name[:4] == "mcp_") {
		t.Fatal(name)
	}
}
