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
	Fix     bool
}

type DoctorResult struct {
	CheckedMachines int
	Issues          []string
	Clean           CleanResult
	Fixed           int
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
		check := checkDoctorMachine(ctx, cfg.Settings, m, opts.Fix)
		result.Issues = append(result.Issues, check.Issues...)
		result.Fixed += check.Fixed
	}

	cleanResult, err := CleanWithOptions(ctx, cfg, statePath, CleanOptions{DryRun: true})
	if err != nil {
		return result, err
	}
	result.Clean = cleanResult

	if len(result.Issues) == 0 {
		if result.Fixed > 0 {
			fmt.Printf("Doctor: fixed %d machine configuration issue(s).\n", result.Fixed)
		} else {
			fmt.Println("Doctor: no machine configuration issues found.")
		}
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

type doctorMachineCheck struct {
	Issues []string
	Fixed  int
}

func checkDoctorMachine(ctx context.Context, settings config.Settings, m config.Machine, fix bool) doctorMachineCheck {
	fmt.Printf("Machine %s:\n", m.Name)
	result := doctorMachineCheck{}

	if _, err := fleetexec.RunWithTimeout(ctx, m, "true", 5*time.Second); err != nil {
		issue := fmt.Sprintf("ssh reachability failed: %v", err)
		fmt.Printf("  SSH: error (%v)\n", err)
		fmt.Printf("  Hint: %s\n", doctorSSHRemediation(m))
		result.Issues = append(result.Issues, fmt.Sprintf("%s: %s", m.Name, issue))
		return result
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
			result.Issues = append(result.Issues, fmt.Sprintf("%s: %s", m.Name, issue))
			continue
		}
		if !doctorPathExists(ctx, m, check.path) {
			if fix {
				mkdirCmd := fmt.Sprintf("mkdir -p %s", shellQuotePath(check.path))
				if _, err := fleetexec.RunWithTimeout(ctx, m, mkdirCmd, 10*time.Second); err != nil {
					issue := fmt.Sprintf("%s create failed: %s", check.name, check.path)
					fmt.Printf("  %s: error (%s: %v)\n", check.name, issue, err)
					result.Issues = append(result.Issues, fmt.Sprintf("%s: %s", m.Name, issue))
					continue
				}
				result.Fixed++
				fmt.Printf("  %s: created (%s)\n", check.name, check.path)
				continue
			}
			issue := fmt.Sprintf("%s missing or inaccessible: %s", check.name, check.path)
			fmt.Printf("  %s: error (%s)\n", check.name, issue)
			result.Issues = append(result.Issues, fmt.Sprintf("%s: %s", m.Name, issue))
			continue
		}
		fmt.Printf("  %s: ok (%s)\n", check.name, check.path)
	}

	return result
}

func doctorSSHRemediation(m config.Machine) string {
	return fmt.Sprintf("fix SSH for %s or set enabled = false for %q in config.toml", m.SSHTarget(), m.Name)
}

func doctorPathExists(ctx context.Context, m config.Machine, path string) bool {
	cmd := fmt.Sprintf("test -d %s", shellQuotePath(path))
	_, err := fleetexec.RunWithTimeout(ctx, m, cmd, 5*time.Second)
	return err == nil
}
