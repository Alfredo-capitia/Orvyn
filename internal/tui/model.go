package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fredy-arias/orvyn/internal/config"
	"github.com/fredy-arias/orvyn/internal/discovery"
	"github.com/fredy-arias/orvyn/internal/docker"
	"github.com/fredy-arias/orvyn/internal/executil"
)

type viewState int

const (
	viewOverview viewState = iota
	viewLogs
	viewHelp
)

type model struct {
	table      table.Model
	viewport   viewport.Model
	containers []docker.Container
	logs       []string
	view       viewState
	err        error
	width      int
	height     int
	ready      bool
	statusMsg  string
	confirming bool
	action     string
	actionSvc  string
}

type tickMsg time.Time
type refreshMsg []docker.Container
type logsMsg []string
type errMsg error
type actionMsg struct {
	msg string
	err error
}
type clearMsg struct{}

func initialModel() model {
	columns := []table.Column{
		{Title: "NAME", Width: 22},
		{Title: "SERVICE", Width: 14},
		{Title: "STATUS", Width: 28},
		{Title: "STATE", Width: 10},
		{Title: "HEALTH", Width: 10},
	}
	t := table.New(table.WithColumns(columns), table.WithFocused(true), table.WithHeight(12))
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("39")).BorderBottom(true).Bold(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("39")).Bold(false)
	t.SetStyles(s)

	vp := viewport.New(80, 20)

	return model{
		table:    t,
		viewport: vp,
		view:     viewOverview,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), refreshCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(4*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
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

func clearStatusCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearMsg{} })
}

func (m model) doAction(action string) tea.Cmd {
	if len(m.containers) == 0 {
		return nil
	}
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.containers) {
		idx = 0
	}
	ct := m.containers[idx]
	name := ct.Names[0]
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}
	svc := ct.Labels["com.orvyn.service"]
	if svc == "" {
		svc = name
	}
	cfg := config.Default()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		var out []byte
		var err error
		switch action {
		case "start":
			out, err = executil.RunWithTimeout(ctx, 120*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "start", svc)
		case "stop":
			out, err = executil.RunWithTimeout(ctx, 120*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "stop", svc)
		case "restart":
			out, err = executil.RunWithTimeout(ctx, 180*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "restart", svc)
		default:
			return actionMsg{msg: "", err: fmt.Errorf("unknown action")}
		}
		if err != nil {
			return actionMsg{msg: string(out), err: err}
		}
		return actionMsg{msg: fmt.Sprintf("[%s] %s done", strings.ToUpper(action), svc), err: nil}
	}
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
		if len(lines) > 80 {
			lines = lines[len(lines)-80:]
		}
		return logsMsg(lines)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if m.confirming {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "y" || msg.String() == "Y" {
				m.confirming = false
				m.statusMsg = fmt.Sprintf("[%s] running...", strings.ToUpper(m.action))
				cmds = append(cmds, m.doAction(m.action))
				return m, tea.Batch(cmds...)
			}
			if msg.String() == "n" || msg.String() == "N" || msg.String() == "q" || msg.String() == "esc" {
				m.confirming = false
				m.statusMsg = "cancelled"
				cmds = append(cmds, clearStatusCmd())
				return m, tea.Batch(cmds...)
			}
			return m, nil
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			if m.view == viewOverview {
				cmds = append(cmds, refreshCmd())
			} else if m.view == viewLogs {
				cmds = append(cmds, m.fetchLogs())
			}
			return m, tea.Batch(cmds...)
		case "s":
			if m.view == viewOverview && len(m.containers) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.containers) {
					ct := m.containers[idx]
					if ct.State == "running" {
						m.action = "stop"
					} else {
						m.action = "start"
					}
					name := ct.Names[0]
					if len(name) > 0 && name[0] == '/' {
						name = name[1:]
					}
					m.actionSvc = name
					m.confirming = true
					m.statusMsg = fmt.Sprintf("Confirm %s %s? (y/n)", m.action, m.actionSvc)
					return m, nil
				}
			}
		case "R":
			if m.view == viewOverview && len(m.containers) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.containers) {
					ct := m.containers[idx]
					m.action = "restart"
					name := ct.Names[0]
					if len(name) > 0 && name[0] == '/' {
						name = name[1:]
					}
					m.actionSvc = name
					m.confirming = true
					m.statusMsg = fmt.Sprintf("Confirm restart %s? (y/n)", m.actionSvc)
					return m, nil
				}
			}
		case "tab":
			if m.view == viewOverview {
				m.view = viewLogs
				cmds = append(cmds, m.fetchLogs())
			} else if m.view == viewLogs {
				m.view = viewOverview
			} else if m.view == viewHelp {
				m.view = viewOverview
			}
			return m, tea.Batch(cmds...)
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
		case "o":
			m.view = viewOverview
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width)
		m.table.SetHeight(msg.Height - 10)
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 12
		m.ready = true
		return m, nil
	case tickMsg:
		if m.view == viewOverview {
			cmds = append(cmds, tickCmd(), refreshCmd())
		} else {
			cmds = append(cmds, tickCmd())
		}
		return m, tea.Batch(cmds...)
	case refreshMsg:
		m.containers = []docker.Container(msg)
		rows := []table.Row{}
		for _, ct := range m.containers {
			name := ct.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
			svc := ct.Labels["com.orvyn.service"]
			if svc == "" {
				svc = "-"
			}
			health := "-"
			if ct.State == "running" {
				health = "unknown"
			}
			rows = append(rows, table.Row{name, svc, ct.Status, ct.State, health})
		}
		m.table.SetRows(rows)
		return m, nil
	case logsMsg:
		m.logs = []string(msg)
		m.viewport.SetContent(strings.Join(m.logs, "\n"))
		return m, nil
	case actionMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("ERR: %v", msg.err)
		} else {
			m.statusMsg = msg.msg
		}
		cmds = append(cmds, clearStatusCmd(), refreshCmd())
		return m, tea.Batch(cmds...)
	case clearMsg:
		m.statusMsg = ""
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

	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render("ORVYN")
	sub := lipgloss.NewStyle().Faint(true).Render(" | k9s-style terminal dashboard")
	title := header + sub

	tabs := []string{"[1/Tab] Overview", "[l] Logs", "[?] Help"}
	var tabStrs []string
	for i, t := range tabs {
		if (i == 0 && m.view == viewOverview) || (i == 1 && m.view == viewLogs) || (i == 2 && m.view == viewHelp) {
			tabStrs = append(tabStrs, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render(t))
		} else {
			tabStrs = append(tabStrs, lipgloss.NewStyle().Faint(true).Render(t))
		}
	}
	tabBar := strings.Join(tabStrs, "  ")

	if m.view == viewOverview {
		help := lipgloss.NewStyle().Faint(true).Render("↑/↓ • s: start/stop • R: restart • l: logs • r: refresh • tab: switch • ?: help • q: quit")
		base := fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n", title, tabBar, m.table.View(), help)
		if m.statusMsg != "" {
			status := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(m.statusMsg)
			base = fmt.Sprintf("%s\n%s\n", base, status)
		}
		if m.confirming {
			conf := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226")).Render(m.statusMsg)
			base = fmt.Sprintf("%s\n%s\n", base, conf)
		}
		return base
	}

	if m.view == viewLogs {
		var svc string
		if len(m.containers) > 0 {
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.containers) {
				ct := m.containers[idx]
				name := ct.Names[0]
				if len(name) > 0 && name[0] == '/' {
					name = name[1:]
				}
				svc = name
			}
		}
		logTitle := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Logs: %s (tail)", svc))
		help := lipgloss.NewStyle().Faint(true).Render("↑/↓/pgup/pgdn scroll • r: refresh • o: overview • tab: switch • q: quit")
		return fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n\n%s\n", title, tabBar, logTitle, m.viewport.View(), help)
	}

	if m.view == viewHelp {
		helpText := `KEYBINDS

  ↑/↓        Navigate
  Tab / o    Switch views
  l          View logs for selected service
  s          Start/Stop selected service (toggle)
  Shift+R    Restart selected service
  r          Refresh
  ?          Toggle help
  q / Ctrl+C Quit

INFO

  Orvyn Dashboard - Manage your isolated observability stack
  Only shows containers with com.orvyn.managed=true
  Actions use docker compose from stack directory`
		help := lipgloss.NewStyle().Faint(true).Render("Press ? or Tab to return")
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
