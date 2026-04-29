package session

import (
	"context"
	"fmt"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	fleetexec "github.com/neonwatty/fleet/internal/exec"
)

type DoctorOptions struct {
	Machine string
}

type DoctorResult struct {
	CheckedMachines int
	Issues          []string
	Clean           CleanResult
}

func (r DoctorResult) OK() bool {
	return len(r.Issues) == 0 && r.Clean.Cleaned() == 0 && r.Clean.ResetLabels == 0
}

func Doctor(ctx context.Context, cfg *config.Config, statePath string, opts DoctorOptions) (DoctorResult, error) {
	result := DoctorResult{}

	fmt.Println("Config: ok")
	state, err := LoadState(statePath)
	if err != nil {
		return result, fmt.Errorf("state: %w", err)
	}
	fmt.Printf("State: ok (%d session(s), %d machine label set(s))\n", len(state.Sessions), len(state.MachineLabels))

	machines, err := doctorMachines(cfg, opts.Machine)
	if err != nil {
		return result, err
	}
	result.CheckedMachines = len(machines)

	for _, m := range machines {
		result.Issues = append(result.Issues, checkDoctorMachine(ctx, cfg.Settings, m)...)
	}

	cleanResult, err := CleanWithOptions(ctx, cfg, statePath, CleanOptions{DryRun: true})
	if err != nil {
		return result, err
	}
	result.Clean = cleanResult

	if len(result.Issues) == 0 {
		fmt.Println("Doctor: no machine configuration issues found.")
	} else {
		fmt.Printf("Doctor: found %d machine configuration issue(s).\n", len(result.Issues))
	}
	return result, nil
}

func doctorMachines(cfg *config.Config, target string) ([]config.Machine, error) {
	enabled := cfg.EnabledMachines()
	if target == "" {
		return enabled, nil
	}
	for _, m := range enabled {
		if m.Name == target {
			return []config.Machine{m}, nil
		}
	}
	return nil, fmt.Errorf("machine %q not found or not enabled", target)
}

func checkDoctorMachine(ctx context.Context, settings config.Settings, m config.Machine) []string {
	fmt.Printf("Machine %s:\n", m.Name)
	var issues []string

	if _, err := fleetexec.RunWithTimeout(ctx, m, "true", 5*time.Second); err != nil {
		issue := fmt.Sprintf("ssh reachability failed: %v", err)
		fmt.Printf("  SSH: error (%v)\n", err)
		issues = append(issues, fmt.Sprintf("%s: %s", m.Name, issue))
		return issues
	}
	fmt.Println("  SSH: ok")

	checks := []struct {
		name string
		path string
	}{
		{name: "worktree base", path: expandRemotePath(settings.WorktreeBase, m)},
		{name: "bare repo base", path: expandRemotePath(settings.BareRepoBase, m)},
	}
	for _, check := range checks {
		if check.path == "" {
			issue := fmt.Sprintf("%s path is empty", check.name)
			fmt.Printf("  %s: error (%s)\n", check.name, issue)
			issues = append(issues, fmt.Sprintf("%s: %s", m.Name, issue))
			continue
		}
		cmd := fmt.Sprintf("test -d %s", shellQuotePath(check.path))
		if _, err := fleetexec.RunWithTimeout(ctx, m, cmd, 5*time.Second); err != nil {
			issue := fmt.Sprintf("%s missing or inaccessible: %s", check.name, check.path)
			fmt.Printf("  %s: error (%s)\n", check.name, issue)
			issues = append(issues, fmt.Sprintf("%s: %s", m.Name, issue))
			continue
		}
		fmt.Printf("  %s: ok (%s)\n", check.name, check.path)
	}

	return issues
}
