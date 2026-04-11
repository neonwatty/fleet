package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/neonwatty/fleet/internal/session"
)

func renderSessionsPanel(sessions []session.Session) string {
	if len(sessions) == 0 {
		return dimStyle.Render("No active sessions")
	}

	var b strings.Builder

	header := fmt.Sprintf("%-8s %-20s %-8s %-10s %-10s",
		"ID", "PROJECT", "MACHINE", "BRANCH", "UPTIME")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for _, s := range sessions {
		uptime := time.Since(s.StartedAt).Truncate(time.Second)
		line := fmt.Sprintf("%-8s %-20s %-8s %-10s %-10s",
			s.ID, truncateStr(s.Project, 20), s.Machine, s.Branch, uptime)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
