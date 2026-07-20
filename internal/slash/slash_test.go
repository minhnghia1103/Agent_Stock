package slash_test

import (
	"strings"
	"testing"

	"agent_stock/internal/skills"
	"agent_stock/internal/slash"
)

func TestHelpAndSkillsDirect(t *testing.T) {
	root := t.TempDir()
	_ = skills.SeedIfMissing(root)
	reg := skills.NewRegistry(root, "")
	_ = reg.Reload()

	r := slash.Dispatch("/help", reg)
	if !r.Handled || r.Action != slash.ActionDirect || !strings.Contains(r.Reply, "/skills") {
		t.Fatalf("%#v", r)
	}

	r = slash.Dispatch("/skills", reg)
	if !r.Handled || r.Action != slash.ActionDirect || !strings.Contains(r.Reply, "stock-summary") {
		t.Fatalf("%#v", r)
	}
}

func TestSkillContinue(t *testing.T) {
	root := t.TempDir()
	_ = skills.SeedIfMissing(root)
	reg := skills.NewRegistry(root, "")
	_ = reg.Reload()

	r := slash.Dispatch("/stock-summary AAPL", reg)
	if !r.Handled || r.Action != slash.ActionContinue {
		t.Fatalf("%#v", r)
	}
	if !strings.Contains(r.EffectiveMsg, "get_stock_quote") {
		t.Fatalf("expected skill body in effective msg: %s", r.EffectiveMsg)
	}
	if !strings.Contains(r.EffectiveMsg, "AAPL") {
		t.Fatalf("expected args: %s", r.EffectiveMsg)
	}
}

func TestUnknownSlash(t *testing.T) {
	r := slash.Dispatch("/nope", nil)
	if !r.Handled || r.Action != slash.ActionDirect {
		t.Fatalf("%#v", r)
	}
}

func TestNotSlash(t *testing.T) {
	r := slash.Dispatch("hello", nil)
	if r.Handled {
		t.Fatal("should not handle")
	}
}
