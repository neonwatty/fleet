package tui

import (
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
)

func TestMachinesPanelRendersAccountSuffixAndLabels(t *testing.T) {
	healths := []machine.Health{
		{
			Name:        "mm1",
			Online:      true,
			TotalMemory: 16 * 1024 * 1024 * 1024,
			AvailMemory: 8 * 1024 * 1024 * 1024,
			SwapUsedMB:  0,
			SwapTotalMB: 4096,
			ClaudeCount: 2,
		},
	}
	sessions := []session.Session{
		{ID: "s1", Machine: "mm1", Account: "personal-max"},
	}
	labels := map[string][]session.MachineLabel{
		"mm1": {
			{Name: "bleep", SessionID: "s1"},     // live: session exists
			{Name: "deckchecker", SessionID: ""}, // stale: orphan, no PID
			{Name: "ghost", SessionID: "gone"},   // stale: linked to removed session
		},
	}
	ccPIDs := map[string][]int{"mm1": {}}
	liveSessionIDs := map[string]bool{"s1": true}

	out := renderMachinesPanel(healths, sessions, labels, ccPIDs, liveSessionIDs, 1024, 4096, 80)
	if !strings.Contains(out, "[personal-max]") {
		t.Errorf("expected [personal-max] suffix:\n%s", out)
	}
	if !strings.Contains(out, "bleep") {
		t.Errorf("expected live label 'bleep':\n%s", out)
	}
	if !strings.Contains(out, "deckchecker") {
		t.Errorf("expected stale label 'deckchecker':\n%s", out)
	}
	if !strings.Contains(out, "ghost") {
		t.Errorf("expected linked-but-dead label 'ghost':\n%s", out)
	}
	// ghost should be dimmed as stale, not bright live
	if !strings.Contains(out, "ghost(stale)") {
		t.Errorf("expected ghost to render with (stale) suffix (linked session removed):\n%s", out)
	}
}
