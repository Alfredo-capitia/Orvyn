package resources

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type Limits struct {
	Prometheus string
	Loki       string
	Alloy      string
	NodeExp    string
	CAdvisor   string
	CPULimit   string
	PIDsLimit  int
}

func Detect() Limits {
	totalMB := 4096
	if f, err := os.Open("/proc/meminfo"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if kb, err := strconv.Atoi(fields[1]); err == nil {
						totalMB = kb / 1024
					}
				}
				break
			}
		}
	}

	cores := runtime.NumCPU()
	_ = cores

	if totalMB < 2048 {
		return Limits{
			Prometheus: "256m",
			Loki:       "192m",
			Alloy:      "128m",
			NodeExp:    "64m",
			CAdvisor:   "128m",
			CPULimit:   "1.0",
			PIDsLimit:  256,
		}
	}
	if totalMB < 4096 {
		return Limits{
			Prometheus: "384m",
			Loki:       "256m",
			Alloy:      "160m",
			NodeExp:    "80m",
			CAdvisor:   "160m",
			CPULimit:   "1.5",
			PIDsLimit:  384,
		}
	}
	if totalMB < 6144 {
		return Limits{
			Prometheus: "512m",
			Loki:       "384m",
			Alloy:      "192m",
			NodeExp:    "96m",
			CAdvisor:   "192m",
			CPULimit:   "2.0",
			PIDsLimit:  512,
		}
	}
	if totalMB < 8192 {
		return Limits{
			Prometheus: "640m",
			Loki:       "512m",
			Alloy:      "256m",
			NodeExp:    "128m",
			CAdvisor:   "256m",
			CPULimit:   "2.0",
			PIDsLimit:  512,
		}
	}
	return Limits{
		Prometheus: "768m",
		Loki:       "640m",
		Alloy:      "320m",
		NodeExp:    "128m",
		CAdvisor:   "320m",
		CPULimit:   "2.5",
		PIDsLimit:  768,
	}
}
