package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
)

const viewMachineName = "mm1"

func TestViewRendersAllPanelsAndModeHelp(t *testing.T) {
	m := viewTestModel(t)
	out := m.View()
	for _, want := range []string{
		"Fleet Dashboard",
		"Machines",
		"Sessions",
		"Tunnels",
		"Processes on mm1",
		"repo",
		"localhost:4001",
		"Dev Servers",
		"tab: switch panel",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("View() missing %q:\n%s", want, out)
		}
	}

	m.renaming = true
	m.renameBuffer = "work"
	if out := m.View(); !strings.Contains(out, "rename label: work") {
		t.Fatalf("rename View() missing prompt:\n%s", out)
	}

	m.renaming = false
	m.confirming = true
	m.pendingAction = actionKillProcess
	if out := m.View(); !strings.Contains(out, "Kill selected process group?") {
		t.Fatalf("confirm View() missing prompt:\n%s", out)
	}

	m.confirming = false
	m.swapScanning = true
	m.swapScanTarget = viewMachineName
	if out := m.View(); !strings.Contains(out, "Scanning swap on "+viewMachineName) {
		t.Fatalf("swap View() missing prompt:\n%s", out)
	}
}

func TestViewLoadingBeforeWindowSize(t *testing.T) {
	m := viewTestModel(t)
	m.width = 0
	if got := m.View(); got != "Loading..." {
		t.Fatalf("View() = %q, want Loading...", got)
	}
}

func TestSelectedMachineNameClampsAndHandlesEmpty(t *testing.T) {
	m := viewTestModel(t)
	m.selectedMachine = 99
	if got := m.selectedMachineName(); got != "mm2" {
		t.Fatalf("selectedMachineName() = %q, want mm2", got)
	}

	m.healths = nil
	if got := m.selectedMachineName(); got != "" {
		t.Fatalf("selectedMachineName() with no healths = %q, want empty", got)
	}
}

func TestFindMachineAndCCPIDsFromProcesses(t *testing.T) {
	m := viewTestModel(t)
	if got := m.findMachine("mm2"); got == nil || got.Host != "mm2" {
		t.Fatalf("findMachine(mm2) = %+v, want configured mm2", got)
	}
	if got := m.findMachine("missing"); got != nil {
		t.Fatalf("findMachine(missing) = %+v, want nil", got)
	}

	pids := ccPIDsFromProcesses(m.processes)
	if got := pids[viewMachineName]; len(got) != 2 || got[0] != 101 || got[1] != 102 {
		t.Fatalf("ccPIDsFromProcesses(mm1) = %+v, want [101 102]", got)
	}
	if _, ok := pids["mm2"]; ok {
		t.Fatalf("ccPIDsFromProcesses should omit machines without Claude Code: %+v", pids)
	}
}

func TestRowCountAcrossPanels(t *testing.T) {
	m := viewTestModel(t)
	tests := []struct {
		panel panel
		want  int
	}{
		{panelMachines, 2},
		{panelSessions, 2},
		{panelTunnels, 1},
		{panelProcesses, 2},
	}
	for _, tt := range tests {
		m.activePanel = tt.panel
		if got := m.rowCount(); got != tt.want {
			t.Fatalf("rowCount(%v) = %d, want %d", tt.panel, got, tt.want)
		}
	}
}

func TestTabNavigationAndWindowSize(t *testing.T) {
	m := viewTestModel(t)
	m.selectedRow = 1

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.activePanel != panelSessions || m.selectedRow != 0 {
		t.Fatalf("tab state = panel %v row %d, want sessions row 0", m.activePanel, m.selectedRow)
	}

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.activePanel != panelMachines || m.selectedRow != 0 {
		t.Fatalf("shift-tab state = panel %v row %d, want machines row 0", m.activePanel, m.selectedRow)
	}

	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	next := result.(model)
	if next.width != 120 || next.height != 40 {
		t.Fatalf("window size = %dx%d, want 120x40", next.width, next.height)
	}
}

func viewTestModel(t *testing.T) model {
	t.Helper()
	cfg := &config.Config{
		Settings: config.Settings{PollInterval: 5, SwapWarnMB: 1024, SwapHighMB: 4096},
		Machines: []config.Machine{
			{Name: viewMachineName, Host: viewMachineName, Enabled: true},
			{Name: "mm2", Host: "mm2", Enabled: true},
		},
	}
	started := time.Now().Add(-5 * time.Minute)
	m := NewModel(cfg, t.TempDir()+"/state.json")
	m.width = 100
	m.height = 40
	m.healths = []machine.Health{
		{Name: viewMachineName, Online: true, TotalMemory: 16 << 30, AvailMemory: 8 << 30, ClaudeCount: 1},
		{Name: "mm2", Online: false},
	}
	m.state = &session.State{
		Sessions: []session.Session{
			{ID: "s1", Project: "org/repo", Machine: viewMachineName, Branch: "main", StartedAt: started, Tunnel: session.TunnelInfo{LocalPort: 4001, RemotePort: 3000}},
			{ID: "s2", Project: "org/other", Machine: "mm2", Branch: "main", StartedAt: started},
		},
		MachineLabels: map[string][]session.MachineLabel{
			viewMachineName: {{Name: "repo", SessionID: "s1"}},
		},
	}
	m.processes = map[string][]machine.ProcessGroup{
		viewMachineName: {
			{Name: "Claude Code", Count: 2, TotalRSS: 256 * 1024, PIDs: []int{101, 102}, Killable: true},
			{Name: "Dev Servers", Count: 1, TotalRSS: 128 * 1024, PIDs: []int{201}, Killable: true, Detail: "vite"},
		},
		"mm2": {{Name: "Codex", Count: 1, TotalRSS: 64 * 1024, PIDs: []int{301}, Killable: true}},
	}
	return m
}
