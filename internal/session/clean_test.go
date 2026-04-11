package session

import (
	"testing"
)

func TestClassifySessions(t *testing.T) {
	sessions := []Session{
		{ID: "alive", Machine: "mm1", WorktreePath: "/exists"},
		{ID: "orphan", Machine: "mm1", WorktreePath: "/exists-no-proc"},
		{ID: "stale", Machine: "mm1", WorktreePath: "/gone"},
	}

	checker := func(sess Session) SessionStatus {
		switch sess.ID {
		case "alive":
			return StatusAlive
		case "orphan":
			return StatusOrphan
		case "stale":
			return StatusStale
		default:
			return StatusStale
		}
	}

	alive, orphan, stale := ClassifySessions(sessions, checker)
	if len(alive) != 1 || alive[0].ID != "alive" {
		t.Errorf("alive = %v, want [alive]", ids(alive))
	}
	if len(orphan) != 1 || orphan[0].ID != "orphan" {
		t.Errorf("orphan = %v, want [orphan]", ids(orphan))
	}
	if len(stale) != 1 || stale[0].ID != "stale" {
		t.Errorf("stale = %v, want [stale]", ids(stale))
	}
}

func ids(sessions []Session) []string {
	out := make([]string, len(sessions))
	for i, s := range sessions {
		out[i] = s.ID
	}
	return out
}
