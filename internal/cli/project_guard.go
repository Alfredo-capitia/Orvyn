package cli

import (
	"context"
	"fmt"

	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/Alfredo-capitia/Orvyn/internal/docker"
	"github.com/Alfredo-capitia/Orvyn/internal/stack"
)

// verifyProjectOwnership refuses to run a destructive Compose command when the
// target project contains containers that Orvyn did not create. Compose derives
// the project boundary from labels, so a foreign container inside the project
// would be stopped or removed along with the stack.
func verifyProjectOwnership(ctx context.Context, cfg *config.Config) error {
	project := stack.ProjectName(cfg)

	cli, err := docker.New()
	if err != nil {
		return fmt.Errorf("verify compose project %q: %w", project, err)
	}
	defer cli.Close()

	if err := cli.Ping(ctx); err != nil {
		return fmt.Errorf("verify compose project %q: docker is not reachable: %w", project, err)
	}

	containers, err := cli.ListContainers(ctx, true)
	if err != nil {
		return fmt.Errorf("verify compose project %q: list containers: %w", project, err)
	}

	foreign := foreignProjectContainers(containers, project)
	if len(foreign) == 0 {
		return nil
	}

	names := make([]string, 0, len(foreign))
	for _, c := range foreign {
		name := c.ID
		if len(c.Names) > 0 && c.Names[0] != "" {
			name = c.Names[0]
		}
		names = append(names, name)
	}
	return fmt.Errorf("refusing to modify compose project %q: %d container(s) are not Orvyn-managed: %v", project, len(foreign), names)
}

// foreignProjectContainers returns the containers that Compose places in
// project but that do not carry the Orvyn managed label.
func foreignProjectContainers(containers []docker.Container, project string) []docker.Container {
	var foreign []docker.Container
	for _, c := range containers {
		if c.Labels["com.docker.compose.project"] != project {
			continue
		}
		if c.Labels["com.orvyn.managed"] == "true" {
			continue
		}
		foreign = append(foreign, c)
	}
	return foreign
}
