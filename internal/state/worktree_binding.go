package state

import (
	"errors"
	"io/fs"
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
	WorktreePath   string `yaml:"worktree_path"`
	WorktreeBranch string `yaml:"worktree_branch,omitempty"`
	// GitCommonDir records the repo identity at write time so a binding that was
	// copied into a different repository checkout is ignored rather than trusted.
	GitCommonDir string `yaml:"git_common_dir,omitempty"`
}

// WorktreeBindingPath returns the git-local worktree-binding record path for slug.
func WorktreeBindingPath(root, slug string) string {
	return filepath.Join(ChangeDir(root, slug), worktreeBindingFileName)
}

// writeWorktreeBinding persists (or clears) the git-local worktree binding for a
// change. An empty WorktreePath removes any existing record so an unbound change
// leaves no stale binding behind.
func writeWorktreeBinding(root string, change model.Change) error {
	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return errors.New("slug is required")
	}
	path := WorktreeBindingPath(root, slug)
	if strings.TrimSpace(change.WorktreePath) == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}

	binding := worktreeBinding{
		WorktreePath:   change.WorktreePath,
		WorktreeBranch: change.WorktreeBranch,
		GitCommonDir:   gitCommonDirIdentity(root),
	}
	raw, err := yaml.Marshal(binding)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw, 0o644)
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
	if recorded := strings.TrimSpace(binding.GitCommonDir); recorded != "" {
		if current := gitCommonDirIdentity(root); current != "" && current != recorded {
			// Binding belongs to a different repository checkout; do not trust it.
			return worktreeBinding{}, false
		}
	}
	return binding, true
}

func gitCommonDirIdentity(root string) string {
	normalized, err := NormalizePath(GitCommonDir(root))
	if err != nil {
		return filepath.Clean(GitCommonDir(root))
	}
	return normalized
}
