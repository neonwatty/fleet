package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
)

func TestBuildStatusJSON(t *testing.T) {
	const machineName = "mm1"
	healths := []machine.Health{
		{
			Name:        machineName,
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
			ID:            "a1b2c3",
			Project:       "neonwatty/bleep",
			Machine:       "mm1",
			Branch:        "main",
			Account:       "personal-max",
			LaunchCommand: "claude --resume",
			Tunnel:        session.TunnelInfo{LocalPort: 3000, RemotePort: 3000},
			StartedAt:     time.Date(2026, 4, 12, 9, 15, 0, 0, time.UTC),
			OwnerPID:      4242,
		},
	}
	labels := map[string][]session.MachineLabel{
		machineName: {
			{Name: "bleep", SessionID: "a1b2c3", LastSeenPID: 4242},
			{Name: "deckchecker", SessionID: ""},
			{Name: "orphan-live", SessionID: "", LastSeenPID: 5555},
			{Name: "orphan-dead", SessionID: "", LastSeenPID: 6666},
			{Name: "ghost", SessionID: "dead-sess"}, // linked to removed session
		},
	}
	ccPIDs := map[string][]int{machineName: {4242, 5555}}
	processGroups := map[string][]machine.ProcessGroup{
		machineName: {
			{Name: "Claude Code", Count: 2, TotalRSS: 1536 * 1024, PIDs: []int{4242, 5555}},
			{Name: "Codex", Count: 1, TotalRSS: 512 * 1024, PIDs: []int{7777}},
		},
	}

	thresholds := thresholdConfig{SwapWarnMB: 1024, SwapHighMB: 4096}
	sshTargets := map[string]string{"mm1": "neonwatty@mm1", "mm2": "mm2"}
	doc := buildStatusJSON(healths, sessions, labels, ccPIDs, sshTargets, processGroups, thresholds, time.Date(2026, 4, 12, 14, 32, 10, 0, time.UTC))
	blob, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(blob)

	for _, want := range []string{
		`"version":"1"`,
		`"timestamp":"2026-04-12T14:32:10Z"`,
		`"name":"mm1"`,
		`"ssh_target":"neonwatty@mm1"`,
		`"status":"online"`,
		`"accounts":["personal-max"]`,
		`"name":"bleep"`,
		`"live":true`,
		`"name":"deckchecker"`,
		`"live":false`,
		`"name":"orphan-live"`,
		`"name":"orphan-dead"`,
		`"name":"ghost"`,
		`"name":"mm2"`,
		`"ssh_target":"mm2"`,
		`"status":"offline"`,
		`"project":"neonwatty/bleep"`,
		`"account":"personal-max"`,
		`"launch_command":"claude --resume"`,
		`"label":"bleep"`,
		`"agent_processes":[{"kind":"claude","count":2,"rss_mb":1536,"pids":[4242,5555]},{"kind":"codex","count":1,"rss_mb":512,"pids":[7777]}]`,
		`"swap_warn_mb":1024`,
		`"swap_high_mb":4096`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("json missing %q:\n%s", want, s)
		}
	}

	if !strings.Contains(s, "\"thresholds\"") {
		t.Errorf("json missing top-level \"thresholds\" key:\n%s", s)
	}

	var doc2 statusDoc
	if err := json.Unmarshal(blob, &doc2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	found := map[string]bool{}
	for _, m := range doc2.Machines {
		if m.Name == machineName {
			for _, l := range m.Labels {
				found[l.Name] = l.Live
			}
		}
	}
	if !found["bleep"] {
		t.Errorf("bleep should be live")
	}
	if found["deckchecker"] {
		t.Errorf("deckchecker should be stale")
	}
	if !found["orphan-live"] {
		t.Errorf("orphan-live (PID match) should be live")
	}
	if found["orphan-dead"] {
		t.Errorf("orphan-dead (no PID match) should be stale")
	}
	if found["ghost"] {
		t.Errorf("ghost (linked to removed session) should be stale")
	}
}
