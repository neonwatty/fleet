package machine

import (
	"context"
	"fmt"
	"testing"

	"github.com/neonwatty/fleet/internal/config"
)

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
