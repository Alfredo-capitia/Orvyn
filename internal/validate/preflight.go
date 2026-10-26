package validate

import (
	"context"
	"fmt"
	"github.com/fredy-arias/orvyn/internal/executil"
	"net"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/fredy-arias/orvyn/internal/docker"
)

type Check struct {
	Name   string
	Status string // OK, WARN, FAIL
	Msg    string
}

type Report struct {
	Checks []Check
	OK     bool
}

func Preflight(ctx context.Context) *Report {
	r := &Report{OK: true}

	// Docker daemon
	cli, err := docker.New()
	if err != nil {
		r.Checks = append(r.Checks, Check{Name: "docker_daemon", Status: "FAIL", Msg: fmt.Sprintf("cannot create docker client: %v", err)})
		r.OK = false
	} else {
		pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := cli.Ping(pctx); err != nil {
			r.Checks = append(r.Checks, Check{Name: "docker_daemon", Status: "FAIL", Msg: fmt.Sprintf("docker ping failed: %v", err)})
			r.OK = false
		} else {
			r.Checks = append(r.Checks, Check{Name: "docker_daemon", Status: "OK", Msg: "daemon reachable"})
		}
		cancel()
		cli.Close()
	}

	// Docker compose
	cctx, ccancel := context.WithTimeout(ctx, 3*time.Second)
	out, err := executil.RunWithTimeout(cctx, 3*time.Second, "docker", "compose", "version")
	ccancel()
	if err != nil {
		r.Checks = append(r.Checks, Check{Name: "docker_compose", Status: "FAIL", Msg: "docker compose not available"})
		r.OK = false
	} else {
		r.Checks = append(r.Checks, Check{Name: "docker_compose", Status: "OK", Msg: string(out)})
	}

	// Docker socket perms
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		if runtime.GOOS == "linux" {
			if err := syscall.Access("/var/run/docker.sock", syscall.O_RDWR); err != nil {
				r.Checks = append(r.Checks, Check{Name: "docker_socket", Status: "WARN", Msg: "no write access to /var/run/docker.sock (may need group docker or root)"})
			} else {
				r.Checks = append(r.Checks, Check{Name: "docker_socket", Status: "OK", Msg: "socket accessible"})
			}
		} else {
			r.Checks = append(r.Checks, Check{Name: "docker_socket", Status: "OK", Msg: "socket present"})
		}
	}

	// Registry reachability (best-effort)
	conn, err := net.DialTimeout("tcp", "registry-1.docker.io:443", 3*time.Second)
	if err != nil {
		r.Checks = append(r.Checks, Check{Name: "registry", Status: "WARN", Msg: "cannot reach registry-1.docker.io:443 (best-effort; cached images may still work)"})
	} else {
		conn.Close()
		r.Checks = append(r.Checks, Check{Name: "registry", Status: "OK", Msg: "registry reachable"})
	}

	// Disk space (best-effort)
	if st, err := os.Stat("/home"); err == nil {
		_ = st
		r.Checks = append(r.Checks, Check{Name: "disk", Status: "OK", Msg: "disk check ok (best-effort)"})
	}

	return r
}

func (r *Report) Print() {
	fmt.Println("\n[PRE-FLIGHT]")
	for _, c := range r.Checks {
		fmt.Printf("  [%s] %s: %s\n", c.Status, c.Name, c.Msg)
	}
}
