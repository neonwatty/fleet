package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigWithDefaultAccount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	toml := `
[settings]
port_range = [4000, 4999]
poll_interval = 5
stress_threshold = 20
worktree_base = "/tmp/fleet-work"
bare_repo_base = "/tmp/fleet-repos"

[[machines]]
name = "mm1"
host = "mm1"
user = "neonwatty"
enabled = true
default_account = "personal-max"

[[machines]]
name = "mm2"
host = "mm2"
user = "neonwatty"
enabled = true
`
	if err := os.WriteFile(path, []byte(toml), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Machines[0].DefaultAccount != "personal-max" {
		t.Errorf("mm1 DefaultAccount = %q, want personal-max", cfg.Machines[0].DefaultAccount)
	}
	if cfg.Machines[1].DefaultAccount != "" {
		t.Errorf("mm2 DefaultAccount = %q, want empty", cfg.Machines[1].DefaultAccount)
	}
}

func TestLoadConfigRejectsInvalidPortRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[settings]
port_range = [5000, 4000]
poll_interval = 5
worktree_base = "/tmp/fleet-work"
bare_repo_base = "/tmp/fleet-repos"

[[machines]]
name = "mm1"
host = "mm1"
enabled = true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "port_range") {
		t.Fatalf("Load() error = %v, want port_range validation error", err)
	}
}

func TestLoadConfigRejectsDuplicateMachineNames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[settings]
port_range = [4000, 4999]
poll_interval = 5
worktree_base = "/tmp/fleet-work"
bare_repo_base = "/tmp/fleet-repos"

[[machines]]
name = "mm1"
host = "mm1"
enabled = true

[[machines]]
name = "mm1"
host = "mm1-alt"
enabled = true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "duplicate machine name") {
		t.Fatalf("Load() error = %v, want duplicate machine validation error", err)
	}
}

func TestWriteDefaultCreatesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".fleet", "config.toml")

	if err := WriteDefault(path, false); err != nil {
		t.Fatalf("WriteDefault() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read default config: %v", err)
	}
	if !strings.Contains(string(data), "[[machines]]") {
		t.Fatalf("default config missing machines section:\n%s", data)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("Load(default config) error: %v", err)
	}
}

func TestWriteDefaultRefusesExistingConfigWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("existing"), 0644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	err := WriteDefault(path, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("WriteDefault() error = %v, want already exists error", err)
	}

	if err := WriteDefault(path, true); err != nil {
		t.Fatalf("WriteDefault(force) error: %v", err)
	}
}
