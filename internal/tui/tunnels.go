package tui

import (
	"fmt"
	"strings"

	"github.com/neonwatty/fleet/internal/session"
)

func renderTunnelsPanel(sessions []session.Session) string {
	tunneled := make([]session.Session, 0)
	for _, s := range sessions {
		if s.Tunnel.LocalPort > 0 {
			tunneled = append(tunneled, s)
		}
	}

	if len(tunneled) == 0 {
		return dimStyle.Render("No active tunnels")
	}

	var b strings.Builder

	header := fmt.Sprintf("%-22s %-12s %-20s",
		"LOCAL", "MACHINE", "PROJECT")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for _, s := range tunneled {
		local := fmt.Sprintf("localhost:%d", s.Tunnel.LocalPort)
		remote := fmt.Sprintf("→ %s:%d", s.Machine, s.Tunnel.RemotePort)
		line := fmt.Sprintf("%-22s %-12s %-20s",
			local+" "+remote, s.Machine, truncateStr(s.Project, 20))
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}
