package machine

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	fleetexec "github.com/neonwatty/fleet/internal/exec"
)

// CommandRunner executes a command on a machine. Matches fleetexec.Run signature.
// Extracted as a type so ScanSwap/KillGroup can be tested without SSH.
type CommandRunner func(ctx context.Context, m config.Machine, command string) (string, error)

type Process struct {
	RSSKB   int
	PID     int
	Command string
}

type ProcessGroup struct {
	Name      string
	Count     int
	TotalRSS  int // in KB
	TotalSwap int // in KB, -1 means not scanned
	PIDs      []int
	Killable  bool
	Detail    string // e.g. "next-server" or "14 tabs"
}

func ProbeProcesses(ctx context.Context, m config.Machine) []ProcessGroup {
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := "ps -eo rss,pid,command -m | head -51 | tail -50"
	out, err := fleetexec.RunWithTimeout(probeCtx, m, cmd, 10*time.Second)
	if err != nil {
		return nil
	}

	procs := parseProcesses(out)
	return ClassifyProcesses(procs)
}

func KillGroup(ctx context.Context, m config.Machine, group ProcessGroup) error {
	return KillGroupWith(ctx, m, group, fleetexec.Run)
}

func KillGroupWith(ctx context.Context, m config.Machine, group ProcessGroup, run CommandRunner) error {
	if len(group.PIDs) == 0 {
		return fmt.Errorf("no PIDs to kill")
	}

	pidStrs := make([]string, len(group.PIDs))
	for i, pid := range group.PIDs {
		pidStrs[i] = strconv.Itoa(pid)
	}

	cmd := "kill " + strings.Join(pidStrs, " ")
	_, err := run(ctx, m, cmd)
	return err
}

// ScanSwap runs vmmap --summary on the top N PIDs by RSS on the given machine
// and populates TotalSwap on each ProcessGroup. This is slow (~1-2s per PID).
func ScanSwap(ctx context.Context, m config.Machine, groups []ProcessGroup, maxProcs int) []ProcessGroup {
	return ScanSwapWith(ctx, m, groups, maxProcs, fleetexec.Run)
}

func ScanSwapWith(
	ctx context.Context, m config.Machine, groups []ProcessGroup, maxProcs int, run CommandRunner,
) []ProcessGroup {
	// Collect all PIDs across groups, limited to maxProcs
	type pidGroup struct {
		pid      int
		groupIdx int
	}
	var pids []pidGroup
	for gi, g := range groups {
		for _, pid := range g.PIDs {
			pids = append(pids, pidGroup{pid: pid, groupIdx: gi})
			if len(pids) >= maxProcs {
				break
			}
		}
		if len(pids) >= maxProcs {
			break
		}
	}

	// Initialize swap to 0 for all groups (marks as scanned)
	result := make([]ProcessGroup, len(groups))
	copy(result, groups)
	for i := range result {
		result[i].TotalSwap = 0
	}

	// Run vmmap for each PID and parse SWAPPED column
	for _, pg := range pids {
		cmd := fmt.Sprintf("vmmap --summary %d 2>/dev/null | grep '^TOTAL ' | head -1", pg.pid)
		scanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		out, err := run(scanCtx, m, cmd)
		cancel()
		if err != nil {
			continue
		}
		swapKB := parseVmmapSwap(strings.TrimSpace(out))
		result[pg.groupIdx].TotalSwap += swapKB
	}

	return result
}

func parseVmmapSwap(totalLine string) int {
	// Format: "TOTAL    10.7G   536.6M   311.3M   204.5M   0K   32K   0K   3628"
	// Fields: LABEL    VIRT    RESIDENT DIRTY    SWAPPED  ...
	fields := strings.Fields(totalLine)
	if len(fields) < 5 {
		return 0
	}
	return parseSizeToKB(fields[4])
}

func parseSizeToKB(s string) int {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0
	}

	suffix := s[len(s)-1]
	numStr := s[:len(s)-1]
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}

	switch suffix {
	case 'K':
		return int(val)
	case 'M':
		return int(val * 1024)
	case 'G':
		return int(val * 1024 * 1024)
	}
	return 0
}

func parseProcesses(out string) []Process {
	lines := strings.Split(out, "\n")
	procs := make([]Process, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, " ", 3)
		if len(fields) < 3 {
			continue
		}

		rss, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(fields[1]))
		if err != nil {
			continue
		}

		procs = append(procs, Process{
			RSSKB:   rss,
			PID:     pid,
			Command: strings.TrimSpace(fields[2]),
		})
	}
	return procs
}

func ClassifyProcesses(procs []Process) []ProcessGroup {
	if len(procs) == 0 {
		return nil
	}

	groups := map[string]*ProcessGroup{
		"Claude Code": {Name: "Claude Code", Killable: true, TotalSwap: -1},
		"Codex":       {Name: "Codex", Killable: true, TotalSwap: -1},
		"Dev Servers": {Name: "Dev Servers", Killable: true, TotalSwap: -1},
		"Chrome":      {Name: "Chrome", Killable: true, TotalSwap: -1},
		"Docker":      {Name: "Docker", Killable: true, TotalSwap: -1},
		"System":      {Name: "System", Killable: false, TotalSwap: -1},
	}

	for _, p := range procs {
		cmd := p.Command
		switch {
		case isClaudeCode(cmd):
			g := groups["Claude Code"]
			g.Count++
			g.TotalRSS += p.RSSKB
			g.PIDs = append(g.PIDs, p.PID)
		case isCodex(cmd):
			g := groups["Codex"]
			g.Count++
			g.TotalRSS += p.RSSKB
			g.PIDs = append(g.PIDs, p.PID)
		case isDevServer(cmd):
			g := groups["Dev Servers"]
			g.Count++
			g.TotalRSS += p.RSSKB
			g.PIDs = append(g.PIDs, p.PID)
			g.Detail = devServerDetail(cmd, g.Detail)
		case isChrome(cmd):
			g := groups["Chrome"]
			g.Count++
			g.TotalRSS += p.RSSKB
			g.PIDs = append(g.PIDs, p.PID)
		case isDocker(cmd):
			g := groups["Docker"]
			g.Count++
			g.TotalRSS += p.RSSKB
			g.PIDs = append(g.PIDs, p.PID)
		case isSystem(cmd) && p.RSSKB > 50*1024:
			g := groups["System"]
			g.Count++
			g.TotalRSS += p.RSSKB
			g.PIDs = append(g.PIDs, p.PID)
		}
	}

	var result []ProcessGroup
	for _, g := range groups {
		if g.Count > 0 {
			result = append(result, *g)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalRSS > result[j].TotalRSS
	})

	return result
}

func isClaudeCode(cmd string) bool {
	return isCommandNamed(cmd, "claude")
}

func isCodex(cmd string) bool {
	return isCommandNamed(cmd, "codex")
}

func isCommandNamed(cmd, name string) bool {
	fields := strings.Fields(cmd)
	return len(fields) > 0 && (fields[0] == name || strings.HasSuffix(fields[0], "/"+name))
}

func isDevServer(cmd string) bool {
	return strings.Contains(cmd, "next-server") ||
		strings.Contains(cmd, "next dev") ||
		strings.Contains(cmd, "vite") ||
		(strings.Contains(cmd, "node") && strings.Contains(cmd, "dev"))
}

func isChrome(cmd string) bool {
	return strings.Contains(cmd, "Google Chrome")
}

func isDocker(cmd string) bool {
	return strings.Contains(cmd, "Docker") || strings.Contains(cmd, "docker")
}

func isSystem(cmd string) bool {
	return strings.HasPrefix(cmd, "/System/") ||
		strings.HasPrefix(cmd, "/usr/libexec/")
}

func devServerDetail(cmd, existing string) string {
	if strings.Contains(cmd, "next-server") || strings.Contains(cmd, "next dev") {
		if !strings.Contains(existing, "next") {
			if existing == "" {
				return "next"
			}
			return existing + ", next"
		}
	}
	if strings.Contains(cmd, "vite") {
		if !strings.Contains(existing, "vite") {
			if existing == "" {
				return "vite"
			}
			return existing + ", vite"
		}
	}
	return existing
}
