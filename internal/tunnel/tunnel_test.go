package tunnel

import (
	"os/exec"
	"testing"
)

func TestAllocatePort(t *testing.T) {
	used := map[int]bool{4000: true, 4001: true}
	port, err := AllocatePort(4000, 4999, used)
	if err != nil {
		t.Fatalf("AllocatePort() error: %v", err)
	}
	if port != 4002 {
		t.Errorf("AllocatePort() = %d, want 4002", port)
	}
}

func TestAllocatePortPinned(t *testing.T) {
	used := map[int]bool{}
	port, err := AllocatePortPinned(3000, used)
	if err != nil {
		t.Fatalf("AllocatePortPinned() error: %v", err)
	}
	if port != 3000 {
		t.Errorf("AllocatePortPinned() = %d, want 3000", port)
	}
}

func TestAllocatePortPinnedConflict(t *testing.T) {
	used := map[int]bool{3000: true}
	_, err := AllocatePortPinned(3000, used)
	if err == nil {
		t.Error("expected error for pinned port conflict")
	}
}

func TestAllocatePortExhausted(t *testing.T) {
	used := map[int]bool{4000: true, 4001: true, 4002: true}
	_, err := AllocatePort(4000, 4002, used)
	if err == nil {
		t.Error("expected error when all ports used")
	}
}

func TestStopKillsAndWaitsForProcess(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}

	tun := &Tunnel{Cmd: cmd}
	if err := tun.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if cmd.ProcessState == nil {
		t.Fatal("Stop() should wait for process and populate ProcessState")
	}
}
