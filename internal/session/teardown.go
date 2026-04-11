package session

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/neonwatty/fleet/internal/config"
	fleetexec "github.com/neonwatty/fleet/internal/exec"
	"github.com/neonwatty/fleet/internal/tunnel"
)

func Teardown(ctx context.Context, m config.Machine, sess Session, tun *tunnel.Tunnel, statePath string) {
	// 1. Stop tunnel
	if tun != nil {
		tun.Stop()
	}

	// 2. Remove remote worktree
	if sess.WorktreePath != "" {
		rmCmd := fmt.Sprintf("rm -rf %s", sess.WorktreePath)
		fleetexec.Run(ctx, m, rmCmd)

		// Prune worktrees on the bare repo
		org, repo := splitProject(sess.Project)
		home := "~"
		bareDir := filepath.Join(home, "fleet-repos", org, repo+".git")
		pruneCmd := fmt.Sprintf("git -C %s worktree prune", bareDir)
		fleetexec.Run(ctx, m, pruneCmd)
	}

	// 3. Remove session from state
	RemoveSession(statePath, sess.ID)
}

func WithSignalCleanup(
	ctx context.Context,
	m config.Machine,
	sess Session,
	tun *tunnel.Tunnel,
	statePath string,
	fn func() error,
) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- fn()
	}()

	select {
	case sig := <-sigCh:
		fmt.Fprintf(os.Stderr, "\nReceived %v, cleaning up...\n", sig)
		Teardown(ctx, m, sess, tun, statePath)
		return fmt.Errorf("interrupted by %v", sig)
	case err := <-doneCh:
		Teardown(ctx, m, sess, tun, statePath)
		return err
	}
}
