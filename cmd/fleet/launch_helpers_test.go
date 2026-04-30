package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/session"
)

func TestChooseTargetMachine(t *testing.T) {
	machines := []config.Machine{
		{Name: "mm1", Host: "mm1", Enabled: true},
		{Name: "mm2", Host: "mm2", Enabled: true},
	}

	got, err := chooseTargetMachine(machines, "mm2")
	if err != nil {
		t.Fatalf("chooseTargetMachine() error: %v", err)
	}
	if got.Name != "mm2" {
		t.Fatalf("chosen machine = %q, want mm2", got.Name)
	}

	_, err = chooseTargetMachine(machines, "missing")
	if err == nil || !strings.Contains(err.Error(), "not found or not enabled") {
		t.Fatalf("chooseTargetMachine() error = %v, want not found", err)
	}
}

func TestAddLaunchLabelWritesLinkedLabel(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	addLaunchLabel(statePath, config.Machine{Name: "mm1"}, "bleep", "s1")

	loaded, err := session.LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	labels := loaded.MachineLabels["mm1"]
	if len(labels) != 1 || labels[0].Name != "bleep" || labels[0].SessionID != "s1" {
		t.Fatalf("labels = %+v, want linked bleep label", labels)
	}
}

func TestPrintLaunchResultIncludesRemoteTunnelOnly(t *testing.T) {
	result := &session.LaunchResult{
		Session: session.Session{
			ID:        "s1",
			StartedAt: time.Now(),
			Tunnel:    session.TunnelInfo{LocalPort: 4001, RemotePort: 3000},
		},
	}

	out := captureStdout(t, func() {
		printLaunchResult(config.Machine{Name: "mm1", Host: "mm1"}, result)
	})
	if !strings.Contains(out, "Tunnel: localhost:4001") || !strings.Contains(out, "Session s1 started") {
		t.Fatalf("remote launch output missing tunnel/session:\n%s", out)
	}

	out = captureStdout(t, func() {
		printLaunchResult(config.Machine{Name: "local", Host: "localhost"}, result)
	})
	if strings.Contains(out, "Tunnel:") {
		t.Fatalf("local launch output should omit tunnel:\n%s", out)
	}
}

func TestFindMachineFallback(t *testing.T) {
	machines := []config.Machine{{Name: "mm1"}, {Name: "mm2"}}
	if got := findMachine(machines, "mm2"); got.Name != "mm2" {
		t.Fatalf("findMachine existing = %q, want mm2", got.Name)
	}
	if got := findMachine(machines, "missing"); got.Name != "mm1" {
		t.Fatalf("findMachine fallback = %q, want first machine", got.Name)
	}
}
