package session

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/config"
)

func TestValidateWorktreeDeletePath(t *testing.T) {
	base := "/tmp/fleet-work"
	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{name: "child", target: "/tmp/fleet-work/repo-123", wantErr: false},
		{name: "nested child", target: "/tmp/fleet-work/org/repo-123", wantErr: false},
		{name: "base", target: "/tmp/fleet-work", wantErr: true},
		{name: "outside sibling", target: "/tmp/fleet-work-other/repo", wantErr: true},
		{name: "parent escape", target: "/tmp/fleet-work/../other", wantErr: true},
		{name: "root", target: "/", wantErr: true},
		{name: "empty", target: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorktreeDeletePath(base, tt.target)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateWorktreeDeletePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWorktreeDeletePathAcceptsRemoteTildeTargets(t *testing.T) {
	if err := validateWorktreeDeletePath("~/fleet-work", "~/fleet-work/repo-123"); err != nil {
		t.Fatalf("validateWorktreeDeletePath() error = %v, want remote tilde target accepted", err)
	}
}

func TestValidateWorktreeDeletePathExpandsBaseForLocalAbsoluteTargets(t *testing.T) {
	target := filepath.Join(config.ExpandPath("~/fleet-work"), "repo-123")
	if err := validateWorktreeDeletePath("~/fleet-work", target); err != nil {
		t.Fatalf("validateWorktreeDeletePath() error = %v, want local expanded target accepted", err)
	}
}

func TestCleanKeepsOrphanWhenDeletePathUnsafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	state := &State{
		Sessions: []Session{{
			ID:           "unsafe",
			Project:      "org/repo",
			Machine:      "local",
			WorktreePath: "/tmp/not-fleet/repo",
			OwnerPID:     1 << 30,
		}},
	}
	if err := Save(path, state); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	cfg := &config.Config{
		Settings: config.Settings{WorktreeBase: "/tmp/fleet-work"},
		Machines: []config.Machine{{Name: "local", Host: "localhost"}},
	}
	result, err := CleanWithOptions(context.Background(), cfg, path, CleanOptions{})
	if err == nil || !strings.Contains(err.Error(), "outside worktree base") {
		t.Fatalf("CleanWithOptions() error = %v, want outside-base error", err)
	}
	if len(result.Failed) != 1 {
		t.Fatalf("len(Failed) = %d, want 1", len(result.Failed))
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if len(loaded.Sessions) != 1 || loaded.Sessions[0].ID != "unsafe" {
		t.Fatalf("loaded sessions = %+v, want unsafe session preserved", loaded.Sessions)
	}
}
