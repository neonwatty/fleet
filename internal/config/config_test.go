package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[settings]
port_range = [4000, 4999]
poll_interval = 5
stress_threshold = 20
worktree_base = "~/fleet-work"
bare_repo_base = "~/fleet-repos"

[[machines]]
name = "local"
host = "localhost"
user = ""
enabled = true

[[machines]]
name = "mm1"
host = "mm1"
user = "neonwatty"
enabled = true

[[machines]]
name = "mm2"
host = "mm2"
user = "jeremywatt"
enabled = false
`
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Settings.PortRange[0] != 4000 || cfg.Settings.PortRange[1] != 4999 {
		t.Errorf("PortRange = %v, want [4000, 4999]", cfg.Settings.PortRange)
	}
	if cfg.Settings.PollInterval != 5 {
		t.Errorf("PollInterval = %d, want 5", cfg.Settings.PollInterval)
	}
	if cfg.Settings.StressThreshold != 20 {
		t.Errorf("StressThreshold = %d, want 20", cfg.Settings.StressThreshold)
	}
	if len(cfg.Machines) != 3 {
		t.Fatalf("len(Machines) = %d, want 3", len(cfg.Machines))
	}
	if cfg.Machines[1].Name != "mm1" {
		t.Errorf("Machines[1].Name = %q, want %q", cfg.Machines[1].Name, "mm1")
	}
}

func TestEnabledMachines(t *testing.T) {
	cfg := &Config{
		Machines: []Machine{
			{Name: "local", Host: "localhost", Enabled: true},
			{Name: "mm1", Host: "mm1", Enabled: true},
			{Name: "mm2", Host: "mm2", Enabled: false},
		},
	}

	enabled := cfg.EnabledMachines()
	if len(enabled) != 2 {
		t.Fatalf("len(EnabledMachines) = %d, want 2", len(enabled))
	}
	if enabled[1].Name != "mm1" {
		t.Errorf("enabled[1].Name = %q, want %q", enabled[1].Name, "mm1")
	}
}

func TestIsLocal(t *testing.T) {
	local := Machine{Name: "local", Host: "localhost"}
	remote := Machine{Name: "mm1", Host: "mm1"}

	if !local.IsLocal() {
		t.Error("expected localhost to be local")
	}
	if remote.IsLocal() {
		t.Error("expected mm1 to not be local")
	}
}

func TestSSHTarget(t *testing.T) {
	tests := []struct {
		name string
		m    Machine
		want string
	}{
		{name: "host only", m: Machine{Host: "mm1"}, want: "mm1"},
		{name: "user host", m: Machine{Host: "mm1", User: "neonwatty"}, want: "neonwatty@mm1"},
		{name: "blank user", m: Machine{Host: "mm1", User: "  "}, want: "mm1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.SSHTarget(); got != tt.want {
				t.Fatalf("SSHTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := ExpandPath("~/fleet-work")
	expected := filepath.Join(home, "fleet-work")
	if result != expected {
		t.Errorf("ExpandPath(~/fleet-work) = %q, want %q", result, expected)
	}
}

func TestLoadConfigSwapHighMBDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Without swap_high_mb set: should default to 4096.
	content := `
[settings]
port_range = [4000, 4999]
poll_interval = 5
worktree_base = "/tmp/fleet-work"
bare_repo_base = "/tmp/fleet-repos"

[[machines]]
name = "mm1"
host = "mm1"
user = "neonwatty"
enabled = true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Settings.SwapHighMB != 4096 {
		t.Errorf("SwapHighMB = %d, want 4096 (default)", cfg.Settings.SwapHighMB)
	}
}

func TestLoadConfigSwapHighMBExplicit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[settings]
port_range = [4000, 4999]
poll_interval = 5
worktree_base = "/tmp/fleet-work"
bare_repo_base = "/tmp/fleet-repos"
swap_high_mb = 8192

[[machines]]
name = "mm1"
host = "mm1"
user = "neonwatty"
enabled = true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Settings.SwapHighMB != 8192 {
		t.Errorf("SwapHighMB = %d, want 8192", cfg.Settings.SwapHighMB)
	}
}

func TestLoadConfigSwapWarnMBDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Without swap_warn_mb set: should default to 1024.
	content := `
[settings]
port_range = [4000, 4999]
poll_interval = 5
worktree_base = "/tmp/fleet-work"
bare_repo_base = "/tmp/fleet-repos"

[[machines]]
name = "mm1"
host = "mm1"
user = "neonwatty"
enabled = true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Settings.SwapWarnMB != 1024 {
		t.Errorf("SwapWarnMB = %d, want 1024 (default)", cfg.Settings.SwapWarnMB)
	}
}

func TestLoadConfigSwapWarnMBExplicit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[settings]
port_range = [4000, 4999]
poll_interval = 5
worktree_base = "/tmp/fleet-work"
bare_repo_base = "/tmp/fleet-repos"
swap_warn_mb = 2048

[[machines]]
name = "mm1"
host = "mm1"
user = "neonwatty"
enabled = true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Settings.SwapWarnMB != 2048 {
		t.Errorf("SwapWarnMB = %d, want 2048", cfg.Settings.SwapWarnMB)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.toml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

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
