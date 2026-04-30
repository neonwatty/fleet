package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/neonwatty/fleet/internal/machine"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.renaming {
			return m.handleRenameKey(msg)
		}
		if m.confirming {
			return m.handleConfirmKey(msg)
		}
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		return m, tea.Batch(refresh(m.cfg, m.statePath), tick(m.pollInterval))
	case refreshMsg:
		m.healths = msg.healths
		m.state = msg.state
		m.processes = msg.processes
		m = m.clampSelection()
	case swapScanMsg:
		m.swapScanning = false
		m.swapScanTarget = ""
		if m.processes == nil {
			m.processes = make(map[string][]machine.ProcessGroup)
		}
		m.processes[msg.machineName] = msg.groups
		m = m.clampSelection()
	}
	return m, nil
}

// handleKey dispatches a keystroke in the normal (non-rename) mode. Navigation
// keys are handled inline; action keys delegate to per-action helpers.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "q" || key == "ctrl+c" {
		return m, tea.Quit
	}
	m.statusMessage = ""
	if nav, ok := m.handleNavKey(key); ok {
		return nav, nil
	}
	return m.handleActionKey(key)
}

// handleNavKey handles tab/j/k panel navigation. Returns the updated model and
// true if the key was a navigation key, otherwise returns the unchanged model
// and false.
func (m model) handleNavKey(key string) (model, bool) {
	switch key {
	case "tab":
		m.activePanel = (m.activePanel + 1) % panelCount
		m.selectedRow = 0
	case "shift+tab":
		m.activePanel = (m.activePanel - 1 + panelCount) % panelCount
		m.selectedRow = 0
	case "j", "down":
		if m.selectedRow < m.rowCount()-1 {
			m.selectedRow++
		}
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
	default:
		return m, false
	}
	return m.clampSelection(), true
}

func (m model) rowCount() int {
	switch m.activePanel {
	case panelMachines:
		return len(m.healths)
	case panelSessions:
		if m.state == nil {
			return 0
		}
		return len(m.state.Sessions)
	case panelTunnels:
		if m.state == nil {
			return 0
		}
		return len(tunneledSessions(m.state.Sessions))
	case panelProcesses:
		if m.processes == nil {
			return 0
		}
		return len(m.processes[m.selectedMachineName()])
	default:
		return 0
	}
}

func (m model) clampSelection() model {
	if len(m.healths) == 0 {
		m.selectedMachine = 0
	} else if m.selectedMachine >= len(m.healths) {
		m.selectedMachine = len(m.healths) - 1
	}
	rows := m.rowCount()
	if rows == 0 {
		m.selectedRow = 0
		return m
	}
	if m.selectedRow >= rows {
		m.selectedRow = rows - 1
	}
	if m.selectedRow < 0 {
		m.selectedRow = 0
	}
	if m.activePanel == panelMachines {
		m.selectedMachine = m.selectedRow
	}
	return m
}

// handleActionKey handles per-panel action keys (o/x/n/s/d).
func (m model) handleActionKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "o":
		return m.handleOpenTunnel()
	case "x":
		return m.handleKillSession()
	case "n":
		if m.activePanel == panelSessions && m.state != nil && m.selectedRow < len(m.state.Sessions) {
			m.renaming = true
			m.renameBuffer = ""
		}
	case "s":
		return m.handleScanSwap()
	case "d":
		return m.handleKillProcess()
	}
	return m, nil
}

func (m model) handleOpenTunnel() (tea.Model, tea.Cmd) {
	if m.activePanel != panelTunnels || m.state == nil {
		return m, nil
	}
	tunneled := tunneledSessions(m.state.Sessions)
	if m.selectedRow < len(tunneled) {
		_ = openBrowserFunc(tunneled[m.selectedRow].Tunnel.LocalPort)
	}
	return m, nil
}

func (m model) handleKillSession() (tea.Model, tea.Cmd) {
	if m.activePanel != panelSessions || m.state == nil {
		return m, nil
	}
	if m.selectedRow >= len(m.state.Sessions) {
		return m, nil
	}
	m.confirming = true
	m.pendingAction = actionKillSession
	return m, nil
}

func (m model) performKillSession() (tea.Model, tea.Cmd) {
	if m.state == nil || m.selectedRow >= len(m.state.Sessions) {
		m.statusMessage = "Error: selected session is no longer available"
		return m, nil
	}
	sess := m.state.Sessions[m.selectedRow]
	if err := killSessionFunc(context.Background(), m.cfg, sess, m.statePath); err != nil {
		m.statusMessage = "Error: " + err.Error()
		return m, nil
	}
	m.statusMessage = "Killed session " + sess.ID
	return m, refresh(m.cfg, m.statePath)
}

func (m model) handleScanSwap() (tea.Model, tea.Cmd) {
	if m.activePanel != panelProcesses || m.swapScanning {
		return m, nil
	}
	machineName := m.selectedMachineName()
	mach := m.findMachine(machineName)
	groups := m.processes[machineName]
	if mach == nil || len(groups) == 0 {
		return m, nil
	}
	m.swapScanning = true
	m.swapScanTarget = machineName
	return m, scanSwap(m.cfg, *mach, groups)
}

func (m model) handleKillProcess() (tea.Model, tea.Cmd) {
	if m.activePanel != panelProcesses || m.processes == nil {
		return m, nil
	}
	machineName := m.selectedMachineName()
	groups := m.processes[machineName]
	if m.selectedRow >= len(groups) || !groups[m.selectedRow].Killable {
		return m, nil
	}
	m.confirming = true
	m.pendingAction = actionKillProcess
	return m, nil
}

func (m model) performKillProcess() (tea.Model, tea.Cmd) {
	if m.processes == nil {
		m.statusMessage = "Error: selected process group is no longer available"
		return m, nil
	}
	machineName := m.selectedMachineName()
	groups := m.processes[machineName]
	if m.selectedRow >= len(groups) || !groups[m.selectedRow].Killable {
		m.statusMessage = "Error: selected process group is no longer available"
		return m, nil
	}
	mach := m.findMachine(machineName)
	if mach == nil {
		return m, nil
	}
	group := groups[m.selectedRow]
	if err := killGroupFunc(context.Background(), *mach, group); err != nil {
		m.statusMessage = "Error: " + err.Error()
		return m, nil
	}
	m.statusMessage = "Killed " + group.Name + " on " + machineName
	return m, refresh(m.cfg, m.statePath)
}
