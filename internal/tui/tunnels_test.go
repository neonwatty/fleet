package tui

import (
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/session"
)

func TestTunnelsPanelRendersOnlyTunneledSessions(t *testing.T) {
	sessions := []session.Session{
		{ID: "s0", Project: "neonwatty/no-tunnel", Machine: "mm0"},
		{
			ID:      "s1",
			Project: "neonwatty/bleep",
			Machine: "mm1",
			Tunnel:  session.TunnelInfo{LocalPort: 4001, RemotePort: 3000},
		},
	}

	out := renderTunnelsPanel(sessions)
	if !strings.Contains(out, "localhost:4001") {
		t.Fatalf("output missing local tunnel:\n%s", out)
	}
	if !strings.Contains(out, "mm1:3000") {
		t.Fatalf("output missing remote tunnel:\n%s", out)
	}
	if strings.Contains(out, "no-tunnel") {
		t.Fatalf("output should omit sessions without local ports:\n%s", out)
	}
}

func TestTunnelsPanelEmptyState(t *testing.T) {
	out := renderTunnelsPanel(nil)
	if !strings.Contains(out, "No active tunnels") {
		t.Fatalf("output missing empty state:\n%s", out)
	}
}
