package discovery

import (
	"context"

	"github.com/Alfredo-capitia/Orvyn/internal/docker"
)

func ManagedStack(ctx context.Context) ([]docker.Container, error) {
	cli, err := docker.New()
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	if err := cli.Ping(ctx); err != nil {
		return nil, err
	}
	conts, err := cli.ListContainers(ctx, true)
	if err != nil {
		return nil, err
	}
	res := []docker.Container{}
	for _, c := range conts {
		if v, ok := c.Labels["com.orvyn.managed"]; ok && v == "true" {
			res = append(res, c)
		}
	}
	return res, nil
}
