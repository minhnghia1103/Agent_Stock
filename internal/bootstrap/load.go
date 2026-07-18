package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Standard bootstrap filenames (ews-style + goclaw aliases).
const (
	FileMAIN     = "MAIN.md"
	FileAGENT    = "AGENT.md"
	FileAGENTS   = "AGENTS.md" // goclaw alias — loaded if AGENT.md missing
	FileSOUL     = "SOUL.md"
	FileTOOLS    = "TOOLS.md"
	FileUSER     = "USER.md"
	FileIDENTITY = "IDENTITY.md"
	FileMEMORY   = "MEMORY.md"
)

// File is one workspace context markdown file.
type File struct {
	Name    string
	Path    string
	Content string
	Missing bool
}

const maxFileChars = 20_000

// orderedNames is the injection order for the system prompt.
var orderedNames = []string{
	FileMAIN,
	FileAGENT,
	FileSOUL,
	FileTOOLS,
	FileUSER,
	FileIDENTITY,
	FileMEMORY,
}

// Load reads bootstrap files from workspaceDir (missing → Missing=true).
// Prefer AGENT.md; if missing, fall back to AGENTS.md content under name AGENT.md.
func Load(workspaceDir string) []File {
	out := make([]File, 0, len(orderedNames))
	for _, name := range orderedNames {
		f := loadOne(workspaceDir, name)
		if name == FileAGENT && f.Missing {
			alt := loadOne(workspaceDir, FileAGENTS)
			if !alt.Missing {
				f = File{Name: FileAGENT, Path: alt.Path, Content: alt.Content, Missing: false}
			}
		}
		out = append(out, f)
	}
	return out
}

func loadOne(dir, name string) File {
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return File{Name: name, Path: path, Missing: true}
	}
	content := string(data)
	if len(content) > maxFileChars {
		content = content[:maxFileChars] + "\n\n...[truncated]"
	}
	return File{Name: name, Path: path, Content: content, Missing: false}
}

// BuildSystemPrompt composes base prompt + present bootstrap sections.
func BuildSystemPrompt(base string, files []File) string {
	var b strings.Builder
	base = strings.TrimSpace(base)
	if base == "" {
		base = "You are a helpful stock research assistant. Use tools when they improve accuracy."
	}
	b.WriteString(base)
	b.WriteString("\n\n# Workspace context\n")
	b.WriteString("The following markdown files define how you operate. Follow them.\n")

	any := false
	for _, f := range files {
		if f.Missing || strings.TrimSpace(f.Content) == "" {
			continue
		}
		any = true
		b.WriteString("\n## ")
		b.WriteString(f.Name)
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(f.Content))
		b.WriteString("\n")
	}
	if !any {
		b.WriteString("\n_(No workspace markdown loaded yet.)_\n")
	}
	return b.String()
}

// PresentNames returns names of files that were loaded (for logging).
func PresentNames(files []File) []string {
	var names []string
	for _, f := range files {
		if !f.Missing && strings.TrimSpace(f.Content) != "" {
			names = append(names, f.Name)
		}
	}
	return names
}

// SeedIfMissing writes default templates for core files that do not exist.
func SeedIfMissing(workspaceDir string) error {
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return fmt.Errorf("mkdir workspace: %w", err)
	}
	seeds := map[string]string{
		FileMAIN:  defaultMAIN,
		FileAGENT: defaultAGENT,
		FileSOUL:  defaultSOUL,
		FileTOOLS: defaultTOOLS,
		FileUSER:  defaultUSER,
	}
	for name, content := range seeds {
		path := filepath.Join(workspaceDir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("seed %s: %w", path, err)
		}
	}
	return nil
}
