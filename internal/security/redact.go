package security

import "regexp"

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(sk-[A-Za-z0-9_-]{16,})\b`),
	regexp.MustCompile(`(?i)\b(api[_-]?key\s*[:=]\s*)([^\s"'\\]{12,})`),
	regexp.MustCompile(`(?i)\b(Bearer\s+)([A-Za-z0-9\-._~+/]+=*)`),
	regexp.MustCompile(`(?i)\b(AGENT_LLM_API_KEY\s*[:=]\s*)(\S+)`),
	regexp.MustCompile(`(?i)\b(xox[baprs]-[0-9A-Za-z-]{10,})\b`),
}

// RedactSecrets masks common secret patterns in text.
func RedactSecrets(s string) string {
	out := s
	for _, re := range secretPatterns {
		out = re.ReplaceAllStringFunc(out, func(m string) string {
			if sub := re.FindStringSubmatch(m); len(sub) >= 3 {
				return sub[1] + "[REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return out
}

// Redact applies policy-gated redaction.
func Redact(pol *Policy, s string) string {
	if pol == nil || !pol.OutputRedaction.Enabled {
		return s
	}
	return RedactSecrets(s)
}
