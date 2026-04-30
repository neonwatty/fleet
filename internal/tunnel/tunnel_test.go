package tunnel

import (
	"errors"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/config"
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

func TestAllocatePortSkipsPortInUseByProcess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close() //nolint:errcheck

	port := ln.Addr().(*net.TCPAddr).Port
	got, err := AllocatePort(port, port+1, map[int]bool{})
	if err != nil {
		t.Fatalf("AllocatePort() error: %v", err)
	}
	if got != port+1 {
		t.Fatalf("AllocatePort() = %d, want %d", got, port+1)
	}
}

func TestAllocatePortPinnedRejectsPortInUseByProcess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close() //nolint:errcheck

	port := ln.Addr().(*net.TCPAddr).Port
	_, err = AllocatePortPinned(port, map[int]bool{})
	if err == nil || !strings.Contains(err.Error(), "another process") {
		t.Fatalf("AllocatePortPinned() error = %v, want process conflict", err)
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

func TestBuildSSHForwardArgsUsesConfiguredUser(t *testing.T) {
	args := buildSSHForwardArgs(config.Machine{Host: "mm1", User: "neonwatty"}, "4000:localhost:3000")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "4000:localhost:3000") {
		t.Fatalf("args missing forward spec: %v", args)
	}
	if !strings.Contains(joined, "neonwatty@mm1") {
		t.Fatalf("args missing user@host: %v", args)
	}
}

func TestStartReturnsCommandStartError(t *testing.T) {
	orig := sshCommandPath
	sshCommandPath = filepath.Join(t.TempDir(), "missing-ssh")
	t.Cleanup(func() { sshCommandPath = orig })

	_, err := Start(config.Machine{Name: "mm1", Host: "mm1"}, 1, 3000)
	if err == nil || !strings.Contains(err.Error(), "start tunnel") {
		t.Fatalf("Start() error = %v, want start tunnel error", err)
	}
}

func TestVerifyStartedReturnsProcessExit(t *testing.T) {
	done := make(chan error, 1)
	done <- errors.New("exit status 42")

	err := verifyStarted(done, 1)
	if err == nil || !strings.Contains(err.Error(), "tunnel exited during startup") {
		t.Fatalf("verifyStarted() error = %v, want startup exit error", err)
	}
}
