package redact

import "github.com/fredy-arias/vpsmon/internal/redaction"

// Redact re-exports redaction for naming compatibility
func Redact(s string) string {
	return redaction.Redact(s)
}
