package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/session"
)

func killSession(ctx context.Context, cfg *config.Config, sess session.Session, statePath string) error {
	machineMap := make(map[string]config.Machine)
	for _, m := range cfg.Machines {
		machineMap[m.Name] = m
	}

	m, ok := machineMap[sess.Machine]
	if !ok {
		return fmt.Errorf("machine %q not found", sess.Machine)
	}

	session.Teardown(ctx, m, sess, nil, statePath)
	return nil
}

func openInBrowser(port int) error {
	url := fmt.Sprintf("http://localhost:%d", port)
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", url)
	} else {
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Run()
}
