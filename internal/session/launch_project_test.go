package session

import (
	"context"
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/config"
)

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
