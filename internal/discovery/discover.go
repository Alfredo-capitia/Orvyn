package discovery

import (
	"bufio"
	"context"
	"os"
	"strings"

	"github.com/fredy-arias/vpsmon/internal/docker"
)

type Result struct {
	OS              string
	DockerAvailable bool
	DockerInfo      map[string]interface{}
	Containers      []docker.Container
	ExistingStack   []string
}

func OSRelease() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "unknown"
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var name, ver string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
		if strings.HasPrefix(line, "NAME=") {
			name = strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
		}
		if strings.HasPrefix(line, "VERSION=") {
			ver = strings.Trim(strings.TrimPrefix(line, "VERSION="), "\"")
		}
	}
	if name != "" && ver != "" {
		return name + " " + ver
	}
	if name != "" {
		return name
	}
	return "unknown"
}

func Run(ctx context.Context) (*Result, error) {
	res := &Result{
		OS: OSRelease(),
	}
	cli, err := docker.New()
	if err != nil {
		return res, nil
	}
	defer cli.Close()
	if err := cli.Ping(ctx); err != nil {
		res.DockerAvailable = false
		return res, nil
	}
	res.DockerAvailable = true
	if info, err := cli.Info(ctx); err == nil {
		res.DockerInfo = info
	}
	conts, err := cli.ListContainers(ctx, true)
	if err == nil {
		res.Containers = conts
		for _, c := range conts {
			img := strings.ToLower(c.Image)
			for _, k := range []string{"prometheus", "grafana", "loki", "alloy", "promtail", "node-exporter", "cadvisor"} {
				if strings.Contains(img, k) {
					res.ExistingStack = append(res.ExistingStack, k+" ("+c.Names[0]+")")
					break
				}
			}
		}
	}
	return res, nil
}
