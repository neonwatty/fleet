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

type Process struct {
	RSSKB   int
	PID     int
	Command string
}

type ProcessGroup struct {
	Name     string
	Count    int
	TotalRSS int // in KB
	PIDs     []int
	Killable bool
	Detail   string // e.g. "next-server" or "14 tabs"
}

func ProbeProcesses(ctx context.Context, m config.Machine) []ProcessGroup {
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := "ps -eo rss,pid,command -m | head -51 | tail -50"
	out, err := fleetexec.Run(probeCtx, m, cmd)
	if err != nil {
		return nil
	}

	procs := parseProcesses(out)
	return ClassifyProcesses(procs)
}

func KillGroup(ctx context.Context, m config.Machine, group ProcessGroup) error {
	if len(group.PIDs) == 0 {
		return fmt.Errorf("no PIDs to kill")
	}

	pidStrs := make([]string, len(group.PIDs))
	for i, pid := range group.PIDs {
		pidStrs[i] = strconv.Itoa(pid)
	}

	cmd := "kill " + strings.Join(pidStrs, " ")
	_, err := fleetexec.Run(ctx, m, cmd)
	return err
}

func parseProcesses(out string) []Process {
	var procs []Process
	for _, line := range strings.Split(out, "\n") {
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
		"Claude Code": {Name: "Claude Code", Killable: true},
		"Dev Servers": {Name: "Dev Servers", Killable: true},
		"Chrome":      {Name: "Chrome", Killable: true},
		"Docker":      {Name: "Docker", Killable: true},
		"System":      {Name: "System", Killable: false},
	}

	for _, p := range procs {
		cmd := p.Command
		switch {
		case isClaudeCode(cmd):
			g := groups["Claude Code"]
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
	return strings.HasPrefix(cmd, "claude ") || cmd == "claude"
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
