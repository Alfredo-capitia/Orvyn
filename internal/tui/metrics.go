package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/fredy-arias/orvyn/internal/docker"
)

type HostMetrics struct {
	CPULoad1  float64
	MemTotal  uint64
	MemUsed   uint64
	MemFree   uint64
	DiskTotal uint64
	DiskUsed  uint64
	DiskFree  uint64
	Uptime    string
}

type ContMetrics struct {
	ID   string
	Name string
	CPU  string
	Mem  string
}

func getHostMetrics() HostMetrics {
	var hm HostMetrics
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(b))
		if len(parts) > 0 {
			if v, err := strconv.ParseFloat(parts[0], 64); err == nil {
				hm.CPULoad1 = v
			}
		}
	}
	if f, err := os.Open("/proc/meminfo"); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			val, _ := strconv.ParseUint(fields[1], 10, 64)
			val = val * 1024
			if strings.HasPrefix(fields[0], "MemTotal:") {
				hm.MemTotal = val
			} else if strings.HasPrefix(fields[0], "MemAvailable:") {
				hm.MemFree = val
			}
		}
		f.Close()
	}
	if hm.MemTotal > 0 && hm.MemFree > 0 {
		hm.MemUsed = hm.MemTotal - hm.MemFree
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err == nil {
		hm.DiskTotal = st.Blocks * uint64(st.Bsize)
		hm.DiskFree = st.Bavail * uint64(st.Bsize)
		hm.DiskUsed = hm.DiskTotal - (st.Bfree * uint64(st.Bsize))
	}
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		parts := strings.Fields(string(b))
		if len(parts) > 0 {
			if s, err := strconv.ParseFloat(parts[0], 64); err == nil {
				d := time.Duration(s) * time.Second
				days := int(d.Hours()) / 24
				hours := int(d.Hours()) % 24
				mins := int(d.Minutes()) % 60
				if days > 0 {
					hm.Uptime = fmt.Sprintf("%dd%dh%dm", days, hours, mins)
				} else {
					hm.Uptime = fmt.Sprintf("%dh%dm", hours, mins)
				}
			}
		}
	}
	return hm
}

func getContainerMetrics(dc *docker.Client) []ContMetrics {
	res := []ContMetrics{}
	if dc == nil {
		return res
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	conts, err := dc.ListContainers(ctx, true)
	if err != nil {
		return res
	}
	running := make([]bool, 0, len(conts))
	for _, c := range conts {
		if v, ok := c.Labels["com.orvyn.managed"]; !ok || v != "true" {
			continue
		}
		name := c.ID
		if len(c.Names) > 0 {
			name = c.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
		}
		// Stopped containers keep their row with placeholder metrics: Docker
		// returns an empty stats record for them, which must not render as 0%.
		res = append(res, ContMetrics{ID: c.ID, Name: name, CPU: "-", Mem: "-"})
		running = append(running, c.State == "running")
	}

	// A non-streaming stats call blocks for two samples, so requests run
	// concurrently under one shared deadline instead of serially.
	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer refreshCancel()

	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i := range res {
		if !running[i] {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			statsCtx, scancel := context.WithTimeout(refreshCtx, 6*time.Second)
			defer scancel()

			rc, err := dc.ContainerStats(statsCtx, res[i].ID, false)
			if err != nil {
				return
			}
			defer rc.Close()

			var stats container.StatsResponse
			if err := json.NewDecoder(rc).Decode(&stats); err != nil {
				return
			}
			res[i].CPU, res[i].Mem = formatContainerStats(&stats)
		}(i)
	}
	wg.Wait()
	return res
}

func formatContainerStats(stats *container.StatsResponse) (cpu, mem string) {
	cpuDelta := stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage
	sysDelta := stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage
	// cgroup v2 hosts leave PercpuUsage empty, so OnlineCPUs is the reliable
	// multiplier and the per-CPU slice length is only a fallback.
	cpuCount := stats.CPUStats.OnlineCPUs
	if cpuCount == 0 {
		cpuCount = uint32(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	cpuPct := 0.0
	if sysDelta > 0 && cpuDelta > 0 && cpuCount > 0 {
		cpuPct = float64(cpuDelta) / float64(sysDelta) * float64(cpuCount) * 100.0
	}

	// MemoryStats.Usage includes page cache; subtract the reclaimable portion
	// so the value matches what `docker stats` reports.
	memUsage := stats.MemoryStats.Usage
	if v := stats.MemoryStats.Stats["total_inactive_file"]; v > 0 && v < memUsage {
		memUsage -= v
	} else if v := stats.MemoryStats.Stats["inactive_file"]; v > 0 && v < memUsage {
		memUsage -= v
	}
	memStr := fmt.Sprintf("%.1fMB", float64(memUsage)/1024/1024)
	if stats.MemoryStats.Limit > 0 {
		memStr = fmt.Sprintf("%.1fMB / %.1fMB", float64(memUsage)/1024/1024, float64(stats.MemoryStats.Limit)/1024/1024)
	}
	return fmt.Sprintf("%.1f%%", cpuPct), memStr
}
