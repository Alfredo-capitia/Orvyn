package tui

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Alfredo-capitia/Orvyn/internal/docker"
	tea "github.com/charmbracelet/bubbletea"
)

// tuiDaemon starts a fake Docker API and points the client at it. The handlers
// are per-container so a test can mix healthy, unhealthy and missing
// containers, and choose whether the log stream is multiplexed.
type tuiDaemon struct {
	health     map[string]string
	logs       map[string][]byte
	tty        map[string]bool
	inspectErr map[string]bool
}

func (d tuiDaemon) start(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/_ping") {
			w.Header().Set("API-Version", "1.44")
			_, _ = io.WriteString(w, "OK")
			return
		}

		rest := r.URL.Path
		if i := strings.Index(rest, "/containers/"); i >= 0 {
			rest = rest[i+len("/containers/"):]
		}
		id, action, _ := strings.Cut(rest, "/")

		switch {
		case action == "json":
			if d.inspectErr[id] {
				http.Error(w, `{"message":"no such container"}`, http.StatusNotFound)
				return
			}
			body := map[string]any{
				"Id":     id,
				"Config": map[string]any{"Tty": d.tty[id]},
			}
			if status, ok := d.health[id]; ok {
				body["State"] = map[string]any{"Health": map[string]any{"Status": status}}
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(body); err != nil {
				t.Errorf("encode inspect: %v", err)
			}
		case strings.HasPrefix(action, "logs"):
			w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
			_, _ = w.Write(d.logs[id])
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+srv.Listener.Addr().String())
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
	t.Setenv("DOCKER_API_VERSION", "")
}

// frame wraps a log line in Docker's 8-byte stream header (stdout).
func frame(line string) []byte {
	payload := []byte(line + "\n")
	header := make([]byte, 8)
	header[0] = 1
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
	return append(header, payload...)
}

func containersFixture() []docker.Container {
	return []docker.Container{
		{
			ID:     "aaa",
			Names:  []string{"/orvyn-loki"},
			Image:  "grafana/loki:3.3.2",
			State:  "running",
			Status: "Up 2 minutes",
			Labels: map[string]string{"com.orvyn.service": "loki", "com.orvyn.managed": "true"},
		},
		{
			ID:     "bbb",
			Names:  []string{"/orvyn-alloy"},
			Image:  "grafana/alloy:v1.21.3",
			State:  "running",
			Status: "Up 1 minute",
			Labels: map[string]string{"com.orvyn.service": "alloy", "com.orvyn.managed": "true"},
		},
		{
			ID:     "ccc",
			Names:  []string{"/orvyn-prometheus"},
			Image:  "prom/prometheus:v2.55.1",
			State:  "exited",
			Status: "Exited (0) 1 minute ago",
			Labels: map[string]string{"com.orvyn.managed": "true"},
		},
	}
}

func applyMessage(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	updated, _ := m.Update(msg)
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", updated)
	}
	return got
}

func TestApplyFilterMatchesNameAndServiceCaseInsensitively(t *testing.T) {
	m := newTestModel(t)
	m.containers = containersFixture()
	m.filterActive = true

	tests := []struct {
		query   string
		wantIDs []string
	}{
		{"loki", []string{"aaa"}},
		{"LOKI", []string{"aaa"}},
		{"alloy", []string{"bbb"}},
		{"orvyn-", []string{"aaa", "bbb", "ccc"}},
		{"missing", nil},
		{"", []string{"aaa", "bbb", "ccc"}},
	}

	for _, tt := range tests {
		m.filterQuery = tt.query
		m = applyMessage(t, m, healthMsg{})

		got := m.getList()
		ids := make([]string, 0, len(got))
		for _, ct := range got {
			ids = append(ids, ct.ID)
		}
		if strings.Join(ids, ",") != strings.Join(tt.wantIDs, ",") {
			t.Errorf("filter %q matched %v, want %v", tt.query, ids, tt.wantIDs)
		}
		if got := len(m.table.Rows()); got != len(tt.wantIDs) {
			t.Errorf("filter %q rendered %d rows, want %d", tt.query, got, len(tt.wantIDs))
		}
	}
}

func TestGetListDoesNotFallBackToAllWhenFilterMatchesNothing(t *testing.T) {
	m := newTestModel(t)
	m.containers = containersFixture()
	m.filterActive = true
	m.filterQuery = "missing"
	m = applyMessage(t, m, healthMsg{})

	if got := m.getList(); len(got) != 0 {
		t.Fatalf("getList returned %d containers for a filter with no matches", len(got))
	}

	m.filterActive = false
	if got := len(m.getList()); got != len(m.containers) {
		t.Fatalf("getList returned %d containers with filtering off, want all %d", got, len(m.containers))
	}
}

func TestApplyFilterShowsHealthFromCacheAndState(t *testing.T) {
	m := newTestModel(t)
	m.containers = containersFixture()
	m.healthCache["aaa"] = "healthy"

	m = applyMessage(t, m, healthMsg{})

	rows := m.table.Rows()
	want := map[string]string{"orvyn-loki": "healthy", "orvyn-alloy": "unknown", "orvyn-prometheus": "-"}
	for _, row := range rows {
		if got := row[4]; got != want[row[0]] {
			t.Errorf("health for %s = %q, want %q", row[0], got, want[row[0]])
		}
	}
}

func TestBuildDetailsUsesFilteredSelectionAndFallsBack(t *testing.T) {
	m := newTestModel(t)
	m.containers = containersFixture()
	m.filterActive = true
	m.filterQuery = "alloy"
	m = applyMessage(t, m, healthMsg{})

	m.view = viewDetails
	m = applyMessage(t, m, healthMsg{})
	if len(m.details) == 0 {
		t.Fatal("details empty for a filtered container")
	}
	if m.details[0][1] != "orvyn-alloy" {
		t.Errorf("details are for %q, want orvyn-alloy", m.details[0][1])
	}

	// A cursor beyond the filtered list must not panic or report the wrong
	// container; it falls back to the first entry.
	m.table.SetCursor(99)
	m = applyMessage(t, m, healthMsg{})
	if m.details[0][1] != "orvyn-alloy" {
		t.Errorf("out-of-range cursor selected %q", m.details[0][1])
	}

	m.containers = nil
	m.filterActive = false
	m.view = viewDetails
	m = applyMessage(t, m, refreshMsg(nil))
	if len(m.details) != 0 {
		t.Errorf("details = %v, want empty without containers", m.details)
	}
}

func TestWindowSizeMakesTheModelReady(t *testing.T) {
	m := initialModel()
	if m.ready {
		t.Fatal("model started ready before a window size arrived")
	}

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	got := updated.(model)
	if !got.ready {
		t.Error("WindowSizeMsg did not mark the model ready")
	}
	if got.width != 100 || got.height != 30 {
		t.Errorf("size = %dx%d, want 100x30", got.width, got.height)
	}
	if got.viewport.Width != 96 || got.viewport.Height != 18 {
		t.Errorf("viewport = %dx%d, want 96x18", got.viewport.Width, got.viewport.Height)
	}
}

func TestKeyBindings(t *testing.T) {
	tests := []struct {
		name  string
		start viewState
		key   string
		want  viewState
	}{
		{"tab overview to logs", viewOverview, "tab", viewLogs},
		{"tab logs to metrics", viewLogs, "tab", viewMetrics},
		{"tab metrics to details", viewMetrics, "tab", viewDetails},
		{"tab details to overview", viewDetails, "tab", viewOverview},
		{"tab help to overview", viewHelp, "tab", viewOverview},
		{"l opens logs", viewOverview, "l", viewLogs},
		{"m opens metrics", viewOverview, "m", viewMetrics},
		{"d opens details", viewOverview, "d", viewDetails},
		{"d closes details", viewDetails, "d", viewOverview},
		{"help toggles on", viewOverview, "?", viewHelp},
		{"help toggles off", viewHelp, "?", viewOverview},
		{"one returns to overview", viewLogs, "1", viewOverview},
		{"o returns to overview", viewDetails, "o", viewOverview},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.view = tt.start
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
			if got := updated.(model).view; got != tt.want {
				t.Errorf("key %q in view %d -> %d, want %d", tt.key, tt.start, got, tt.want)
			}
		})
	}
}

func TestQuitKeys(t *testing.T) {
	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("q")},
		{Type: tea.KeyCtrlC},
	} {
		m := newTestModel(t)
		_, cmd := m.Update(msg)
		if cmd == nil {
			t.Fatalf("%v returned no command", msg)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%v did not quit", msg)
		}
	}
}

func TestFilterKeyBindings(t *testing.T) {
	m := newTestModel(t)
	m.containers = containersFixture()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = updated.(model)
	if !m.filtering || !m.filterInput.Focused() {
		t.Fatal("/ did not open the filter input")
	}
	if cmd == nil {
		t.Error("/ returned no blink command")
	}

	// Typing only updates the input until Enter is pressed.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("loki")})
	m = updated.(model)
	if !m.filtering {
		t.Error("typing closed the filter input")
	}
	if m.filterActive {
		t.Error("typing activated the filter before Enter")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.filtering || !m.filterActive || m.filterQuery != "loki" {
		t.Fatalf("enter left filtering=%v active=%v query=%q", m.filtering, m.filterActive, m.filterQuery)
	}
	if got := len(m.getList()); got != 1 {
		t.Errorf("filter matched %d containers, want 1 (containers=%d query=%q", got, len(m.containers), m.filterQuery)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if m.filterActive || m.filterQuery != "" {
		t.Errorf("esc left active=%v query=%q", m.filterActive, m.filterQuery)
	}

	// Esc while typing must also discard the query.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("al")})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if m.filtering || m.filterActive || m.filterQuery != "" || m.filterInput.Value() != "" {
		t.Errorf("esc while typing left filtering=%v active=%v query=%q input=%q", m.filtering, m.filterActive, m.filterQuery, m.filterInput.Value())
	}
}

func TestFilterKeyIsOnlyHandledInOverview(t *testing.T) {
	m := newTestModel(t)
	m.view = viewLogs

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if got := updated.(model); got.filtering {
		t.Error("/ opened the filter outside the overview")
	}
}

func TestTickSchedulesRefreshPerView(t *testing.T) {
	tests := []struct {
		view viewState
		name string
	}{
		{viewOverview, "overview refreshes and keeps ticking"},
		{viewMetrics, "metrics collects and keeps ticking"},
		{viewLogs, "logs only keeps ticking"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.view = tt.view
			_, cmd := m.Update(tickMsg{})
			if cmd == nil {
				t.Fatal("tick returned no command")
			}
		})
	}
}

func TestRefreshMsgReappliesFilterAndRebuildsDetails(t *testing.T) {
	m := newTestModel(t)
	m.containers = containersFixture()
	m.filterActive = true
	m.filterQuery = "loki"
	m.view = viewDetails
	m.applyFilter()
	m.healthCache["gone"] = "healthy"

	updated, cmd := m.Update(refreshMsg([]docker.Container{
		{ID: "aaa", Names: []string{"/orvyn-loki"}, State: "running", Labels: map[string]string{"com.orvyn.service": "loki"}},
	}))
	got := updated.(model)

	if cmd == nil {
		t.Error("refresh did not schedule a health inspection")
	}
	if _, ok := got.healthCache["gone"]; ok {
		t.Error("refresh kept health for a container that no longer exists")
	}
	if gotLen := len(got.getList()); gotLen != 1 {
		t.Errorf("filter was not reapplied: %d containers (query=%q filtered=%d)", gotLen, got.filterQuery, len(got.filtered))
	}
	if len(got.details) == 0 || got.details[0][1] != "orvyn-loki" {
		t.Errorf("details were not rebuilt: %v", got.details)
	}
}

func TestLogsMsgAndErrMsgUpdateState(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(logsMsg{"first", "second"})
	m = updated.(model)
	if strings.Join(m.logs, ",") != "first,second" {
		t.Errorf("logs = %v", m.logs)
	}
	if !strings.Contains(m.viewport.View(), "second") {
		t.Errorf("viewport does not show the logs: %q", m.viewport.View())
	}

	updated, _ = m.Update(errMsg(fmt.Errorf("daemon unreachable")))
	m = updated.(model)
	if m.err == nil || !strings.Contains(m.View(), "daemon unreachable") {
		t.Errorf("error state not rendered: %v", m.err)
	}
}

func TestFetchHealthCmdReportsOnlyInspectedHealth(t *testing.T) {
	tuiDaemon{
		health:     map[string]string{"aaa": "healthy"},
		inspectErr: map[string]bool{"bbb": true},
		tty:        map[string]bool{"aaa": true},
	}.start(t)

	cmd := fetchHealthCmd(containersFixture())
	if cmd == nil {
		t.Fatal("fetchHealthCmd returned nil")
	}
	msg, ok := cmd().(healthMsg)
	if !ok {
		t.Fatalf("fetchHealthCmd returned %T, want healthMsg", msg)
	}
	if len(msg) != 1 || msg["aaa"] != "healthy" {
		t.Errorf("health = %+v, want only aaa=healthy", msg)
	}
}

func TestRefreshCmdReturnsManagedContainers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/_ping") {
			w.Header().Set("API-Version", "1.44")
			_, _ = io.WriteString(w, "OK")
			return
		}
		if strings.HasSuffix(r.URL.Path, "/containers/json") {
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"Id": "aaa", "Names": []string{"/orvyn-loki"}, "State": "running",
				"Labels": map[string]string{"com.orvyn.managed": "true"},
			}})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+srv.Listener.Addr().String())
	t.Setenv("DOCKER_API_VERSION", "")

	msg := refreshCmd()()
	refresh, ok := msg.(refreshMsg)
	if !ok {
		t.Fatalf("refreshCmd returned %T, want refreshMsg", msg)
	}
	if len(refresh) != 1 || refresh[0].ID != "aaa" {
		t.Errorf("refresh = %+v, want the managed container", refresh)
	}
}

func TestRefreshCmdReportsDaemonFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+srv.Listener.Addr().String())
	t.Setenv("DOCKER_API_VERSION", "")

	msg := refreshCmd()()
	if _, ok := msg.(errMsg); !ok {
		t.Fatalf("refreshCmd returned %T, want errMsg", msg)
	}
}

func TestFetchLogsReturnsNilWithoutContainers(t *testing.T) {
	m := newTestModel(t)
	if cmd := m.fetchLogs(); cmd != nil {
		t.Fatal("fetchLogs returned a command with no containers")
	}
}

func TestFetchLogsRedactsMultiplexedStream(t *testing.T) {
	body := append(frame("user=admin password=hunter2"), frame("hello from loki")...)
	tuiDaemon{
		logs: map[string][]byte{"aaa": body},
		tty:  map[string]bool{"aaa": false},
	}.start(t)

	m := newTestModel(t)
	m.containers = containersFixture()
	m.table.SetCursor(99) // falls back to the first container

	msg := m.fetchLogs()()
	logs, ok := msg.(logsMsg)
	if !ok {
		t.Fatalf("fetchLogs returned %T, want logsMsg", msg)
	}
	joined := strings.Join(logs, "\n")
	if strings.Contains(joined, "hunter2") {
		t.Errorf("logs leaked a redacted secret: %q", joined)
	}
	for _, want := range []string{"user=admin", "hello from loki"} {
		if !strings.Contains(joined, want) {
			t.Errorf("logs missing %q: %q", want, joined)
		}
	}
	if len(logs) != 2 {
		t.Errorf("got %d lines, want 2: %q", len(logs), logs)
	}
}

func TestFetchLogsReportsEmptyAndInspectErrors(t *testing.T) {
	t.Run("empty stream", func(t *testing.T) {
		tuiDaemon{logs: map[string][]byte{"aaa": nil}, tty: map[string]bool{"aaa": true}}.start(t)
		m := newTestModel(t)
		m.containers = containersFixture()

		msg := m.fetchLogs()()
		logs, ok := msg.(logsMsg)
		if !ok {
			t.Fatalf("fetchLogs returned %T, want logsMsg", msg)
		}
		if len(logs) != 1 || logs[0] != "<no logs>" {
			t.Errorf("empty stream = %v, want <no logs>", logs)
		}
	})

	t.Run("inspect failure", func(t *testing.T) {
		tuiDaemon{inspectErr: map[string]bool{"aaa": true}}.start(t)
		m := newTestModel(t)
		m.containers = containersFixture()

		if _, ok := m.fetchLogs()().(errMsg); !ok {
			t.Fatal("fetchLogs did not report the inspect failure")
		}
	})
}

// The async commands must only read the model snapshot they are given. A
// regression that mutates containers or the health cache from a command would
// race with rendering, so pin the invariant here.
func TestAsyncCommandsDoNotMutateModel(t *testing.T) {
	tuiDaemon{
		health: map[string]string{"aaa": "healthy"},
		logs:   map[string][]byte{"aaa": frame("line")},
		tty:    map[string]bool{"aaa": true},
	}.start(t)

	m := newTestModel(t)
	m.containers = containersFixture()
	m.healthCache["aaa"] = "starting"
	before := map[string]string{"aaa": "starting"}

	_ = refreshCmd()()
	_ = fetchHealthCmd(m.containers)()
	_ = m.fetchLogs()()

	if len(m.containers) != len(containersFixture()) {
		t.Errorf("a command mutated the container list: %d entries", len(m.containers))
	}
	if len(m.healthCache) != len(before) || m.healthCache["aaa"] != "starting" {
		t.Errorf("a command mutated the health cache: %+v", m.healthCache)
	}
}

// Rows must remain aligned with the filtered list when a health update lands
// while a filter is active.
func TestHealthMsgWhileFilteredKeepsRowsAligned(t *testing.T) {
	m := newTestModel(t)
	m.containers = containersFixture()
	m.filterActive = true
	m.filterQuery = "alloy"
	m.applyFilter()

	updated, _ := m.Update(healthMsg{"aaa": "healthy", "bbb": "unhealthy"})
	got := updated.(model)

	rows := got.table.Rows()
	if len(rows) != 1 {
		t.Fatalf("filtered rows = %d, want 1: %v", len(rows), rows)
	}
	if rows[0][0] != "orvyn-alloy" || rows[0][4] != "unhealthy" {
		t.Errorf("row = %v, want the alloy row with unhealthy health", rows[0])
	}
}

func TestTableIsPopulatedOnRefresh(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(refreshMsg(containersFixture()))
	got := updated.(model)

	rows := got.table.Rows()
	if len(rows) != 3 {
		t.Fatalf("table has %d rows, want 3", len(rows))
	}
	if rows[0][0] != "orvyn-loki" || rows[0][1] != "loki" {
		t.Errorf("first row = %v", rows[0])
	}
	if rows[2][4] != "-" {
		t.Errorf("exited container health = %q, want -", rows[2][4])
	}
}
