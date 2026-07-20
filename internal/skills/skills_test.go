package skills_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent_stock/internal/skills"
)

func TestSeedAndList(t *testing.T) {
	root := t.TempDir()
	if err := skills.SeedIfMissing(root); err != nil {
		t.Fatal(err)
	}
	reg := skills.NewRegistry(root, "")
	if err := reg.Reload(); err != nil {
		t.Fatal(err)
	}
	list := reg.List()
	if len(list) < 2 {
		t.Fatalf("expected seeded skills, got %#v", list)
	}
	sk, ok := reg.Get("stock-summary")
	if !ok || sk.Body == "" {
		t.Fatalf("stock-summary missing body: %#v", sk)
	}
	sec := reg.FormatMetadataSection()
	if !strings.Contains(sec, "stock-summary") {
		t.Fatalf("metadata section missing skill: %s", sec)
	}
	if strings.Contains(sec, "get_stock_quote") {
		// body instruction should NOT be in metadata section
		t.Fatalf("progressive disclosure violated: body leaked into metadata")
	}
}

func TestSearch(t *testing.T) {
	root := t.TempDir()
	_ = skills.SeedIfMissing(root)
	reg := skills.NewRegistry(root, "")
	_ = reg.Reload()
	hits := reg.Search("macro")
	if len(hits) != 1 || hits[0].Slug != "macro-brief" {
		t.Fatalf("search macro: %#v", hits)
	}
}

func TestParseFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: demo\ndescription: Hello skill\n---\n\n# Body\nDo the thing.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := skills.NewRegistry(dir, "")
	if err := reg.Reload(); err != nil {
		t.Fatal(err)
	}
	sk, ok := reg.Get("demo")
	if !ok {
		t.Fatal("missing demo")
	}
	if sk.Description != "Hello skill" {
		t.Fatalf("desc=%q", sk.Description)
	}
	if !strings.Contains(sk.Body, "Do the thing") {
		t.Fatalf("body=%q", sk.Body)
	}
}
