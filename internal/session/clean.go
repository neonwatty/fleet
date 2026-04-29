package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

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
	Failed      []Session
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
	var result CleanResult
	var cleanupErr error
	err := WithStateLock(statePath, func(state *State) error {
		checker := MakeRemoteChecker(ctx, cfg.Machines)
		alive, orphans, stales := ClassifySessions(state.Sessions, checker)
		result = CleanResult{
			Alive:       alive,
			Orphans:     orphans,
			Stales:      stales,
			ResetLabels: countDanglingLabels(state, sessionIDSet(alive)),
		}
		printCleanPlan(result, opts)

		if opts.DryRun {
			fmt.Println("Dry run: no state, worktrees, or tunnels changed.")
			return errSkipStateSave
		}
		if len(state.Sessions) == 0 && result.ResetLabels == 0 {
			fmt.Println("No sessions in state. Nothing to clean.")
			return errSkipStateSave
		}

		survivors := append([]Session{}, alive...)
		for _, sess := range orphans {
			if err := cleanOrphan(ctx, cfg, sess); err != nil {
				fmt.Printf("  Failed to clean orphan: %s on %s (%s): %v\n",
					sess.Project, sess.Machine, sess.WorktreePath, err)
				result.Failed = append(result.Failed, sess)
				survivors = append(survivors, sess)
				cleanupErr = errors.Join(cleanupErr, err)
			}
		}
		for _, sess := range stales {
			fmt.Printf("  Removing stale state: %s on %s (%s)\n", sess.Project, sess.Machine, sess.WorktreePath)
		}

		state.Sessions = survivors
		if n := resetDanglingLabels(state, sessionIDSet(survivors)); n > 0 {
			fmt.Printf("  Reset %d dangling label(s) to orphan status\n", n)
		}
		return nil
	})
	if errors.Is(err, errSkipStateSave) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if cleanupErr != nil {
		return result, cleanupErr
	}

	fmt.Printf("Cleaned %d sessions.\n", result.Cleaned())
	killOrphanTunnels(result.Alive)

	return result, nil
}

var errSkipStateSave = errors.New("skip state save")

func remotePathExists(ctx context.Context, m config.Machine, path string) bool {
	if path == "" {
		return false
	}
	checkDir := fmt.Sprintf("test -d %s", shellQuotePath(path))
	_, err := fleetexec.RunWithTimeout(ctx, m, checkDir, 5*time.Second)
	return err == nil
}

func remoteClaudeOwnsWorktree(ctx context.Context, m config.Machine, worktreePath string) bool {
	checkProc := fmt.Sprintf("ps aux | grep '[c]laude' | grep -q -- %s",
		shellQuote(filepath.Base(worktreePath)))
	_, err := fleetexec.RunWithTimeout(ctx, m, checkProc, 5*time.Second)
	return err == nil
}

func cleanOrphan(ctx context.Context, cfg *config.Config, sess Session) error {
	if err := validateWorktreeDeletePath(cfg.Settings.WorktreeBase, sess.WorktreePath); err != nil {
		return err
	}

	m, ok := findSessionMachine(cfg.Machines, sess.Machine)
	if !ok {
		fmt.Printf("  Removing orphan state with missing machine: %s on %s (%s)\n",
			sess.Project, sess.Machine, sess.WorktreePath)
		return nil
	}

	fmt.Printf("  Cleaning orphan: %s on %s (%s)\n", sess.Project, sess.Machine, sess.WorktreePath)
	rmCmd := fmt.Sprintf("rm -rf -- %s", shellQuotePath(sess.WorktreePath))
	if _, err := fleetexec.RunWithTimeout(ctx, m, rmCmd, 10*time.Second); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}

	pruneCmd := fmt.Sprintf("git -C %s worktree prune 2>/dev/null || true", shellQuotePath(bareRepoPathForSession(sess)))
	if _, err := fleetexec.RunWithTimeout(ctx, m, pruneCmd, 10*time.Second); err != nil {
		return fmt.Errorf("prune worktree: %w", err)
	}
	return nil
}

func validateWorktreeDeletePath(base, target string) error {
	if base == "" {
		return fmt.Errorf("settings.worktree_base is required before deleting worktrees")
	}
	if target == "" {
		return fmt.Errorf("refusing to delete empty worktree path")
	}

	cleanBase := filepath.Clean(base)
	cleanTarget := filepath.Clean(target)
	if cleanTarget == "." || cleanTarget == string(filepath.Separator) {
		return fmt.Errorf("refusing to delete unsafe worktree path %q", target)
	}
	if cleanTarget == cleanBase {
		return fmt.Errorf("refusing to delete worktree base %q", target)
	}
	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil {
		return fmt.Errorf("compare worktree path: %w", err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("refusing to delete %q outside worktree base %q", target, base)
	}
	return nil
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
