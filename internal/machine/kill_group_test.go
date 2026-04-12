package machine

import (
	"context"
	"fmt"
	"testing"

	"github.com/neonwatty/fleet/internal/config"
)

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
