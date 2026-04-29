package session

import "testing"

func TestBareRepoPathForSessionUsesStoredPath(t *testing.T) {
	sess := Session{
		Project:      "org/repo",
		BareRepoPath: "/tmp/custom repos/org/repo.git",
	}

	if got := bareRepoPathForSession(sess); got != sess.BareRepoPath {
		t.Errorf("bareRepoPathForSession() = %q, want stored path %q", got, sess.BareRepoPath)
	}
}

func TestBareRepoPathForSessionLegacyFallback(t *testing.T) {
	sess := Session{Project: "org/repo"}
	want := "~/fleet-repos/org/repo.git"

	if got := bareRepoPathForSession(sess); got != want {
		t.Errorf("bareRepoPathForSession() = %q, want legacy fallback %q", got, want)
	}
}
