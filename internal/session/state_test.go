package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := &State{
		Sessions: []Session{
			{
				ID:           "abc123",
				Project:      "neonwatty/seatify",
				Machine:      "mm2",
				Branch:       "main",
				WorktreePath: "/Users/jeremywatt/fleet-work/seatify-1234",
				Tunnel:       TunnelInfo{LocalPort: 4001, RemotePort: 3000},
				StartedAt:    time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC),
				PID:          12345,
			},
		},
	}

	if err := Save(path, s); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}

	if len(loaded.Sessions) != 1 {
		t.Fatalf("len(Sessions) = %d, want 1", len(loaded.Sessions))
	}
	if loaded.Sessions[0].ID != "abc123" {
		t.Errorf("ID = %q, want %q", loaded.Sessions[0].ID, "abc123")
	}
	if loaded.Sessions[0].Tunnel.LocalPort != 4001 {
		t.Errorf("LocalPort = %d, want 4001", loaded.Sessions[0].Tunnel.LocalPort)
	}
}

func TestLoadStateMissingFile(t *testing.T) {
	s, err := LoadState("/nonexistent/state.json")
	if err != nil {
		t.Fatalf("LoadState() should return empty state, got error: %v", err)
	}
	if len(s.Sessions) != 0 {
		t.Errorf("expected empty sessions for missing file")
	}
}

func TestAddAndRemoveSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	sess := Session{
		ID:      "test1",
		Project: "org/repo",
		Machine: "mm1",
	}

	if err := AddSession(path, sess); err != nil {
		t.Fatalf("AddSession() error: %v", err)
	}

	s, _ := LoadState(path)
	if len(s.Sessions) != 1 {
		t.Fatalf("expected 1 session after add")
	}

	if err := RemoveSession(path, "test1"); err != nil {
		t.Fatalf("RemoveSession() error: %v", err)
	}

	s, _ = LoadState(path)
	if len(s.Sessions) != 0 {
		t.Errorf("expected 0 sessions after remove, got %d", len(s.Sessions))
	}
}

func TestUsedPorts(t *testing.T) {
	s := &State{
		Sessions: []Session{
			{ID: "a", Tunnel: TunnelInfo{LocalPort: 4001}},
			{ID: "b", Tunnel: TunnelInfo{LocalPort: 4005}},
		},
	}

	ports := s.UsedPorts()
	if len(ports) != 2 {
		t.Fatalf("len(UsedPorts) = %d, want 2", len(ports))
	}
	if !ports[4001] || !ports[4005] {
		t.Errorf("expected ports 4001 and 4005 in set")
	}
}

func TestDefaultPath(t *testing.T) {
	path := DefaultStatePath()
	if !filepath.IsAbs(path) {
		t.Errorf("DefaultStatePath() = %q, want absolute path", path)
	}
	if !strings.Contains(path, ".fleet") {
		t.Errorf("DefaultStatePath() = %q, want to contain .fleet", path)
	}
}

func TestGenerateID(t *testing.T) {
	id := GenerateID()
	if len(id) != 8 {
		t.Errorf("GenerateID() len = %d, want 8", len(id))
	}

	id2 := GenerateID()
	if id == id2 {
		t.Error("two GenerateID() calls returned same value")
	}
}

func TestStateRoundTripWithLabelsAndAccounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	created := time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC)
	s := &State{
		Sessions: []Session{
			{
				ID:      "abc123",
				Project: "neonwatty/bleep",
				Machine: "mm1",
				Branch:  "main",
				Account: "personal-max",
			},
		},
		MachineLabels: map[string][]MachineLabel{
			"mm1": {
				{Name: "bleep", SessionID: "abc123", CreatedAt: created, LastSeenPID: 4242},
				{Name: "deckchecker", SessionID: "", CreatedAt: created, LastSeenPID: 0},
			},
		},
	}

	if err := Save(path, s); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}

	if loaded.Sessions[0].Account != "personal-max" {
		t.Errorf("Account = %q, want %q", loaded.Sessions[0].Account, "personal-max")
	}
	if len(loaded.MachineLabels["mm1"]) != 2 {
		t.Fatalf("len(MachineLabels[mm1]) = %d, want 2", len(loaded.MachineLabels["mm1"]))
	}
	if loaded.MachineLabels["mm1"][0].Name != "bleep" {
		t.Errorf("label[0].Name = %q, want %q", loaded.MachineLabels["mm1"][0].Name, "bleep")
	}
	if loaded.MachineLabels["mm1"][0].SessionID != "abc123" {
		t.Errorf("label[0].SessionID = %q, want %q", loaded.MachineLabels["mm1"][0].SessionID, "abc123")
	}
	if loaded.MachineLabels["mm1"][1].SessionID != "" {
		t.Errorf("label[1].SessionID = %q, want empty (orphan)", loaded.MachineLabels["mm1"][1].SessionID)
	}
}

func TestLoadStateBackCompatNoLabels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	legacy := `{"sessions":[{"id":"x","project":"p","machine":"m","branch":"main","worktree_path":"/tmp","tunnel":{"local_port":0,"remote_port":0},"started_at":"2026-04-11T08:00:00Z","pid":1}]}`
	if err := os.WriteFile(path, []byte(legacy), 0644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if len(loaded.Sessions) != 1 {
		t.Fatalf("len(Sessions) = %d, want 1", len(loaded.Sessions))
	}
	if loaded.Sessions[0].Account != "" {
		t.Errorf("legacy Account = %q, want empty", loaded.Sessions[0].Account)
	}
	if loaded.MachineLabels != nil && len(loaded.MachineLabels) > 0 {
		t.Errorf("legacy MachineLabels should be nil/empty, got %v", loaded.MachineLabels)
	}
}
