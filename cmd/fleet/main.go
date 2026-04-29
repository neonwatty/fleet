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

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type commandContext struct {
	configPath string
	statePath  string
}

func newCommandContext() *commandContext {
	return &commandContext{
		configPath: config.DefaultPath(),
		statePath:  session.DefaultStatePath(),
	}
}

func (c *commandContext) loadConfig() (*config.Config, error) {
	cfg, err := config.Load(c.configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

func main() {
	root := newRootCommand(newCommandContext())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand(ctx *commandContext) *cobra.Command {
	root := &cobra.Command{
		Use:     "fleet",
		Short:   "Distribute Claude Code instances across your local Mac fleet",
		Version: versionString(),
	}

	root.PersistentFlags().StringVar(&ctx.configPath, "config", ctx.configPath, "Config path")
	root.PersistentFlags().StringVar(&ctx.statePath, "state", ctx.statePath, "State path")

	root.AddCommand(launchCmd(ctx))
	root.AddCommand(statusCmd(ctx))
	root.AddCommand(cleanCmd(ctx))
	root.AddCommand(doctorCmd(ctx))
	root.AddCommand(labelCmd(ctx))
	root.AddCommand(accountCmd(ctx))
	root.AddCommand(initCmd(ctx))

	return root
}

func versionString() string {
	if commit == "none" && date == "unknown" {
		return version
	}
	return fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)
}

func initCmd(ctx *commandContext) *cobra.Command {
	var path string
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a starter fleet config",
		RunE: func(cmd *cobra.Command, args []string) error {
			target := path
			if target == "" {
				target = ctx.configPath
			}
			if err := config.WriteDefault(target, force); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Wrote config to %s\n", target); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Edit the machines list, then run: fleet doctor"); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "Config path to write (default ~/.fleet/config.toml)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing config")
	return cmd
}

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

			if label != "" {
				if err := session.AddLabel(
					app.statePath,
					chosen.Name,
					label,
					result.Session.ID,
					0, // OwnerPID is the fleet CLI PID, not the remote claude PID
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

func statusCmd(app *commandContext) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show fleet dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.loadConfig()
			if err != nil {
				return err
			}
			if jsonOut {
				return runStatusJSON(cfg, app.statePath)
			}
			return tui.Run(cfg, app.statePath)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit fleet status as JSON and exit (no TUI)")
	return cmd
}

func cleanCmd(app *commandContext) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean up orphaned worktrees and stale sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.loadConfig()
			if err != nil {
				return err
			}
			_, err = session.CleanWithOptions(context.Background(), cfg, app.statePath, session.CleanOptions{
				DryRun: dryRun,
			})
			return err
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report cleanup actions without changing state, worktrees, or tunnels")
	return cmd
}

func doctorCmd(app *commandContext) *cobra.Command {
	var target string
	var fix bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Inspect fleet state and optionally repair setup issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.loadConfig()
			if err != nil {
				return err
			}
			result, err := session.Doctor(context.Background(), cfg, app.statePath, session.DoctorOptions{
				Machine: target,
				Fix:     fix,
			})
			if err != nil {
				return err
			}
			if !result.OK() {
				return fmt.Errorf("doctor found issues")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&target, "machine", "m", "", "Inspect a single enabled machine")
	cmd.Flags().BoolVar(&fix, "fix", false, "Create missing configured base directories")
	return cmd
}

func findMachine(machines []config.Machine, name string) config.Machine {
	for _, m := range machines {
		if m.Name == name {
			return m
		}
	}
	return machines[0]
}
