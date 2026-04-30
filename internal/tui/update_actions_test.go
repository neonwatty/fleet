package tui

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
)

func restoreTUIDeps(t *testing.T) {
	t.Helper()
	origKillSession := killSessionFunc
	origOpenBrowser := openBrowserFunc
	origKillGroup := killGroupFunc
	origScanSwap := scanSwapFunc
	t.Cleanup(func() {
		killSessionFunc = origKillSession
		openBrowserFunc = origOpenBrowser
		killGroupFunc = origKillGroup
		scanSwapFunc = origScanSwap
	})
}

func TestConfirmKillSessionPerformsActionAndRefreshes(t *testing.T) {
	restoreTUIDeps(t)
	statePath := filepath.Join(t.TempDir(), "state.json")
	m := newRenameTestModel(t, statePath)
	m.confirming = true
	m.pendingAction = actionKillSession

	var killed session.Session
	killSessionFunc = func(_ context.Context, _ *config.Config, sess session.Session, _ string) error {
		killed = sess
		return nil
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := result.(model)

	if killed.ID != "abcd1234" {
		t.Fatalf("killed session ID = %q, want abcd1234", killed.ID)
	}
	if next.confirming || next.pendingAction != actionNone {
		t.Fatalf("confirmation state not cleared: confirming=%v pending=%v", next.confirming, next.pendingAction)
	}
	if next.statusMessage != "Killed session abcd1234" {
		t.Fatalf("statusMessage = %q", next.statusMessage)
	}
	if cmd == nil {
		t.Fatalf("expected refresh command after successful kill")
	}
}

func TestConfirmKillSessionReportsError(t *testing.T) {
	restoreTUIDeps(t)
	statePath := filepath.Join(t.TempDir(), "state.json")
	m := newRenameTestModel(t, statePath)
	m.confirming = true
	m.pendingAction = actionKillSession

	killSessionFunc = func(context.Context, *config.Config, session.Session, string) error {
		return errors.New("boom")
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := result.(model)

	if cmd != nil {
		t.Fatalf("expected no refresh command on failed kill")
	}
	if next.statusMessage != "Error: boom" {
		t.Fatalf("statusMessage = %q, want Error: boom", next.statusMessage)
	}
}

func TestOpenTunnelUsesSelectedTunnelPort(t *testing.T) {
	restoreTUIDeps(t)
	statePath := filepath.Join(t.TempDir(), "state.json")
	m := newRenameTestModel(t, statePath)
	m.activePanel = panelTunnels
	m.selectedRow = 1
	m.state.Sessions = []session.Session{
		{ID: "without-tunnel", Project: "p0", Machine: "mm1"},
		{ID: "first", Project: "p1", Machine: "mm1", Tunnel: session.TunnelInfo{LocalPort: 4001, RemotePort: 3000}},
		{ID: "second", Project: "p2", Machine: "mm2", Tunnel: session.TunnelInfo{LocalPort: 4002, RemotePort: 3000}},
	}

	var opened int
	openBrowserFunc = func(port int) error {
		opened = port
		return nil
	}

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if opened != 4002 {
		t.Fatalf("opened port = %d, want 4002", opened)
	}
}

func TestScanSwapCommandUsesConfiguredScanner(t *testing.T) {
	restoreTUIDeps(t)
	statePath := filepath.Join(t.TempDir(), "state.json")
	cfg := &config.Config{
		Settings: config.Settings{PollInterval: 5, SwapScanProcs: 3},
		Machines: []config.Machine{{Name: "mm1", Host: "mm1", Enabled: true}},
	}
	m := NewModel(cfg, statePath)
	m.activePanel = panelProcesses
	m.selectedMachine = 0
	m.healths = []machine.Health{{Name: "mm1", Online: true}}
	m.processes = map[string][]machine.ProcessGroup{
		"mm1": {{Name: "Dev Servers", PIDs: []int{1234}, Killable: true, TotalSwap: -1}},
	}

	var scannedMachine string
	var scannedMax int
	scanSwapFunc = func(_ context.Context, mach config.Machine, groups []machine.ProcessGroup, maxProcs int) []machine.ProcessGroup {
		scannedMachine = mach.Name
		scannedMax = maxProcs
		out := append([]machine.ProcessGroup{}, groups...)
		out[0].TotalSwap = 128 * 1024
		return out
	}

	result, cmd := m.handleScanSwap()
	next := result.(model)
	if !next.swapScanning || next.swapScanTarget != "mm1" {
		t.Fatalf("scan state not set: scanning=%v target=%q", next.swapScanning, next.swapScanTarget)
	}
	if cmd == nil {
		t.Fatalf("expected scan command")
	}

	msg := cmd()
	swapMsg, ok := msg.(swapScanMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want swapScanMsg", msg)
	}
	if scannedMachine != "mm1" || scannedMax != 3 {
		t.Fatalf("scan called with machine=%q max=%d", scannedMachine, scannedMax)
	}
	if swapMsg.groups[0].TotalSwap != 128*1024 {
		t.Fatalf("TotalSwap = %d, want %d", swapMsg.groups[0].TotalSwap, 128*1024)
	}
}

func TestConfirmKillProcessPerformsActionAndRefreshes(t *testing.T) {
	restoreTUIDeps(t)
	statePath := filepath.Join(t.TempDir(), "state.json")
	cfg := &config.Config{
		Settings: config.Settings{PollInterval: 5},
		Machines: []config.Machine{{Name: "mm1", Host: "mm1", Enabled: true}},
	}
	m := NewModel(cfg, statePath)
	m.activePanel = panelProcesses
	m.selectedRow = 0
	m.selectedMachine = 0
	m.confirming = true
	m.pendingAction = actionKillProcess
	m.healths = []machine.Health{{Name: "mm1", Online: true}}
	m.processes = map[string][]machine.ProcessGroup{
		"mm1": {{Name: "Dev Servers", PIDs: []int{1234}, Killable: true}},
	}

	var killed machine.ProcessGroup
	killGroupFunc = func(_ context.Context, _ config.Machine, group machine.ProcessGroup) error {
		killed = group
		return nil
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := result.(model)

	if killed.Name != "Dev Servers" {
		t.Fatalf("killed group = %q, want Dev Servers", killed.Name)
	}
	if next.statusMessage != "Killed Dev Servers on mm1" {
		t.Fatalf("statusMessage = %q", next.statusMessage)
	}
	if cmd == nil {
		t.Fatalf("expected refresh command after successful process kill")
	}
}
