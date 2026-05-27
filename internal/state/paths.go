package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

// ResolvedChangePaths captures the canonical runtime, archive, and governed
// bundle paths for a change.
type ResolvedChangePaths struct {
	ProjectRoot           string
	WorkspaceRoot         string
	ChangeDir             string
	CodebaseMapDir        string
	EvidenceDir           string
	GovernanceEvidenceDir string
	RunEvidenceDir        string
	GovernedBundleDir     string
	GovernedBundleArchive string
}

// ResolveChangePaths returns the canonical path set for the given change.
func ResolveChangePaths(root string, change model.Change) (ResolvedChangePaths, error) {
	projectRoot, err := NormalizePath(root)
	if err != nil {
		return ResolvedChangePaths{}, fmt.Errorf("normalize root: %w", err)
	}

	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return ResolvedChangePaths{}, fmt.Errorf("slug is required")
	}

	workspaceRoot, err := changeWorkspaceRoot(projectRoot, change)
	if err != nil {
		return ResolvedChangePaths{}, err
	}

	changeDir := ChangeDir(projectRoot, slug)
	evidenceDir := filepath.Join(changeDir, "evidence")

	return ResolvedChangePaths{
		ProjectRoot:           projectRoot,
		WorkspaceRoot:         workspaceRoot,
		ChangeDir:             changeDir,
		CodebaseMapDir:        filepath.Join(workspaceRoot, "artifacts", "codebase"),
		EvidenceDir:           evidenceDir,
		GovernanceEvidenceDir: filepath.Join(evidenceDir, "governance"),
		RunEvidenceDir:        filepath.Join(evidenceDir, "runs"),
		GovernedBundleDir:     filepath.Join(workspaceRoot, "artifacts", "changes", slug),
		GovernedBundleArchive: filepath.Join(workspaceRoot, "artifacts", "changes", "archived", slug),
	}, nil
}

// WorkspaceRootForChange resolves the authoritative workspace root for a
// change, accounting for worktree-bound scope roots.
func WorkspaceRootForChange(root string, change model.Change) (string, error) {
	projectRoot, err := NormalizePath(root)
	if err != nil {
		return "", fmt.Errorf("normalize root: %w", err)
	}
	return changeWorkspaceRoot(projectRoot, change)
}

// GovernedBundleDir resolves the canonical governed bundle root for the change.
func GovernedBundleDir(root string, change model.Change) (string, error) {
	paths, err := ResolveChangePaths(root, change)
	if err != nil {
		return "", err
	}
	return paths.GovernedBundleDir, nil
}

// ConfigPathForChange resolves the authoritative .slipway.yaml path for a
// change. Scope/worktree-local copies are discovery metadata only and must not
// become a second governance authority.
func ConfigPathForChange(root string, _ model.Change) (string, error) {
	projectRoot, err := NormalizePath(root)
	if err != nil {
		return "", err
	}
	return ConfigPath(projectRoot), nil
}

func changeWorkspaceRoot(projectRoot string, change model.Change) (string, error) {
	worktreePath := strings.TrimSpace(change.WorktreePath)
	if worktreePath == "" {
		// Worktree not yet bound. Fall back to project root.
		return projectRoot, nil
	}
	normalized, err := NormalizePath(worktreePath)
	if err != nil {
		return "", fmt.Errorf("normalize worktree_path: %w", err)
	}
	workspaceRoot, err := scopeRootInWorkspace(projectRoot, normalized)
	if err != nil {
		return "", fmt.Errorf("resolve scope root in worktree: %w", err)
	}
	return workspaceRoot, nil
}

// DisplayPath returns a project-relative slash path when the target is inside
// the project root, otherwise it returns the canonical absolute slash path.
func DisplayPath(root, target string) string {
	normalizedRoot, rootErr := NormalizePath(root)
	normalizedTarget, targetErr := NormalizePath(target)
	if rootErr == nil && targetErr == nil {
		if rel, err := filepath.Rel(normalizedRoot, normalizedTarget); err == nil {
			if rel == "." {
				return "."
			}
			if rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				return filepath.ToSlash(rel)
			}
		}
		return filepath.ToSlash(normalizedTarget)
	}
	return filepath.ToSlash(target)
}
