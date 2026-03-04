package state

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/speclane/internal/model"
)

const (
	WorktreeReasonMetadataRequired = "dedicated_worktree_metadata_required"
	WorktreeReasonPathInvalid      = "dedicated_worktree_path_invalid"
	WorktreeReasonBranchMismatch   = "dedicated_worktree_branch_mismatch"
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
	reasons, err := ValidateWorktreeAuthenticityReasons(repoRoot, worktreePath, expectedBranch)
	if err != nil {
		return err
	}
	if len(reasons) == 0 {
		return nil
	}
	return fmt.Errorf("worktree authenticity failed: %s", strings.Join(reasons, ", "))
}

func ValidateWorktreeAuthenticityReasons(repoRoot, worktreePath, expectedBranch string) ([]string, error) {
	reasons := []string{}
	if strings.TrimSpace(worktreePath) == "" || strings.TrimSpace(expectedBranch) == "" {
		reasons = append(reasons, WorktreeReasonMetadataRequired)
		return reasons, nil
	}

	normalizedPath, err := normalizePath(worktreePath)
	if err != nil {
		reasons = append(reasons, WorktreeReasonPathInvalid)
		return reasons, nil
	}
	stat, err := os.Stat(normalizedPath)
	if err != nil {
		reasons = append(reasons, WorktreeReasonPathInvalid)
		return reasons, nil
	}
	if !stat.IsDir() {
		reasons = append(reasons, WorktreeReasonPathInvalid)
		return reasons, nil
	}

	registered, err := listGitWorktrees(repoRoot)
	if err != nil {
		return nil, err
	}
	if _, exists := registered[normalizedPath]; !exists {
		reasons = append(reasons, WorktreeReasonPathInvalid)
		return uniqueSortedReasons(reasons), nil
	}

	cmd := exec.Command("git", "-C", normalizedPath, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("resolve worktree branch: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	actualBranch := strings.TrimSpace(string(out))
	if actualBranch != expectedBranch {
		reasons = append(reasons, WorktreeReasonBranchMismatch)
	}
	return uniqueSortedReasons(reasons), nil
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

func uniqueSortedReasons(reasons []string) []string {
	if len(reasons) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if _, ok := seen[reason]; ok {
			continue
		}
		seen[reason] = struct{}{}
		out = append(out, reason)
	}
	slices.Sort(out)
	return out
}
