package tui

import (
	"strings"
	"testing"

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
