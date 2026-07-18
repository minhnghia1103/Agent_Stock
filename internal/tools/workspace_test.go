package tools_test

import (
	"path/filepath"
	"testing"

	"agent_stock/internal/tools"
)

func TestWorkspacePathJail(t *testing.T) {
	root := t.TempDir()
	ws, err := tools.NewWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := ws.Resolve("notes/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(ws.Root(), "notes", "a.txt")
	if ok != want {
		t.Fatalf("got %s want %s", ok, want)
	}

	if _, err := ws.Resolve("../etc/passwd"); err == nil {
		t.Fatal("expected path escape blocked")
	}
	if _, err := ws.Resolve("/etc/passwd"); err == nil {
		t.Fatal("expected absolute escape blocked")
	}
}
