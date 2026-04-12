package session

import (
	"fmt"

	"github.com/neonwatty/fleet/internal/config"
)

// ResolveAccount returns the account to stamp on a new session. An explicit
// --account flag wins; otherwise the machine's DefaultAccount is used; otherwise
// the result is empty.
func ResolveAccount(explicit string, m config.Machine) string {
	if explicit != "" {
		return explicit
	}
	return m.DefaultAccount
}

// SetSessionAccount updates the Account field of an existing session by ID.
// Returns an error if the session is not found.
func SetSessionAccount(statePath, sessionID, account string) error {
	s, err := LoadState(statePath)
	if err != nil {
		return err
	}
	for i := range s.Sessions {
		if s.Sessions[i].ID == sessionID {
			s.Sessions[i].Account = account
			return Save(statePath, s)
		}
	}
	return fmt.Errorf("session %q not found", sessionID)
}
