package tui

import (
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/session"
)

// newRenameTestModel constructs a minimal model wired up for the rename-mode
// tests: one machine in config, one in-memory session, sessions panel active.
// The state file at statePath is NOT created automatically — tests that need
// it on disk should seed it themselves via session.AddSession.
func newRenameTestModel(t *testing.T, statePath string) model {
	t.Helper()
	cfg := &config.Config{
		Settings: config.Settings{PollInterval: 5},
		Machines: []config.Machine{{Name: "mm1", Host: "mm1", Enabled: true}},
	}
	m := NewModel(cfg, statePath)
	m.state = &session.State{
		Sessions: []session.Session{
			{
				ID:        "abcd1234",
				Project:   "neonwatty/bleep",
				Machine:   "mm1",
				Branch:    "main",
				StartedAt: time.Now(),
			},
		},
	}
	m.activePanel = panelSessions
	m.selectedRow = 0
	return m
}

// updateModel feeds a tea.KeyMsg through model.Update and returns the resulting
// model. The returned tea.Cmd is intentionally discarded — Update may return a
// refresh command (which would shell out via SSH if executed), but we never
// invoke it in tests.
func updateModel(t *testing.T, m model, msg tea.KeyMsg) model {
	t.Helper()
	result, _ := m.Update(msg)
	next, ok := result.(model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T, want tui.model", result)
	}
	return next
}

func TestRenameKeyEntersRenameMode(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	m := newRenameTestModel(t, statePath)

	if m.renaming {
		t.Fatalf("precondition: model should not start in rename mode")
	}

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if !m.renaming {
		t.Errorf("expected renaming = true after pressing 'n', got false")
	}
	if m.renameBuffer != "" {
		t.Errorf("expected empty renameBuffer after entering rename mode, got %q", m.renameBuffer)
	}
}

func TestRenameTypeAndEnterCommitsLabel(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	// Seed a real state file with a session — AddLabel reads from disk.
	sess := session.Session{
		ID:        "abcd1234",
		Project:   "neonwatty/bleep",
		Machine:   "mm1",
		Branch:    "main",
		StartedAt: time.Now(),
	}
	if err := session.AddSession(statePath, sess); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	m := newRenameTestModel(t, statePath)
	// Force rename mode directly (scenario 1 already proves the entry path).
	m.renaming = true
	m.renameBuffer = ""

	for _, r := range "bleep" {
		m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.renameBuffer != "bleep" {
		t.Fatalf("expected renameBuffer = %q after typing, got %q", "bleep", m.renameBuffer)
	}

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.renaming {
		t.Errorf("expected renaming = false after Enter commits, got true")
	}
	if m.renameBuffer != "" {
		t.Errorf("expected empty renameBuffer after Enter commits, got %q", m.renameBuffer)
	}

	// Verify the label was actually written through to disk.
	loaded, err := session.LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	labels := loaded.MachineLabels["mm1"]
	if len(labels) != 1 {
		t.Fatalf("expected 1 label on mm1 after Enter commit, got %d (%+v)", len(labels), labels)
	}
	if labels[0].Name != "bleep" {
		t.Errorf("expected label Name = %q, got %q", "bleep", labels[0].Name)
	}
	if labels[0].SessionID != sess.ID {
		t.Errorf("expected label SessionID = %q, got %q", sess.ID, labels[0].SessionID)
	}
}

func TestRenameEscCancelsWithoutWriting(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	// Seed an empty state file with the session — Esc must NOT mutate it.
	sess := session.Session{
		ID:        "abcd1234",
		Project:   "neonwatty/bleep",
		Machine:   "mm1",
		Branch:    "main",
		StartedAt: time.Now(),
	}
	if err := session.AddSession(statePath, sess); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	m := newRenameTestModel(t, statePath)
	m.renaming = true
	m.renameBuffer = "bl"

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.renaming {
		t.Errorf("expected renaming = false after Esc, got true")
	}
	if m.renameBuffer != "" {
		t.Errorf("expected empty renameBuffer after Esc, got %q", m.renameBuffer)
	}

	loaded, err := session.LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if labels := loaded.MachineLabels["mm1"]; len(labels) != 0 {
		t.Errorf("expected no labels on mm1 after Esc, got %d (%+v)", len(labels), labels)
	}
}
