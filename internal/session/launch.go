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
	Project       string // "org/repo", a GitHub URL, or any git clone URL
	Branch        string
	Account       string
	LaunchCommand string
	Machine       config.Machine
	Settings      config.Settings
	StatePath     string
}

type LaunchResult struct {
	Session       Session
	Tunnel        *tunnel.Tunnel
	LaunchCommand string
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

	spec, err := parseProjectSpec(opts.Project)
	if err != nil {
		return nil, err
	}
	bareDir := filepath.Join(append([]string{opts.Settings.BareRepoBase}, append(spec.PathParts, spec.Repo+".git")...)...)
	timestamp := deps.now().Unix()
	worktreeDir := filepath.Join(opts.Settings.WorktreeBase, fmt.Sprintf("%s-%d", spec.Repo, timestamp))

	// Expand paths for remote machine
	remoteBare := expandRemotePath(bareDir, opts.Machine)
	remoteWork := expandRemotePath(worktreeDir, opts.Machine)

	// Step 1: Ensure bare clone exists
	checkCmd := fmt.Sprintf("test -d %s", shellQuotePath(remoteBare))
	if _, err := deps.run(ctx, opts.Machine, checkCmd); err != nil {
		mkdirCmd := fmt.Sprintf("mkdir -p %s && git clone --bare %s %s",
			shellQuotePath(filepath.Dir(remoteBare)), shellQuote(spec.CloneURL), shellQuotePath(remoteBare))
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

	// Step 4: Detect project config
	projectConfig := detectRemoteProjectConfig(ctx, opts.Machine, remoteWork, deps.run)
	launchCommand := resolveLaunchCommand(opts.LaunchCommand, projectConfig.LaunchCommand)

	// Step 5: Set up tunnel
	state, err := deps.loadState(opts.StatePath)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}
	usedPorts := state.UsedPorts()

	localPort, err := allocateLaunchPort(opts.Machine, opts.Settings, projectConfig.TunnelLocalPort, usedPorts)
	if err != nil {
		return nil, fmt.Errorf("allocate port: %w", err)
	}

	if !opts.Machine.IsLocal() && localPort > 0 {
		tun, err = deps.startTunnel(opts.Machine, localPort, projectConfig.DevPort)
		if err != nil {
			return nil, fmt.Errorf("tunnel: %w", err)
		}
	}

	// Step 6: Record session
	sess := Session{
		ID:            GenerateID(),
		Project:       opts.Project,
		Machine:       opts.Machine.Name,
		Branch:        opts.Branch,
		Account:       ResolveAccount(opts.Account, opts.Machine),
		LaunchCommand: launchCommand,
		WorktreePath:  remoteWork,
		BareRepoPath:  remoteBare,
		Tunnel:        TunnelInfo{LocalPort: localPort, RemotePort: projectConfig.DevPort},
		StartedAt:     deps.now().UTC(),
		OwnerPID:      deps.pid(),
	}

	if err := deps.addSession(opts.StatePath, sess); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	launchSucceeded = true
	return &LaunchResult{Session: sess, Tunnel: tun, LaunchCommand: launchCommand}, nil
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
	cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rmCmd := fmt.Sprintf("rm -rf -- %s", shellQuotePath(remoteWork))
	_, _ = run(cleanupCtx, m, rmCmd)

	if remoteBare != "" {
		pruneCmd := fmt.Sprintf("git -C %s worktree prune 2>/dev/null || true", shellQuotePath(remoteBare))
		_, _ = run(cleanupCtx, m, pruneCmd)
	}
}

func ExecClaude(m config.Machine, worktreePath string) error {
	return ExecCommand(m, worktreePath, "claude")
}

func ExecCommand(m config.Machine, worktreePath string, command string) error {
	command = resolveLaunchCommand(command, "")
	if m.IsLocal() {
		cmd := exec.Command("sh", "-lc", command)
		cmd.Dir = worktreePath
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	sshCmd := buildRemoteExecCommand(worktreePath, command)
	cmd := exec.Command("ssh", "-t", m.SSHTarget(), sshCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type projectSpec struct {
	CloneURL  string
	Repo      string
	PathParts []string
}

func buildRemoteExecCommand(worktreePath string, command string) string {
	return fmt.Sprintf("cd %s && %s", shellQuotePath(worktreePath), resolveLaunchCommand(command, ""))
}

func parseProjectSpec(project string) (projectSpec, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		return projectSpec{}, fmt.Errorf("project is required")
	}

	if strings.Contains(project, "://") || strings.HasPrefix(project, "git@") || strings.HasPrefix(project, "ssh://") {
		parts := pathPartsFromProject(project)
		if len(parts) == 0 {
			return projectSpec{}, fmt.Errorf("project URL must include a repository path")
		}
		repo := stripGitSuffix(parts[len(parts)-1])
		if repo == "" {
			return projectSpec{}, fmt.Errorf("project URL must include a repository name")
		}
		return projectSpec{
			CloneURL:  project,
			Repo:      repo,
			PathParts: parts[:len(parts)-1],
		}, nil
	}

	parts := splitCleanPath(project)
	if len(parts) < 2 {
		return projectSpec{}, fmt.Errorf("project must be org/repo or a git URL")
	}
	repo := parts[len(parts)-1]
	return projectSpec{
		CloneURL:  fmt.Sprintf("https://github.com/%s.git", project),
		Repo:      stripGitSuffix(repo),
		PathParts: parts[:len(parts)-1],
	}, nil
}

func splitProject(project string) (org, repo string) {
	spec, err := parseProjectSpec(project)
	if err != nil {
		return "", ""
	}
	if len(spec.PathParts) > 0 {
		return spec.PathParts[0], spec.Repo
	}
	return "", spec.Repo
}

func pathPartsFromProject(project string) []string {
	pathish := project
	if i := strings.Index(pathish, "://"); i >= 0 {
		pathish = pathish[i+3:]
		if slash := strings.Index(pathish, "/"); slash >= 0 {
			pathish = pathish[slash+1:]
		}
	} else if strings.HasPrefix(pathish, "git@") {
		if colon := strings.Index(pathish, ":"); colon >= 0 {
			pathish = pathish[colon+1:]
		}
	}
	parts := splitCleanPath(pathish)
	if len(parts) > 1 {
		parts[len(parts)-1] = stripGitSuffix(parts[len(parts)-1])
	}
	return parts
}

func splitCleanPath(path string) []string {
	raw := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return []string{"repo"}
	}
	return parts
}

func stripGitSuffix(repo string) string {
	return strings.TrimSuffix(repo, ".git")
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

func resolveLaunchCommand(explicit string, project string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	if strings.TrimSpace(project) != "" {
		return strings.TrimSpace(project)
	}
	return "claude"
}

func detectRemoteProjectConfig(
	ctx context.Context,
	m config.Machine,
	worktree string,
	run func(context.Context, config.Machine, string) (string, error),
) tunnel.ProjectConfig {
	// Try to read .fleet.toml from remote worktree
	catCmd := fmt.Sprintf("cat %s 2>/dev/null || true", shellQuotePath(filepath.Join(worktree, ".fleet.toml")))
	fleetToml, _ := run(ctx, m, catCmd)

	if strings.TrimSpace(fleetToml) != "" {
		tmpDir, err := os.MkdirTemp("", "fleet-detect-*")
		if err != nil {
			return tunnel.ProjectConfig{DevPort: 3000}
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck
		_ = os.WriteFile(filepath.Join(tmpDir, ".fleet.toml"), []byte(fleetToml), 0644)
		return tunnel.DetectProjectConfig(tmpDir)
	}

	// Try package.json
	catCmd = fmt.Sprintf("cat %s 2>/dev/null || true", shellQuotePath(filepath.Join(worktree, "package.json")))
	pkgJSON, _ := run(ctx, m, catCmd)
	if strings.Contains(pkgJSON, "scripts") {
		tmpDir, err := os.MkdirTemp("", "fleet-detect-*")
		if err != nil {
			return tunnel.ProjectConfig{DevPort: 3000}
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck
		_ = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644)
		return tunnel.DetectProjectConfig(tmpDir)
	}

	return tunnel.ProjectConfig{DevPort: 3000}
}
