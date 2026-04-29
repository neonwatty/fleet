package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
	"github.com/spf13/cobra"
)

var errLaunchAborted = errors.New("launch aborted")

func launchCmd(app *commandContext) *cobra.Command {
	var branch string
	var target string
	var account string
	var label string
	var launchCommand string

	cmd := &cobra.Command{
		Use:   "launch <org/repo|git-url>",
		Short: "Launch Claude Code on the best available machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			ctx := context.Background()

			cfg, err := app.loadConfig()
			if err != nil {
				return err
			}

			enabled := cfg.EnabledMachines()
			if len(enabled) == 0 {
				return fmt.Errorf("no enabled machines in config")
			}

			chosen, err := chooseLaunchMachine(ctx, enabled, target, cfg.Settings.StressThreshold)
			if err != nil {
				if errors.Is(err, errLaunchAborted) {
					return nil
				}
				return err
			}

			fmt.Printf("Setting up %s on %s...\n", project, chosen.Name)
			result, err := session.Launch(ctx, session.LaunchOpts{
				Project:       project,
				Branch:        branch,
				Account:       account,
				LaunchCommand: launchCommand,
				Machine:       chosen,
				Settings:      cfg.Settings,
				StatePath:     app.statePath,
			})
			if err != nil {
				return fmt.Errorf("launch: %w", err)
			}

			addLaunchLabel(app.statePath, chosen, label, result.Session.ID)
			printLaunchResult(chosen, result)

			return session.WithSignalCleanup(
				ctx, chosen, result.Session, result.Tunnel,
				app.statePath,
				func() error {
					return session.ExecCommand(chosen, result.Session.WorktreePath, result.LaunchCommand)
				},
			)
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "main", "Branch to check out")
	cmd.Flags().StringVarP(&target, "target", "t", "", "Force a specific machine")
	cmd.Flags().StringVar(&account, "account", "", "Claude account label for this session (falls back to machine default)")
	cmd.Flags().StringVar(&label, "name", "", "Nickname to attach to the machine (creates a linked label)")
	cmd.Flags().StringVar(&launchCommand, "cmd", "", "Command to run inside the worktree (defaults to .fleet.toml launch_command, then claude)")
	return cmd
}

func chooseLaunchMachine(
	ctx context.Context,
	enabled []config.Machine,
	target string,
	stressThreshold int,
) (config.Machine, error) {
	if target != "" {
		return chooseTargetMachine(enabled, target)
	}

	fmt.Println("Probing machines...")
	healths := machine.ProbeAll(ctx, enabled)
	for _, h := range healths {
		if h.Online {
			availPct := float64(h.AvailMemory) / float64(h.TotalMemory) * 100
			fmt.Printf("  %s: %.0f%% mem avail, %.0fMB swap, %d claude instances (score: %.1f)\n",
				h.Name, availPct, h.SwapUsedMB, h.ClaudeCount, machine.Score(h))
		} else {
			fmt.Printf("  %s: offline\n", h.Name)
		}
	}

	best, score := machine.PickBest(healths)
	if !best.Online {
		return config.Machine{}, fmt.Errorf("no machines are reachable")
	}
	if score < float64(stressThreshold) && !confirmStressedLaunch(best.Name, score) {
		return config.Machine{}, errLaunchAborted
	}

	chosen := findMachine(enabled, best.Name)
	fmt.Printf("\nSelected: %s (score: %.1f)\n", chosen.Name, score)
	return chosen, nil
}

func chooseTargetMachine(enabled []config.Machine, target string) (config.Machine, error) {
	for _, m := range enabled {
		if m.Name == target {
			fmt.Printf("Using specified target: %s\n", m.Name)
			return m, nil
		}
	}
	return config.Machine{}, fmt.Errorf("machine %q not found or not enabled", target)
}

func confirmStressedLaunch(name string, score float64) bool {
	fmt.Printf("\nAll machines are stressed. Best: %s (score: %.1f)\n", name, score)
	fmt.Print("Launch anyway? [y/n]: ")
	var answer string
	_, _ = fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		fmt.Println("Aborted.")
		return false
	}
	return true
}

func addLaunchLabel(statePath string, chosen config.Machine, label string, sessionID string) {
	if label == "" {
		return
	}
	if err := session.AddLabel(statePath, chosen.Name, label, sessionID, 0); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to add label: %v\n", err)
	}
}

func printLaunchResult(chosen config.Machine, result *session.LaunchResult) {
	if result.Session.Tunnel.LocalPort > 0 && !chosen.IsLocal() {
		fmt.Printf("Tunnel: localhost:%d → %s:%d\n",
			result.Session.Tunnel.LocalPort, chosen.Name, result.Session.Tunnel.RemotePort)
	}
	fmt.Printf("Session %s started. Launching Claude Code...\n\n", result.Session.ID)
}
