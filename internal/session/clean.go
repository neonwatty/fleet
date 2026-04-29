package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

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

type CleanOptions struct {
	DryRun bool
}

type CleanResult struct {
	Alive       []Session
	Orphans     []Session
	Stales      []Session
	ResetLabels int
}

func (r CleanResult) Cleaned() int {
	return len(r.Orphans) + len(r.Stales)
}

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
		if sess.OwnerPID > 0 && !localProcessExists(sess.OwnerPID) {
			return StatusOrphan
		}
		if !remotePathExists(ctx, m, sess.WorktreePath) {
			return StatusStale
		}
		if !remoteClaudeOwnsWorktree(ctx, m, sess.WorktreePath) {
			return StatusOrphan
		}
		return StatusAlive
	}
}

func Clean(ctx context.Context, cfg *config.Config, statePath string) error {
	_, err := CleanWithOptions(ctx, cfg, statePath, CleanOptions{})
	return err
}

func CleanWithOptions(ctx context.Context, cfg *config.Config, statePath string, opts CleanOptions) (CleanResult, error) {
	state, err := LoadState(statePath)
	if err != nil {
		return CleanResult{}, fmt.Errorf("load state: %w", err)
	}

	checker := MakeRemoteChecker(ctx, cfg.Machines)
	alive, orphans, stales := ClassifySessions(state.Sessions, checker)
	result := CleanResult{
		Alive:       alive,
		Orphans:     orphans,
		Stales:      stales,
		ResetLabels: countDanglingLabels(state, sessionIDSet(alive)),
	}
	printCleanPlan(result, opts)

	if opts.DryRun {
		fmt.Println("Dry run: no state, worktrees, or tunnels changed.")
		return result, nil
	}
	if len(state.Sessions) == 0 && result.ResetLabels == 0 {
		fmt.Println("No sessions in state. Nothing to clean.")
		return result, nil
	}

	for _, sess := range orphans {
		cleanOrphan(ctx, cfg.Machines, sess)
	}
	for _, sess := range stales {
		fmt.Printf("  Removing stale state: %s on %s (%s)\n", sess.Project, sess.Machine, sess.WorktreePath)
	}

	state.Sessions = alive
	if n := resetDanglingLabels(state, sessionIDSet(alive)); n > 0 {
		fmt.Printf("  Reset %d dangling label(s) to orphan status\n", n)
	}
	if err := Save(statePath, state); err != nil {
		return CleanResult{}, fmt.Errorf("save state: %w", err)
	}

	fmt.Printf("Cleaned %d sessions.\n", result.Cleaned())
	killOrphanTunnels(alive)

	return result, nil
}

func remotePathExists(ctx context.Context, m config.Machine, path string) bool {
	if path == "" {
		return false
	}
	checkDir := fmt.Sprintf("test -d %s", shellQuotePath(path))
	_, err := fleetexec.Run(ctx, m, checkDir)
	return err == nil
}

func remoteClaudeOwnsWorktree(ctx context.Context, m config.Machine, worktreePath string) bool {
	checkProc := fmt.Sprintf("ps aux | grep '[c]laude' | grep -q -- %s",
		shellQuote(filepath.Base(worktreePath)))
	_, err := fleetexec.Run(ctx, m, checkProc)
	return err == nil
}

func cleanOrphan(ctx context.Context, machines []config.Machine, sess Session) {
	m, ok := findSessionMachine(machines, sess.Machine)
	if !ok {
		fmt.Printf("  Removing orphan state with missing machine: %s on %s (%s)\n",
			sess.Project, sess.Machine, sess.WorktreePath)
		return
	}

	fmt.Printf("  Cleaning orphan: %s on %s (%s)\n", sess.Project, sess.Machine, sess.WorktreePath)
	rmCmd := fmt.Sprintf("rm -rf -- %s", shellQuotePath(sess.WorktreePath))
	_, _ = fleetexec.Run(ctx, m, rmCmd)

	pruneCmd := fmt.Sprintf("git -C %s worktree prune 2>/dev/null || true", shellQuotePath(bareRepoPathForSession(sess)))
	_, _ = fleetexec.Run(ctx, m, pruneCmd)
}

func findSessionMachine(machines []config.Machine, name string) (config.Machine, bool) {
	for _, m := range machines {
		if m.Name == name {
			return m, true
		}
	}
	return config.Machine{}, false
}

// resetDanglingLabels clears the SessionID field on any label whose
// SessionID no longer references a surviving session. Returns the count
// of labels that were reset. The label's Name and LastSeenPID are
// preserved so the label continues to render (as stale/orphan) after the
// linked session has been torn down.
func resetDanglingLabels(state *State, aliveIDs map[string]bool) int {
	count := 0
	for machineName, labels := range state.MachineLabels {
		for i := range labels {
			if labels[i].SessionID != "" && !aliveIDs[labels[i].SessionID] {
				labels[i].SessionID = ""
				count++
			}
		}
		state.MachineLabels[machineName] = labels
	}
	return count
}

func countDanglingLabels(state *State, aliveIDs map[string]bool) int {
	count := 0
	for _, labels := range state.MachineLabels {
		for _, label := range labels {
			if label.SessionID != "" && !aliveIDs[label.SessionID] {
				count++
			}
		}
	}
	return count
}

func sessionIDSet(sessions []Session) map[string]bool {
	ids := make(map[string]bool, len(sessions))
	for _, sess := range sessions {
		ids[sess.ID] = true
	}
	return ids
}

func printCleanPlan(result CleanResult, opts CleanOptions) {
	action := "Clean"
	if opts.DryRun {
		action = "Would clean"
	}
	fmt.Printf("Sessions: %d alive, %d orphaned, %d stale\n",
		len(result.Alive), len(result.Orphans), len(result.Stales))
	for _, sess := range result.Orphans {
		fmt.Printf("  %s orphan: %s on %s (%s)\n",
			action, sess.Project, sess.Machine, sess.WorktreePath)
	}
	for _, sess := range result.Stales {
		fmt.Printf("  %s stale state: %s on %s (%s)\n",
			action, sess.Project, sess.Machine, sess.WorktreePath)
	}
	if result.ResetLabels > 0 {
		fmt.Printf("  %s %d dangling label(s) to orphan status\n", action, result.ResetLabels)
	}
}

func localProcessExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || err == syscall.EPERM
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
