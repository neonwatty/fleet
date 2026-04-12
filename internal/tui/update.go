package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.renaming {
			return m.handleRenameKey(msg)
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
	case swapScanMsg:
		m.swapScanning = false
		m.swapScanTarget = ""
		if m.processes == nil {
			m.processes = make(map[string][]machine.ProcessGroup)
		}
		m.processes[msg.machineName] = msg.groups
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
	default:
		return m, false
	}
	return m, true
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
		_ = openInBrowser(tunneled[m.selectedRow].Tunnel.LocalPort)
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
	sess := m.state.Sessions[m.selectedRow]
	_ = killSession(context.Background(), m.cfg, sess, m.statePath)
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
	mach := m.findMachine(machineName)
	if mach == nil {
		return m, nil
	}
	_ = machine.KillGroup(context.Background(), *mach, groups[m.selectedRow])
	return m, refresh(m.cfg, m.statePath)
}

// handleRenameKey processes a keystroke while the model is in label-rename
// mode. Enter commits via session.AddLabel, Esc cancels, backspace trims,
// printable runes append.
func (m model) handleRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.renaming = false
		m.renameBuffer = ""
	case "enter":
		if m.state != nil && m.selectedRow < len(m.state.Sessions) {
			sess := m.state.Sessions[m.selectedRow]
			if strings.TrimSpace(m.renameBuffer) != "" {
				_ = session.AddLabel(
					m.statePath,
					sess.Machine,
					strings.TrimSpace(m.renameBuffer),
					sess.ID,
					sess.PID,
				)
			}
		}
		m.renaming = false
		m.renameBuffer = ""
		return m, refresh(m.cfg, m.statePath)
	case "backspace":
		if len(m.renameBuffer) > 0 {
			m.renameBuffer = m.renameBuffer[:len(m.renameBuffer)-1]
		}
	default:
		if len(msg.Runes) == 1 {
			m.renameBuffer += string(msg.Runes)
		}
	}
	return m, nil
}
