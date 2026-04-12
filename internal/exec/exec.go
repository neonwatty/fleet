package exec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/neonwatty/fleet/internal/config"
)

func Run(ctx context.Context, m config.Machine, command string) (string, error) {
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
		return "", fmt.Errorf("%s: %w (stderr: %s)", m.Name, err, stderr.String())
	}

	return stdout.String(), nil
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
