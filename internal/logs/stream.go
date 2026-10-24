package logs

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/fredy-arias/vpsmon/internal/docker"
	"github.com/fredy-arias/vpsmon/internal/redaction"
)

func Stream(ctx context.Context, service string, follow bool, tail string, timestamps bool) error {
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

	rc, err := cli.ContainerLogs(ctx, sum.ID, follow, tail, timestamps)
	if err != nil {
		return err
	}
	defer rc.Close()

	sc := bufio.NewScanner(rc)
	const maxTokenSize = 1024 * 64
	sc.Buffer(make([]byte, maxTokenSize), maxTokenSize)
	sc.Split(bufio.ScanLines)

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		line := redaction.Redact(sc.Text())
		fmt.Fprintln(os.Stdout, line)
	}

	return sc.Err()
}
