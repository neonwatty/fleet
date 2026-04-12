package session

import (
	"fmt"
	"time"
)

// AddLabel creates or updates a label on the given machine. If a label with
// the same name already exists it is overwritten (last-write-wins). sessionID
// and lastSeenPID are optional (pass "" and 0 for an orphan label).
func AddLabel(statePath, machineName, labelName, sessionID string, lastSeenPID int) error {
	if labelName == "" {
		return fmt.Errorf("label name is required")
	}
	s, err := LoadState(statePath)
	if err != nil {
		return err
	}
	if s.MachineLabels == nil {
		s.MachineLabels = make(map[string][]MachineLabel)
	}

	label := MachineLabel{
		Name:        labelName,
		SessionID:   sessionID,
		CreatedAt:   time.Now().UTC(),
		LastSeenPID: lastSeenPID,
	}

	existing := s.MachineLabels[machineName]
	replaced := false
	for i := range existing {
		if existing[i].Name == labelName {
			existing[i] = label
			replaced = true
			break
		}
	}
	if !replaced {
		existing = append(existing, label)
	}
	s.MachineLabels[machineName] = existing

	return Save(statePath, s)
}

// RemoveLabel removes a single label by name from a machine. No error if the
// label does not exist (idempotent).
func RemoveLabel(statePath, machineName, labelName string) error {
	s, err := LoadState(statePath)
	if err != nil {
		return err
	}
	if s.MachineLabels == nil {
		return nil
	}
	existing := s.MachineLabels[machineName]
	filtered := existing[:0]
	for _, l := range existing {
		if l.Name != labelName {
			filtered = append(filtered, l)
		}
	}
	s.MachineLabels[machineName] = filtered
	return Save(statePath, s)
}

// ClearLabels removes all labels from a machine.
func ClearLabels(statePath, machineName string) error {
	s, err := LoadState(statePath)
	if err != nil {
		return err
	}
	if s.MachineLabels == nil {
		return nil
	}
	delete(s.MachineLabels, machineName)
	return Save(statePath, s)
}

// ListLabels returns a copy of the labels for a machine, or nil if none.
func ListLabels(statePath, machineName string) ([]MachineLabel, error) {
	s, err := LoadState(statePath)
	if err != nil {
		return nil, err
	}
	if s.MachineLabels == nil {
		return nil, nil
	}
	out := make([]MachineLabel, len(s.MachineLabels[machineName]))
	copy(out, s.MachineLabels[machineName])
	return out, nil
}
