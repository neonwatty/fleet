package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/tunnel"
)

func TestLaunchCleansWorktreeWhenSaveSessionFails(t *testing.T) {
	var commands []string
	opts := LaunchOpts{
		Project: "org/repo",
		Branch:  "feature/with space",
		Machine: config.Machine{
			Name:    "local",
			Host:    "localhost",
			Enabled: true,
		},
		Settings: config.Settings{
			WorktreeBase: "/tmp/fleet work",
			BareRepoBase: "/tmp/fleet repos",
			PortRange:    [2]int{4000, 4999},
		},
		StatePath: "/tmp/fleet state.json",
	}

	deps := launchDeps{
		run: func(_ context.Context, _ config.Machine, command string) (string, error) {
			commands = append(commands, command)
			return "", nil
		},
		loadState: func(string) (*State, error) {
			return &State{}, nil
		},
		addSession: func(string, Session) error {
			return errors.New("disk full")
		},
		startTunnel: func(config.Machine, int, int) (*tunnel.Tunnel, error) {
			t.Fatal("local launches should not start an SSH tunnel")
			return nil, nil
		},
		now: func() time.Time {
			return time.Unix(1700000000, 0)
		},
		pid: func() int {
			return 1234
		},
	}

	_, err := launchWithDeps(context.Background(), opts, deps)
	if err == nil || !strings.Contains(err.Error(), "save session") {
		t.Fatalf("launchWithDeps error = %v, want save session error", err)
	}

	wantWorktree := "git -C '/tmp/fleet repos/org/repo.git' worktree add '/tmp/fleet work/repo-1700000000' 'origin/feature/with space'"
	if !hasCommand(commands, wantWorktree) {
		t.Fatalf("missing quoted worktree command %q in commands:\n%s", wantWorktree, strings.Join(commands, "\n"))
	}

	wantRemove := "rm -rf -- '/tmp/fleet work/repo-1700000000'"
	if !hasCommand(commands, wantRemove) {
		t.Fatalf("missing cleanup rm command %q in commands:\n%s", wantRemove, strings.Join(commands, "\n"))
	}

	wantPrune := "git -C '/tmp/fleet repos/org/repo.git' worktree prune 2>/dev/null || true"
	if !hasCommand(commands, wantPrune) {
		t.Fatalf("missing cleanup prune command %q in commands:\n%s", wantPrune, strings.Join(commands, "\n"))
	}
}

func TestLaunchRecordsBareRepoPath(t *testing.T) {
	var saved Session
	opts := LaunchOpts{
		Project: "org/repo",
		Branch:  "main",
		Machine: config.Machine{
			Name:    "local",
			Host:    "localhost",
			Enabled: true,
		},
		Settings: config.Settings{
			WorktreeBase: "/tmp/fleet work",
			BareRepoBase: "/tmp/custom repos",
			PortRange:    [2]int{4000, 4999},
		},
		StatePath: "/tmp/fleet state.json",
	}

	deps := launchDeps{
		run: func(_ context.Context, _ config.Machine, _ string) (string, error) {
			return "", nil
		},
		loadState: func(string) (*State, error) {
			return &State{}, nil
		},
		addSession: func(_ string, sess Session) error {
			saved = sess
			return nil
		},
		startTunnel: func(config.Machine, int, int) (*tunnel.Tunnel, error) {
			t.Fatal("local launches should not start an SSH tunnel")
			return nil, nil
		},
		now: func() time.Time {
			return time.Unix(1700000000, 0)
		},
		pid: func() int {
			return 1234
		},
	}

	if _, err := launchWithDeps(context.Background(), opts, deps); err != nil {
		t.Fatalf("launchWithDeps error: %v", err)
	}

	if saved.BareRepoPath != "/tmp/custom repos/org/repo.git" {
		t.Errorf("BareRepoPath = %q, want custom bare repo path", saved.BareRepoPath)
	}
}

func hasCommand(commands []string, want string) bool {
	for _, got := range commands {
		if got == want {
			return true
		}
	}
	return false
}
