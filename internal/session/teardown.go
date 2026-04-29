package session

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/neonwatty/fleet/internal/config"
	fleetexec "github.com/neonwatty/fleet/internal/exec"
	"github.com/neonwatty/fleet/internal/tunnel"
)

func Teardown(ctx context.Context, m config.Machine, sess Session, tun *tunnel.Tunnel, statePath string) {
	// 1. Stop tunnel
	if tun != nil {
		_ = tun.Stop()
	}

	// 2. Remove remote worktree
	if sess.WorktreePath != "" {
		rmCmd := fmt.Sprintf("rm -rf -- %s", shellQuotePath(sess.WorktreePath))
		_, _ = fleetexec.Run(ctx, m, rmCmd)

		pruneCmd := fmt.Sprintf("git -C %s worktree prune", shellQuotePath(bareRepoPathForSession(sess)))
		_, _ = fleetexec.Run(ctx, m, pruneCmd)
	}

	// 3. Remove session from state
	_ = RemoveSession(statePath, sess.ID)
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
