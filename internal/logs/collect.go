package logs

import (
	"bytes"
	"io"
	"strings"
)

// Collect reads at most maxBytes from src, demultiplexes Docker's stream unless
// tty is set, and returns the last maxLines lines. Each line goes through the
// same redaction and length bound as Stream, so a dashboard cannot bypass the
// filtering the CLI applies.
//
// It is meant for bounded reads (the TUI asks for a tail, not a follow); the
// caller must pass a positive maxBytes and maxLines to bound memory and screen
// space.
func Collect(src io.Reader, tty bool, maxBytes int64, maxLines int) ([]string, error) {
	raw, err := io.ReadAll(io.LimitReader(src, maxBytes))
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	// A capped read can cut the last frame in half. Keep what was decoded as
	// long as something survived; only report the error when nothing did.
	if derr := copyStream(&out, bytes.NewReader(raw), tty); derr != nil && out.Len() == 0 {
		return nil, derr
	}

	text := strings.TrimRight(out.String(), "\n")
	if text == "" {
		return nil, nil
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}
