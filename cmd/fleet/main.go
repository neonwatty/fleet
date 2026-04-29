package main

import (
	"context"
	"fmt"
	"os"

	"github.com/neonwatty/fleet/internal/config"
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
