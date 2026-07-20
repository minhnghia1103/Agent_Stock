package slash

import (
	"fmt"
	"strings"

	"agent_stock/internal/skills"
)

// Action kinds for slash dispatch.
const (
	ActionNone     = ""
	ActionDirect   = "direct"   // reply without LLM
	ActionContinue = "continue" // rewrite message, continue agent loop
)

// Result is the outcome of slash preprocessing.
type Result struct {
	Handled      bool
	Action       string
	Reply        string // for ActionDirect
	EffectiveMsg string // for ActionContinue (may include skill body)
	Command      string
}

// IsSlash reports whether the message looks like a slash command.
func IsSlash(message string) bool {
	msg := strings.TrimSpace(message)
	return strings.HasPrefix(msg, "/") && len(msg) > 1
}

// Dispatch handles /help, /skills, /<skill-slug> [args].
// Non-slash messages return Handled=false.
func Dispatch(message string, reg *skills.Registry) Result {
	msg := strings.TrimSpace(message)
	if !IsSlash(msg) {
		return Result{Handled: false}
	}

	body := strings.TrimSpace(strings.TrimPrefix(msg, "/"))
	parts := strings.Fields(body)
	if len(parts) == 0 {
		return Result{
			Handled: true,
			Action:  ActionDirect,
			Reply:   "Empty slash command. Try /help",
			Command: "",
		}
	}
	cmd := strings.ToLower(parts[0])
	args := strings.TrimSpace(strings.TrimPrefix(body, parts[0]))

	switch cmd {
	case "help":
		return Result{
			Handled: true,
			Action:  ActionDirect,
			Command: cmd,
			Reply:   helpText(reg),
		}
	case "skills", "list-skills":
		return Result{
			Handled: true,
			Action:  ActionDirect,
			Command: cmd,
			Reply:   listSkillsText(reg),
		}
	default:
		if reg == nil {
			return Result{
				Handled: true,
				Action:  ActionDirect,
				Command: cmd,
				Reply:   fmt.Sprintf("Unknown slash command /%s. Try /help", cmd),
			}
		}
		sk, ok := reg.Get(cmd)
		if !ok {
			return Result{
				Handled: true,
				Action:  ActionDirect,
				Command: cmd,
				Reply:   fmt.Sprintf("Unknown slash command /%s. Try /help or /skills", cmd),
			}
		}
		var b strings.Builder
		b.WriteString("Follow the skill playbook below to answer the user.\n\n")
		b.WriteString("<<<SKILL:")
		b.WriteString(sk.Slug)
		b.WriteString(">>>\n")
		b.WriteString(sk.Body)
		b.WriteString("\n<<<END_SKILL>>>\n")
		if args != "" {
			b.WriteString("\nUser request after slash: ")
			b.WriteString(args)
			b.WriteByte('\n')
		} else {
			b.WriteString("\nUser invoked this skill with no extra args. Ask for missing details if needed.\n")
		}
		return Result{
			Handled:      true,
			Action:       ActionContinue,
			Command:      cmd,
			EffectiveMsg: b.String(),
		}
	}
}

func helpText(reg *skills.Registry) string {
	var b strings.Builder
	b.WriteString("Slash commands:\n")
	b.WriteString("- /help — show this help\n")
	b.WriteString("- /skills — list available skills (metadata only)\n")
	b.WriteString("- /<skill-slug> [args] — load skill playbook and continue with the agent\n")
	if reg != nil {
		list := reg.List()
		if len(list) > 0 {
			b.WriteString("\nSkill slugs:\n")
			for _, sk := range list {
				b.WriteString("  /")
				b.WriteString(sk.Slug)
				if sk.Description != "" {
					b.WriteString(" — ")
					b.WriteString(sk.Description)
				}
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func listSkillsText(reg *skills.Registry) string {
	if reg == nil {
		return "No skills registry configured."
	}
	list := reg.List()
	if len(list) == 0 {
		return "No skills found. Add workspace/skills/<slug>/SKILL.md"
	}
	var b strings.Builder
	b.WriteString("Available skills:\n\n")
	b.WriteString("| Slash | Description | Source |\n")
	b.WriteString("|-------|-------------|--------|\n")
	for _, sk := range list {
		desc := sk.Description
		if desc == "" {
			desc = "(no description)"
		}
		b.WriteString("| /")
		b.WriteString(sk.Slug)
		b.WriteString(" | ")
		b.WriteString(strings.ReplaceAll(desc, "|", "/"))
		b.WriteString(" | ")
		b.WriteString(sk.Source)
		b.WriteString(" |\n")
	}
	b.WriteString("\nInvoke with /<slug> [args]. Use skill_search / use_skill tools in normal chat.")
	return b.String()
}
