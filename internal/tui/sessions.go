package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/neonwatty/fleet/internal/session"
)

// emDash renders "no value" in a fixed-width cell (empty account, empty
// label, offline machine).
const emDash = "\u2014"

func renderSessionsPanel(sessions []session.Session, labels map[string][]session.MachineLabel) string {
	if len(sessions) == 0 {
		return dimStyle.Render("No active sessions")
	}

	var b strings.Builder

	header := fmt.Sprintf("%-8s %-20s %-8s %-10s %-10s %-14s %-14s",
		"ID", "PROJECT", "MACHINE", "BRANCH", "UPTIME", "ACCOUNT", "LABEL")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for _, s := range sessions {
		uptime := time.Since(s.StartedAt).Truncate(time.Second)
		account := s.Account
		if account == "" {
			account = emDash
		}
		label := labelForSession(labels, s)
		line := fmt.Sprintf("%-8s %-20s %-8s %-10s %-10s %-14s %-14s",
			s.ID, truncateStr(s.Project, 20), s.Machine, s.Branch, uptime,
			truncateStr(account, 14), truncateStr(label, 14))
		b.WriteString(line)
		b.WriteString("\n")
		if s.LaunchCommand != "" && s.LaunchCommand != "claude" {
			b.WriteString(dimStyle.Render("  cmd: " + truncateStr(s.LaunchCommand, 72)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func labelForSession(labels map[string][]session.MachineLabel, s session.Session) string {
	if labels == nil {
		return emDash
	}
	for _, l := range labels[s.Machine] {
		if l.SessionID == s.ID {
			return l.Name
		}
	}
	return emDash
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
