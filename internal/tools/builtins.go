package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent_stock/internal/security"
	"agent_stock/internal/skills"
)

// --- get_time ---

type getTimeTool struct{}

func NewGetTimeTool() Tool { return &getTimeTool{} }

func (t *getTimeTool) Name() string { return "get_time" }
func (t *getTimeTool) Description() string {
	return "Return the current date/time in UTC and local timezone."
}
func (t *getTimeTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}
func (t *getTimeTool) Execute(ctx context.Context, args map[string]any) Result {
	_ = ctx
	_ = args
	now := time.Now()
	return OK(fmt.Sprintf("utc=%s local=%s", now.UTC().Format(time.RFC3339), now.Format(time.RFC3339)))
}

// --- read_file ---

type readFileTool struct{ ws *Workspace }

func NewReadFileTool(ws *Workspace) Tool { return &readFileTool{ws: ws} }

func (t *readFileTool) Name() string { return "read_file" }
func (t *readFileTool) Description() string {
	return "Read a UTF-8 text file from the agent workspace (relative path)."
}
func (t *readFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Relative path under workspace"},
		},
		"required": []string{"path"},
	}
}
func (t *readFileTool) Execute(ctx context.Context, args map[string]any) Result {
	ws := t.ws
	if root := JailRootFromContext(ctx); root != "" {
		ws = &Workspace{root: root}
	}
	path := StringArg(args, "path")
	abs, err := ws.Resolve(path)
	if err != nil {
		return Err(err.Error())
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return Err(err.Error())
	}
	const max = 100_000
	if len(data) > max {
		return OK(string(data[:max]) + "\n...[truncated]")
	}
	return OK(string(data))
}

// --- list_dir ---

type listDirTool struct{ ws *Workspace }

func NewListDirTool(ws *Workspace) Tool { return &listDirTool{ws: ws} }

func (t *listDirTool) Name() string { return "list_dir" }
func (t *listDirTool) Description() string {
	return "List files and directories under a workspace path."
}
func (t *listDirTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Relative directory path (default .)"},
		},
	}
}
func (t *listDirTool) Execute(ctx context.Context, args map[string]any) Result {
	ws := t.ws
	if root := JailRootFromContext(ctx); root != "" {
		ws = &Workspace{root: root}
	}
	path := StringArg(args, "path")
	if path == "" {
		path = "."
	}
	abs, err := ws.Resolve(path)
	if err != nil {
		return Err(err.Error())
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return Err(err.Error())
	}
	var b strings.Builder
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		b.WriteString(name)
		b.WriteByte('\n')
	}
	if b.Len() == 0 {
		return OK("(empty)")
	}
	return OK(b.String())
}

// --- write_file ---

type writeFileTool struct{ ws *Workspace }

func NewWriteFileTool(ws *Workspace) Tool { return &writeFileTool{ws: ws} }

func (t *writeFileTool) Name() string { return "write_file" }
func (t *writeFileTool) Description() string {
	return "Write UTF-8 text to a file under the agent workspace (creates parent dirs)."
}
func (t *writeFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "Relative path under workspace"},
			"content": map[string]any{"type": "string", "description": "File contents"},
		},
		"required": []string{"path", "content"},
	}
}
func (t *writeFileTool) Execute(ctx context.Context, args map[string]any) Result {
	ws := t.ws
	if root := JailRootFromContext(ctx); root != "" {
		ws = &Workspace{root: root}
	}
	path := StringArg(args, "path")
	content := StringArg(args, "content")
	abs, err := ws.Resolve(path)
	if err != nil {
		return Err(err.Error())
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return Err(err.Error())
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return Err(err.Error())
	}
	return OK(fmt.Sprintf("wrote %d bytes to %s", len(content), path))
}

// RegisterBuiltins registers Phase 4 filesystem + time tools (+ Phase 9 skills if reg non-nil).
func RegisterBuiltins(reg *Registry, ws *Workspace, pol *security.Policy) {
	reg.Register(NewGetTimeTool())
	reg.Register(NewReadFileTool(ws))
	reg.Register(NewListDirTool(ws))
	reg.Register(NewWriteFileTool(ws))
	reg.Register(NewWebFetchTool(pol))
	reg.Register(NewStockQuoteTool(pol))
}

// RegisterSkillTools registers skill_search and use_skill.
func RegisterSkillTools(reg *Registry, skillsReg *skills.Registry) {
	if skillsReg == nil {
		return
	}
	reg.Register(NewSkillSearchTool(skillsReg))
	reg.Register(NewUseSkillTool(skillsReg))
}
