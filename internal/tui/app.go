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
	swapScanning    bool   // true while a swap scan is in progress
	swapScanTarget  string // machine name being scanned
	renaming        bool
	renameBuffer    string
}

type tickMsg time.Time
type refreshMsg struct {
	healths   []machine.Health
	state     *session.State
	processes map[string][]machine.ProcessGroup
}
type swapScanMsg struct {
	machineName string
	groups      []machine.ProcessGroup
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

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	title := titleStyle.Render("Fleet Dashboard")
	panelWidth := m.width - 4

	var sessions []session.Session
	var labels map[string][]session.MachineLabel
	if m.state != nil {
		sessions = m.state.Sessions
		labels = m.state.MachineLabels
	}
	ccPIDs := ccPIDsFromProcesses(m.processes)
	liveSessionIDs := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		liveSessionIDs[s.ID] = true
	}
	machinesContent := renderMachinesPanel(m.healths, sessions, labels, ccPIDs, liveSessionIDs, panelWidth)
	machinesPanel := wrapPanel("Machines", machinesContent, panelWidth, m.activePanel == panelMachines)

	sessionsContent := renderSessionsPanel(sessions, labels)
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

	helpParts := "tab: switch panel | j/k: navigate | o: open in browser | x: kill session | n: rename label | s: scan swap | d: kill process group | q: quit"
	if m.renaming {
		helpParts = fmt.Sprintf("rename label: %s▌  (enter: save, esc: cancel)", m.renameBuffer)
	} else if m.swapScanning {
		helpParts = fmt.Sprintf("Scanning swap on %s... | q: quit", m.swapScanTarget)
	}
	help := helpStyle.Render(helpParts)

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

func scanSwap(cfg *config.Config, m config.Machine, groups []machine.ProcessGroup) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		scanned := machine.ScanSwap(ctx, m, groups, cfg.Settings.SwapScanProcs)
		return swapScanMsg{machineName: m.Name, groups: scanned}
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

func ccPIDsFromProcesses(procs map[string][]machine.ProcessGroup) map[string][]int {
	out := make(map[string][]int, len(procs))
	for name, groups := range procs {
		for _, g := range groups {
			if g.Name == "Claude Code" {
				out[name] = append(out[name], g.PIDs...)
			}
		}
	}
	return out
}

func Run(cfg *config.Config, statePath string) error {
	p := tea.NewProgram(NewModel(cfg, statePath), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
