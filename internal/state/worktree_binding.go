package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"gopkg.in/yaml.v3"
)

// worktreeBindingFileName is the git-local runtime record that binds an active
// change to its dedicated worktree. It lives under the per-change runtime dir so
// it is removed together with the rest of that change's local runtime state on
// archive/cancel, and is never part of the portable, tracked change bundle.
const worktreeBindingFileName = "worktree-binding.yaml"

// worktreeBinding is the git-local authority for a change's machine-local
// worktree path. The tracked change.yaml intentionally carries no absolute path
// (see Change.MarshalYAML); resolution reads this runtime record instead, and
// falls back to the bundle's own location when the record is absent.
type worktreeBinding struct {
	WorktreePath string `yaml:"worktree_path"`
	// WorktreeBranch is diagnostic metadata. The portable branch authority stays
	// in tracked change.yaml, so hydration only reads WorktreePath.
	WorktreeBranch string `yaml:"worktree_branch,omitempty"`
	// GitCommonDir records the repo identity at write time so a binding that was
	// copied into a different repository checkout is ignored rather than trusted.
	GitCommonDir string `yaml:"git_common_dir,omitempty"`
}

// WorktreeBindingPath returns the git-local worktree-binding record path for slug.
func WorktreeBindingPath(root, slug string) string {
	return filepath.Join(ChangeDir(root, slug), worktreeBindingFileName)
}

// worktreeBindingTransactionOp persists or clears the git-local worktree
// binding. An empty WorktreePath removes any existing record so an unbound
// change leaves no stale binding behind.
func worktreeBindingTransactionOp(root string, change model.Change) (fsutil.FileTransactionOp, error) {
	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return fsutil.FileTransactionOp{}, errors.New("slug is required")
	}
	path := WorktreeBindingPath(root, slug)
	if strings.TrimSpace(change.WorktreePath) == "" {
		return fsutil.RemoveFileTransactionOp(path), nil
	}
	normalizedWorktreePath, err := NormalizePath(change.WorktreePath)
	if err != nil {
		return fsutil.FileTransactionOp{}, fmt.Errorf("normalize worktree binding path: %w", err)
	}

	binding := worktreeBinding{
		WorktreePath:   normalizedWorktreePath,
		WorktreeBranch: change.WorktreeBranch,
		GitCommonDir:   gitCommonDirIdentity(root),
	}
	raw, err := yaml.Marshal(binding)
	if err != nil {
		return fsutil.FileTransactionOp{}, err
	}
	return fsutil.WriteFileTransactionOp(path, raw, 0o644), nil
}

// readWorktreeBinding returns the recorded worktree binding for slug. The second
// return is false when no usable binding exists (missing, unreadable, blank, or
// written against a different repository).
func readWorktreeBinding(root, slug string) (worktreeBinding, bool) {
	raw, err := os.ReadFile(WorktreeBindingPath(root, slug))
	if err != nil {
		return worktreeBinding{}, false
	}
	var binding worktreeBinding
	if err := yaml.Unmarshal(raw, &binding); err != nil {
		return worktreeBinding{}, false
	}
	if strings.TrimSpace(binding.WorktreePath) == "" {
		return worktreeBinding{}, false
	}
	normalizedWorktreePath, err := NormalizePath(binding.WorktreePath)
	if err != nil {
		return worktreeBinding{}, false
	}
	binding.WorktreePath = normalizedWorktreePath
	if recorded := strings.TrimSpace(binding.GitCommonDir); recorded != "" {
		if current := gitCommonDirIdentity(root); current != "" && current != recorded {
			// Binding belongs to a different repository checkout; do not trust it.
			return worktreeBinding{}, false
		}
	}
	return binding, true
}

// FindActiveChangeByWorktreeBinding resolves the active change bound to the
// current git worktree using only runtime binding records. Unlike
// FindActiveChangeForWorktree, it deliberately avoids ListChanges so callers can
// still prefer the current bound worktree when another workspace has stale
// orphaned bundle residue.
func FindActiveChangeByWorktreeBinding(root, currentWorktreePath string) (model.Change, error) {
	normalizedCurrent, err := NormalizePath(currentWorktreePath)
	if err != nil {
		return model.Change{}, fmt.Errorf("normalize worktree path: %w", err)
	}

	entries, err := os.ReadDir(ChangesDir(root))
	if err != nil {
		if os.IsNotExist(err) {
			return model.Change{}, ErrNoActiveChange
		}
		return model.Change{}, err
	}

	var matches []model.Change
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		slug := entry.Name()
		if err := ValidateChangeSlug(slug); err != nil {
			continue
		}
		binding, ok := readWorktreeBinding(root, slug)
		if !ok || binding.WorktreePath != normalizedCurrent {
			continue
		}
		change, err := LoadChange(root, slug)
		if err != nil {
			return model.Change{}, err
		}
		if change.Status != model.ChangeStatusActive {
			continue
		}
		matches = append(matches, change)
	}

	if len(matches) == 0 {
		return model.Change{}, ErrNoActiveChange
	}
	if len(matches) > 1 {
		return model.Change{}, ErrMultipleActiveChanges
	}
	return matches[0], nil
}

func gitCommonDirIdentity(root string) string {
	normalized, err := NormalizePath(GitCommonDir(root))
	if err != nil {
		return filepath.Clean(GitCommonDir(root))
	}
	return normalized
}
