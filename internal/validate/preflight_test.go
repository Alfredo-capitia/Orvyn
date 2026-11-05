package validate

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeDocker struct {
	pingErr error
	closed  bool
}

func (f *fakeDocker) Ping(context.Context) error { return f.pingErr }
func (f *fakeDocker) Close() error               { f.closed = true; return nil }

type probes struct {
	client    dockerPinger
	clientErr error
	cmdOut    []byte
	cmdErr    error
	stat      func(string) (os.FileInfo, error)
	access    func(string, uint32) error
	dial      func(string, string, time.Duration) (net.Conn, error)
}

// presentFile returns a real FileInfo for every path, simulating an existing
// socket and /home without depending on the test machine's layout.
func presentFile(t *testing.T) func(string) (os.FileInfo, error) {
	t.Helper()
	p := t.TempDir() + "/present"
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat seed file: %v", err)
	}
	return func(string) (os.FileInfo, error) { return info, nil }
}

func useProbes(t *testing.T, p probes) {
	t.Helper()
	oldNew, oldRun, oldDial := newDockerClient, runCommand, dialRegistry
	oldStat, oldAccess := statPath, accessPath
	t.Cleanup(func() {
		newDockerClient, runCommand, dialRegistry = oldNew, oldRun, oldDial
		statPath, accessPath = oldStat, oldAccess
	})

	newDockerClient = func() (dockerPinger, error) {
		if p.clientErr != nil {
			return nil, p.clientErr
		}
		return p.client, nil
	}
	runCommand = func(context.Context, time.Duration, string, ...string) ([]byte, error) {
		return p.cmdOut, p.cmdErr
	}
	statPath = p.stat
	accessPath = p.access
	dialRegistry = p.dial
}

func closedConn(t *testing.T) func(string, string, time.Duration) (net.Conn, error) {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return func(string, string, time.Duration) (net.Conn, error) { return client, nil }
}

func checkByName(r *Report, name string) *Check {
	for i := range r.Checks {
		if r.Checks[i].Name == name {
			return &r.Checks[i]
		}
	}
	return nil
}

func TestPreflightAllChecksPass(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("socket permission check only runs on linux")
	}
	cli := &fakeDocker{}
	useProbes(t, probes{
		client: cli,
		cmdOut: []byte("Docker Compose version v2.40.0"),
		stat:   presentFile(t),
		access: func(string, uint32) error { return nil },
		dial:   closedConn(t),
	})

	r := Preflight(context.Background())
	if !r.OK {
		t.Fatalf("report not OK: %+v", r.Checks)
	}
	for _, name := range []string{"docker_daemon", "docker_compose", "docker_socket", "registry", "disk"} {
		c := checkByName(r, name)
		if c == nil {
			t.Fatalf("missing check %q", name)
		}
		if c.Status != "OK" {
			t.Errorf("%s = %s (%s), want OK", name, c.Status, c.Msg)
		}
	}
	if c := checkByName(r, "docker_compose"); !strings.Contains(c.Msg, "v2.40.0") {
		t.Errorf("compose message lost the command output: %q", c.Msg)
	}
	if !cli.closed {
		t.Error("docker client was not closed")
	}
}

func TestPreflightFailsWithoutDockerClient(t *testing.T) {
	useProbes(t, probes{
		clientErr: errors.New("bad DOCKER_HOST"),
		cmdOut:    []byte("ok"),
		stat:      func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		access:    func(string, uint32) error { return nil },
		dial:      closedConn(t),
	})

	r := Preflight(context.Background())
	if r.OK {
		t.Fatal("report OK without a docker client")
	}
	c := checkByName(r, "docker_daemon")
	if c == nil || c.Status != "FAIL" || !strings.Contains(c.Msg, "bad DOCKER_HOST") {
		t.Fatalf("daemon check = %+v, want FAIL with the client error", c)
	}
}

func TestPreflightFailsWhenDaemonIsUnreachable(t *testing.T) {
	useProbes(t, probes{
		client: &fakeDocker{pingErr: errors.New("connection refused")},
		cmdOut: []byte("ok"),
		stat:   func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		access: func(string, uint32) error { return nil },
		dial:   closedConn(t),
	})

	r := Preflight(context.Background())
	if r.OK {
		t.Fatal("report OK with an unreachable daemon")
	}
	if c := checkByName(r, "docker_daemon"); c == nil || c.Status != "FAIL" {
		t.Fatalf("daemon check = %+v, want FAIL", c)
	}
}

func TestPreflightFailsWithoutCompose(t *testing.T) {
	useProbes(t, probes{
		client: &fakeDocker{},
		cmdErr: errors.New("exec: docker: not found"),
		stat:   func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		access: func(string, uint32) error { return nil },
		dial:   closedConn(t),
	})

	r := Preflight(context.Background())
	if r.OK {
		t.Fatal("report OK without docker compose")
	}
	if c := checkByName(r, "docker_compose"); c == nil || c.Status != "FAIL" {
		t.Fatalf("compose check = %+v, want FAIL", c)
	}
}

func TestPreflightWarnsOnReadOnlySocket(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("socket permission check only runs on linux")
	}
	useProbes(t, probes{
		client: &fakeDocker{},
		cmdOut: []byte("ok"),
		stat:   presentFile(t),
		access: func(string, uint32) error { return syscall.EACCES },
		dial:   closedConn(t),
	})

	r := Preflight(context.Background())
	if !r.OK {
		t.Fatal("a read-only socket must warn, not fail")
	}
	if c := checkByName(r, "docker_socket"); c == nil || c.Status != "WARN" {
		t.Fatalf("socket check = %+v, want WARN", c)
	}
}

func TestPreflightSkipsMissingSocket(t *testing.T) {
	useProbes(t, probes{
		client: &fakeDocker{},
		cmdOut: []byte("ok"),
		stat:   func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		access: func(string, uint32) error { return nil },
		dial:   closedConn(t),
	})

	r := Preflight(context.Background())
	if c := checkByName(r, "docker_socket"); c != nil {
		t.Fatalf("socket check reported for a missing socket: %+v", c)
	}
}

func TestPreflightWarnsWhenRegistryIsUnreachable(t *testing.T) {
	useProbes(t, probes{
		client: &fakeDocker{},
		cmdOut: []byte("ok"),
		stat:   func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		access: func(string, uint32) error { return nil },
		dial: func(string, string, time.Duration) (net.Conn, error) {
			return nil, errors.New("dial tcp: i/o timeout")
		},
	})

	r := Preflight(context.Background())
	if !r.OK {
		t.Fatal("an unreachable registry is best-effort and must not fail the report")
	}
	if c := checkByName(r, "registry"); c == nil || c.Status != "WARN" {
		t.Fatalf("registry check = %+v, want WARN", c)
	}
}

func TestReportPrint(t *testing.T) {
	r := &Report{OK: true, Checks: []Check{
		{Name: "docker_daemon", Status: "OK", Msg: "daemon reachable"},
		{Name: "registry", Status: "WARN", Msg: "unreachable"},
	}}

	out := captureStdout(t, r.Print)
	for _, want := range []string{"[PRE-FLIGHT]", "[OK] docker_daemon: daemon reachable", "[WARN] registry: unreachable"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w

	fn()

	os.Stdout = old
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	r.Close()
	return string(data)
}
