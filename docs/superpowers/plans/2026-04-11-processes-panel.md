# Processes Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Processes" panel to the fleet TUI that shows memory consumers on the selected machine grouped by category, with the ability to kill process groups.

**Architecture:** New probe command collects top processes via `ps`. A new classifier groups them into categories. A new TUI panel renders the grouped view and handles kill actions via SSH.

**Tech Stack:** Same as main project — Go, Bubble Tea, system `ssh`/`ps`

**Spec:** `docs/superpowers/specs/2026-04-11-fleet-design.md` (Memory consumers panel section)

---

## File Map

| File | Responsibility |
|------|----------------|
| `internal/machine/processes.go` | Probe top processes, classify into categories |
| `internal/machine/processes_test.go` | Parsing and classification tests with fixtures |
| `internal/tui/processes.go` | Render the processes panel |
| `internal/tui/app.go` | Add processes panel, wire selected machine, kill keybinding |

---

### Task 1: Process Probing and Classification

**Files:**
- Create: `internal/machine/processes.go`
- Create: `internal/machine/processes_test.go`

- [ ] **Step 1: Write classification tests with fixture data**

Create `internal/machine/processes_test.go`:

```go
package machine

import (
	"testing"
)

const fixturePS = `631 19648 next-server (v15.5.14)
518 45760 claude --dangerously-skip-permissions
298 15083 /Applications/Google Chrome.app/Contents/Frameworks/Google Chrome Helper
185 15066 /Applications/Google Chrome.app/Contents/MacOS/Google Chrome
185 55964 /Applications/Docker.app/Contents/MacOS/com.docker.backend services
139 21077 /System/Library/PrivateFrameworks/MediaAnalysis.framework/Versions/A/mediaanalysisd
128 9406 /Applications/Google Chrome.app/Contents/MacOS/Google Chrome
98 16831 claude --dangerously-skip-permissions --resume
79 55983 /Applications/Docker.app/Contents/MacOS/com.docker.build
72 45850 node /Users/neonwatty/.npm/_npx/node_modules/.bin/playwright-mcp
67 849 /System/Library/Frameworks/CoreServices.framework/mds_stores
48 19642 node /Users/neonwatty/Desktop/issuectl/node_modules/.bin/../next/dist/bin/next dev
`

func TestParseProcesses(t *testing.T) {
	procs := parseProcesses(fixturePS)
	if len(procs) < 10 {
		t.Fatalf("parseProcesses() returned %d procs, want >= 10", len(procs))
	}
	if procs[0].RSSKB != 631*1024 {
		t.Errorf("first proc RSS = %d, want %d", procs[0].RSSKB, 631*1024)
	}
}

func TestClassifyProcesses(t *testing.T) {
	procs := parseProcesses(fixturePS)
	groups := ClassifyProcesses(procs)

	claude := findGroup(groups, "Claude Code")
	if claude == nil {
		t.Fatal("expected Claude Code group")
	}
	if claude.Count != 2 {
		t.Errorf("Claude Code count = %d, want 2", claude.Count)
	}

	chrome := findGroup(groups, "Chrome")
	if chrome == nil {
		t.Fatal("expected Chrome group")
	}
	if chrome.Count != 3 {
		t.Errorf("Chrome count = %d, want 3", chrome.Count)
	}

	docker := findGroup(groups, "Docker")
	if docker == nil {
		t.Fatal("expected Docker group")
	}

	devServers := findGroup(groups, "Dev Servers")
	if devServers == nil {
		t.Fatal("expected Dev Servers group")
	}
}

func TestClassifyEmpty(t *testing.T) {
	groups := ClassifyProcesses(nil)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for nil input, got %d", len(groups))
	}
}

func findGroup(groups []ProcessGroup, name string) *ProcessGroup {
	for i := range groups {
		if groups[i].Name == name {
			return &groups[i]
		}
	}
	return nil
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/machine/ -run TestParseProcesses -v`
Expected: Compilation error.

- [ ] **Step 3: Implement process probing and classification**

Create `internal/machine/processes.go`:

```go
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

	// Get top 50 processes by RSS, output: rss(KB) pid command
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
```

- [ ] **Step 4: Run tests**

Run: `go test -race ./internal/machine/ -v`
Expected: All tests pass (existing probe/score tests + new process tests).

- [ ] **Step 5: Commit**

```bash
git add internal/machine/processes.go internal/machine/processes_test.go
git commit -m "feat: add process probing and classification by category"
```

---

### Task 2: Processes TUI Panel

**Files:**
- Create: `internal/tui/processes.go`

- [ ] **Step 1: Implement the processes panel renderer**

Create `internal/tui/processes.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/neonwatty/fleet/internal/machine"
)

func renderProcessesPanel(machineName string, groups []machine.ProcessGroup, selectedRow int) string {
	if machineName == "" {
		return dimStyle.Render("Select a machine to view processes")
	}

	if len(groups) == 0 {
		return dimStyle.Render(fmt.Sprintf("No significant processes on %s", machineName))
	}

	var b strings.Builder

	header := fmt.Sprintf("%-14s %-6s %-10s %-20s",
		"CATEGORY", "COUNT", "RSS", "DETAIL")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for i, g := range groups {
		category := g.Name
		count := fmt.Sprintf("%d", g.Count)
		rss := formatRSS(g.TotalRSS)

		detail := g.Detail
		if g.Name == "Chrome" {
			detail = fmt.Sprintf("%d tabs/procs", g.Count)
		}
		if g.Name == "Docker" {
			detail = fmt.Sprintf("%d procs", g.Count)
		}
		if g.Name == "System" {
			detail = fmt.Sprintf("%d services", g.Count)
		}

		categoryCol := fmt.Sprintf("%-14s ", category)
		countCol := fmt.Sprintf("%-6s ", count)
		detailCol := fmt.Sprintf("%-20s", detail)

		var rssCol string
		if g.TotalRSS > 500*1024 { // > 500MB
			rssCol = warnStyle.Render(fmt.Sprintf("%-10s", rss)) + " "
		} else {
			rssCol = fmt.Sprintf("%-10s ", rss)
		}

		line := categoryCol + countCol + rssCol + detailCol

		if i == selectedRow {
			if !g.Killable {
				line = dimStyle.Render("> " + line)
			} else {
				line = "> " + line
			}
		} else {
			if !g.Killable {
				line = dimStyle.Render("  " + line)
			} else {
				line = "  " + line
			}
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func formatRSS(kb int) string {
	if kb >= 1024*1024 {
		return fmt.Sprintf("%.1fGB", float64(kb)/(1024*1024))
	}
	return fmt.Sprintf("%dMB", kb/1024)
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/tui/`
Expected: Compiles.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/processes.go
git commit -m "feat: add processes panel renderer for TUI"
```

---

### Task 3: Wire Processes Panel Into TUI App

**Files:**
- Modify: `internal/tui/app.go`

- [ ] **Step 1: Add processes state and panel to the model**

Update the `model` struct to include:
- `processGroups []machine.ProcessGroup` — current process data for selected machine
- `selectedMachine int` — which machine row is selected in the Machines panel
- Add `panelProcesses` to the panel enum (between `panelTunnels` and `panelCount`)

Update `refreshMsg` to include a map of process groups per machine:
```go
type refreshMsg struct {
    healths    []machine.Health
    state      *session.State
    processes  map[string][]machine.ProcessGroup // keyed by machine name
}
```

- [ ] **Step 2: Update the refresh command to probe processes**

In the `refresh` function, after probing health, also probe processes for each machine:
```go
processes := make(map[string][]machine.ProcessGroup)
for _, m := range cfg.EnabledMachines() {
    processes[m.Name] = machine.ProbeProcesses(ctx, m)
}
```

- [ ] **Step 3: Update Update() for the new panel and kill action**

In the `Update` method:
- When on `panelMachines`, j/k changes `selectedMachine` and updates `processGroups` from the cached map
- Add `panelProcesses` to tab cycling
- When on `panelProcesses`, pressing `k` (kill):
  - Get the selected process group
  - If not killable, ignore
  - If killable, call `machine.KillGroup()` then trigger a refresh
- Update help text to include `k: kill process group`

- [ ] **Step 4: Update View() to render the processes panel**

Add the processes panel between tunnels and the help bar:
```go
processesContent := renderProcessesPanel(selectedMachineName, m.processGroups, m.selectedRow)
processesPanel := wrapPanel("Processes", processesContent, panelWidth, m.activePanel == panelProcesses)
```

- [ ] **Step 5: Build and verify**

Run: `go build -o bin/fleet ./cmd/fleet/`
Expected: Compiles.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: wire processes panel into TUI with kill action"
```

---

### Task 4: Test and Polish

- [ ] **Step 1: Run all tests**

Run: `go test -race ./...`
Expected: All tests pass.

- [ ] **Step 2: Run linter**

Run: `golangci-lint run ./...`
Expected: 0 issues.

- [ ] **Step 3: Manual test**

Run: `bin/fleet status`
- Tab to Machines panel, navigate with j/k — processes panel should update per machine
- Tab to Processes panel — see grouped memory consumers
- Verify Chrome, Docker, Claude Code, Dev Servers, System categories appear correctly
- Try selecting a killable group (don't actually kill unless you want to)

- [ ] **Step 4: Commit and push**

```bash
git add -A
git commit -m "chore: processes panel polish and testing"
git push
```
