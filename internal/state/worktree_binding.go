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

// ArchivedChangeLoadError reports a candidate archived change that should have
// been local to the invocation worktree but could not be loaded.
type ArchivedChangeLoadError struct {
	Slug         string
	WorktreePath string
	Err          error
}

func (err *ArchivedChangeLoadError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("load archived change %q for worktree %q: %v", err.Slug, err.WorktreePath, err.Err)
}

func (err *ArchivedChangeLoadError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
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
// current git worktree using only runtime binding records. Unlike the
// ListChanges-based resolvers, it deliberately avoids ListChanges so callers can
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

// FindArchivedChangeForWorktree resolves the archived change whose dedicated
// worktree is the given invocation worktree. The git-local worktree-binding
// record is removed at archive time, so the runtime-binding lookups used for
// active changes cannot recover this association; instead it is reconstructed
// from the portable archived bundle using the worktree authority that survives
// archival: the default `.worktrees/<slug>` path convention and the bundle's
// recorded worktree_branch.
//
// It returns (_, false, nil) when the invocation worktree hosts no local
// archived change (the common case, including the project root and any active
// worktree), so callers fall through to active-change resolution unchanged.
func FindArchivedChangeForWorktree(root, worktreePath string) (model.Change, bool, error) {
	normalizedWorktree, err := NormalizePath(worktreePath)
	if err != nil {
		return model.Change{}, false, fmt.Errorf("normalize worktree path: %w", err)
	}
	branch, _ := resolveWorktreeActualBranch(normalizedWorktree)
	for _, slug := range archivedWorktreeSlugCandidates(normalizedWorktree, branch) {
		change, err := LoadArchivedChange(root, slug)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return model.Change{}, false, &ArchivedChangeLoadError{
				Slug:         slug,
				WorktreePath: normalizedWorktree,
				Err:          err,
			}
		}
		if archivedChangeBoundToWorktree(root, change, normalizedWorktree, branch) {
			return change, true, nil
		}
	}
	return model.Change{}, false, nil
}

// archivedWorktreeSlugCandidates derives the archived slugs that could own the
// invocation worktree from the worktree directory name and its git branch,
// following Slipway's `.worktrees/<slug>` + `feat/<slug>` creation convention.
// Deriving candidates (instead of enumerating every archived bundle) keeps
// resolution delegated to LoadArchivedChange's own path authority.
func archivedWorktreeSlugCandidates(worktreePath, branch string) []string {
	seen := map[string]struct{}{}
	candidates := make([]string, 0, 2)
	add := func(slug string) {
		slug = strings.TrimSpace(slug)
		if slug == "" {
			return
		}
		if err := ValidateChangeSlug(slug); err != nil {
			return
		}
		if _, ok := seen[slug]; ok {
			return
		}
		seen[slug] = struct{}{}
		candidates = append(candidates, slug)
	}
	add(filepath.Base(worktreePath))
	add(strings.TrimPrefix(strings.TrimSpace(branch), "feat/"))
	return candidates
}

// archivedChangeBoundToWorktree confirms a candidate archived change actually
// owns the invocation worktree via authority that survives archival: either the
// default dedicated worktree path matches, or a non-root worktree's branch
// matches the archived bundle's recorded worktree_branch. The project root never
// matches by branch alone; root invocations must fall through to active/global
// resolution.
func archivedChangeBoundToWorktree(root string, change model.Change, worktreePath, branch string) bool {
	if defaultPath, err := NormalizePath(DefaultWorktreePath(root, change.Slug)); err == nil && defaultPath == worktreePath {
		return true
	}
	normalizedRoot, err := NormalizePath(root)
	if err != nil || normalizedRoot == worktreePath {
		return false
	}
	recordedBranch := strings.TrimSpace(change.WorktreeBranch)
	return recordedBranch != "" && branch != "" && recordedBranch == branch
}

func gitCommonDirIdentity(root string) string {
	normalized, err := NormalizePath(GitCommonDir(root))
	if err != nil {
		return filepath.Clean(GitCommonDir(root))
	}
	return normalized
}
