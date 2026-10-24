package docker

import (
	"context"
	"time"

	"github.com/docker/docker/client"
)

type Client struct {
	cli *client.Client
}

func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error {
	if c == nil || c.cli == nil {
		return nil
	}
	return c.cli.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := c.cli.Ping(ctx)
	return err
}

func (c *Client) Info(ctx context.Context) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	info, err := c.cli.Info(ctx)
	if err != nil {
		return nil, err
	}
	// Return basic info
	res := map[string]interface{}{
		"Containers":        info.Containers,
		"ContainersRunning": info.ContainersRunning,
		"Images":            info.Images,
		"ServerVersion":     info.ServerVersion,
		"OperatingSystem":   info.OperatingSystem,
	}
	return res, nil
}
