package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/neonwatty/fleet/internal/session"
)

func (m model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "N":
		m.confirming = false
		m.pendingAction = actionNone
		m.statusMessage = "Cancelled"
		return m, nil
	case "enter", "y", "Y":
		action := m.pendingAction
		m.confirming = false
		m.pendingAction = actionNone
		switch action {
		case actionKillSession:
			return m.performKillSession()
		case actionKillProcess:
			return m.performKillProcess()
		default:
			return m, nil
		}
	default:
		return m, nil
	}
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
					0, // OwnerPID is the fleet CLI PID, not the remote claude PID
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
