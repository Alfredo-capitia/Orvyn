package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	for _, c := range conts {
		if v, ok := c.Labels["com.orvyn.managed"]; !ok || v != "true" {
			continue
		}
		name := c.Names[0]
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		res = append(res, ContMetrics{ID: c.ID, Name: name, CPU: "-", Mem: "-"})
	}
	for i := range res {
		statsCtx, scancel := context.WithTimeout(context.Background(), 6*time.Second)
		rc, err := dc.ContainerStats(statsCtx, res[i].ID, false)
		if err != nil {
			scancel()
			continue
		}
		var stats container.StatsResponse
		if err := json.NewDecoder(rc).Decode(&stats); err != nil {
			rc.Close()
			scancel()
			continue
		}
		rc.Close()
		scancel()
		cpuDelta := stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage
		sysDelta := stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage
		cpuPct := 0.0
		cpuCount := uint64(len(stats.CPUStats.CPUUsage.PercpuUsage))
		if stats.CPUStats.OnlineCPUs > 0 {
			cpuCount = stats.CPUStats.OnlineCPUs
		}
		if sysDelta > 0 && cpuDelta > 0 && cpuCount > 0 {
			cpuPct = float64(cpuDelta) / float64(sysDelta) * float64(cpuCount) * 100.0
		}
		memUsage := stats.MemoryStats.Usage
		if stats.MemoryStats.Stats != nil {
			if v, ok := stats.MemoryStats.Stats["total_inactive_file"]; ok && v > 0 && v < memUsage {
				memUsage -= v
			} else if v, ok := stats.MemoryStats.Stats["inactive_file"]; ok && v > 0 && v < memUsage {
				memUsage -= v
			}
		}
		memLimit := stats.MemoryStats.Limit
		memStr := fmt.Sprintf("%.1fMB", float64(memUsage)/1024/1024)
		if memLimit > 0 {
			memStr = fmt.Sprintf("%.1fMB / %.1fMB", float64(memUsage)/1024/1024, float64(memLimit)/1024/1024)
		}
		res[i].CPU = fmt.Sprintf("%.1f%%", cpuPct)
		res[i].Mem = memStr
	}
	return res
}
