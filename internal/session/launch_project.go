package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/tunnel"
)

type projectSpec struct {
	CloneURL  string
	Repo      string
	PathParts []string
}

func parseProjectSpec(project string) (projectSpec, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		return projectSpec{}, fmt.Errorf("project is required")
	}

	if strings.Contains(project, "://") || strings.HasPrefix(project, "git@") || strings.HasPrefix(project, "ssh://") {
		return parseURLProjectSpec(project)
	}

	parts := splitCleanPath(project)
	if len(parts) < 2 {
		return projectSpec{}, fmt.Errorf("project must be org/repo or a git URL")
	}
	repo := parts[len(parts)-1]
	return projectSpec{
		CloneURL:  fmt.Sprintf("https://github.com/%s.git", project),
		Repo:      stripGitSuffix(repo),
		PathParts: parts[:len(parts)-1],
	}, nil
}

func parseURLProjectSpec(project string) (projectSpec, error) {
	parts := pathPartsFromProject(project)
	if len(parts) == 0 {
		return projectSpec{}, fmt.Errorf("project URL must include a repository path")
	}
	repo := stripGitSuffix(parts[len(parts)-1])
	if repo == "" {
		return projectSpec{}, fmt.Errorf("project URL must include a repository name")
	}
	return projectSpec{
		CloneURL:  project,
		Repo:      repo,
		PathParts: parts[:len(parts)-1],
	}, nil
}

func splitProject(project string) (org, repo string) {
	spec, err := parseProjectSpec(project)
	if err != nil {
		return "", ""
	}
	if len(spec.PathParts) > 0 {
		return spec.PathParts[0], spec.Repo
	}
	return "", spec.Repo
}

func pathPartsFromProject(project string) []string {
	pathish := project
	if i := strings.Index(pathish, "://"); i >= 0 {
		pathish = pathish[i+3:]
		if slash := strings.Index(pathish, "/"); slash >= 0 {
			pathish = pathish[slash+1:]
		}
	} else if strings.HasPrefix(pathish, "git@") {
		if colon := strings.Index(pathish, ":"); colon >= 0 {
			pathish = pathish[colon+1:]
		}
	}
	parts := splitCleanPath(pathish)
	if len(parts) > 1 {
		parts[len(parts)-1] = stripGitSuffix(parts[len(parts)-1])
	}
	return parts
}

func splitCleanPath(path string) []string {
	raw := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return []string{"repo"}
	}
	return parts
}

func stripGitSuffix(repo string) string {
	return strings.TrimSuffix(repo, ".git")
}

func expandRemotePath(path string, m config.Machine) string {
	if m.IsLocal() {
		return config.ExpandPath(path)
	}
	if strings.HasPrefix(path, "~/") {
		return path // SSH expands ~ on the remote side
	}
	return path
}

func resolveLaunchCommand(explicit string, project string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	if strings.TrimSpace(project) != "" {
		return strings.TrimSpace(project)
	}
	return "claude"
}

func detectRemoteProjectConfig(
	ctx context.Context,
	m config.Machine,
	worktree string,
	run func(context.Context, config.Machine, string) (string, error),
) tunnel.ProjectConfig {
	if cfg, ok := detectRemoteProjectFile(ctx, m, worktree, ".fleet.toml", run); ok {
		return cfg
	}
	if cfg, ok := detectRemoteProjectFile(ctx, m, worktree, "package.json", run); ok {
		return cfg
	}
	return tunnel.ProjectConfig{DevPort: 3000}
}

func detectRemoteProjectFile(
	ctx context.Context,
	m config.Machine,
	worktree string,
	name string,
	run func(context.Context, config.Machine, string) (string, error),
) (tunnel.ProjectConfig, bool) {
	catCmd := fmt.Sprintf("cat %s 2>/dev/null || true", shellQuotePath(filepath.Join(worktree, name)))
	content, _ := run(ctx, m, catCmd)
	if strings.TrimSpace(content) == "" || (name == "package.json" && !strings.Contains(content, "scripts")) {
		return tunnel.ProjectConfig{}, false
	}

	tmpDir, err := os.MkdirTemp("", "fleet-detect-*")
	if err != nil {
		return tunnel.ProjectConfig{DevPort: 3000}, true
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck
	_ = os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
	return tunnel.DetectProjectConfig(tmpDir), true
}
