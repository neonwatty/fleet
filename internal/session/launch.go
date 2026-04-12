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

func Launch(ctx context.Context, opts LaunchOpts) (*LaunchResult, error) {
	if opts.Branch == "" {
		opts.Branch = "main"
	}

	org, repo := splitProject(opts.Project)
	bareDir := filepath.Join(opts.Settings.BareRepoBase, org, repo+".git")
	timestamp := time.Now().Unix()
	worktreeDir := filepath.Join(opts.Settings.WorktreeBase, fmt.Sprintf("%s-%d", repo, timestamp))

	// Expand paths for remote machine
	remoteBare := expandRemotePath(bareDir, opts.Machine)
	remoteWork := expandRemotePath(worktreeDir, opts.Machine)

	// Step 1: Ensure bare clone exists
	checkCmd := fmt.Sprintf("test -d %s", remoteBare)
	if _, err := fleetexec.Run(ctx, opts.Machine, checkCmd); err != nil {
		cloneURL := fmt.Sprintf("https://github.com/%s.git", opts.Project)
		mkdirCmd := fmt.Sprintf("mkdir -p %s && git clone --bare %s %s",
			filepath.Dir(remoteBare), cloneURL, remoteBare)
		if _, err := fleetexec.Run(ctx, opts.Machine, mkdirCmd); err != nil {
			return nil, fmt.Errorf("bare clone: %w", err)
		}
	}

	// Step 2: Fetch latest
	fetchCmd := fmt.Sprintf("git -C %s fetch origin", remoteBare)
	if _, err := fleetexec.Run(ctx, opts.Machine, fetchCmd); err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	// Step 3: Create worktree
	worktreeCmd := fmt.Sprintf("git -C %s worktree add %s origin/%s",
		remoteBare, remoteWork, opts.Branch)
	if _, err := fleetexec.Run(ctx, opts.Machine, worktreeCmd); err != nil {
		return nil, fmt.Errorf("worktree: %w", err)
	}

	// Step 4: Detect dev server port
	devPort, pinnedLocal := detectRemotePorts(ctx, opts.Machine, remoteWork)

	// Step 5: Set up tunnel
	state, err := LoadState(opts.StatePath)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}
	usedPorts := state.UsedPorts()

	var localPort int
	if pinnedLocal > 0 {
		localPort, err = tunnel.AllocatePortPinned(pinnedLocal, usedPorts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v. Auto-assigning port.\n", err)
			localPort, err = tunnel.AllocatePort(
				opts.Settings.PortRange[0], opts.Settings.PortRange[1], usedPorts)
		}
	} else if !opts.Machine.IsLocal() {
		localPort, err = tunnel.AllocatePort(
			opts.Settings.PortRange[0], opts.Settings.PortRange[1], usedPorts)
	}
	if err != nil {
		return nil, fmt.Errorf("allocate port: %w", err)
	}

	var tun *tunnel.Tunnel
	if !opts.Machine.IsLocal() && localPort > 0 {
		tun, err = tunnel.Start(opts.Machine, localPort, devPort)
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
		Tunnel:       TunnelInfo{LocalPort: localPort, RemotePort: devPort},
		StartedAt:    time.Now().UTC(),
		OwnerPID:     os.Getpid(),
	}

	if err := AddSession(opts.StatePath, sess); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	return &LaunchResult{Session: sess, Tunnel: tun}, nil
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

	sshCmd := fmt.Sprintf("cd %s && claude", worktreePath)
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

func detectRemotePorts(ctx context.Context, m config.Machine, worktree string) (int, int) {
	// Try to read .fleet.toml from remote worktree
	catCmd := fmt.Sprintf("cat %s/.fleet.toml 2>/dev/null || true", worktree)
	fleetToml, _ := fleetexec.Run(ctx, m, catCmd)

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
	catCmd = fmt.Sprintf("cat %s/package.json 2>/dev/null || true", worktree)
	pkgJSON, _ := fleetexec.Run(ctx, m, catCmd)
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
