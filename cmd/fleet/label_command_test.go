package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/session"
)

func TestLabelCommandsSetListRemoveAndClear(t *testing.T) {
	configPath, statePath := commandTestPaths(t)
	if err := session.Save(statePath, &session.State{Sessions: []session.Session{{ID: "s1", Machine: "mm1"}}}); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	runRoot(t, configPath, statePath, "label", "set", "mm1", "bleep", "--session", "s1")
	out := captureStdout(t, func() {
		runRoot(t, configPath, statePath, "label", "list", "mm1")
	})
	if !strings.Contains(out, "mm1:") || !strings.Contains(out, "bleep  session=s1") {
		t.Fatalf("label list output missing linked label:\n%s", out)
	}

	runRoot(t, configPath, statePath, "label", "set", "mm1", "bleep", "--remove")
	out = captureStdout(t, func() {
		runRoot(t, configPath, statePath, "label", "list", "mm1")
	})
	if strings.Contains(out, "bleep") {
		t.Fatalf("removed label still listed:\n%s", out)
	}

	runRoot(t, configPath, statePath, "label", "set", "mm1", "one")
	runRoot(t, configPath, statePath, "label", "set", "mm1", "two")
	runRoot(t, configPath, statePath, "label", "set", "mm1", "--clear")
	loaded, err := session.LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(loaded.MachineLabels["mm1"]) != 0 {
		t.Fatalf("labels after clear = %+v, want none", loaded.MachineLabels["mm1"])
	}
}

func TestLabelCommandRejectsUnknownMachine(t *testing.T) {
	configPath, statePath := commandTestPaths(t)
	err := runRootErr(configPath, statePath, "label", "set", "missing", "bleep")
	if err == nil || !strings.Contains(err.Error(), "unknown machine") {
		t.Fatalf("Execute() error = %v, want unknown machine", err)
	}
}

func commandTestPaths(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	statePath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(configPath, []byte(testConfigTOML), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath, statePath
}

func runRoot(t *testing.T, configPath, statePath string, args ...string) {
	t.Helper()
	if err := runRootErr(configPath, statePath, args...); err != nil {
		t.Fatalf("Execute(%v) error: %v", args, err)
	}
}

func runRootErr(configPath, statePath string, args ...string) error {
	ctx := newCommandContext()
	root := newRootCommand(ctx)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	fullArgs := append([]string{"--config", configPath, "--state", statePath}, args...)
	root.SetArgs(fullArgs)
	return root.Execute()
}

const testConfigTOML = `
[settings]
port_range = [4000, 4999]
poll_interval = 5
stress_threshold = 20
worktree_base = "/tmp/fleet-work"
bare_repo_base = "/tmp/fleet-repos"

[[machines]]
name = "mm1"
host = "mm1"
user = "me"
enabled = true

[[machines]]
name = "mm2"
host = "mm2"
enabled = false
`
