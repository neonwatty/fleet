package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
)

func TestBuildStatusJSON(t *testing.T) {
	healths := []machine.Health{
		{
			Name:        "mm1",
			Online:      true,
			TotalMemory: 16 * 1024 * 1024 * 1024,
			AvailMemory: 4 * 1024 * 1024 * 1024,
			SwapUsedMB:  2048,
			SwapTotalMB: 4096,
			ClaudeCount: 2,
		},
		{Name: "mm2", Online: false},
	}
	sessions := []session.Session{
		{
			ID:        "a1b2c3",
			Project:   "neonwatty/bleep",
			Machine:   "mm1",
			Branch:    "main",
			Account:   "personal-max",
			Tunnel:    session.TunnelInfo{LocalPort: 3000, RemotePort: 3000},
			StartedAt: time.Date(2026, 4, 12, 9, 15, 0, 0, time.UTC),
			PID:       4242,
		},
	}
	labels := map[string][]session.MachineLabel{
		"mm1": {
			{Name: "bleep", SessionID: "a1b2c3", LastSeenPID: 4242},
			{Name: "deckchecker", SessionID: ""},
		},
	}
	ccPIDs := map[string][]int{"mm1": {4242}}
	cfg := &config.Config{Machines: []config.Machine{{Name: "mm1"}, {Name: "mm2"}}}

	doc := buildStatusJSON(cfg, healths, sessions, labels, ccPIDs, time.Date(2026, 4, 12, 14, 32, 10, 0, time.UTC))
	blob, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(blob)

	for _, want := range []string{
		`"timestamp":"2026-04-12T14:32:10Z"`,
		`"name":"mm1"`,
		`"status":"online"`,
		`"accounts":["personal-max"]`,
		`"name":"bleep"`,
		`"live":true`,
		`"name":"deckchecker"`,
		`"live":false`,
		`"name":"mm2"`,
		`"status":"offline"`,
		`"project":"neonwatty/bleep"`,
		`"account":"personal-max"`,
		`"label":"bleep"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("json missing %q:\n%s", want, s)
		}
	}
}
