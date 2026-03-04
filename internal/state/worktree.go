package state

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/signalridge/speclane/internal/model"
)

func PersistScopeWorktreeMetadata(change *model.ChangeState, worktreePath, worktreeBranch string) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}
	if strings.TrimSpace(worktreePath) == "" {
		return fmt.Errorf("worktree_path is required")
	}
	if strings.TrimSpace(worktreeBranch) == "" {
		return fmt.Errorf("worktree_branch is required")
	}
	change.WorktreePath = worktreePath
	change.WorktreeBranch = worktreeBranch
	return nil
}

func ValidateWorktreeAuthenticity(repoRoot, worktreePath, expectedBranch string) error {
	normalizedPath, err := normalizePath(worktreePath)
	if err != nil {
		return fmt.Errorf("invalid worktree_path: %w", err)
	}
	if strings.TrimSpace(expectedBranch) == "" {
		return fmt.Errorf("worktree_branch is required")
	}

	stat, err := os.Stat(normalizedPath)
	if err != nil {
		return fmt.Errorf("worktree_path does not exist: %w", err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("worktree_path is not a directory: %q", normalizedPath)
	}

	registered, err := listGitWorktrees(repoRoot)
	if err != nil {
		return err
	}
	if _, exists := registered[normalizedPath]; !exists {
		return fmt.Errorf("worktree_path is not a registered git worktree: %q", normalizedPath)
	}

	cmd := exec.Command("git", "-C", normalizedPath, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("resolve worktree branch: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	actualBranch := strings.TrimSpace(string(out))
	if actualBranch != expectedBranch {
		return fmt.Errorf("worktree branch mismatch: expected %q got %q", expectedBranch, actualBranch)
	}
	return nil
}

func listGitWorktrees(repoRoot string) (map[string]struct{}, error) {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list git worktrees: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	worktrees := map[string]struct{}{}
	lines := bytes.Split(out, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("worktree ")) {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(string(line), "worktree "))
		normalized, err := normalizePath(path)
		if err != nil {
			return nil, err
		}
		worktrees[normalized] = struct{}{}
	}
	return worktrees, nil
}

func normalizePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	return filepath.Clean(abs), nil
}
