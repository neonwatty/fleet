package session

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/tunnel"
)

func TestLaunchLifecycleSavesLockedStateAndStartsTunnel(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	var commands []string
	var tunnelMachine config.Machine
	var tunnelLocal, tunnelRemote int

	result, err := launchWithDeps(context.Background(), lifecycleOpts(statePath), launchDeps{
		run: func(_ context.Context, _ config.Machine, command string) (string, error) {
			commands = append(commands, command)
			if strings.HasPrefix(command, "test -d ") {
				return "", errors.New("missing")
			}
			return "", nil
		},
		withStateLock: WithStateLock,
		startTunnel: func(m config.Machine, localPort, remotePort int) (*tunnel.Tunnel, error) {
			tunnelMachine = m
			tunnelLocal = localPort
			tunnelRemote = remotePort
			return &tunnel.Tunnel{LocalPort: localPort, RemotePort: remotePort, Machine: m}, nil
		},
		now: func() time.Time { return time.Unix(1700000000, 0) },
		pid: func() int { return 4321 },
	})
	if err != nil {
		t.Fatalf("launchWithDeps error: %v", err)
	}

	if result.Session.ID == "" || result.Session.OwnerPID != 4321 {
		t.Fatalf("unexpected session metadata: %+v", result.Session)
	}
	if tunnelMachine.Name != "mm1" || tunnelLocal == 0 || tunnelRemote != 3000 {
		t.Fatalf("unexpected tunnel start: machine=%+v local=%d remote=%d", tunnelMachine, tunnelLocal, tunnelRemote)
	}

	state, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(state.Sessions) != 1 {
		t.Fatalf("saved sessions = %d, want 1", len(state.Sessions))
	}
	if state.Sessions[0].Tunnel.LocalPort != tunnelLocal {
		t.Fatalf("saved local port = %d, want %d", state.Sessions[0].Tunnel.LocalPort, tunnelLocal)
	}

	for _, want := range []string{
		"mkdir -p '/tmp/fleet-repos/org' && git clone --bare 'https://github.com/org/repo.git' '/tmp/fleet-repos/org/repo.git'",
		"git -C '/tmp/fleet-repos/org/repo.git' fetch --prune origin '+refs/heads/*:refs/remotes/origin/*'",
		"git -C '/tmp/fleet-repos/org/repo.git' worktree add '/tmp/fleet-work/repo-1700000000' 'origin/main'",
	} {
		if !hasCommand(commands, want) {
			t.Fatalf("missing command %q in:\n%s", want, strings.Join(commands, "\n"))
		}
	}
}

func TestLaunchLifecycleTunnelFailureCleansWorktreeWithoutSavingState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	var commands []string

	_, err := launchWithDeps(context.Background(), lifecycleOpts(statePath), launchDeps{
		run: func(_ context.Context, _ config.Machine, command string) (string, error) {
			commands = append(commands, command)
			if strings.HasPrefix(command, "test -d ") {
				return "", errors.New("missing")
			}
			return "", nil
		},
		withStateLock: WithStateLock,
		startTunnel: func(config.Machine, int, int) (*tunnel.Tunnel, error) {
			return nil, errors.New("forward failed")
		},
		now: func() time.Time { return time.Unix(1700000000, 0) },
		pid: func() int { return 4321 },
	})
	if err == nil || !strings.Contains(err.Error(), "forward failed") {
		t.Fatalf("launchWithDeps error = %v, want tunnel failure", err)
	}

	state, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(state.Sessions) != 0 {
		t.Fatalf("saved sessions = %d, want 0", len(state.Sessions))
	}
	for _, want := range []string{
		"rm -rf -- '/tmp/fleet-work/repo-1700000000'",
		"git -C '/tmp/fleet-repos/org/repo.git' worktree prune 2>/dev/null || true",
	} {
		if !hasCommand(commands, want) {
			t.Fatalf("missing cleanup command %q in:\n%s", want, strings.Join(commands, "\n"))
		}
	}
}

func lifecycleOpts(statePath string) LaunchOpts {
	return LaunchOpts{
		Project: "org/repo",
		Branch:  "main",
		Machine: config.Machine{Name: "mm1", Host: "mm1", User: "user", Enabled: true},
		Settings: config.Settings{
			WorktreeBase: "/tmp/fleet-work",
			BareRepoBase: "/tmp/fleet-repos",
			PortRange:    [2]int{43000, 43100},
		},
		StatePath: statePath,
	}
}
