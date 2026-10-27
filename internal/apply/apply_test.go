package apply

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// isolateState points config.Default at a temporary home so the tracker never
// reads or writes the real user state.
func isolateState(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("config.Default uses system-wide paths when running as root")
	}
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { _ = Clear() })
}

func TestNewSessionStartsEmpty(t *testing.T) {
	s := NewSession()
	if s.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress", s.Status)
	}
	if s.Compose != "" {
		t.Errorf("compose = %q, want empty", s.Compose)
	}
	if len(s.Steps) != 0 || len(s.Created) != 0 {
		t.Errorf("new session is not empty: steps=%v created=%v", s.Steps, s.Created)
	}
	if s.StartedAt.IsZero() {
		t.Error("started_at is zero")
	}
	if _, err := time.Parse("20060102-150405", s.ID); err != nil {
		t.Errorf("session ID %q does not match the timestamp format: %v", s.ID, err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	isolateState(t)

	s := NewSession()
	s.Compose = "/srv/stack/docker-compose.yml"
	s.AddStep("preflight", "done", "-")
	s.AddStep("generate_stack", "failed", "/srv/stack")
	s.Steps[len(s.Steps)-1].Err = "no space left on device"
	s.AddCreated("/srv/stack")
	s.Status = "failed"

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Status != "failed" {
		t.Errorf("status = %q, want failed", got.Status)
	}
	if got.Compose != s.Compose {
		t.Errorf("compose = %q, want %q", got.Compose, s.Compose)
	}
	if len(got.Steps) != 2 {
		t.Fatalf("loaded %d steps, want 2: %+v", len(got.Steps), got.Steps)
	}
	last := got.Steps[1]
	if last.Name != "generate_stack" || last.Status != "failed" || last.Err != "no space left on device" {
		t.Errorf("failed step did not survive the round trip: %+v", last)
	}
	if len(got.Created) != 1 || got.Created[0] != "/srv/stack" {
		t.Errorf("created = %v, want [/srv/stack]", got.Created)
	}
}

func TestLoadMissingState(t *testing.T) {
	isolateState(t)

	if _, err := Load(); !os.IsNotExist(err) {
		t.Fatalf("Load = %v, want a not-exist error", err)
	}
}

func TestSaveUsesOwnerOnlyPermissions(t *testing.T) {
	isolateState(t)

	if err := NewSession().Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	p := sessionPath()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("state file mode = %o, want 0600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(p))
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("state dir mode = %o, want 0700", got)
	}
}

func TestHasPartialOnlyForInterruptedSessions(t *testing.T) {
	isolateState(t)

	if HasPartial() {
		t.Fatal("HasPartial is true without a saved session")
	}

	s := NewSession()
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !HasPartial() {
		t.Error("HasPartial did not report an in-progress session")
	}

	// apply sets status=failed before returning, after printing the recovery
	// report itself. The next run must not treat that finished failure as an
	// interrupted apply; an in_progress file means the process died mid-run.
	s.Status = "failed"
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if HasPartial() {
		t.Error("HasPartial reported a failed session as partial")
	}

	s.Status = "done"
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if HasPartial() {
		t.Error("HasPartial reported a completed session as partial")
	}
}

func TestHasPartialIgnoresCorruptState(t *testing.T) {
	isolateState(t)

	p := sessionPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if HasPartial() {
		t.Error("HasPartial reported a corrupt state file as partial")
	}
}

func TestSecondSessionReplacesPreviousState(t *testing.T) {
	isolateState(t)

	first := NewSession()
	first.AddStep("docker_compose_up", "done", "/srv/stack/docker-compose.yml")
	first.AddCreated("/srv/stack")
	first.Status = "done"
	if err := first.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Re-running apply over an already-applied state starts a new session. The
	// tracker is a per-run journal, so it must replace the previous run instead
	// of appending to it or resurrecting its created paths.
	second := NewSession()
	second.AddStep("preflight", "done", "-")
	if err := second.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Steps) != 1 || got.Steps[0].Name != "preflight" {
		t.Errorf("steps after re-run = %+v, want only the new preflight step", got.Steps)
	}
	if len(got.Created) != 0 {
		t.Errorf("created after re-run = %v, want empty", got.Created)
	}
	if got.Status != "in_progress" {
		t.Errorf("status after re-run = %q, want in_progress", got.Status)
	}
}

func TestRecoveryReportAfterPartialFailure(t *testing.T) {
	isolateState(t)

	dir := t.TempDir()
	compose := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(compose, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(dir, "backups", "backup-20260101T000000Z.tar.gz")
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath, []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}

	// A failure after the snapshot but before the stack is fully generated is
	// the realistic partial state: the backup is the rollback point and the
	// generated files must not be deleted or trusted blindly.
	s := NewSession()
	s.Compose = compose
	s.AddStep("preflight", "done", "-")
	s.AddStep("backup", "done", backupPath)
	s.AddCreated(compose)
	s.AddStep("generate_stack", "in_progress", dir)
	s.AddStep("generate_stack", "failed", dir)
	s.Steps[len(s.Steps)-1].Err = "permission denied"
	s.Status = "failed"
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var out bytes.Buffer
	PrintRecoveryTo(&out, loaded)
	report := out.String()

	for _, want := range []string{
		"PARTIAL FAILURE - RECOVERY",
		s.ID,
		"status: failed",
		compose,
		backupPath,
		"permission denied",
		"orvyn apply",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("recovery report does not mention %q:\n%s", want, report)
		}
	}

	// Recovery is advisory: it must not delete the files it lists, and the
	// backup has to stay available for 'orvyn restore'.
	for _, p := range []string{compose, backupPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("recovery removed %s: %v", p, err)
		}
	}
}

func TestClearRemovesSession(t *testing.T) {
	isolateState(t)

	if err := NewSession().Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := Load(); !os.IsNotExist(err) {
		t.Fatalf("Load after Clear = %v, want a not-exist error", err)
	}
	if HasPartial() {
		t.Error("HasPartial is true after Clear")
	}
}
