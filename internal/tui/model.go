package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fredy-arias/orvyn/internal/discovery"
	"github.com/fredy-arias/orvyn/internal/docker"
)

type viewState int

const (
	viewOverview viewState = iota
	viewLogs
	viewMetrics
	viewDetails
	viewHelp
)

type model struct {
	table        table.Model
	viewport     viewport.Model
	mtable       table.Model
	dtable       table.Model
	containers   []docker.Container
	filtered     []docker.Container
	logs         []string
	view         viewState
	hostm        HostMetrics
	contm        []ContMetrics
	err          error
	width        int
	height       int
	ready        bool
	dc           *docker.Client
	filterInput  textinput.Model
	filtering    bool
	filterActive bool
	filterQuery  string
	details      []table.Row
	healthCache  map[string]string
	metricsBusy  bool
}

type tickMsg time.Time
type refreshMsg []docker.Container
type logsMsg []string
type metricsMsg struct {
	host HostMetrics
	cont []ContMetrics
}
type errMsg error

func initialModel() model {
	columns := []table.Column{
		{Title: "NAME", Width: 22},
		{Title: "SERVICE", Width: 14},
		{Title: "STATUS", Width: 24},
		{Title: "STATE", Width: 10},
		{Title: "HEALTH", Width: 10},
	}
	t := table.New(table.WithColumns(columns), table.WithFocused(true), table.WithHeight(10))
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("39")).BorderBottom(true).Bold(true).Foreground(lipgloss.Color("39"))
	s.Selected = s.Selected.Foreground(lipgloss.Color("255")).Background(lipgloss.Color("39")).Bold(true)
	s.Cell = s.Cell.Foreground(lipgloss.Color("252"))
	t.SetStyles(s)

	vp := viewport.New(80, 16)

	mcols := []table.Column{
		{Title: "NAME", Width: 22},
		{Title: "CPU", Width: 12},
		{Title: "MEMORY", Width: 24},
	}
	mt := table.New(table.WithColumns(mcols), table.WithFocused(false), table.WithHeight(10))
	mt.SetStyles(s)

	dcols := []table.Column{
		{Title: "PROPERTY", Width: 20},
		{Title: "VALUE", Width: 60},
	}
	dt := table.New(table.WithColumns(dcols), table.WithFocused(false), table.WithHeight(16))
	dt.SetStyles(s)

	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = "filter by name or service..."
	ti.CharLimit = 64
	ti.Width = 50

	return model{
		table:       t,
		viewport:    vp,
		mtable:      mt,
		dtable:      dt,
		view:        viewOverview,
		filterInput: ti,
		healthCache: make(map[string]string),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), refreshCmd(), metricsCmd())
}

// containerName returns the display name for a container, falling back to the
// short ID when Docker reports no name, and strips the leading slash.
func containerName(ct docker.Container) string {
	if len(ct.Names) == 0 {
		return ct.ID
	}
	return strings.TrimPrefix(ct.Names[0], "/")
}

func (m model) applyFilter() {
	if !m.filterActive || m.filterQuery == "" {
		m.filtered = nil
		rows := []table.Row{}
		for _, ct := range m.containers {
			name := containerName(ct)
			svc := ct.Labels["com.orvyn.service"]
			if svc == "" {
				svc = "-"
			}
			health := ct.Labels["health"]
			if health == "" {
				if h, ok := m.healthCache[ct.ID]; ok {
					health = h
				} else if ct.State == "running" {
					health = "unknown"
				} else {
					health = "-"
				}
			}
			rows = append(rows, table.Row{name, svc, ct.Status, ct.State, health})
		}
		m.table.SetRows(rows)
		return
	}
	q := strings.ToLower(m.filterQuery)
	filtered := []docker.Container{}
	rows := []table.Row{}
	for _, ct := range m.containers {
		name := containerName(ct)
		svc := ct.Labels["com.orvyn.service"]
		if svc == "" {
			svc = "-"
		}
		if strings.Contains(strings.ToLower(name), q) || strings.Contains(strings.ToLower(svc), q) {
			filtered = append(filtered, ct)
			health := ct.Labels["health"]
			if health == "" {
				if h, ok := m.healthCache[ct.ID]; ok {
					health = h
				} else if ct.State == "running" {
					health = "unknown"
				} else {
					health = "-"
				}
			}
			rows = append(rows, table.Row{name, svc, ct.Status, ct.State, health})
		}
	}
	m.filtered = filtered
	m.table.SetRows(rows)
}

func (m model) getList() []docker.Container {
	if m.filterActive && m.filterQuery != "" {
		if len(m.filtered) > 0 {
			return m.filtered
		}
		return []docker.Container{}
	}
	return m.containers
}

func (m model) buildDetails() {
	list := m.getList()
	if len(list) == 0 {
		list = m.containers
	}
	if len(list) == 0 {
		m.details = []table.Row{}
		m.dtable.SetRows(m.details)
		return
	}
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(list) {
		idx = 0
	}
	ct := list[idx]
	name := containerName(ct)
	svc := ct.Labels["com.orvyn.service"]
	if svc == "" {
		svc = "-"
	}
	comp := ct.Labels["com.docker.compose.project"]
	if comp == "" {
		comp = "-"
	}
	rows := []table.Row{
		{"Name", name},
		{"Service", svc},
		{"Compose Project", comp},
		{"ID", ct.ID},
		{"Image", ct.Image},
		{"State", ct.State},
		{"Status", ct.Status},
		{"Managed", ct.Labels["com.orvyn.managed"]},
	}
	m.details = rows
	m.dtable.SetRows(rows)
}

func tickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func refreshCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		containers, err := discovery.ManagedStack(ctx)
		if err != nil {
			return errMsg(err)
		}
		return refreshMsg(containers)
	}
}

func metricsCmd() tea.Cmd {
	return func() tea.Msg {
		dc, err := docker.New()
		if err != nil {
			return metricsMsg{host: getHostMetrics(), cont: []ContMetrics{}}
		}
		defer dc.Close()
		return metricsMsg{host: getHostMetrics(), cont: getContainerMetrics(dc)}
	}
}

// startMetrics returns nil when a collection is already running, so overlapping
// ticks cannot pile up requests to the Docker daemon.
func (m *model) startMetrics() tea.Cmd {
	if m.metricsBusy {
		return nil
	}
	m.metricsBusy = true
	return metricsCmd()
}

func (m model) fetchLogs() tea.Cmd {
	if len(m.containers) == 0 {
		return nil
	}
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.containers) {
		idx = 0
	}
	ct := m.containers[idx]
	id := ct.ID
	if len(id) > 12 {
		id = id[:12]
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		dc, err := docker.New()
		if err != nil {
			return errMsg(err)
		}
		defer dc.Close()
		rc, err := dc.ContainerLogs(ctx, id, false, "80", false)
		if err != nil {
			return errMsg(err)
		}
		defer rc.Close()
		buf := make([]byte, 64*1024)
		n, _ := rc.Read(buf)
		if n == 0 {
			return logsMsg([]string{"<no logs>"})
		}
		raw := string(buf[:n])
		lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
		if len(lines) > 100 {
			lines = lines[len(lines)-100:]
		}
		return logsMsg(lines)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if m.filtering {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "enter" {
				m.filtering = false
				m.filterInput.Blur()
				m.filterActive = true
				m.filterQuery = m.filterInput.Value()
				m.applyFilter()
				return m, nil
			}
			if msg.String() == "esc" {
				m.filtering = false
				m.filterInput.Blur()
				m.filterInput.SetValue("")
				m.filterActive = false
				m.filterQuery = ""
				m.applyFilter()
				return m, nil
			}
		}
		m.filterInput, cmd = m.filterInput.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			if m.view == viewOverview {
				m.filtering = true
				m.filterInput.Focus()
				return m, textinput.Blink
			}
		case "esc":
			if m.view == viewOverview && m.filterActive {
				m.filterActive = false
				m.filterQuery = ""
				m.filterInput.SetValue("")
				m.applyFilter()
				return m, nil
			}
		case "r":
			if m.view == viewOverview {
				cmds = append(cmds, refreshCmd())
			} else if m.view == viewLogs {
				cmds = append(cmds, m.fetchLogs())
			} else if m.view == viewMetrics {
				cmds = append(cmds, m.startMetrics())
			}
			return m, tea.Batch(cmds...)
		case "1":
			m.view = viewOverview
			return m, nil
		case "2":
			if m.view != viewLogs {
				m.view = viewLogs
				cmds = append(cmds, m.fetchLogs())
			}
			return m, tea.Batch(cmds...)
		case "3":
			m.view = viewMetrics
			cmds = append(cmds, m.startMetrics())
			return m, tea.Batch(cmds...)
		case "4":
			m.view = viewDetails
			m.buildDetails()
			return m, nil
		case "tab":
			if m.view == viewOverview {
				m.view = viewLogs
				cmds = append(cmds, m.fetchLogs())
			} else if m.view == viewLogs {
				m.view = viewMetrics
				cmds = append(cmds, m.startMetrics())
			} else if m.view == viewMetrics {
				m.view = viewDetails
			} else if m.view == viewDetails {
				m.view = viewOverview
			} else if m.view == viewHelp {
				m.view = viewOverview
			}
			return m, tea.Batch(cmds...)
		case "d":
			if m.view == viewOverview {
				m.view = viewDetails
				m.buildDetails()
				return m, nil
			} else if m.view == viewDetails {
				m.view = viewOverview
				return m, nil
			}
		case "?":
			if m.view == viewHelp {
				m.view = viewOverview
			} else {
				m.view = viewHelp
			}
			return m, nil
		case "l":
			if m.view == viewOverview {
				m.view = viewLogs
				cmds = append(cmds, m.fetchLogs())
				return m, tea.Batch(cmds...)
			}
		case "m":
			m.view = viewMetrics
			cmds = append(cmds, m.startMetrics())
			return m, tea.Batch(cmds...)
		case "o":
			m.view = viewOverview
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width)
		m.table.SetHeight(msg.Height - 12)
		m.mtable.SetWidth(msg.Width)
		m.mtable.SetHeight(msg.Height - 18)
		m.dtable.SetWidth(msg.Width)
		m.dtable.SetHeight(msg.Height - 12)
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 12
		m.filterInput.Width = msg.Width - 20
		m.ready = true
		return m, nil
	case tickMsg:
		cmds = append(cmds, tickCmd())
		if m.view == viewOverview {
			cmds = append(cmds, refreshCmd())
		}
		if m.view == viewMetrics {
			cmds = append(cmds, m.startMetrics())
		}
		return m, tea.Batch(cmds...)
	case refreshMsg:
		m.containers = []docker.Container(msg)
		go func(conts []docker.Container, hc map[string]string) {
			dc, err := docker.New()
			if err != nil {
				return
			}
			defer dc.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			for _, ct := range conts {
				ins, err := dc.InspectContainer(ctx, ct.ID)
				if err != nil {
					continue
				}
				if ins.State.Health != nil && ins.State.Health.Status != "" {
					hc[ct.ID] = ins.State.Health.Status
				}
			}
		}(m.containers, m.healthCache)
		m.applyFilter()
		if m.view == viewDetails {
			m.buildDetails()
		}
		return m, nil
	case logsMsg:
		m.logs = []string(msg)
		m.viewport.SetContent(strings.Join(m.logs, "\n"))
		return m, nil
	case metricsMsg:
		m.metricsBusy = false
		m.hostm = msg.host
		m.contm = msg.cont
		rows := []table.Row{}
		for _, cm := range m.contm {
			rows = append(rows, table.Row{cm.Name, cm.CPU, cm.Mem})
		}
		m.mtable.SetRows(rows)
		return m, nil
	case errMsg:
		m.err = error(msg)
		return m, nil
	}

	if m.view == viewOverview {
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.view == viewLogs {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit", m.err)
	}

	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51")).Render("ORVYN")
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(" | k9s-style terminal dashboard")
	title := header + sub

	tabs := []string{"[1] Overview", "[2] Logs", "[3] Metrics", "[4] Details", "[?] Help"}
	var tabStrs []string
	for i, t := range tabs {
		if (i == 0 && m.view == viewOverview) || (i == 1 && m.view == viewLogs) || (i == 2 && m.view == viewMetrics) || (i == 3 && m.view == viewDetails) || (i == 4 && m.view == viewHelp) {
			tabStrs = append(tabStrs, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51")).Background(lipgloss.Color("237")).Padding(0, 1).Render(t))
		} else {
			tabStrs = append(tabStrs, lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Background(lipgloss.Color("235")).Padding(0, 1).Render(t))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabStrs...)

	if m.view == viewOverview {
		help := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("↑/↓ • /: filter • Esc: clear • d: details • l: logs • m: metrics • r: refresh • Tab: switch • ?: help • q: quit")
		base := fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n", title, tabBar, m.table.View(), help)
		if m.filtering {
			filterBar := lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Render(m.filterInput.View())
			base = fmt.Sprintf("%s\n%s\n", base, filterBar)
		} else if m.filterActive && m.filterQuery != "" {
			filterBar := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(fmt.Sprintf("Filter: %s (%d/%d)", m.filterQuery, len(m.filtered), len(m.containers)))
			base = fmt.Sprintf("%s\n%s\n", base, filterBar)
		}
		return base
	}

	if m.view == viewLogs {
		var svc string
		if len(m.containers) > 0 {
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.containers) {
				svc = containerName(m.containers[idx])
			}
		}
		logTitle := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Logs: %s (tail)", svc))
		help := lipgloss.NewStyle().Faint(true).Render("↑/↓/pgup/pgdn scroll • r: refresh • o: overview • m: metrics • tab: switch • q: quit")
		return fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n\n%s\n", title, tabBar, logTitle, m.viewport.View(), help)
	}

	if m.view == viewMetrics {
		h := m.hostm
		memPct := 0.0
		if h.MemTotal > 0 {
			memPct = float64(h.MemUsed) / float64(h.MemTotal) * 100.0
		}
		diskPct := 0.0
		if h.DiskTotal > 0 {
			diskPct = float64(h.DiskUsed) / float64(h.DiskTotal) * 100.0
		}
		host := lipgloss.NewStyle().Bold(true).Render("HOST") + "\n"
		host += fmt.Sprintf("  Load: %.2f   Uptime: %s\n", h.CPULoad1, h.Uptime)
		host += fmt.Sprintf("  RAM:   %.1fGB / %.1fGB (%.1f%%) used\n", float64(h.MemUsed)/1024/1024/1024, float64(h.MemTotal)/1024/1024/1024, memPct)
		host += fmt.Sprintf("  DISK:  %.1fGB / %.1fGB (%.1f%%) used  (/)\n", float64(h.DiskUsed)/1024/1024/1024, float64(h.DiskTotal)/1024/1024/1024, diskPct)
		ctitle := lipgloss.NewStyle().Bold(true).Render("CONTAINERS (managed)")
		help := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("r: refresh • o: overview • d: details • l: logs • /: filter • Tab: switch • q: quit")
		return fmt.Sprintf("%s\n%s\n\n%s\n%s\n\n%s\n\n%s\n", title, tabBar, host, ctitle, m.mtable.View(), help)
	}

	if m.view == viewDetails {
		list := m.getList()
		if len(list) == 0 {
			list = m.containers
		}
		var svc string
		if len(list) > 0 {
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(list) {
				svc = containerName(list[idx])
			}
		}
		dtitle := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Details: %s", svc))
		help := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("r: refresh • o: overview • l: logs • Tab: switch • q: quit")
		return fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n\n%s\n", title, tabBar, dtitle, m.dtable.View(), help)
	}

	if m.view == viewHelp {
		helpText := `KEYBINDS

  ↑/↓        Navigate
  Tab        Cycle views (Overview → Logs → Metrics → Details → Help)
  1..4       Jump to Overview / Logs / Metrics / Details
  /          Filter containers by name/service (Overview)
  Esc        Clear filter
  d          View details for selected service
  l          View logs for selected service
  m          View metrics (host + containers)
  s          Start/Stop selected service (toggle)
  Shift+R    Restart selected service
  r          Refresh
  ?          Toggle help
  o          Back to overview
  q / Ctrl+C Quit

INFO

  Orvyn Dashboard - Manage your isolated observability stack
  Only shows containers with com.orvyn.managed=true
  Metrics collected from /proc and Docker API`
		help := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("Press ? or Tab to return")
		return fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n", title, tabBar, helpText, help)
	}

	return ""
}

// Run starts the TUI
func Run() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
