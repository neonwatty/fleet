package exec

import (
	"context"
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/config"
)

func TestRunLocal(t *testing.T) {
	local := config.Machine{Name: "local", Host: "localhost"}
	out, err := Run(context.Background(), local, "echo hello")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Errorf("Run() = %q, want %q", strings.TrimSpace(out), "hello")
	}
}

func TestRunLocalFailure(t *testing.T) {
	local := config.Machine{Name: "local", Host: "localhost"}
	_, err := Run(context.Background(), local, "false")
	if err == nil {
		t.Error("expected error from failing command")
	}
}

func TestBuildSSHCommand(t *testing.T) {
	m := config.Machine{Name: "mm1", Host: "mm1", User: "neonwatty"}
	args := buildSSHArgs(m, "uname -a")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "mm1") {
		t.Errorf("expected host in args, got: %v", args)
	}
	if !strings.Contains(joined, "uname -a") {
		t.Errorf("expected command in args, got: %v", args)
	}
}

func TestRunWithTimeout(t *testing.T) {
	local := config.Machine{Name: "local", Host: "localhost"}
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	_, err := Run(ctx, local, "sleep 10")
	if err == nil {
		t.Error("expected timeout error")
	}
}
