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
