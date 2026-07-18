package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Workspace roots file tools under a jail directory.
type Workspace struct {
	root string
}

func NewWorkspace(root string) (*Workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("workspace abs: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("workspace mkdir: %w", err)
	}
	return &Workspace{root: abs}, nil
}

func (w *Workspace) Root() string { return w.root }

// Resolve maps a relative (or workspace-absolute) path into the jail.
func (w *Workspace) Resolve(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		rel = "."
	}
	if filepath.IsAbs(rel) {
		// Allow absolute only if already under root.
		clean := filepath.Clean(rel)
		if !isUnder(w.root, clean) {
			return "", fmt.Errorf("path escapes workspace: %s", rel)
		}
		return clean, nil
	}
	joined := filepath.Clean(filepath.Join(w.root, rel))
	if !isUnder(w.root, joined) {
		return "", fmt.Errorf("path escapes workspace: %s", rel)
	}
	return joined, nil
}

func isUnder(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}
