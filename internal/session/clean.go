package session

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/neonwatty/fleet/internal/config"
	fleetexec "github.com/neonwatty/fleet/internal/exec"
)

type SessionStatus int

const (
	StatusAlive SessionStatus = iota
	StatusOrphan
	StatusStale
)

type StatusChecker func(Session) SessionStatus

func ClassifySessions(sessions []Session, check StatusChecker) (alive, orphan, stale []Session) {
	for _, s := range sessions {
		switch check(s) {
		case StatusAlive:
			alive = append(alive, s)
		case StatusOrphan:
			orphan = append(orphan, s)
		case StatusStale:
			stale = append(stale, s)
		}
	}
	return
}

func MakeRemoteChecker(ctx context.Context, machines []config.Machine) StatusChecker {
	machineMap := make(map[string]config.Machine)
	for _, m := range machines {
		machineMap[m.Name] = m
	}

	return func(sess Session) SessionStatus {
		m, ok := machineMap[sess.Machine]
		if !ok {
			return StatusStale
		}

		checkDir := fmt.Sprintf("test -d %s", sess.WorktreePath)
		if _, err := fleetexec.Run(ctx, m, checkDir); err != nil {
			return StatusStale
		}

		checkProc := fmt.Sprintf("ps aux | grep '[c]laude' | grep -q %s",
			filepath.Base(sess.WorktreePath))
		if _, err := fleetexec.Run(ctx, m, checkProc); err != nil {
			return StatusOrphan
		}

		return StatusAlive
	}
}

func Clean(ctx context.Context, cfg *config.Config, statePath string) error {
	state, err := LoadState(statePath)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	if len(state.Sessions) == 0 {
		fmt.Println("No sessions in state. Nothing to clean.")
		return nil
	}

	checker := MakeRemoteChecker(ctx, cfg.Machines)
	alive, orphans, stales := ClassifySessions(state.Sessions, checker)

	fmt.Printf("Sessions: %d alive, %d orphaned, %d stale\n",
		len(alive), len(orphans), len(stales))

	machineMap := make(map[string]config.Machine)
	for _, m := range cfg.Machines {
		machineMap[m.Name] = m
	}

	for _, sess := range orphans {
		m := machineMap[sess.Machine]
		fmt.Printf("  Cleaning orphan: %s on %s (%s)\n", sess.Project, sess.Machine, sess.WorktreePath)

		rmCmd := fmt.Sprintf("rm -rf %s", sess.WorktreePath)
		fleetexec.Run(ctx, m, rmCmd)

		org, repo := splitProject(sess.Project)
		bareDir := filepath.Join("~", "fleet-repos", org, repo+".git")
		pruneCmd := fmt.Sprintf("git -C %s worktree prune 2>/dev/null || true", bareDir)
		fleetexec.Run(ctx, m, pruneCmd)
	}

	state.Sessions = alive
	if err := Save(statePath, state); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	cleaned := len(orphans) + len(stales)
	fmt.Printf("Cleaned %d sessions.\n", cleaned)

	killOrphanTunnels(alive)

	return nil
}

func killOrphanTunnels(aliveSessions []Session) {
	aliveLocalPorts := make(map[int]bool)
	for _, s := range aliveSessions {
		if s.Tunnel.LocalPort > 0 {
			aliveLocalPorts[s.Tunnel.LocalPort] = true
		}
	}

	out, err := fleetexec.Run(context.Background(),
		config.Machine{Host: "localhost"},
		"ps aux | grep 'ssh -N -L' | grep -v grep || true")
	if err != nil {
		return
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		isNeeded := false
		for port := range aliveLocalPorts {
			if strings.Contains(line, fmt.Sprintf("%d:localhost:", port)) {
				isNeeded = true
				break
			}
		}
		if !isNeeded {
			fmt.Printf("  Found orphaned tunnel process: %s\n", truncateString(line, 80))
		}
	}
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
