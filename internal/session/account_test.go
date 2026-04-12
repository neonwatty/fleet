package session

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/config"
)

func TestResolveAccountExplicitWins(t *testing.T) {
	m := config.Machine{Name: "mm1", DefaultAccount: "personal-max"}
	got := ResolveAccount("work-team", m)
	if got != "work-team" {
		t.Errorf("ResolveAccount(explicit) = %q, want work-team", got)
	}
}

func TestResolveAccountFallsBackToMachineDefault(t *testing.T) {
	m := config.Machine{Name: "mm1", DefaultAccount: "personal-max"}
	got := ResolveAccount("", m)
	if got != "personal-max" {
		t.Errorf("ResolveAccount(fallback) = %q, want personal-max", got)
	}
}

func TestResolveAccountEmptyWhenNoneSet(t *testing.T) {
	m := config.Machine{Name: "mm1"}
	got := ResolveAccount("", m)
	if got != "" {
		t.Errorf("ResolveAccount(none) = %q, want empty", got)
	}
}

func TestSetSessionAccountHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	sess := Session{
		ID:      "abc123",
		Project: "neonwatty/bleep",
		Machine: "mm1",
		Branch:  "main",
		Account: "",
	}
	if err := AddSession(path, sess); err != nil {
		t.Fatalf("AddSession() error: %v", err)
	}

	if err := SetSessionAccount(path, "abc123", "personal-max"); err != nil {
		t.Fatalf("SetSessionAccount() error: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if loaded.Sessions[0].Account != "personal-max" {
		t.Errorf("Account = %q, want personal-max", loaded.Sessions[0].Account)
	}

	if err := SetSessionAccount(path, "abc123", ""); err != nil {
		t.Fatalf("SetSessionAccount(clear) error: %v", err)
	}
	loaded2, _ := LoadState(path)
	if loaded2.Sessions[0].Account != "" {
		t.Errorf("after clear, Account = %q, want empty", loaded2.Sessions[0].Account)
	}
}

func TestSetSessionAccountNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	sess := Session{ID: "exists", Project: "p", Machine: "m", Branch: "main"}
	if err := AddSession(path, sess); err != nil {
		t.Fatalf("AddSession() error: %v", err)
	}

	err := SetSessionAccount(path, "does-not-exist", "personal-max")
	if err == nil {
		t.Fatal("expected error for missing session ID, got nil")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error %q should mention session ID", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q should contain 'not found'", err.Error())
	}
}
