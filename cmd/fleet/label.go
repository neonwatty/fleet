package main

import (
	"fmt"
	"sort"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/session"
	"github.com/spf13/cobra"
)

func labelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Manage machine-scoped session labels",
	}
	cmd.AddCommand(labelSetCmd())
	cmd.AddCommand(labelListCmd())
	return cmd
}

func labelSetCmd() *cobra.Command {
	var remove bool
	var clear bool
	var sessionID string

	cmd := &cobra.Command{
		Use:   "set <machine> [name]",
		Short: "Add, remove, or clear labels on a machine",
		Long: `Add a new label, remove one, or clear all labels from a machine.

Examples:
  fleet label set mm1 bleep                    # add orphan label
  fleet label set mm1 bleep --session a1b2c3   # add label linked to a session
  fleet label set mm1 bleep --remove           # remove a single label
  fleet label set mm1 --clear                  # remove all labels on mm1
`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.DefaultPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if err := assertKnownMachine(cfg, args[0]); err != nil {
				return err
			}

			statePath := session.DefaultStatePath()
			machineName := args[0]

			if clear {
				if len(args) != 1 {
					return fmt.Errorf("--clear takes no label name")
				}
				return session.ClearLabels(statePath, machineName)
			}
			if len(args) != 2 {
				return fmt.Errorf("label name is required (or use --clear)")
			}
			name := args[1]
			if remove {
				return session.RemoveLabel(statePath, machineName, name)
			}
			return session.AddLabel(statePath, machineName, name, sessionID, 0)
		},
	}
	cmd.Flags().BoolVar(&remove, "remove", false, "Remove the given label")
	cmd.Flags().BoolVar(&clear, "clear", false, "Remove all labels on this machine")
	cmd.Flags().StringVar(&sessionID, "session", "", "Link the label to a session ID")
	return cmd
}

func labelListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [machine]",
		Short: "List labels across the fleet or on one machine",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			statePath := session.DefaultStatePath()
			state, err := session.LoadState(statePath)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}
			if state.MachineLabels == nil {
				fmt.Println("(no labels)")
				return nil
			}
			machines := make([]string, 0, len(state.MachineLabels))
			for name := range state.MachineLabels {
				if len(args) == 1 && name != args[0] {
					continue
				}
				machines = append(machines, name)
			}
			sort.Strings(machines)
			for _, name := range machines {
				fmt.Printf("%s:\n", name)
				for _, l := range state.MachineLabels[name] {
					linkage := "(orphan)"
					if l.SessionID != "" {
						linkage = "session=" + l.SessionID
					}
					fmt.Printf("  - %s  %s\n", l.Name, linkage)
				}
			}
			return nil
		},
	}
}

func assertKnownMachine(cfg *config.Config, name string) error {
	for _, m := range cfg.Machines {
		if m.Name == name {
			return nil
		}
	}
	known := make([]string, 0, len(cfg.Machines))
	for _, m := range cfg.Machines {
		known = append(known, m.Name)
	}
	return fmt.Errorf("unknown machine %q; known: %v", name, known)
}
