package machine

import (
	"context"
	"fmt"
	"testing"

	"github.com/neonwatty/fleet/internal/config"
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
	if procs[0].RSSKB != 631 {
		t.Errorf("first proc RSS = %d KB, want 631", procs[0].RSSKB)
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

func TestParseVmmapSwap(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		wantKB int
	}{
		{
			name:   "megabytes",
			line:   "TOTAL                             10.7G   536.6M   311.3M   204.5M       0K      32K       0K     3628",
			wantKB: 209408, // 204.5 * 1024
		},
		{
			name:   "gigabytes",
			line:   "TOTAL                             10.7G   536.6M   311.3M   1.2G       0K      32K       0K     3628",
			wantKB: 1258291, // 1.2 * 1024 * 1024
		},
		{
			name:   "kilobytes",
			line:   "TOTAL                             10.7G   536.6M   311.3M   512K       0K      32K       0K     3628",
			wantKB: 512,
		},
		{
			name:   "zero",
			line:   "TOTAL                             10.7G   536.6M   311.3M   0K       0K      32K       0K     3628",
			wantKB: 0,
		},
		{
			name:   "empty",
			line:   "",
			wantKB: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVmmapSwap(tt.line)
			if got != tt.wantKB {
				t.Errorf("parseVmmapSwap() = %d, want %d", got, tt.wantKB)
			}
		})
	}
}

func TestParseSizeToKB(t *testing.T) {
	tests := []struct {
		input  string
		wantKB int
	}{
		{"204.5M", 209408},
		{"1.2G", 1258291},
		{"512K", 512},
		{"0K", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseSizeToKB(tt.input)
		if got != tt.wantKB {
			t.Errorf("parseSizeToKB(%q) = %d, want %d", tt.input, got, tt.wantKB)
		}
	}
}

func TestClassifyInitializesSwapToNegOne(t *testing.T) {
	procs := parseProcesses(fixturePS)
	groups := ClassifyProcesses(procs)
	for _, g := range groups {
		if g.TotalSwap != -1 {
			t.Errorf("group %q TotalSwap = %d, want -1 (not scanned)", g.Name, g.TotalSwap)
		}
	}
}

// --- ScanSwapWith tests ---

func mockRunner(responses map[int]string) CommandRunner {
	return func(_ context.Context, _ config.Machine, cmd string) (string, error) {
		for pid, resp := range responses {
			if fmt.Sprintf("vmmap --summary %d", pid) == cmd[:len(fmt.Sprintf("vmmap --summary %d", pid))] {
				return resp, nil
			}
		}
		return "", fmt.Errorf("no mock for command: %s", cmd)
	}
}

func TestScanSwapAssignsToCorrectGroups(t *testing.T) {
	groups := []ProcessGroup{
		{Name: "Chrome", PIDs: []int{100, 101}, TotalSwap: -1},
		{Name: "Claude Code", PIDs: []int{200}, TotalSwap: -1},
	}

	runner := mockRunner(map[int]string{
		100: "TOTAL    10.7G   500M   300M   150.0M   0K   0K   0K   100",
		101: "TOTAL    10.7G   500M   300M   50.0M    0K   0K   0K   100",
		200: "TOTAL    10.7G   500M   300M   300.0M   0K   0K   0K   100",
	})

	m := config.Machine{Name: "test", Host: "localhost"}
	result := ScanSwapWith(context.Background(), m, groups, 10, runner)

	chrome := findGroup(result, "Chrome")
	if chrome.TotalSwap != (150+50)*1024 {
		t.Errorf("Chrome TotalSwap = %d KB, want %d KB", chrome.TotalSwap, (150+50)*1024)
	}

	claude := findGroup(result, "Claude Code")
	if claude.TotalSwap != 300*1024 {
		t.Errorf("Claude Code TotalSwap = %d KB, want %d KB", claude.TotalSwap, 300*1024)
	}
}

func TestScanSwapRespectsMaxProcs(t *testing.T) {
	groups := []ProcessGroup{
		{Name: "Chrome", PIDs: []int{100, 101, 102, 103, 104}, TotalSwap: -1},
		{Name: "Claude Code", PIDs: []int{200, 201}, TotalSwap: -1},
	}

	scannedPIDs := make(map[int]bool)
	runner := func(_ context.Context, _ config.Machine, cmd string) (string, error) {
		for _, pid := range []int{100, 101, 102, 103, 104, 200, 201} {
			prefix := fmt.Sprintf("vmmap --summary %d", pid)
			if len(cmd) >= len(prefix) && cmd[:len(prefix)] == prefix {
				scannedPIDs[pid] = true
				return "TOTAL    10G   500M   300M   10.0M   0K   0K   0K   100", nil
			}
		}
		return "", fmt.Errorf("no mock")
	}

	m := config.Machine{Name: "test", Host: "localhost"}
	ScanSwapWith(context.Background(), m, groups, 3, runner)

	if len(scannedPIDs) != 3 {
		t.Errorf("scanned %d PIDs, want 3 (maxProcs=3)", len(scannedPIDs))
	}
}

func TestScanSwapHandlesVmmapErrors(t *testing.T) {
	groups := []ProcessGroup{
		{Name: "Chrome", PIDs: []int{100, 101}, TotalSwap: -1},
	}

	runner := func(_ context.Context, _ config.Machine, cmd string) (string, error) {
		// PID 100 succeeds, PID 101 fails (process died)
		if fmt.Sprintf("vmmap --summary %d", 100) == cmd[:len(fmt.Sprintf("vmmap --summary %d", 100))] {
			return "TOTAL    10G   500M   300M   100.0M   0K   0K   0K   100", nil
		}
		return "", fmt.Errorf("process not found")
	}

	m := config.Machine{Name: "test", Host: "localhost"}
	result := ScanSwapWith(context.Background(), m, groups, 10, runner)

	chrome := findGroup(result, "Chrome")
	if chrome.TotalSwap != 100*1024 {
		t.Errorf("Chrome TotalSwap = %d KB, want %d KB (should count only successful PID)", chrome.TotalSwap, 100*1024)
	}
}

func TestScanSwapHandlesGarbageOutput(t *testing.T) {
	groups := []ProcessGroup{
		{Name: "Chrome", PIDs: []int{100}, TotalSwap: -1},
	}

	runner := func(_ context.Context, _ config.Machine, _ string) (string, error) {
		return "some garbage output that doesn't match TOTAL format", nil
	}

	m := config.Machine{Name: "test", Host: "localhost"}
	result := ScanSwapWith(context.Background(), m, groups, 10, runner)

	chrome := findGroup(result, "Chrome")
	if chrome.TotalSwap != 0 {
		t.Errorf("Chrome TotalSwap = %d, want 0 (garbage vmmap output)", chrome.TotalSwap)
	}
}

func TestScanSwapEmptyGroups(t *testing.T) {
	m := config.Machine{Name: "test", Host: "localhost"}
	runner := func(_ context.Context, _ config.Machine, _ string) (string, error) {
		return "", nil
	}

	result := ScanSwapWith(context.Background(), m, nil, 10, runner)
	if len(result) != 0 {
		t.Errorf("expected 0 groups for nil input, got %d", len(result))
	}

	result = ScanSwapWith(context.Background(), m, []ProcessGroup{}, 10, runner)
	if len(result) != 0 {
		t.Errorf("expected 0 groups for empty input, got %d", len(result))
	}
}

func TestScanSwapSetsAllGroupsToZero(t *testing.T) {
	groups := []ProcessGroup{
		{Name: "A", PIDs: []int{1}, TotalSwap: -1},
		{Name: "B", PIDs: []int{2}, TotalSwap: -1},
	}

	runner := func(_ context.Context, _ config.Machine, _ string) (string, error) {
		return "TOTAL    10G   500M   300M   0K   0K   0K   0K   100", nil
	}

	m := config.Machine{Name: "test", Host: "localhost"}
	result := ScanSwapWith(context.Background(), m, groups, 10, runner)

	for _, g := range result {
		if g.TotalSwap < 0 {
			t.Errorf("group %q TotalSwap = %d, want >= 0 after scan", g.Name, g.TotalSwap)
		}
	}
}

// --- KillGroupWith tests ---

func TestKillGroupBuildsCorrectCommand(t *testing.T) {
	var capturedCmd string
	runner := func(_ context.Context, _ config.Machine, cmd string) (string, error) {
		capturedCmd = cmd
		return "", nil
	}

	group := ProcessGroup{PIDs: []int{123, 456, 789}}
	m := config.Machine{Name: "test", Host: "localhost"}

	err := KillGroupWith(context.Background(), m, group, runner)
	if err != nil {
		t.Fatalf("KillGroupWith() error: %v", err)
	}

	expected := "kill 123 456 789"
	if capturedCmd != expected {
		t.Errorf("command = %q, want %q", capturedCmd, expected)
	}
}

func TestKillGroupEmptyPIDs(t *testing.T) {
	runner := func(_ context.Context, _ config.Machine, _ string) (string, error) {
		t.Fatal("runner should not be called for empty PIDs")
		return "", nil
	}

	group := ProcessGroup{PIDs: []int{}}
	m := config.Machine{Name: "test", Host: "localhost"}

	err := KillGroupWith(context.Background(), m, group, runner)
	if err == nil {
		t.Error("expected error for empty PIDs")
	}
}

func TestKillGroupPropagatesRunnerError(t *testing.T) {
	runner := func(_ context.Context, _ config.Machine, _ string) (string, error) {
		return "", fmt.Errorf("ssh connection refused")
	}

	group := ProcessGroup{PIDs: []int{123}}
	m := config.Machine{Name: "test", Host: "localhost"}

	err := KillGroupWith(context.Background(), m, group, runner)
	if err == nil {
		t.Error("expected error when runner fails")
	}
}

func TestKillGroupSinglePID(t *testing.T) {
	var capturedCmd string
	runner := func(_ context.Context, _ config.Machine, cmd string) (string, error) {
		capturedCmd = cmd
		return "", nil
	}

	group := ProcessGroup{PIDs: []int{42}}
	m := config.Machine{Name: "test", Host: "localhost"}
	_ = KillGroupWith(context.Background(), m, group, runner)

	if capturedCmd != "kill 42" {
		t.Errorf("command = %q, want %q", capturedCmd, "kill 42")
	}
}

// --- Classification edge cases ---

func TestClassifySortsByRSSDescending(t *testing.T) {
	procs := parseProcesses(fixturePS)
	groups := ClassifyProcesses(procs)

	for i := 1; i < len(groups); i++ {
		if groups[i].TotalRSS > groups[i-1].TotalRSS {
			t.Errorf("groups not sorted: %q (%d KB) before %q (%d KB)",
				groups[i-1].Name, groups[i-1].TotalRSS, groups[i].Name, groups[i].TotalRSS)
		}
	}
}

func TestClassifyDevServerDetail(t *testing.T) {
	procs := parseProcesses(fixturePS)
	groups := ClassifyProcesses(procs)

	dev := findGroup(groups, "Dev Servers")
	if dev == nil {
		t.Fatal("expected Dev Servers group")
	}
	if dev.Detail != "next" {
		t.Errorf("Dev Servers detail = %q, want %q", dev.Detail, "next")
	}
}

func TestClassifyDockerKillable(t *testing.T) {
	procs := parseProcesses(fixturePS)
	groups := ClassifyProcesses(procs)

	docker := findGroup(groups, "Docker")
	if docker == nil {
		t.Fatal("expected Docker group")
	}
	if !docker.Killable {
		t.Error("Docker should be killable")
	}
}

func TestClassifySystemNotKillable(t *testing.T) {
	// Use a process big enough to cross the 50KB threshold
	procs := []Process{
		{RSSKB: 60 * 1024, PID: 1, Command: "/System/Library/something/big"},
	}
	groups := ClassifyProcesses(procs)
	sys := findGroup(groups, "System")
	if sys == nil {
		t.Fatal("expected System group for large system process")
	}
	if sys.Killable {
		t.Error("System should not be killable")
	}
}

func TestClassifySystemBelowThresholdExcluded(t *testing.T) {
	procs := []Process{
		{RSSKB: 10 * 1024, PID: 1, Command: "/System/Library/something/small"},
	}
	groups := ClassifyProcesses(procs)
	sys := findGroup(groups, "System")
	if sys != nil {
		t.Error("System process below 50MB threshold should be excluded")
	}
}

func TestParseProcessesSkipsMalformed(t *testing.T) {
	input := `631 19648 next-server
not-a-number 123 something
456 not-a-pid something
incomplete
98 16831 claude --resume
`
	procs := parseProcesses(input)
	if len(procs) != 2 {
		t.Errorf("parseProcesses() returned %d procs, want 2 (skipping malformed)", len(procs))
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
