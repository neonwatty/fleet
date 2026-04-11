package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
)

type panel int

const (
	panelMachines panel = iota
	panelSessions
	panelTunnels
	panelProcesses
	panelCount
)

type model struct {
	cfg             *config.Config
	statePath       string
	healths         []machine.Health
	state           *session.State
	processes       map[string][]machine.ProcessGroup // keyed by machine name
	activePanel     panel
	selectedRow     int
	selectedMachine int // which machine is selected in Machines panel
	width           int
	height          int
	pollInterval    time.Duration
}

type tickMsg time.Time
type refreshMsg struct {
	healths   []machine.Health
	state     *session.State
	processes map[string][]machine.ProcessGroup
}

func NewModel(cfg *config.Config, statePath string) model {
	return model{
		cfg:          cfg,
		statePath:    statePath,
		pollInterval: time.Duration(cfg.Settings.PollInterval) * time.Second,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(refresh(m.cfg, m.statePath), tick(m.pollInterval))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activePanel = (m.activePanel + 1) % panelCount
			m.selectedRow = 0
		case "shift+tab":
			m.activePanel = (m.activePanel - 1 + panelCount) % panelCount
			m.selectedRow = 0
		case "j", "down":
			m.selectedRow++
			if m.activePanel == panelMachines {
				m.selectedMachine = m.selectedRow
			}
		case "k", "up":
			if m.selectedRow > 0 {
				m.selectedRow--
				if m.activePanel == panelMachines {
					m.selectedMachine = m.selectedRow
				}
			}
		case "o":
			if m.activePanel == panelTunnels && m.state != nil {
				tunneled := tunneledSessions(m.state.Sessions)
				if m.selectedRow < len(tunneled) {
					_ = openInBrowser(tunneled[m.selectedRow].Tunnel.LocalPort)
				}
			}
		case "x":
			if m.activePanel == panelSessions && m.state != nil {
				if m.selectedRow < len(m.state.Sessions) {
					sess := m.state.Sessions[m.selectedRow]
					_ = killSession(context.Background(), m.cfg, sess, m.statePath)
					return m, refresh(m.cfg, m.statePath)
				}
			}
		case "d":
			if m.activePanel == panelProcesses && m.processes != nil {
				machineName := m.selectedMachineName()
				groups := m.processes[machineName]
				if m.selectedRow < len(groups) && groups[m.selectedRow].Killable {
					mach := m.findMachine(machineName)
					if mach != nil {
						_ = machine.KillGroup(context.Background(), *mach, groups[m.selectedRow])
						return m, refresh(m.cfg, m.statePath)
					}
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		return m, tea.Batch(refresh(m.cfg, m.statePath), tick(m.pollInterval))
	case refreshMsg:
		m.healths = msg.healths
		m.state = msg.state
		m.processes = msg.processes
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	title := titleStyle.Render("Fleet Dashboard")
	panelWidth := m.width - 4

	machinesContent := renderMachinesPanel(m.healths, panelWidth)
	machinesPanel := wrapPanel("Machines", machinesContent, panelWidth, m.activePanel == panelMachines)

	var sessions []session.Session
	if m.state != nil {
		sessions = m.state.Sessions
	}
	sessionsContent := renderSessionsPanel(sessions)
	sessionsPanel := wrapPanel("Sessions", sessionsContent, panelWidth, m.activePanel == panelSessions)

	tunnelsContent := renderTunnelsPanel(sessions)
	tunnelsPanel := wrapPanel("Tunnels", tunnelsContent, panelWidth, m.activePanel == panelTunnels)

	machineName := m.selectedMachineName()
	var procGroups []machine.ProcessGroup
	if m.processes != nil {
		procGroups = m.processes[machineName]
	}
	processesTitle := "Processes"
	if machineName != "" {
		processesTitle = fmt.Sprintf("Processes on %s", machineName)
	}
	var procSelectedRow int
	if m.activePanel == panelProcesses {
		procSelectedRow = m.selectedRow
	} else {
		procSelectedRow = -1
	}
	processesContent := renderProcessesPanel(machineName, procGroups, procSelectedRow)
	processesPanel := wrapPanel(processesTitle, processesContent, panelWidth, m.activePanel == panelProcesses)

	help := helpStyle.Render("tab: switch panel | j/k: navigate | o: open in browser | x: kill session | d: kill process group | q: quit")

	return fmt.Sprintf("%s\n\n%s\n%s\n%s\n%s\n\n%s",
		title, machinesPanel, sessionsPanel, tunnelsPanel, processesPanel, help)
}

func wrapPanel(title, content string, width int, active bool) string {
	style := panelStyle
	if active {
		style = activePanelStyle
	}

	header := titleStyle.Render(title)
	inner := lipgloss.JoinVertical(lipgloss.Left, header, content)
	return style.Width(width).Render(inner)
}

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func refresh(cfg *config.Config, statePath string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		enabled := cfg.EnabledMachines()
		healths := machine.ProbeAll(ctx, enabled)
		state, _ := session.LoadState(statePath)

		processes := make(map[string][]machine.ProcessGroup)
		for _, m := range enabled {
			processes[m.Name] = machine.ProbeProcesses(ctx, m)
		}

		return refreshMsg{healths: healths, state: state, processes: processes}
	}
}

func tunneledSessions(sessions []session.Session) []session.Session {
	var out []session.Session
	for _, s := range sessions {
		if s.Tunnel.LocalPort > 0 {
			out = append(out, s)
		}
	}
	return out
}

func (m model) selectedMachineName() string {
	if len(m.healths) == 0 {
		return ""
	}
	idx := m.selectedMachine
	if idx >= len(m.healths) {
		idx = len(m.healths) - 1
	}
	return m.healths[idx].Name
}

func (m model) findMachine(name string) *config.Machine {
	for _, mach := range m.cfg.Machines {
		if mach.Name == name {
			return &mach
		}
	}
	return nil
}

func Run(cfg *config.Config, statePath string) error {
	p := tea.NewProgram(NewModel(cfg, statePath), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
