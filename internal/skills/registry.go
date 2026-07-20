package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const maxBodyChars = 40_000

// Skill is one loaded SKILL.md (metadata always; body on demand).
type Skill struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Source      string `json:"source"` // workspace | bundled
	Body        string `json:"-"`
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Registry discovers skills under workspace/skills (+ optional bundled dir).
type Registry struct {
	mu           sync.RWMutex
	workspaceDir string
	bundledDir   string
	directDir    string // optional: scan this dir as skills root
	bySlug       map[string]*Skill
}

// NewRegistry creates a skills registry rooted at workspaceDir/skills.
func NewRegistry(workspaceDir, bundledDir string) *Registry {
	return &Registry{
		workspaceDir: workspaceDir,
		bundledDir:   bundledDir,
		bySlug:       map[string]*Skill{},
	}
}

// NewRegistryWithDir scans a skills directory directly (AGENT_SKILLS_PATH override).
func NewRegistryWithDir(skillsDir string) *Registry {
	r := &Registry{
		workspaceDir: "",
		bundledDir:   "",
		bySlug:       map[string]*Skill{},
		directDir:    skillsDir,
	}
	return r
}

// WorkspaceSkillsDir returns <workspace>/skills.
func WorkspaceSkillsDir(workspaceDir string) string {
	return filepath.Join(workspaceDir, "skills")
}

// Reload rescans skill directories. Workspace overrides bundled by slug.
func (r *Registry) Reload() error {
	bySlug := map[string]*Skill{}

	if r.bundledDir != "" {
		if err := scanDir(r.bundledDir, "bundled", bySlug); err != nil {
			return err
		}
	}
	if r.directDir != "" {
		if err := scanDir(r.directDir, "workspace", bySlug); err != nil {
			return err
		}
	} else if r.workspaceDir != "" {
		if err := scanDir(WorkspaceSkillsDir(r.workspaceDir), "workspace", bySlug); err != nil {
			return err
		}
	}

	r.mu.Lock()
	r.bySlug = bySlug
	r.mu.Unlock()
	return nil
}

func scanDir(dir, source string, into map[string]*Skill) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read skills dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		slug := e.Name()
		path := filepath.Join(dir, slug, "SKILL.md")
		sk, err := loadSkillFile(path, slug, source)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		into[sk.Slug] = sk
	}
	return nil
}

func loadSkillFile(path, slug, source string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	meta, body, err := parseSKILL(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		name = slug
	}
	if len(body) > maxBodyChars {
		body = body[:maxBodyChars] + "\n\n...[truncated]"
	}
	return &Skill{
		Name:        name,
		Slug:        slug,
		Description: strings.TrimSpace(meta.Description),
		Path:        path,
		Source:      source,
		Body:        body,
	}, nil
}

func parseSKILL(raw string) (frontmatter, string, error) {
	raw = strings.TrimSpace(raw)
	var meta frontmatter
	if !strings.HasPrefix(raw, "---") {
		return meta, raw, nil
	}
	rest := strings.TrimPrefix(raw, "---")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return meta, raw, fmt.Errorf("unclosed frontmatter")
	}
	fm := strings.TrimSpace(rest[:end])
	body := strings.TrimSpace(rest[end+len("\n---"):])
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return meta, "", fmt.Errorf("frontmatter yaml: %w", err)
	}
	return meta, body, nil
}

// List returns skills sorted by slug.
func (r *Registry) List() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Skill, 0, len(r.bySlug))
	for _, sk := range r.bySlug {
		cp := *sk
		cp.Body = "" // metadata-only for list
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// Get returns a skill by slug or name (case-insensitive).
func (r *Registry) Get(id string) (*Skill, bool) {
	id = strings.TrimSpace(strings.ToLower(id))
	if id == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if sk, ok := r.bySlug[id]; ok {
		cp := *sk
		return &cp, true
	}
	for _, sk := range r.bySlug {
		if strings.EqualFold(sk.Name, id) || strings.EqualFold(sk.Slug, id) {
			cp := *sk
			return &cp, true
		}
	}
	return nil, false
}

// Search filters skills by substring in name/slug/description.
func (r *Registry) Search(query string) []Skill {
	query = strings.TrimSpace(strings.ToLower(query))
	all := r.List()
	if query == "" {
		return all
	}
	var out []Skill
	for _, sk := range all {
		hay := strings.ToLower(sk.Name + " " + sk.Slug + " " + sk.Description)
		if strings.Contains(hay, query) {
			out = append(out, sk)
		}
	}
	return out
}

// FormatMetadataSection builds progressive-disclosure text for the system prompt.
func (r *Registry) FormatMetadataSection() string {
	list := r.List()
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# Available skills\n")
	b.WriteString("Skills provide specialized playbooks. Only metadata is listed here.\n")
	b.WriteString("Use tool skill_search to find skills, use_skill to load full instructions.\n")
	b.WriteString("Users may also invoke /skills or /<skill-slug> as slash commands.\n\n")
	for _, sk := range list {
		b.WriteString("- ")
		b.WriteString(sk.Slug)
		if sk.Description != "" {
			b.WriteString(": ")
			b.WriteString(sk.Description)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// SeedIfMissing writes demo skills under workspace/skills when missing.
func SeedIfMissing(workspaceDir string) error {
	root := WorkspaceSkillsDir(workspaceDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("mkdir skills: %w", err)
	}
	for slug, content := range defaultSkills {
		dir := filepath.Join(root, slug)
		path := filepath.Join(dir, "SKILL.md")
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("seed %s: %w", path, err)
		}
	}
	return nil
}

var defaultSkills = map[string]string{
	"stock-summary": `---
name: stock-summary
description: Tóm tắt nhanh 1 mã cổ phiếu (giá + ngữ cảnh ngắn). Dùng khi user hỏi giá hoặc overview 1 ticker.
---

# Stock summary

Khi user hỏi về một mã (vd AAPL, VNM.VN):

1. Gọi tool get_stock_quote với symbol đó.
2. Tóm tắt ngắn: giá gần nhất, đơn vị tiền, và 1-2 câu lưu ý (không phải lời khuyên đầu tư).
3. Nếu thiếu symbol, hỏi lại user.
`,
	"macro-brief": `---
name: macro-brief
description: Khung trả lời ngắn về macro/market (lãi suất, risk-on/off). Dùng khi hỏi tổng quan thị trường.
---

# Macro brief

1. Làm rõ khung thời gian (hôm nay / tuần / tháng) nếu user chưa nói.
2. Dùng web_fetch chỉ với nguồn công khai đáng tin nếu cần số liệu; không bịa số.
3. Trả lời gọn: 3-5 bullet (drivers, rủi ro, điều cần theo dõi tiếp).
4. Nhắc đây không phải khuyến nghị đầu tư.
`,
}
