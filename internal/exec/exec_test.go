package exec

import (
	"context"
	"strings"
	"testing"
	"time"

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
	if !strings.Contains(joined, "neonwatty@mm1") {
		t.Errorf("expected user@host in args, got: %v", args)
	}
	if !strings.Contains(joined, "uname -a") {
		t.Errorf("expected command in args, got: %v", args)
	}
}

func TestBuildSSHCommandNonInteractive(t *testing.T) {
	// Verify SSH never prompts for a password — fleet must fail fast
	// when key auth doesn't work, not block on stdin.
	m := config.Machine{Name: "mm1", Host: "mm1"}
	args := buildSSHArgs(m, "echo test")
	joined := strings.Join(args, " ")

	required := []string{
		"BatchMode=yes",
		"PasswordAuthentication=no",
		"KbdInteractiveAuthentication=no",
		"ConnectTimeout=5",
	}
	for _, opt := range required {
		if !strings.Contains(joined, opt) {
			t.Errorf("expected %q in SSH args, got: %v", opt, args)
		}
	}
}

func TestRunWithTimeout(t *testing.T) {
	local := config.Machine{Name: "local", Host: "localhost"}

	_, err := RunWithTimeout(context.Background(), local, "sleep 10", time.Nanosecond)
	if err == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %q, want timeout message", err.Error())
	}
	if !strings.Contains(err.Error(), "machine local") {
		t.Fatalf("error = %q, want machine name", err.Error())
	}
}

func TestRunFailureIncludesMachineCommandAndStderr(t *testing.T) {
	local := config.Machine{Name: "local", Host: "localhost"}

	_, err := Run(context.Background(), local, "echo nope >&2; exit 7")
	if err == nil {
		t.Fatal("expected command error")
	}

	msg := err.Error()
	for _, want := range []string{"machine local", "echo nope", "stderr: nope"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error = %q, want to contain %q", msg, want)
		}
	}
}
