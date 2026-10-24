package redaction

import (
	"regexp"
	"strings"
)

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`),
	regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----[\s\S]*?-----END\s+(RSA\s+)?PRIVATE\s+KEY-----`),
	regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|api[_-]?key|auth|bearer)\s*[:=]\s*["']?[^\s"'&]+["']?`),
	regexp.MustCompile(`[a-zA-Z0-9._%+-]+:[^@\s]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`), // basic connection strings
}

func Redact(s string) string {
	if s == "" {
		return s
	}
	result := s
	for _, p := range patterns {
		result = p.ReplaceAllStringFunc(result, func(m string) string {
			if strings.Contains(m, "BEGIN") && strings.Contains(m, "PRIVATE") {
				return "[REDACTED-PRIVATE-KEY]"
			}
			// For key-value style, try to preserve key name
			if strings.Contains(m, "=") || strings.Contains(m, ":") {
				// Keep prefix up to = or :
				for i := 0; i < len(m); i++ {
					if m[i] == '=' || m[i] == ':' {
						return m[:i+1] + " [REDACTED]"
					}
				}
			}
			return "[REDACTED]"
		})
	}
	return result
}
