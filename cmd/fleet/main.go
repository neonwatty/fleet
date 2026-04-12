package main

import (
	"context"
	"fmt"
	"os"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
	"github.com/neonwatty/fleet/internal/tui"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "fleet",
		Short:   "Distribute Claude Code instances across your local Mac fleet",
		Version: version,
	}

	root.AddCommand(launchCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(cleanCmd())
	root.AddCommand(labelCmd())
	root.AddCommand(accountCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func launchCmd() *cobra.Command {
	var branch string
	var target string
	var account string
	var label string

	cmd := &cobra.Command{
		Use:   "launch <org/repo>",
		Short: "Launch Claude Code on the best available machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			ctx := context.Background()

			cfg, err := config.Load(config.DefaultPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			enabled := cfg.EnabledMachines()
			if len(enabled) == 0 {
				return fmt.Errorf("no enabled machines in config")
			}

			var chosen config.Machine

			if target != "" {
				found := false
				for _, m := range enabled {
					if m.Name == target {
						chosen = m
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("machine %q not found or not enabled", target)
				}
				fmt.Printf("Using specified target: %s\n", chosen.Name)
			} else {
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
					return fmt.Errorf("no machines are reachable")
				}

				if score < float64(cfg.Settings.StressThreshold) {
					fmt.Printf("\nAll machines are stressed. Best: %s (score: %.1f)\n", best.Name, score)
					fmt.Print("Launch anyway? [y/n]: ")
					var answer string
					_, _ = fmt.Scanln(&answer)
					if answer != "y" && answer != "Y" {
						fmt.Println("Aborted.")
						return nil
					}
				}

				chosen = findMachine(enabled, best.Name)
				fmt.Printf("\nSelected: %s (score: %.1f)\n", chosen.Name, score)
			}

			fmt.Printf("Setting up %s on %s...\n", project, chosen.Name)

			result, err := session.Launch(ctx, session.LaunchOpts{
				Project:   project,
				Branch:    branch,
				Account:   account,
				Machine:   chosen,
				Settings:  cfg.Settings,
				StatePath: session.DefaultStatePath(),
			})
			if err != nil {
				return fmt.Errorf("launch: %w", err)
			}

			if label != "" {
				if err := session.AddLabel(
					session.DefaultStatePath(),
					chosen.Name,
					label,
					result.Session.ID,
					result.Session.PID,
				); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to add label: %v\n", err)
				}
			}

			if result.Session.Tunnel.LocalPort > 0 && !chosen.IsLocal() {
				fmt.Printf("Tunnel: localhost:%d → %s:%d\n",
					result.Session.Tunnel.LocalPort, chosen.Name, result.Session.Tunnel.RemotePort)
			}

			fmt.Printf("Session %s started. Launching Claude Code...\n\n", result.Session.ID)

			return session.WithSignalCleanup(
				ctx, chosen, result.Session, result.Tunnel,
				session.DefaultStatePath(),
				func() error {
					return session.ExecClaude(chosen, result.Session.WorktreePath)
				},
			)
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "main", "Branch to check out")
	cmd.Flags().StringVarP(&target, "target", "t", "", "Force a specific machine")
	cmd.Flags().StringVar(&account, "account", "", "Claude account label for this session (falls back to machine default)")
	cmd.Flags().StringVar(&label, "name", "", "Nickname to attach to the machine (creates a linked label)")
	return cmd
}

func statusCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show fleet dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.DefaultPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if jsonOut {
				return runStatusJSON(cfg)
			}
			return tui.Run(cfg, session.DefaultStatePath())
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit fleet status as JSON and exit (no TUI)")
	return cmd
}

func cleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Clean up orphaned worktrees and stale sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.DefaultPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return session.Clean(context.Background(), cfg, session.DefaultStatePath())
		},
	}
}

func findMachine(machines []config.Machine, name string) config.Machine {
	for _, m := range machines {
		if m.Name == name {
			return m
		}
	}
	return machines[0]
}
