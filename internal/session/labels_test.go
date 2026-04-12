package session

import (
	"path/filepath"
	"testing"
)

func TestAddLabelOrphan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := AddLabel(path, "mm1", "bleep", "", 0); err != nil {
		t.Fatalf("AddLabel() error: %v", err)
	}

	s, _ := LoadState(path)
	if len(s.MachineLabels["mm1"]) != 1 {
		t.Fatalf("len = %d, want 1", len(s.MachineLabels["mm1"]))
	}
	if s.MachineLabels["mm1"][0].Name != "bleep" {
		t.Errorf("Name = %q, want %q", s.MachineLabels["mm1"][0].Name, "bleep")
	}
	if s.MachineLabels["mm1"][0].SessionID != "" {
		t.Errorf("SessionID = %q, want empty (orphan)", s.MachineLabels["mm1"][0].SessionID)
	}
}

func TestAddLabelLinkedToSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := AddLabel(path, "mm1", "bleep", "sess-123", 4242); err != nil {
		t.Fatalf("AddLabel() error: %v", err)
	}

	s, _ := LoadState(path)
	if s.MachineLabels["mm1"][0].SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want sess-123", s.MachineLabels["mm1"][0].SessionID)
	}
	if s.MachineLabels["mm1"][0].LastSeenPID != 4242 {
		t.Errorf("LastSeenPID = %d, want 4242", s.MachineLabels["mm1"][0].LastSeenPID)
	}
}

func TestAddLabelDuplicateOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	_ = AddLabel(path, "mm1", "bleep", "s1", 1)
	_ = AddLabel(path, "mm1", "bleep", "s2", 2)

	s, _ := LoadState(path)
	if len(s.MachineLabels["mm1"]) != 1 {
		t.Fatalf("len = %d, want 1 (duplicate should overwrite)", len(s.MachineLabels["mm1"]))
	}
	if s.MachineLabels["mm1"][0].SessionID != "s2" {
		t.Errorf("SessionID = %q, want s2", s.MachineLabels["mm1"][0].SessionID)
	}
}

func TestRemoveLabel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	_ = AddLabel(path, "mm1", "bleep", "", 0)
	_ = AddLabel(path, "mm1", "deckchecker", "", 0)

	if err := RemoveLabel(path, "mm1", "bleep"); err != nil {
		t.Fatalf("RemoveLabel() error: %v", err)
	}

	s, _ := LoadState(path)
	if len(s.MachineLabels["mm1"]) != 1 {
		t.Fatalf("len = %d, want 1", len(s.MachineLabels["mm1"]))
	}
	if s.MachineLabels["mm1"][0].Name != "deckchecker" {
		t.Errorf("Name = %q, want deckchecker", s.MachineLabels["mm1"][0].Name)
	}
}

func TestIsLabelLive(t *testing.T) {
	tests := []struct {
		name         string
		label        MachineLabel
		liveSessions map[string]bool
		livePIDs     []int
		want         bool
	}{
		{
			name:         "linked label with live session",
			label:        MachineLabel{Name: "a", SessionID: "s1"},
			liveSessions: map[string]bool{"s1": true},
			livePIDs:     nil,
			want:         true,
		},
		{
			name:         "linked label with dead session",
			label:        MachineLabel{Name: "a", SessionID: "gone"},
			liveSessions: map[string]bool{"s1": true},
			livePIDs:     nil,
			want:         false,
		},
		{
			name:         "orphan label with PID in livePIDs",
			label:        MachineLabel{Name: "a", SessionID: "", LastSeenPID: 4242},
			liveSessions: nil,
			livePIDs:     []int{1, 4242, 9999},
			want:         true,
		},
		{
			name:         "orphan label with PID NOT in livePIDs",
			label:        MachineLabel{Name: "a", SessionID: "", LastSeenPID: 4242},
			liveSessions: nil,
			livePIDs:     []int{1, 2, 3},
			want:         false,
		},
		{
			name:         "orphan label with LastSeenPID == 0",
			label:        MachineLabel{Name: "a", SessionID: "", LastSeenPID: 0},
			liveSessions: nil,
			livePIDs:     []int{1, 2, 3},
			want:         false,
		},
		{
			name:         "linked label still works with empty livePIDs",
			label:        MachineLabel{Name: "a", SessionID: "s1"},
			liveSessions: map[string]bool{"s1": true},
			livePIDs:     []int{},
			want:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsLabelLive(tc.label, tc.liveSessions, tc.livePIDs)
			if got != tc.want {
				t.Errorf("IsLabelLive() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClearLabels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	_ = AddLabel(path, "mm1", "a", "", 0)
	_ = AddLabel(path, "mm1", "b", "", 0)

	if err := ClearLabels(path, "mm1"); err != nil {
		t.Fatalf("ClearLabels() error: %v", err)
	}

	s, _ := LoadState(path)
	if len(s.MachineLabels["mm1"]) != 0 {
		t.Errorf("len = %d, want 0 after clear", len(s.MachineLabels["mm1"]))
	}
}
