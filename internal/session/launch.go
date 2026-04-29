package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	fleetexec "github.com/neonwatty/fleet/internal/exec"
	"github.com/neonwatty/fleet/internal/tunnel"
)

type LaunchOpts struct {
	Project   string // "org/repo"
	Branch    string
	Account   string
	Machine   config.Machine
	Settings  config.Settings
	StatePath string
}

type LaunchResult struct {
	Session Session
	Tunnel  *tunnel.Tunnel
}

type launchDeps struct {
	run         func(context.Context, config.Machine, string) (string, error)
	loadState   func(string) (*State, error)
	addSession  func(string, Session) error
	startTunnel func(config.Machine, int, int) (*tunnel.Tunnel, error)
	now         func() time.Time
	pid         func() int
}

func defaultLaunchDeps() launchDeps {
	return launchDeps{
		run:         fleetexec.Run,
		loadState:   LoadState,
		addSession:  AddSession,
		startTunnel: tunnel.Start,
		now:         time.Now,
		pid:         os.Getpid,
	}
}

func Launch(ctx context.Context, opts LaunchOpts) (*LaunchResult, error) {
	return launchWithDeps(ctx, opts, defaultLaunchDeps())
}

func launchWithDeps(ctx context.Context, opts LaunchOpts, deps launchDeps) (*LaunchResult, error) {
	if opts.Branch == "" {
		opts.Branch = "main"
	}

	org, repo := splitProject(opts.Project)
	bareDir := filepath.Join(opts.Settings.BareRepoBase, org, repo+".git")
	timestamp := deps.now().Unix()
	worktreeDir := filepath.Join(opts.Settings.WorktreeBase, fmt.Sprintf("%s-%d", repo, timestamp))

	// Expand paths for remote machine
	remoteBare := expandRemotePath(bareDir, opts.Machine)
	remoteWork := expandRemotePath(worktreeDir, opts.Machine)

	// Step 1: Ensure bare clone exists
	checkCmd := fmt.Sprintf("test -d %s", shellQuotePath(remoteBare))
	if _, err := deps.run(ctx, opts.Machine, checkCmd); err != nil {
		cloneURL := fmt.Sprintf("https://github.com/%s.git", opts.Project)
		mkdirCmd := fmt.Sprintf("mkdir -p %s && git clone --bare %s %s",
			shellQuotePath(filepath.Dir(remoteBare)), shellQuote(cloneURL), shellQuotePath(remoteBare))
		if _, err := deps.run(ctx, opts.Machine, mkdirCmd); err != nil {
			return nil, fmt.Errorf("bare clone: %w", err)
		}
	}

	// Step 2: Fetch latest
	fetchCmd := fmt.Sprintf("git -C %s fetch origin", shellQuotePath(remoteBare))
	if _, err := deps.run(ctx, opts.Machine, fetchCmd); err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	// Step 3: Create worktree
	worktreeCmd := fmt.Sprintf("git -C %s worktree add %s %s",
		shellQuotePath(remoteBare), shellQuotePath(remoteWork), shellQuote("origin/"+opts.Branch))
	if _, err := deps.run(ctx, opts.Machine, worktreeCmd); err != nil {
		return nil, fmt.Errorf("worktree: %w", err)
	}
	worktreeCreated := true
	var tun *tunnel.Tunnel
	launchSucceeded := false
	defer func() {
		if launchSucceeded {
			return
		}
		cleanupFailedLaunch(ctx, opts.Machine, remoteBare, remoteWork, worktreeCreated, tun, deps.run)
	}()

	// Step 4: Detect dev server port
	devPort, pinnedLocal := detectRemotePorts(ctx, opts.Machine, remoteWork, deps.run)

	// Step 5: Set up tunnel
	state, err := deps.loadState(opts.StatePath)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}
	usedPorts := state.UsedPorts()

	localPort, err := allocateLaunchPort(opts.Machine, opts.Settings, pinnedLocal, usedPorts)
	if err != nil {
		return nil, fmt.Errorf("allocate port: %w", err)
	}

	if !opts.Machine.IsLocal() && localPort > 0 {
		tun, err = deps.startTunnel(opts.Machine, localPort, devPort)
		if err != nil {
			return nil, fmt.Errorf("tunnel: %w", err)
		}
	}

	// Step 6: Record session
	sess := Session{
		ID:           GenerateID(),
		Project:      opts.Project,
		Machine:      opts.Machine.Name,
		Branch:       opts.Branch,
		Account:      ResolveAccount(opts.Account, opts.Machine),
		WorktreePath: remoteWork,
		BareRepoPath: remoteBare,
		Tunnel:       TunnelInfo{LocalPort: localPort, RemotePort: devPort},
		StartedAt:    deps.now().UTC(),
		OwnerPID:     deps.pid(),
	}

	if err := deps.addSession(opts.StatePath, sess); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	launchSucceeded = true
	return &LaunchResult{Session: sess, Tunnel: tun}, nil
}

func allocateLaunchPort(
	m config.Machine,
	settings config.Settings,
	pinnedLocal int,
	usedPorts map[int]bool,
) (int, error) {
	if pinnedLocal > 0 {
		localPort, err := tunnel.AllocatePortPinned(pinnedLocal, usedPorts)
		if err == nil {
			return localPort, nil
		}
		fmt.Fprintf(os.Stderr, "Warning: %v. Auto-assigning port.\n", err)
		return tunnel.AllocatePort(settings.PortRange[0], settings.PortRange[1], usedPorts)
	}
	if !m.IsLocal() {
		return tunnel.AllocatePort(settings.PortRange[0], settings.PortRange[1], usedPorts)
	}
	return 0, nil
}

func cleanupFailedLaunch(
	ctx context.Context,
	m config.Machine,
	remoteBare string,
	remoteWork string,
	worktreeCreated bool,
	tun *tunnel.Tunnel,
	run func(context.Context, config.Machine, string) (string, error),
) {
	if tun != nil {
		_ = tun.Stop()
	}
	if !worktreeCreated || remoteWork == "" {
		return
	}
	rmCmd := fmt.Sprintf("rm -rf -- %s", shellQuotePath(remoteWork))
	_, _ = run(ctx, m, rmCmd)

	if remoteBare != "" {
		pruneCmd := fmt.Sprintf("git -C %s worktree prune 2>/dev/null || true", shellQuotePath(remoteBare))
		_, _ = run(ctx, m, pruneCmd)
	}
}

func ExecClaude(m config.Machine, worktreePath string) error {
	if m.IsLocal() {
		cmd := exec.Command("claude")
		cmd.Dir = worktreePath
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	sshCmd := fmt.Sprintf("cd %s && claude", shellQuotePath(worktreePath))
	cmd := exec.Command("ssh", "-t", m.Host, sshCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func splitProject(project string) (org, repo string) {
	parts := strings.SplitN(project, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}

func expandRemotePath(path string, m config.Machine) string {
	if m.IsLocal() {
		return config.ExpandPath(path)
	}
	if strings.HasPrefix(path, "~/") {
		return path // SSH expands ~ on the remote side
	}
	return path
}

func detectRemotePorts(
	ctx context.Context,
	m config.Machine,
	worktree string,
	run func(context.Context, config.Machine, string) (string, error),
) (int, int) {
	// Try to read .fleet.toml from remote worktree
	catCmd := fmt.Sprintf("cat %s 2>/dev/null || true", shellQuotePath(filepath.Join(worktree, ".fleet.toml")))
	fleetToml, _ := run(ctx, m, catCmd)

	if strings.Contains(fleetToml, "dev_port") {
		tmpDir, err := os.MkdirTemp("", "fleet-detect-*")
		if err != nil {
			return 3000, 0
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck
		_ = os.WriteFile(filepath.Join(tmpDir, ".fleet.toml"), []byte(fleetToml), 0644)
		return tunnel.DetectPorts(tmpDir)
	}

	// Try package.json
	catCmd = fmt.Sprintf("cat %s 2>/dev/null || true", shellQuotePath(filepath.Join(worktree, "package.json")))
	pkgJSON, _ := run(ctx, m, catCmd)
	if strings.Contains(pkgJSON, "scripts") {
		tmpDir, err := os.MkdirTemp("", "fleet-detect-*")
		if err != nil {
			return 3000, 0
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck
		_ = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644)
		return tunnel.DetectPorts(tmpDir)
	}

	return 3000, 0
}
