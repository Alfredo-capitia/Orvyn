package logs

import (
	"bytes"
	"strings"
	"testing"
)

func TestCollectDemultiplexesAndRedacts(t *testing.T) {
	src := multiplex(t, "user=admin password=hunter2\nhello\n", "token:abc123\n")

	lines, err := Collect(bytes.NewReader(src), false, 64*1024, 100)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	joined := strings.Join(lines, "\n")
	for _, secret := range []string{"hunter2", "abc123"} {
		if strings.Contains(joined, secret) {
			t.Errorf("output leaked %q: %q", secret, joined)
		}
	}
	for _, want := range []string{"user=admin", "hello"} {
		if !strings.Contains(joined, want) {
			t.Errorf("output missing %q: %q", want, joined)
		}
	}
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3: %q", len(lines), lines)
	}
}

func TestCollectKeepsOnlyTheTail(t *testing.T) {
	var src bytes.Buffer
	for _, line := range []string{"one", "two", "three", "four", "five"} {
		src.WriteString(line + "\n")
	}

	lines, err := Collect(&src, true, 64*1024, 2)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if got, want := strings.Join(lines, ","), "four,five"; got != want {
		t.Errorf("tail = %q, want %q", got, want)
	}
}

func TestCollectStripsCRLF(t *testing.T) {
	lines, err := Collect(strings.NewReader("first\r\nsecond\r\n"), true, 64*1024, 10)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	for _, line := range lines {
		if strings.HasSuffix(line, "\r") {
			t.Errorf("line keeps a carriage return: %q", line)
		}
	}
}

func TestCollectEmptyStream(t *testing.T) {
	lines, err := Collect(strings.NewReader(""), false, 64*1024, 10)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("got %d lines from an empty stream: %q", len(lines), lines)
	}
}

func TestCollectBoundsLongLines(t *testing.T) {
	long := strings.Repeat("a", MaxLineBytes+128)
	lines, err := Collect(strings.NewReader(long+"\nnext\n"), true, 2*MaxLineBytes, 10)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if !strings.HasSuffix(lines[0], truncationMarker) {
		t.Errorf("long line was not marked truncated: %d bytes", len(lines[0]))
	}
	if lines[1] != "next" {
		t.Errorf("second line = %q, want next", lines[1])
	}
}

func TestCollectSurvivesTruncatedFrame(t *testing.T) {
	src := multiplex(t, "complete line\n", "")
	// Cut the stream in the middle of the last frame header.
	truncated := src[:len(src)-4]

	lines, err := Collect(bytes.NewReader(truncated), false, 64*1024, 10)
	if err != nil && len(lines) == 0 {
		t.Fatalf("Collect discarded everything for a truncated frame: %v", err)
	}
	if len(lines) > 0 && lines[0] != "complete line" {
		t.Errorf("line = %q, want complete line", lines[0])
	}
}
