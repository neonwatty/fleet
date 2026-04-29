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
			if strings.HasPrefix(command, "test -d ") {
				return "", errors.New("missing")
			}
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

func TestLaunchAcceptsGitHubURL(t *testing.T) {
	var commands []string
	var saved Session
	opts := LaunchOpts{
		Project: "https://github.com/acme/widget.git",
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
			if strings.HasPrefix(command, "test -d ") {
				return "", errors.New("missing")
			}
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

	wantClone := "mkdir -p '/tmp/fleet repos/acme' && git clone --bare 'https://github.com/acme/widget.git' '/tmp/fleet repos/acme/widget.git'"
	if !hasCommand(commands, wantClone) {
		t.Fatalf("missing clone command %q in commands:\n%s", wantClone, strings.Join(commands, "\n"))
	}
	if saved.WorktreePath != "/tmp/fleet work/widget-1700000000" {
		t.Errorf("WorktreePath = %q, want repo-derived worktree path", saved.WorktreePath)
	}
	if saved.BareRepoPath != "/tmp/fleet repos/acme/widget.git" {
		t.Errorf("BareRepoPath = %q, want URL-derived bare repo path", saved.BareRepoPath)
	}
}

func TestLaunchUsesProjectLaunchCommand(t *testing.T) {
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

	result, err := launchWithDeps(context.Background(), opts, projectCommandLaunchDeps(t))
	if err != nil {
		t.Fatalf("launchWithDeps error: %v", err)
	}
	if result.LaunchCommand != "npm run agent" {
		t.Errorf("LaunchCommand = %q, want project command", result.LaunchCommand)
	}
	if result.Session.LaunchCommand != "npm run agent" {
		t.Errorf("Session.LaunchCommand = %q, want project command", result.Session.LaunchCommand)
	}
}

func TestLaunchCommandFlagOverridesProjectLaunchCommand(t *testing.T) {
	opts := LaunchOpts{
		Project:       "org/repo",
		Branch:        "main",
		LaunchCommand: "claude --resume",
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

	result, err := launchWithDeps(context.Background(), opts, projectCommandLaunchDeps(t))
	if err != nil {
		t.Fatalf("launchWithDeps error: %v", err)
	}
	if result.LaunchCommand != "claude --resume" {
		t.Errorf("LaunchCommand = %q, want explicit command", result.LaunchCommand)
	}
}

func TestLaunchRejectsInvalidProject(t *testing.T) {
	opts := LaunchOpts{
		Project: "repo-only",
		Branch:  "main",
		Machine: config.Machine{Name: "local", Host: "localhost", Enabled: true},
		Settings: config.Settings{
			WorktreeBase: "/tmp/fleet work",
			BareRepoBase: "/tmp/fleet repos",
			PortRange:    [2]int{4000, 4999},
		},
		StatePath: "/tmp/fleet state.json",
	}

	_, err := launchWithDeps(context.Background(), opts, projectCommandLaunchDeps(t))
	if err == nil || !strings.Contains(err.Error(), "org/repo") {
		t.Fatalf("launchWithDeps error = %v, want invalid project error", err)
	}
}

func TestResolveLaunchCommandPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		project  string
		want     string
	}{
		{name: "explicit wins", explicit: " claude --resume ", project: "npm run agent", want: "claude --resume"},
		{name: "project fallback", project: " npm run agent ", want: "npm run agent"},
		{name: "default", want: "claude"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveLaunchCommand(tt.explicit, tt.project); got != tt.want {
				t.Fatalf("resolveLaunchCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildRemoteExecCommandQuotesWorktree(t *testing.T) {
	got := buildRemoteExecCommand("~/fleet work/repo's", "claude --resume")
	want := "cd ~/'fleet work/repo'\\''s' && claude --resume"
	if got != want {
		t.Fatalf("buildRemoteExecCommand() = %q, want %q", got, want)
	}
}

func projectCommandLaunchDeps(t *testing.T) launchDeps {
	t.Helper()
	return launchDeps{
		run: func(_ context.Context, _ config.Machine, command string) (string, error) {
			if strings.HasSuffix(command, ".fleet.toml' 2>/dev/null || true") {
				return "launch_command = \"npm run agent\"\n", nil
			}
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
}

func TestParseProjectSpecSSHURL(t *testing.T) {
	spec, err := parseProjectSpec("git@github.com:acme/widget.git")
	if err != nil {
		t.Fatalf("parseProjectSpec() error: %v", err)
	}

	if spec.CloneURL != "git@github.com:acme/widget.git" {
		t.Errorf("CloneURL = %q", spec.CloneURL)
	}
	if spec.Repo != "widget" {
		t.Errorf("Repo = %q, want widget", spec.Repo)
	}
	if got := strings.Join(spec.PathParts, "/"); got != "acme" {
		t.Errorf("PathParts = %q, want acme", got)
	}
}

func TestParseProjectSpecRejectsRepoOnly(t *testing.T) {
	_, err := parseProjectSpec("widget")
	if err == nil || !strings.Contains(err.Error(), "org/repo") {
		t.Fatalf("parseProjectSpec() error = %v, want org/repo error", err)
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
