package security

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// ErrPromptBlocked is returned when prompt_injection.action=block and patterns match.
var ErrPromptBlocked = errors.New("prompt blocked by security policy")

type guardPattern struct {
	name    string
	pattern *regexp.Regexp
}

// InputGuard scans user messages for common prompt-injection patterns.
type InputGuard struct {
	patterns []guardPattern
}

func NewInputGuard() *InputGuard {
	return &InputGuard{patterns: defaultGuardPatterns()}
}

func (g *InputGuard) Scan(message string, maxChars int) []string {
	if message == "" {
		return nil
	}
	if maxChars > 0 && len(message) > maxChars {
		message = message[:maxChars]
	}
	var matches []string
	for _, gp := range g.patterns {
		if gp.pattern.MatchString(message) {
			matches = append(matches, gp.name)
		}
	}
	return matches
}

// CheckUserMessage applies policy action: off|log|warn|block.
// Returns error only when action=block and patterns match.
func CheckUserMessage(pol *Policy, guard *InputGuard, message string) error {
	if pol == nil || !pol.PromptInjection.Enabled || guard == nil {
		return nil
	}
	action := pol.PromptInjection.Action
	if action == "off" {
		return nil
	}
	matches := guard.Scan(message, pol.PromptInjection.MaxScanChars)
	if len(matches) == 0 {
		return nil
	}
	msg := fmt.Sprintf("prompt injection patterns matched: %s", strings.Join(matches, ", "))
	switch action {
	case "block":
		slog.Warn("security.prompt_injection.block", "matches", matches)
		return fmt.Errorf("%w: %s", ErrPromptBlocked, msg)
	case "log":
		slog.Info("security.prompt_injection", "matches", matches)
	default: // warn
		slog.Warn("security.prompt_injection", "matches", matches)
	}
	return nil
}

func defaultGuardPatterns() []guardPattern {
	return []guardPattern{
		{
			name:    "ignore_instructions",
			pattern: regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous|prior|above|earlier|preceding)\s+(instructions?|rules?|prompts?|directives?|guidelines?)`),
		},
		{
			name:    "role_override",
			pattern: regexp.MustCompile(`(?i)(you are now|from now on you are|pretend you are|act as if you are)\s+`),
		},
		{
			name:    "system_tags",
			pattern: regexp.MustCompile(`(?i)</?system>|\[SYSTEM\]|\[INST\]|<<SYS>>|<\|im_start\|>system`),
		},
		{
			name:    "instruction_injection",
			pattern: regexp.MustCompile(`(?i)(new instructions?:|override:|system prompt:|<\|system\|>)`),
		},
		{
			name:    "null_bytes",
			pattern: regexp.MustCompile(`\x00`),
		},
		{
			name:    "delimiter_escape",
			pattern: regexp.MustCompile(`(?i)(end of system|begin user input|</?(instructions?|rules|prompt|context)>)`),
		},
	}
}
