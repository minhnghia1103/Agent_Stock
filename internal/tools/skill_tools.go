package tools

import (
	"context"
	"fmt"
	"strings"

	"agent_stock/internal/skills"
)

type skillSearchTool struct{ reg *skills.Registry }

func NewSkillSearchTool(reg *skills.Registry) Tool { return &skillSearchTool{reg: reg} }

func (t *skillSearchTool) Name() string { return "skill_search" }
func (t *skillSearchTool) Description() string {
	return "Search available skills by keyword (name/slug/description). Returns metadata only."
}
func (t *skillSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query (optional; empty = list all)",
			},
		},
	}
}
func (t *skillSearchTool) Execute(ctx context.Context, args map[string]any) Result {
	_ = ctx
	if t.reg == nil {
		return Err("skills registry not configured")
	}
	query := StringArg(args, "query")
	list := t.reg.Search(query)
	if len(list) == 0 {
		return OK("(no matching skills)")
	}
	var b strings.Builder
	for _, sk := range list {
		b.WriteString("- ")
		b.WriteString(sk.Slug)
		b.WriteString(": ")
		b.WriteString(sk.Description)
		b.WriteByte('\n')
	}
	return OK(b.String())
}

type useSkillTool struct{ reg *skills.Registry }

func NewUseSkillTool(reg *skills.Registry) Tool { return &useSkillTool{reg: reg} }

func (t *useSkillTool) Name() string { return "use_skill" }
func (t *useSkillTool) Description() string {
	return "Load the full SKILL.md body for a skill slug/name (progressive disclosure)."
}
func (t *useSkillTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Skill slug or name (e.g. stock-summary)",
			},
		},
		"required": []string{"name"},
	}
}
func (t *useSkillTool) Execute(ctx context.Context, args map[string]any) Result {
	_ = ctx
	if t.reg == nil {
		return Err("skills registry not configured")
	}
	name := StringArg(args, "name")
	sk, ok := t.reg.Get(name)
	if !ok {
		return Err(fmt.Sprintf("skill not found: %s", name))
	}
	var b strings.Builder
	b.WriteString("Skill: ")
	b.WriteString(sk.Slug)
	b.WriteString("\nDescription: ")
	b.WriteString(sk.Description)
	b.WriteString("\n\n")
	b.WriteString(sk.Body)
	return OK(b.String())
}
