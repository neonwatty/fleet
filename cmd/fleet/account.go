package main

import (
	"fmt"

	"github.com/neonwatty/fleet/internal/session"
	"github.com/spf13/cobra"
)

func accountCmd(app *commandContext) *cobra.Command {
	var clear bool

	cmd := &cobra.Command{
		Use:   "account <session-id> [name]",
		Short: "Set or clear the Claude account assigned to a session",
		Long: `Set or clear the Claude account label on an existing session.

Session IDs are matched exactly.

Examples:
  fleet account a1b2c3d4 personal-max
  fleet account a1b2c3d4 --clear
`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]
			statePath := app.statePath
			if clear {
				if len(args) != 1 {
					return fmt.Errorf("--clear takes no account name")
				}
				return session.SetSessionAccount(statePath, sessionID, "")
			}
			if len(args) != 2 {
				return fmt.Errorf("account name is required (or use --clear)")
			}
			return session.SetSessionAccount(statePath, sessionID, args[1])
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "Unset the account on this session")
	return cmd
}
