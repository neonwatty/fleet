package tui

import (
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/machine"
)

func TestProcessesPanelRendersGroupsAndSelection(t *testing.T) {
	out := renderProcessesPanel("mm1", []machine.ProcessGroup{
		{
			Name:      "Dev Servers",
			Count:     2,
			TotalRSS:  768 * 1024,
			TotalSwap: -1,
			Detail:    "next",
			PIDs:      []int{100, 101},
			Killable:  true,
		},
		{
			Name:      "System",
			Count:     1,
			TotalRSS:  128 * 1024,
			TotalSwap: 16 * 1024,
			Killable:  false,
		},
	}, 0)

	for _, want := range []string{"CATEGORY", "Dev Servers", "768MB", "next", ">"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestProcessesPanelEmptyStates(t *testing.T) {
	if out := renderProcessesPanel("", nil, -1); !strings.Contains(out, "Select a machine") {
		t.Fatalf("output missing select-machine empty state:\n%s", out)
	}
	if out := renderProcessesPanel("mm1", nil, -1); !strings.Contains(out, "No significant processes") {
		t.Fatalf("output missing no-processes empty state:\n%s", out)
	}
}

func TestFormatRSS(t *testing.T) {
	if got := formatRSS(512 * 1024); got != "512MB" {
		t.Fatalf("formatRSS(512MB) = %q", got)
	}
	if got := formatRSS(1536 * 1024); got != "1.5GB" {
		t.Fatalf("formatRSS(1.5GB) = %q", got)
	}
}
