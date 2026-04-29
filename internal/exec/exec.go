package exec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/neonwatty/fleet/internal/config"
)

type RunOptions struct {
	Timeout time.Duration
}

func Run(ctx context.Context, m config.Machine, command string) (string, error) {
	return RunWithOptions(ctx, m, command, RunOptions{})
}

func RunWithTimeout(ctx context.Context, m config.Machine, command string, timeout time.Duration) (string, error) {
	return RunWithOptions(ctx, m, command, RunOptions{Timeout: timeout})
}

func RunWithOptions(ctx context.Context, m config.Machine, command string, opts RunOptions) (string, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	var cmd *exec.Cmd

	if m.IsLocal() {
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	} else {
		args := buildSSHArgs(m, command)
		cmd = exec.CommandContext(ctx, "ssh", args...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", formatCommandError(ctx, m, command, opts.Timeout, stderr.String(), err)
	}

	return stdout.String(), nil
}

func formatCommandError(
	ctx context.Context,
	m config.Machine,
	command string,
	timeout time.Duration,
	stderr string,
	err error,
) error {
	target := m.Name
	if target == "" {
		target = m.Host
	}
	if target == "" {
		target = "local"
	}

	if ctx.Err() == context.DeadlineExceeded {
		if timeout > 0 {
			return fmt.Errorf("machine %s: command timed out after %s: %s", target, timeout, command)
		}
		return fmt.Errorf("machine %s: command timed out: %s", target, command)
	}

	msg := strings.TrimSpace(stderr)
	if msg == "" {
		return fmt.Errorf("machine %s: command failed: %s: %w", target, command, err)
	}
	return fmt.Errorf("machine %s: command failed: %s: %w (stderr: %s)", target, command, err, msg)
}

func buildSSHArgs(m config.Machine, command string) []string {
	args := make([]string, 0, 12)
	args = append(args,
		"-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=accept-new",
		// Never prompt for a password — fail fast if key auth doesn't work.
		// Otherwise fleet would block on a password prompt and look hung.
		"-o", "BatchMode=yes",
		"-o", "PasswordAuthentication=no",
		"-o", "KbdInteractiveAuthentication=no",
	)
	args = append(args, m.Host, command)
	return args
}
