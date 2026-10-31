package docker

import (
	"context"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
)

type Container struct {
	ID     string
	Names  []string
	Image  string
	State  string
	Status string
	Labels map[string]string
}

func (c *Client) ListContainers(ctx context.Context, all bool) ([]Container, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: all})
	if err != nil {
		return nil, err
	}
	res := make([]Container, 0, len(list))
	for _, cnt := range list {
		id := cnt.ID
		if len(id) > 12 {
			id = id[:12]
		}
		res = append(res, Container{
			ID:     id,
			Names:  cnt.Names,
			Image:  cnt.Image,
			State:  cnt.State,
			Status: cnt.Status,
			Labels: cnt.Labels,
		})
	}
	return res, nil
}

func (c *Client) ContainerLogs(ctx context.Context, id string, follow bool, tail string, timestamps bool) (io.ReadCloser, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: timestamps,
	}
	if tail != "" {
		opts.Tail = tail
	} else if !follow {
		opts.Tail = "100"
	}
	return c.cli.ContainerLogs(ctx, id, opts)
}

// FindContainerByNameOrService resolves name against Orvyn-managed containers
// only. Unmanaged containers are never returned: callers use this for
// privileged operations such as exec and logs.
func (c *Client) FindContainerByNameOrService(ctx context.Context, name string) (*container.Summary, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	return findManagedContainer(list, name), nil
}

// findManagedContainer is the pure matching logic behind
// FindContainerByNameOrService. It ignores containers that do not carry the
// com.orvyn.managed=true label.
func findManagedContainer(list []container.Summary, name string) *container.Summary {
	for i := range list {
		cnt := &list[i]
		if cnt.Labels["com.orvyn.managed"] != "true" {
			continue
		}
		for _, n := range cnt.Names {
			if n == "/"+name || n == name {
				return cnt
			}
		}
		if svc, ok := cnt.Labels["com.docker.compose.service"]; ok && svc == name {
			return cnt
		}
		if len(cnt.ID) >= 12 && cnt.ID[:12] == name {
			return cnt
		}
	}
	return nil
}

func (c *Client) InspectContainer(ctx context.Context, id string) (container.InspectResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.cli.ContainerInspect(ctx, id)
}

// ContainerTTY reports whether the container was created with a TTY. Docker
// multiplexes the log stream when it is false, so callers must demultiplex.
func (c *Client) ContainerTTY(ctx context.Context, id string) (bool, error) {
	ins, err := c.InspectContainer(ctx, id)
	if err != nil {
		return false, err
	}
	if ins.Config == nil {
		return false, nil
	}
	return ins.Config.Tty, nil
}

func (c *Client) ContainerStats(ctx context.Context, id string, stream bool) (io.ReadCloser, error) {
	resp, err := c.cli.ContainerStats(ctx, id, stream)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
