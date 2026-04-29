package session

import "path/filepath"

func bareRepoPathForSession(sess Session) string {
	if sess.BareRepoPath != "" {
		return sess.BareRepoPath
	}
	org, repo := splitProject(sess.Project)
	return filepath.Join("~", "fleet-repos", org, repo+".git")
}
