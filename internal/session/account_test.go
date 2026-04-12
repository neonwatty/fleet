package session

import (
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
