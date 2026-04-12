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
			{Name: "bleep", SessionID: "s1"},     // live
			{Name: "deckchecker", SessionID: ""}, // stale
		},
	}
	ccPIDs := map[string][]int{"mm1": {}}

	out := renderMachinesPanel(healths, sessions, labels, ccPIDs, 80)
	if !strings.Contains(out, "[personal-max]") {
		t.Errorf("expected [personal-max] suffix:\n%s", out)
	}
	if !strings.Contains(out, "bleep") {
		t.Errorf("expected live label 'bleep':\n%s", out)
	}
	if !strings.Contains(out, "deckchecker") {
		t.Errorf("expected stale label 'deckchecker':\n%s", out)
	}
}
