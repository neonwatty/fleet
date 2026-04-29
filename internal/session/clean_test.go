package session

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/neonwatty/fleet/internal/config"
)

func TestClassifySessions(t *testing.T) {
	sessions := []Session{
		{ID: "alive", Machine: "mm1", WorktreePath: "/exists"},
		{ID: "orphan", Machine: "mm1", WorktreePath: "/exists-no-proc"},
		{ID: "stale", Machine: "mm1", WorktreePath: "/gone"},
	}

	checker := func(sess Session) SessionStatus {
		switch sess.ID {
		case "alive":
			return StatusAlive
		case "orphan":
			return StatusOrphan
		case "stale":
			return StatusStale
		default:
			return StatusStale
		}
	}

	alive, orphan, stale := ClassifySessions(sessions, checker)
	if len(alive) != 1 || alive[0].ID != "alive" {
		t.Errorf("alive = %v, want [alive]", ids(alive))
	}
	if len(orphan) != 1 || orphan[0].ID != "orphan" {
		t.Errorf("orphan = %v, want [orphan]", ids(orphan))
	}
	if len(stale) != 1 || stale[0].ID != "stale" {
		t.Errorf("stale = %v, want [stale]", ids(stale))
	}
}

func TestRemoteCheckerTreatsDeadOwnerPIDAsOrphan(t *testing.T) {
	checker := MakeRemoteChecker(context.Background(), []config.Machine{
		{Name: "local", Host: "localhost", Enabled: true},
	})

	got := checker(Session{
		ID:           "dead-owner",
		Machine:      "local",
		WorktreePath: "/path/does/not/matter",
		OwnerPID:     1 << 30,
	})
	if got != StatusOrphan {
		t.Fatalf("status = %v, want StatusOrphan", got)
	}
}

func TestResetDanglingLabels(t *testing.T) {
	now := time.Now().UTC()
	state := &State{
		MachineLabels: map[string][]MachineLabel{
			"mm1": {
				{Name: "alive-label", SessionID: "s1", CreatedAt: now, LastSeenPID: 1111},
				{Name: "dangling-label", SessionID: "s2", CreatedAt: now, LastSeenPID: 2222},
				{Name: "already-orphan", SessionID: "", CreatedAt: now, LastSeenPID: 3333},
			},
			"mm2": {
				{Name: "also-dangling", SessionID: "s3", CreatedAt: now},
			},
		},
	}

	aliveIDs := map[string]bool{"s1": true}

	n := resetDanglingLabels(state, aliveIDs)
	if n != 2 {
		t.Errorf("resetDanglingLabels returned %d, want 2", n)
	}

	mm1 := state.MachineLabels["mm1"]

	// Alive-linked label preserved intact.
	if mm1[0].Name != "alive-label" {
		t.Errorf("mm1[0].Name = %q, want alive-label", mm1[0].Name)
	}
	if mm1[0].SessionID != "s1" {
		t.Errorf("alive-label SessionID = %q, want s1", mm1[0].SessionID)
	}
	if mm1[0].LastSeenPID != 1111 {
		t.Errorf("alive-label LastSeenPID = %d, want 1111", mm1[0].LastSeenPID)
	}

	// Dangling label has SessionID cleared; Name and LastSeenPID preserved.
	if mm1[1].Name != "dangling-label" {
		t.Errorf("dangling-label Name = %q, want preserved as 'dangling-label'", mm1[1].Name)
	}
	if mm1[1].SessionID != "" {
		t.Errorf("dangling-label SessionID = %q, want empty (reset)", mm1[1].SessionID)
	}
	if mm1[1].LastSeenPID != 2222 {
		t.Errorf("dangling-label LastSeenPID = %d, want preserved 2222", mm1[1].LastSeenPID)
	}

	// Already-orphan label untouched.
	if mm1[2].SessionID != "" || mm1[2].LastSeenPID != 3333 {
		t.Errorf("already-orphan label mutated: %+v", mm1[2])
	}

	// Cross-machine dangling label also reset.
	mm2 := state.MachineLabels["mm2"]
	if mm2[0].SessionID != "" {
		t.Errorf("mm2 also-dangling SessionID = %q, want empty", mm2[0].SessionID)
	}
	if mm2[0].Name != "also-dangling" {
		t.Errorf("mm2 also-dangling Name = %q, want preserved", mm2[0].Name)
	}
}

func TestResetDanglingLabelsEmpty(t *testing.T) {
	state := &State{}
	if n := resetDanglingLabels(state, map[string]bool{}); n != 0 {
		t.Errorf("resetDanglingLabels on empty state returned %d, want 0", n)
	}
}

func TestCleanDryRunDoesNotRewriteDanglingLabels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	state := &State{
		MachineLabels: map[string][]MachineLabel{
			"mm1": {{Name: "ghost", SessionID: "gone", CreatedAt: time.Now().UTC()}},
		},
	}
	if err := Save(path, state); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	result, err := CleanWithOptions(context.Background(), &config.Config{}, path, CleanOptions{DryRun: true})
	if err != nil {
		t.Fatalf("CleanWithOptions() error: %v", err)
	}
	if result.ResetLabels != 1 {
		t.Fatalf("ResetLabels = %d, want 1", result.ResetLabels)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if loaded.MachineLabels["mm1"][0].SessionID != "gone" {
		t.Fatalf("dry-run rewrote label: %+v", loaded.MachineLabels["mm1"][0])
	}
}

func TestCleanResetsDanglingLabelsWithoutSessions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	state := &State{
		MachineLabels: map[string][]MachineLabel{
			"mm1": {{Name: "ghost", SessionID: "gone", CreatedAt: time.Now().UTC()}},
		},
	}
	if err := Save(path, state); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	result, err := CleanWithOptions(context.Background(), &config.Config{}, path, CleanOptions{})
	if err != nil {
		t.Fatalf("CleanWithOptions() error: %v", err)
	}
	if result.ResetLabels != 1 {
		t.Fatalf("ResetLabels = %d, want 1", result.ResetLabels)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if loaded.MachineLabels["mm1"][0].SessionID != "" {
		t.Fatalf("SessionID = %q, want reset", loaded.MachineLabels["mm1"][0].SessionID)
	}
}

func TestCleanResultCleaned(t *testing.T) {
	result := CleanResult{
		Orphans: []Session{{ID: "o1"}, {ID: "o2"}},
		Stales:  []Session{{ID: "s1"}},
	}
	if result.Cleaned() != 3 {
		t.Fatalf("Cleaned() = %d, want 3", result.Cleaned())
	}
}

func TestTunnelLocalPortFromPSLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		port int
		ok   bool
	}{
		{
			name: "separate forward flag",
			line: "12345 ssh -N -L 4001:localhost:3000 -o ExitOnForwardFailure=yes neonwatty@mm1",
			port: 4001,
			ok:   true,
		},
		{
			name: "combined forward flag",
			line: "12345 ssh -N -L4002:localhost:3000 mm1",
			port: 4002,
			ok:   true,
		},
		{
			name: "no tunnel",
			line: "12345 ssh mm1 uptime",
			ok:   false,
		},
		{
			name: "invalid port",
			line: "12345 ssh -N -L nope:localhost:3000 mm1",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, ok := tunnelLocalPortFromPSLine(tt.line)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if port != tt.port {
				t.Fatalf("port = %d, want %d", port, tt.port)
			}
		})
	}
}

func ids(sessions []Session) []string {
	out := make([]string, len(sessions))
	for i, s := range sessions {
		out[i] = s.ID
	}
	return out
}
