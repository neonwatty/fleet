package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/tunnel"
)

func TestLaunchFetchesRemoteTrackingRefsForBareRepo(t *testing.T) {
	var commands []string
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

	wantFetch := "git -C '/tmp/fleet repos/org/repo.git' fetch --prune origin '+refs/heads/*:refs/remotes/origin/*'"
	if !hasCommand(commands, wantFetch) {
		t.Fatalf("missing fetch command %q in commands:\n%s", wantFetch, strings.Join(commands, "\n"))
	}
}
