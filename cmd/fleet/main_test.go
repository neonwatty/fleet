package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/session"
)

func TestInitCmdWritesConfigToCustomPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cmd := initCmd(newCommandContext())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--path", path})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(out.String(), "Wrote config") {
		t.Fatalf("output = %q, want success message", out.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "[[machines]]") {
		t.Fatalf("written config missing machines section:\n%s", data)
	}
}

func TestRootFlagsOverrideConfigAndStatePaths(t *testing.T) {
	ctx := newCommandContext()
	root := newRootCommand(ctx)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	statePath := filepath.Join(t.TempDir(), "state.json")
	root.SetArgs([]string{"--config", configPath, "--state", statePath, "init"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if ctx.configPath != configPath {
		t.Fatalf("configPath = %q, want %q", ctx.configPath, configPath)
	}
	if ctx.statePath != statePath {
		t.Fatalf("statePath = %q, want %q", ctx.statePath, statePath)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config not written to override path: %v", err)
	}
}

func TestRootStateFlagUsedByAccountCommand(t *testing.T) {
	ctx := newCommandContext()
	root := newRootCommand(ctx)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := session.Save(statePath, &session.State{Sessions: []session.Session{{ID: "s1"}}}); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	root.SetArgs([]string{"--state", statePath, "account", "s1", "personal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	loaded, err := session.LoadState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if loaded.Sessions[0].Account != "personal" {
		t.Fatalf("Account = %q, want personal", loaded.Sessions[0].Account)
	}
}

func TestInitCmdRefusesExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("existing"), 0644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	cmd := initCmd(newCommandContext())
	cmd.SetArgs([]string{"--path", path})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Execute() error = %v, want already exists", err)
	}
}

func TestLaunchCmdHelpDocumentsGitURLAndCommandOverride(t *testing.T) {
	cmd := launchCmd(newCommandContext())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	help := out.String()
	for _, want := range []string{
		"launch <org/repo|git-url>",
		"--cmd string",
		"launch_command",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
	}
}

func TestDoctorCmdHelpDocumentsFix(t *testing.T) {
	cmd := doctorCmd(newCommandContext())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	help := out.String()
	for _, want := range []string{
		"--fix",
		"Create missing configured base directories",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
	}
}

func TestVersionStringDefaultsToVersionOnly(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	t.Cleanup(func() {
		version, commit, date = oldVersion, oldCommit, oldDate
	})
	version, commit, date = "dev", "none", "unknown"

	if got := versionString(); got != "dev" {
		t.Fatalf("versionString() = %q, want dev", got)
	}
}

func TestVersionStringIncludesBuildMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	t.Cleanup(func() {
		version, commit, date = oldVersion, oldCommit, oldDate
	})
	version, commit, date = "1.2.3", "abc123", "2026-04-29T15:00:00Z"

	want := "1.2.3 (commit abc123, built 2026-04-29T15:00:00Z)"
	if got := versionString(); got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}
