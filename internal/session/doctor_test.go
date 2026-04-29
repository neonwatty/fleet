package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/neonwatty/fleet/internal/config"
)

func TestDoctorOKForLocalMachineWithExistingBaseDirs(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	if err := Save(statePath, &State{}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	cfg := &config.Config{
		Settings: config.Settings{
			WorktreeBase: filepath.Join(dir, "worktrees"),
			BareRepoBase: filepath.Join(dir, "repos"),
		},
		Machines: []config.Machine{{Name: "local", Host: "localhost", Enabled: true}},
	}
	if err := mkdirAll(cfg.Settings.WorktreeBase, cfg.Settings.BareRepoBase); err != nil {
		t.Fatalf("mkdir test dirs: %v", err)
	}

	result, err := Doctor(context.Background(), cfg, statePath, DoctorOptions{})
	if err != nil {
		t.Fatalf("Doctor() error: %v", err)
	}
	if !result.OK() {
		t.Fatalf("Doctor OK = false, result = %+v", result)
	}
	if result.CheckedMachines != 1 {
		t.Fatalf("CheckedMachines = %d, want 1", result.CheckedMachines)
	}
}

func TestDoctorReportsMissingBaseDirs(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	if err := Save(statePath, &State{}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	cfg := &config.Config{
		Settings: config.Settings{
			WorktreeBase: filepath.Join(dir, "missing-worktrees"),
			BareRepoBase: filepath.Join(dir, "missing-repos"),
		},
		Machines: []config.Machine{{Name: "local", Host: "localhost", Enabled: true}},
	}

	result, err := Doctor(context.Background(), cfg, statePath, DoctorOptions{Machine: "local"})
	if err != nil {
		t.Fatalf("Doctor() error: %v", err)
	}
	if result.OK() {
		t.Fatalf("Doctor OK = true, want missing dirs issue")
	}
	if len(result.Issues) != 2 {
		t.Fatalf("Issues = %v, want 2 missing dir issues", result.Issues)
	}
}

func TestDoctorFixCreatesMissingBaseDirs(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	if err := Save(statePath, &State{}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	cfg := &config.Config{
		Settings: config.Settings{
			WorktreeBase: filepath.Join(dir, "missing-worktrees"),
			BareRepoBase: filepath.Join(dir, "missing-repos"),
		},
		Machines: []config.Machine{{Name: "local", Host: "localhost", Enabled: true}},
	}

	result, err := Doctor(context.Background(), cfg, statePath, DoctorOptions{Machine: "local", Fix: true})
	if err != nil {
		t.Fatalf("Doctor() error: %v", err)
	}
	if !result.OK() {
		t.Fatalf("Doctor OK = false after fix, result = %+v", result)
	}
	if result.Fixed != 2 {
		t.Fatalf("Fixed = %d, want 2", result.Fixed)
	}
	for _, path := range []string{cfg.Settings.WorktreeBase, cfg.Settings.BareRepoBase} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat fixed path %s: %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("fixed path %s is not a directory", path)
		}
	}
}

func TestDoctorUnknownMachine(t *testing.T) {
	cfg := &config.Config{
		Machines: []config.Machine{{Name: "local", Host: "localhost", Enabled: true}},
	}

	_, err := Doctor(context.Background(), cfg, filepath.Join(t.TempDir(), "state.json"), DoctorOptions{Machine: "mm1"})
	if err == nil {
		t.Fatal("Doctor() error = nil, want unknown machine error")
	}
}

func TestDoctorSSHRemediation(t *testing.T) {
	m := config.Machine{Name: "mm0", Host: "mm0", User: "jeremywatt"}
	want := `fix SSH for jeremywatt@mm0 or set enabled = false for "mm0" in config.toml`
	if got := doctorSSHRemediation(m); got != want {
		t.Fatalf("doctorSSHRemediation() = %q, want %q", got, want)
	}
}

func mkdirAll(paths ...string) error {
	for _, path := range paths {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}
