package session

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/neonwatty/fleet/internal/config"
)

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

func buildRemoteExecCommand(worktreePath string, command string) string {
	return fmt.Sprintf("cd %s && %s", shellQuotePath(worktreePath), resolveLaunchCommand(command, ""))
}
