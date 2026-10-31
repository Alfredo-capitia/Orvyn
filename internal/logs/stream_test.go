package logs

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/stdcopy"
)

// multiplex builds the framed stream Docker returns for containers without a
// TTY: an 8-byte header per write followed by the payload.
func multiplex(t *testing.T, stdout, stderr string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if stdout != "" {
		w := stdcopy.NewStdWriter(&buf, stdcopy.Stdout)
		if _, err := io.WriteString(w, stdout); err != nil {
			t.Fatalf("write stdout frame: %v", err)
		}
	}
	if stderr != "" {
		w := stdcopy.NewStdWriter(&buf, stdcopy.Stderr)
		if _, err := io.WriteString(w, stderr); err != nil {
			t.Fatalf("write stderr frame: %v", err)
		}
	}
	return buf.Bytes()
}

func TestCopyStreamDemultiplexesAndRedacts(t *testing.T) {
	src := multiplex(t, "user=admin password=hunter2\nhello\n", "token:abc123\n")

	var out bytes.Buffer
	if err := copyStream(&out, bytes.NewReader(src), false); err != nil {
		t.Fatalf("copyStream: %v", err)
	}

	got := out.String()
	for _, secret := range []string{"hunter2", "abc123"} {
		if strings.Contains(got, secret) {
			t.Errorf("output leaked %q:\n%s", secret, got)
		}
	}
	for _, want := range []string{"user=admin", "hello"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	// The 8-byte frame headers must not reach the terminal.
	if strings.ContainsRune(got, '\x01') || strings.ContainsRune(got, '\x00') {
		t.Errorf("output contains stream framing bytes: %q", got)
	}
}

func TestCopyStreamRawTTY(t *testing.T) {
	var out bytes.Buffer
	if err := copyStream(&out, strings.NewReader("password=hunter2\nplain\n"), true); err != nil {
		t.Fatalf("copyStream: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "hunter2") {
		t.Errorf("output leaked secret: %q", got)
	}
	if !strings.Contains(got, "plain") {
		t.Errorf("output missing payload: %q", got)
	}
}

func TestCopyStreamTruncatesLongLines(t *testing.T) {
	long := strings.Repeat("a", MaxLineBytes+4096)
	src := strings.NewReader(long + "\nafter-long-line\n")

	var out bytes.Buffer
	if err := copyStream(&out, src, true); err != nil {
		t.Fatalf("copyStream: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, truncationMarker) {
		t.Errorf("long line was not marked truncated")
	}
	if !strings.Contains(got, "after-long-line") {
		t.Errorf("stream did not resume after the long line")
	}
	if len(got) > MaxLineBytes+len(truncationMarker)+len("after-long-line\n")+2 {
		t.Errorf("output not bounded: %d bytes", len(got))
	}
}

func TestLineWriterFlushesPartialLine(t *testing.T) {
	var out bytes.Buffer
	lw := &lineWriter{w: &out, max: MaxLineBytes}
	if _, err := lw.Write([]byte("no trailing newline")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := lw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := out.String(), "no trailing newline\n"; got != want {
		t.Errorf("Flush output = %q, want %q", got, want)
	}
}

func TestCopyStreamRejectsUnknownFrameType(t *testing.T) {
	// Stream type 9 is not a valid Docker stream.
	src := []byte{9, 0, 0, 0, 0, 0, 0, 0}

	var out bytes.Buffer
	if err := copyStream(&out, bytes.NewReader(src), false); err == nil {
		t.Fatal("copyStream accepted an unknown frame type")
	}
}
