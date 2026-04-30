package main

import (
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/session"
)

func TestAccountCommandClear(t *testing.T) {
	configPath, statePath := commandTestPaths(t)
	if err := session.Save(statePath, &session.State{
		Sessions: []session.Session{{ID: "s1", Machine: "mm1", Account: "personal"}},
	}); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	runRoot(t, configPath, statePath, "account", "s1", "--clear")

	loaded, err := session.LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if loaded.Sessions[0].Account != "" {
		t.Fatalf("Account = %q, want cleared", loaded.Sessions[0].Account)
	}
}

func TestAccountCommandValidatesClearAndNameArgs(t *testing.T) {
	configPath, statePath := commandTestPaths(t)

	err := runRootErr(configPath, statePath, "account", "s1", "personal", "--clear")
	if err == nil || !strings.Contains(err.Error(), "--clear takes no account name") {
		t.Fatalf("Execute() error = %v, want clear validation", err)
	}

	err = runRootErr(configPath, statePath, "account", "s1")
	if err == nil || !strings.Contains(err.Error(), "account name is required") {
		t.Fatalf("Execute() error = %v, want name required", err)
	}
}
