// Package logs streams container logs with best-effort secret redaction.
package logs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Alfredo-capitia/Orvyn/internal/docker"
	"github.com/Alfredo-capitia/Orvyn/internal/redaction"
	"github.com/docker/docker/pkg/stdcopy"
)

// MaxLineBytes bounds the amount of a single log line that is buffered before
// it is emitted truncated. It also bounds memory use for hostile output.
const MaxLineBytes = 64 * 1024

const truncationMarker = " [truncated]"

// Stream resolves service to a container and writes its logs to w, one redacted
// line at a time. Docker multiplexes the stream for containers without a TTY;
// the frames are demultiplexed before any line is written.
func Stream(ctx context.Context, w io.Writer, service string, follow bool, tail string, timestamps bool) error {
	cli, err := docker.New()
	if err != nil {
		return err
	}
	defer cli.Close()

	if err := cli.Ping(ctx); err != nil {
		return err
	}

	sum, err := cli.FindContainerByNameOrService(ctx, service)
	if err != nil {
		return err
	}
	if sum == nil {
		return fmt.Errorf("container or service not found: %s", service)
	}

	tty, err := cli.ContainerTTY(ctx, sum.ID)
	if err != nil {
		return err
	}

	rc, err := cli.ContainerLogs(ctx, sum.ID, follow, tail, timestamps)
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := copyStream(w, rc, tty); err != nil {
		return err
	}
	return ctx.Err()
}

// copyStream copies src to w, demultiplexing the Docker stream unless the
// container has a TTY. It is separated from Stream so it can be tested without
// a Docker daemon.
func copyStream(w io.Writer, src io.Reader, tty bool) error {
	lw := &lineWriter{w: w, max: MaxLineBytes}
	var err error
	if tty {
		_, err = io.Copy(lw, src)
	} else {
		_, err = stdcopy.StdCopy(lw, lw, src)
	}
	if ferr := lw.Flush(); err == nil {
		err = ferr
	}
	return err
}

// lineWriter splits an arbitrary byte stream into lines, redacts each line and
// writes it to w. Lines longer than max are truncated instead of failing the
// whole stream, which keeps memory bounded for hostile output.
type lineWriter struct {
	w        io.Writer
	buf      []byte
	max      int
	dropping bool
}

func (l *lineWriter) Write(p []byte) (int, error) {
	total := len(p)
	for len(p) > 0 {
		if l.dropping {
			i := bytes.IndexByte(p, '\n')
			if i < 0 {
				return total, nil
			}
			l.dropping = false
			p = p[i+1:]
			continue
		}

		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			space := l.max - len(l.buf)
			if len(p) <= space {
				l.buf = append(l.buf, p...)
				return total, nil
			}
			l.buf = append(l.buf, p[:space]...)
			if err := l.emit(true); err != nil {
				return total, err
			}
			l.dropping = true
			return total, nil
		}

		if len(l.buf)+i+1 > l.max {
			space := l.max - len(l.buf)
			if space > 0 {
				l.buf = append(l.buf, p[:space]...)
			}
			if err := l.emit(true); err != nil {
				return total, err
			}
			// The newline in p terminates the over-long line, so the next
			// byte already belongs to a fresh line.
			p = p[i+1:]
			continue
		}

		l.buf = append(l.buf, p[:i+1]...)
		if err := l.emit(false); err != nil {
			return total, err
		}
		p = p[i+1:]
	}
	return total, nil
}

func (l *lineWriter) emit(truncated bool) error {
	line := redaction.Redact(string(l.buf))
	if truncated {
		line = strings.TrimRight(line, "\n") + truncationMarker + "\n"
	}
	l.buf = l.buf[:0]
	_, err := io.WriteString(l.w, line)
	return err
}

// Flush writes any buffered partial line.
func (l *lineWriter) Flush() error {
	if len(l.buf) == 0 {
		return nil
	}
	if !l.dropping {
		l.buf = append(l.buf, '\n')
	}
	return l.emit(l.dropping)
}
