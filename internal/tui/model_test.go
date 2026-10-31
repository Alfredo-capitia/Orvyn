package tui

import (
	"strings"
	"testing"

	"github.com/Alfredo-capitia/Orvyn/internal/docker"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel(t *testing.T) model {
	t.Helper()
	m := initialModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", updated)
	}
	return got
}

func pressKey(m model, key string) model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated.(model)
}

// TestNumericKeysSwitchViews guards the tab bar, which advertises [1]..[4].
// Without these bindings pressing the advertised number did nothing.
func TestNumericKeysSwitchViews(t *testing.T) {
	tests := []struct {
		key  string
		want viewState
	}{
		{"2", viewLogs},
		{"3", viewMetrics},
		{"4", viewDetails},
		{"1", viewOverview},
	}

	m := newTestModel(t)
	for _, tt := range tests {
		m = pressKey(m, tt.key)
		if m.view != tt.want {
			t.Errorf("key %q switched to view %d, want %d", tt.key, m.view, tt.want)
		}
	}
}

// TestMetricsBusyGuard ensures a slow stats collection cannot be started twice
// by overlapping 3s ticks.
func TestMetricsBusyGuard(t *testing.T) {
	m := newTestModel(t)

	if cmd := m.startMetrics(); cmd == nil {
		t.Fatal("first refresh should start a collection")
	}
	if cmd := m.startMetrics(); cmd != nil {
		t.Fatal("refresh started while another collection was in flight")
	}

	m.metricsBusy = false
	if cmd := m.startMetrics(); cmd == nil {
		t.Fatal("refresh did not start after the previous collection finished")
	}
}

// TestViewRendersEveryView catches empty or panicking screens.
func TestViewRendersEveryView(t *testing.T) {
	m := newTestModel(t)

	for _, view := range []viewState{viewOverview, viewLogs, viewMetrics, viewDetails, viewHelp} {
		m.view = view
		if out := m.View(); strings.TrimSpace(out) == "" {
			t.Errorf("view %d rendered an empty screen", view)
		}
	}

	m.view = viewMetrics
	out := m.View()
	for _, want := range []string{"HOST", "CONTAINERS (managed)", "Load:"} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics view is missing %q", want)
		}
	}
}

// TestContainerNameFallsBackToID avoids an index panic when Docker reports no
// name for a container.
func TestContainerNameFallsBackToID(t *testing.T) {
	if got := containerName(docker.Container{ID: "abc123"}); got != "abc123" {
		t.Errorf("containerName without names = %q, want the ID", got)
	}
	if got := containerName(docker.Container{ID: "abc123", Names: []string{"/orvyn-loki"}}); got != "orvyn-loki" {
		t.Errorf("containerName did not strip the leading slash: %q", got)
	}
}

// TestMetricsMessageClearsBusyWhileFiltering ensures an in-flight metrics
// collection cannot leave metricsBusy set forever when the user opens the
// filter box before the result arrives.
func TestMetricsMessageClearsBusyWhileFiltering(t *testing.T) {
	m := newTestModel(t)
	m.filtering = true
	m.filterInput.Focus()
	m.metricsBusy = true

	updated, _ := m.Update(metricsMsg{
		cont: []ContMetrics{{Name: "orvyn-loki", CPU: "1.0%", Mem: "1.0MB"}},
	})
	got := updated.(model)

	if got.metricsBusy {
		t.Error("metricsBusy was not cleared by a metricsMsg during filtering")
	}
	if !got.filtering {
		t.Error("filtering state was dropped by an unrelated message")
	}
	if len(got.contm) != 1 || got.contm[0].Name != "orvyn-loki" {
		t.Errorf("metrics payload was not applied: %+v", got.contm)
	}
}

// TestViewShowsErrorScreen keeps the fatal-error path reachable.
func TestViewShowsErrorScreen(t *testing.T) {
	m := newTestModel(t)
	m.err = errMsgForTest{}
	if out := m.View(); !strings.Contains(out, "Error:") {
		t.Errorf("error screen missing, got %q", out)
	}
}

type errMsgForTest struct{}

func (errMsgForTest) Error() string { return "boom" }

// TestHealthMsgUpdatesCacheOnUIThread ensures inspection results arrive as a
// message and are applied by Update, instead of a goroutine mutating the shared
// map while rows are being rendered (a data race under -race).
func TestHealthMsgUpdatesCacheOnUIThread(t *testing.T) {
	m := newTestModel(t)
	m.containers = []docker.Container{{ID: "aaa"}, {ID: "bbb"}}
	m.healthCache["stale"] = "healthy"

	updated, _ := m.Update(healthMsg{"aaa": "healthy", "bbb": "unhealthy", "zzz": "healthy"})
	got := updated.(model)

	if got.healthCache["aaa"] != "healthy" {
		t.Errorf("healthCache[aaa] = %q, want healthy", got.healthCache["aaa"])
	}
	if got.healthCache["bbb"] != "unhealthy" {
		t.Errorf("healthCache[bbb] = %q, want unhealthy", got.healthCache["bbb"])
	}
	if _, ok := got.healthCache["zzz"]; ok {
		t.Error("cache stored health for a container that does not exist")
	}
	if _, ok := got.healthCache["stale"]; ok {
		t.Error("cache kept an entry for a container that no longer exists")
	}
}

// TestRefreshPrunesHealthCache keeps the cache bounded by the live container
// list, even when the refresh reports no health at all.
func TestRefreshPrunesHealthCache(t *testing.T) {
	m := newTestModel(t)
	m.healthCache["gone"] = "healthy"
	m.healthCache["live"] = "healthy"

	updated, _ := m.Update(refreshMsg([]docker.Container{{ID: "live"}}))
	got := updated.(model)

	if _, ok := got.healthCache["gone"]; ok {
		t.Error("refresh kept health for a removed container")
	}
	if got.healthCache["live"] != "healthy" {
		t.Error("refresh dropped health for a live container")
	}
}
