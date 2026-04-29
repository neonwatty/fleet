package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/neonwatty/fleet/internal/session"
)

func TestSessionsPanelRendersAccountAndLabel(t *testing.T) {
	sessions := []session.Session{
		{
			ID:        "a1b2c3d4",
			Project:   "neonwatty/bleep",
			Machine:   "mm1",
			Branch:    "main",
			Account:   "personal-max",
			StartedAt: time.Now().Add(-10 * time.Minute),
		},
	}
	labels := map[string][]session.MachineLabel{
		"mm1": {{Name: "bleep", SessionID: "a1b2c3d4"}},
	}

	out := renderSessionsPanel(sessions, labels)
	if !strings.Contains(out, "personal-max") {
		t.Errorf("output missing account:\n%s", out)
	}
	if !strings.Contains(out, "bleep") {
		t.Errorf("output missing label:\n%s", out)
	}
	if !strings.Contains(out, "ACCOUNT") || !strings.Contains(out, "LABEL") {
		t.Errorf("output missing headers:\n%s", out)
	}
}

func TestSessionsPanelEmptyWithoutAccountOrLabel(t *testing.T) {
	sessions := []session.Session{
		{ID: "x", Project: "p", Machine: "mm2", Branch: "main", StartedAt: time.Now()},
	}
	out := renderSessionsPanel(sessions, nil)
	if !strings.Contains(out, "—") {
		t.Errorf("expected em-dash placeholder for empty account/label:\n%s", out)
	}
}

func TestSessionsPanelRendersNonDefaultLaunchCommand(t *testing.T) {
	sessions := []session.Session{
		{
			ID:            "a1b2c3d4",
			Project:       "neonwatty/bleep",
			Machine:       "mm1",
			Branch:        "main",
			LaunchCommand: "npm run agent",
			StartedAt:     time.Now().Add(-10 * time.Minute),
		},
	}

	out := renderSessionsPanel(sessions, nil)
	if !strings.Contains(out, "cmd: npm run agent") {
		t.Errorf("output missing launch command:\n%s", out)
	}
}

func TestSessionsPanelHidesDefaultLaunchCommand(t *testing.T) {
	sessions := []session.Session{
		{
			ID:            "a1b2c3d4",
			Project:       "neonwatty/bleep",
			Machine:       "mm1",
			Branch:        "main",
			LaunchCommand: "claude",
			StartedAt:     time.Now().Add(-10 * time.Minute),
		},
	}

	out := renderSessionsPanel(sessions, nil)
	if strings.Contains(out, "cmd:") {
		t.Errorf("output should hide default launch command:\n%s", out)
	}
}
