package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fredy-arias/orvyn/internal/discovery"
	"github.com/fredy-arias/orvyn/internal/docker"
)

type model struct {
	table      table.Model
	containers []docker.Container
	err        error
	width      int
	height     int
}

type tickMsg time.Time
type refreshMsg []docker.Container

func initialModel() model {
	columns := []table.Column{
		{Title: "NAME", Width: 24},
		{Title: "SERVICE", Width: 16},
		{Title: "STATUS", Width: 28},
		{Title: "STATE", Width: 10},
		{Title: "HEALTH", Width: 10},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).BorderBottom(true).Bold(false)
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(false)
	t.SetStyles(s)
	return model{table: t}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), refreshCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func refreshCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		containers, err := discovery.ManagedStack(ctx)
		if err != nil {
			return err
		}
		return refreshMsg(containers)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			return m, refreshCmd()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width)
		return m, nil
	case tickMsg:
		return m, tea.Batch(tickCmd(), refreshCmd())
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
	case error:
		m.err = msg
		return m, nil
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit", m.err)
	}
	title := lipgloss.NewStyle().Bold(true).Render("Orvyn Dashboard")
	help := lipgloss.NewStyle().Faint(true).Render("r: refresh | q: quit | ↑/↓: navigate")
	return fmt.Sprintf("%s\n\n%s\n\n%s\n", title, m.table.View(), help)
}

// Run starts the TUI
func Run() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
