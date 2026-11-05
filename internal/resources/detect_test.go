package resources

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// useMeminfo points Detect at a fixture so every tier can be exercised without
// depending on the memory of the host running the tests.
func useMeminfo(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "meminfo")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write meminfo: %v", err)
	}
	old := meminfoPath
	meminfoPath = path
	t.Cleanup(func() { meminfoPath = old })
}

// meminfoFor renders a /proc/meminfo body with MemTotal in kB.
func meminfoFor(totalMB int) string {
	return fmt.Sprintf("MemTotal:       %d kB\nMemFree:        1024 kB\n", totalMB*1024)
}

func TestDetectTiers(t *testing.T) {
	fifth := Limits{"768m", "640m", "320m", "128m", "320m", "2.5", 768}
	fourth := Limits{"640m", "512m", "256m", "128m", "256m", "2.0", 512}
	third := Limits{"512m", "384m", "192m", "96m", "192m", "2.0", 512}
	second := Limits{"384m", "256m", "160m", "80m", "160m", "1.5", 384}
	first := Limits{"256m", "192m", "128m", "64m", "128m", "1.0", 256}

	tests := []struct {
		name    string
		totalMB int
		want    Limits
	}{
		{"just below the first boundary", 2047, first},
		{"first boundary is inclusive", 2048, second},
		{"just below the second boundary", 4095, second},
		{"second boundary is inclusive", 4096, third},
		{"just below the third boundary", 6143, third},
		{"third boundary is inclusive", 6144, fourth},
		{"just below the fourth boundary", 8191, fourth},
		{"fourth boundary is inclusive", 8192, fifth},
		{"large host", 65536, fifth},
		{"tiny host", 256, first},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			useMeminfo(t, meminfoFor(tc.totalMB))
			if got := Detect(); got != tc.want {
				t.Errorf("Detect() with %d MB = %+v, want %+v", tc.totalMB, got, tc.want)
			}
		})
	}
}

func TestDetectFallsBackToTheDefault(t *testing.T) {
	defaults := Limits{"512m", "384m", "192m", "96m", "192m", "2.0", 512}

	tests := []struct {
		name string
		body string
	}{
		{"MemTotal is missing", "MemFree: 1024 kB\n"},
		{"MemTotal has no value", "MemTotal:\n"},
		{"MemTotal is not a number", "MemTotal: lots kB\n"},
		{"file is empty", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			useMeminfo(t, tc.body)
			if got := Detect(); got != defaults {
				t.Errorf("Detect() = %+v, want the 4 GB defaults %+v", got, defaults)
			}
		})
	}
}

func TestDetectWithoutMeminfo(t *testing.T) {
	old := meminfoPath
	meminfoPath = filepath.Join(t.TempDir(), "does-not-exist")
	t.Cleanup(func() { meminfoPath = old })

	defaults := Limits{"512m", "384m", "192m", "96m", "192m", "2.0", 512}
	if got := Detect(); got != defaults {
		t.Errorf("Detect() = %+v, want the 4 GB defaults %+v", got, defaults)
	}
}

func TestDetectReadsTheFirstMemTotalLine(t *testing.T) {
	useMeminfo(t, "MemTotal:       1048576 kB\nMemTotal:       8388608 kB\n")

	// 1048576 kB is exactly 1024 MB, so the first line selects the smallest
	// tier; a parser that kept scanning would land on the largest one.
	want := Limits{"256m", "192m", "128m", "64m", "128m", "1.0", 256}
	if got := Detect(); got != want {
		t.Errorf("Detect() = %+v, want %+v", got, want)
	}
}
