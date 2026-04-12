package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/neonwatty/fleet/internal/session"
)

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
			account = "—"
		}
		label := labelForSession(labels, s)
		line := fmt.Sprintf("%-8s %-20s %-8s %-10s %-10s %-14s %-14s",
			s.ID, truncateStr(s.Project, 20), s.Machine, s.Branch, uptime,
			truncateStr(account, 14), truncateStr(label, 14))
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func labelForSession(labels map[string][]session.MachineLabel, s session.Session) string {
	if labels == nil {
		return "—"
	}
	for _, l := range labels[s.Machine] {
		if l.SessionID == s.ID {
			return l.Name
		}
	}
	return "—"
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
