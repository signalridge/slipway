package state

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"gopkg.in/yaml.v3"
)

var (
	ErrNoActiveChange         = errors.New("no active change")
	ErrMultipleActiveChanges  = errors.New("multiple active changes")
	errMissingBundleAuthority = errors.New("change bundle missing authority file")
)

// RuntimeDir returns the parent directory for all non-authoritative runtime state.
func RuntimeDir(root string) string {
	return GitRuntimeDir(root)
}

// ChangesDir returns the parent directory for all active per-change runtime state.
func ChangesDir(root string) string {
	return filepath.Join(RuntimeDir(root), "changes")
}

// ChangeDir returns the per-change local evidence directory under the git-common runtime area.
func ChangeDir(root, slug string) string {
	return filepath.Join(ChangesDir(root), slug)
}

// ActiveBundlesDir returns the canonical directory containing active change bundles.
func ActiveBundlesDir(root string) string {
	return filepath.Join(root, "artifacts", "changes")
}

// BundleChangeFilePath returns the change.yaml path in the governed bundle directory.
// This is the canonical path for change state (single authority).
func BundleChangeFilePath(root, slug string) string {
	return filepath.Join(root, "artifacts", "changes", slug, "change.yaml")
}

// ArchivedBundlesDir returns the canonical directory containing archived change bundles.
func ArchivedBundlesDir(root string) string {
	return filepath.Join(root, "artifacts", "changes", "archived")
}

// BundleArchivedChangeFilePath returns the archived change.yaml authority path.
func BundleArchivedChangeFilePath(root, slug string) string {
	return filepath.Join(ArchivedBundlesDir(root), slug, "change.yaml")
}

// bundleChangeFilePathForChange resolves the bundle change.yaml path, accounting
// for worktree-bound changes that write to their worktree workspace root.
// Returns an error if the worktree path is set but cannot be resolved, to
// prevent silently writing to the wrong location.
func bundleChangeFilePathForChange(root string, change model.Change) (string, error) {
	wsRoot, err := changeWorkspaceRoot(root, change)
	if err != nil {
		if wp := strings.TrimSpace(change.WorktreePath); wp != "" {
			return "", fmt.Errorf("resolve worktree path %q for %q: %w", wp, change.Slug, err)
		}
		return "", fmt.Errorf("resolve bundle workspace root for %q: %w", change.Slug, err)
	}
	return filepath.Join(wsRoot, "artifacts", "changes", change.Slug, "change.yaml"), nil
}

// CodebaseMapDir returns the durable brownfield map directory for the provided
// workspace root.
func CodebaseMapDir(root string) string {
	return filepath.Join(root, "artifacts", "codebase")
}

// EvidenceTasksDir returns the per-change task evidence directory for a given run version.
func EvidenceTasksDir(root, slug string, runVersion int) string {
	return filepath.Join(ChangeDir(root, slug), "evidence", "tasks", fmt.Sprintf("rv%d", runVersion))
}

func removePerChangeLocalRuntimeState(root, slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return errors.New("slug is required")
	}
	for _, path := range []string{
		ChangeDir(root, slug),
		filepath.Dir(TaskPIDFilePath(root, slug)),
		filepath.Dir(GovernanceSnapshotCachePath(root, slug)),
	} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func allWorkspaceRoots(root string) ([]string, error) {
	normalizedRoot, err := NormalizePath(root)
	if err != nil {
		normalizedRoot = filepath.Clean(root)
	}
	scopeRel := scopeRelativePath(normalizedRoot)

	roots := []string{normalizedRoot}
	worktrees, err := listGitWorktrees(normalizedRoot)
	if err != nil {
		if _, repoErr := gitWorkspaceRoot(normalizedRoot); repoErr != nil && gitCommandReportsNotRepository(repoErr) {
			return roots, nil
		}
		return roots, err
	}

	existing := map[string]struct{}{normalizedRoot: {}}
	extras := make([]string, 0, len(worktrees))
	for worktreeRoot := range worktrees {
		candidateRoot := worktreeRoot
		if scopeRel != "" {
			candidateRoot = filepath.Clean(filepath.Join(worktreeRoot, scopeRel))
		}
		info, err := os.Stat(candidateRoot)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, ok := existing[candidateRoot]; ok {
			continue
		}
		existing[candidateRoot] = struct{}{}
		extras = append(extras, candidateRoot)
	}
	slices.Sort(extras)
	return append(roots, extras...), nil
}

func candidateWorkspaceRoots(root string) ([]string, error) {
	roots, err := allWorkspaceRoots(root)
	if err != nil {
		return nil, err
	}
	if len(roots) <= 1 {
		return roots, nil
	}
	visible := make([]string, 0, len(roots))
	visible = append(visible, roots[0])
	for _, workspaceRoot := range roots[1:] {
		if !workspaceScopeVisible(workspaceRoot) {
			continue
		}
		visible = append(visible, workspaceRoot)
	}
	return visible, nil
}

type bundleCandidate struct {
	WorkspaceRoot string
	Path          string
	Archived      bool
}

func bundleCandidatesForRoots(workspaceRoots []string, slug string) []bundleCandidate {
	paths := make([]bundleCandidate, 0, len(workspaceRoots))
	for _, workspaceRoot := range workspaceRoots {
		paths = append(paths, bundleCandidate{
			WorkspaceRoot: workspaceRoot,
			Path:          BundleChangeFilePath(workspaceRoot, slug),
		})
	}
	return paths
}

func archivedBundleCandidatesForRoots(workspaceRoots []string, slug string) []bundleCandidate {
	paths := make([]bundleCandidate, 0, len(workspaceRoots))
	for _, workspaceRoot := range workspaceRoots {
		paths = append(paths, bundleCandidate{
			WorkspaceRoot: workspaceRoot,
			Path:          BundleArchivedChangeFilePath(workspaceRoot, slug),
			Archived:      true,
		})
	}
	return paths
}

func candidateBundlePaths(root, slug string) ([]bundleCandidate, error) {
	// Priority is deterministic: the caller's resolved scope/workspace first,
	// then sibling worktree scope roots. Visibility checks still reject stale
	// caller-local duplicates when the change is owned by a sibling workspace.
	roots, err := candidateWorkspaceRoots(root)
	if err != nil {
		return nil, err
	}
	return bundleCandidatesForRoots(roots, slug), nil
}

func candidateArchivedBundlePaths(root, slug string) ([]bundleCandidate, error) {
	roots, err := candidateWorkspaceRoots(root)
	if err != nil {
		return nil, err
	}
	return archivedBundleCandidatesForRoots(roots, slug), nil
}

type activeChangeDiscovery struct {
	HasAuthority   bool
	FirstOrphanDir string
}

func discoverActiveChangeSlugs(root string) ([]string, error) {
	return discoverActiveChangeSlugsWithMode(root, false)
}

func discoverActiveChangeSlugsWithMode(root string, bestEffort bool) ([]string, error) {
	return discoverActiveChangeSlugsAcrossRoots(root, false, bestEffort)
}

func discoverActiveChangeSlugsRegardlessOfVisibility(root string, bestEffort bool) ([]string, error) {
	return discoverActiveChangeSlugsAcrossRoots(root, true, bestEffort)
}

func discoverActiveChangeSlugsAcrossRoots(root string, includeHidden, bestEffort bool) ([]string, error) {
	var (
		workspaceRoots []string
		err            error
	)
	if includeHidden {
		workspaceRoots, err = allWorkspaceRoots(root)
	} else {
		workspaceRoots, err = candidateWorkspaceRoots(root)
	}
	if err != nil {
		return nil, err
	}
	found := make(map[string]*activeChangeDiscovery)

	for _, workspaceRoot := range workspaceRoots {
		entries, err := os.ReadDir(ActiveBundlesDir(workspaceRoot))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			slug := entry.Name()
			if slug == "archived" {
				continue
			}
			discovery := found[slug]
			if discovery == nil {
				discovery = &activeChangeDiscovery{}
				found[slug] = discovery
			}
			bundlePath := BundleChangeFilePath(workspaceRoot, slug)
			if _, err := os.Stat(bundlePath); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					return nil, err
				}
				if discovery.FirstOrphanDir == "" {
					discovery.FirstOrphanDir = filepath.Dir(bundlePath)
				}
				continue
			}
			discovery.HasAuthority = true
		}
	}

	slugs := make([]string, 0, len(found))
	for slug := range found {
		slugs = append(slugs, slug)
	}
	slices.Sort(slugs)

	visible := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		discovery := found[slug]
		if discovery.HasAuthority {
			visible = append(visible, slug)
			continue
		}
		if !bestEffort {
			return nil, missingBundleAuthorityError(discovery.FirstOrphanDir)
		}
	}
	return visible, nil
}

// ChangeSlugExists reports whether a slug is already used by an active or archived change bundle.
func ChangeSlugExists(root, slug string) (bool, error) {
	if _, err := loadChangeRegardlessOfVisibility(root, slug); err == nil {
		return true, nil
	} else if errors.Is(err, errMissingBundleAuthority) {
		return true, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}

	if _, err := os.Stat(BundleArchivedChangeFilePath(root, slug)); err == nil {
		return true, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	return false, nil
}

func loadChangeFromCandidates(root string, paths []bundleCandidate) (model.Change, error) {
	return loadChangeFromCandidatesWithLoader(root, paths, loadChangeCandidate)
}

func loadChangeFromCandidatesWithLoader(
	root string,
	paths []bundleCandidate,
	load func(string) (model.Change, error),
) (model.Change, error) {
	var firstAuthorityErr error
	for _, candidate := range paths {
		change, err := load(candidate.Path)
		if err == nil {
			if !changeVisibleFromRoot(root, candidate.WorkspaceRoot, change, candidate.Archived) {
				continue
			}
			return change, nil
		}
		if errors.Is(err, fs.ErrNotExist) {
			if bundleErr := validateBundleAuthorityPath(candidate.Path); bundleErr != nil {
				if errors.Is(bundleErr, fs.ErrNotExist) {
					continue
				}
				if errors.Is(bundleErr, errMissingBundleAuthority) {
					if firstAuthorityErr == nil {
						firstAuthorityErr = bundleErr
					}
					continue
				}
				return model.Change{}, bundleErr
			}
			continue
		}
		return model.Change{}, err
	}
	if firstAuthorityErr != nil {
		return model.Change{}, firstAuthorityErr
	}
	return model.Change{}, fs.ErrNotExist
}

// LoadChange loads the active change state for the given slug from the canonical bundle paths.
func LoadChange(root, slug string) (model.Change, error) {
	paths, rootsErr := candidateBundlePaths(root, slug)
	if rootsErr != nil {
		return model.Change{}, rootsErr
	}
	return loadChangeFromCandidates(root, paths)
}

func loadChangeRegardlessOfVisibility(root, slug string) (model.Change, error) {
	roots, err := allWorkspaceRoots(root)
	if err != nil {
		return model.Change{}, err
	}
	return loadChangeFromCandidates(root, bundleCandidatesForRoots(roots, slug))
}

func loadChangeRegardlessOfVisibilityForDiagnostics(root, slug string) (model.Change, error) {
	roots, err := allWorkspaceRoots(root)
	if err != nil {
		return model.Change{}, err
	}
	return loadChangeFromCandidatesWithLoader(root, bundleCandidatesForRoots(roots, slug), loadChangeCandidate)
}

func LoadChangeForDiagnostics(root, slug string) (model.Change, error) {
	paths, rootsErr := candidateBundlePaths(root, slug)
	if rootsErr != nil {
		return model.Change{}, rootsErr
	}
	return loadChangeFromCandidatesWithLoader(root, paths, loadChangeCandidate)
}

// ListChangesForCreateGuard returns active authoritative changes across all
// registered workspace roots, including hidden sibling worktrees. It exists so
// `slipway new` can fail closed on hidden authorities without broadening normal
// user-facing discovery semantics.
func ListChangesForCreateGuard(root string) ([]model.Change, error) {
	slugs, err := discoverActiveChangeSlugsRegardlessOfVisibility(root, false)
	if err != nil {
		return nil, err
	}
	changes, _, err := loadChangesForSlugsWithLoader(slugs, false, func(slug string) (model.Change, error) {
		return loadChangeRegardlessOfVisibility(root, slug)
	})
	return changes, err
}

func changeVisibleFromRoot(root, workspaceRoot string, change model.Change, archivedCandidate bool) bool {
	if archivedCandidate && change.Status != model.ChangeStatusActive && strings.TrimSpace(change.WorktreePath) == "" {
		return true
	}
	normalizedRoot, err := NormalizePath(root)
	if err != nil {
		normalizedRoot = filepath.Clean(root)
	}
	normalizedWorkspace, err := NormalizePath(workspaceRoot)
	if err != nil {
		normalizedWorkspace = filepath.Clean(workspaceRoot)
	}
	boundWorkspace, err := WorkspaceRootForChange(normalizedRoot, change)
	if err != nil {
		return false
	}
	normalizedBoundWorkspace, err := NormalizePath(boundWorkspace)
	if err != nil {
		normalizedBoundWorkspace = filepath.Clean(boundWorkspace)
	}
	return normalizedBoundWorkspace == normalizedWorkspace
}

// SaveChange persists the change state for the given change (keyed by slug).
// The governed bundle copy is the single authority.
func SaveChange(root string, st model.Change) error {
	if st.Slug == "" {
		return errors.New("slug is required")
	}

	st.Normalize()
	if err := st.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(st.WorktreePath) != "" {
		if err := EnsureWorkspaceScopeMarker(root, st.WorktreePath); err != nil {
			return err
		}
		if err := EnsureWorkspaceScopeConfig(root, st.WorktreePath); err != nil {
			return err
		}
	}

	b, err := yaml.Marshal(st)
	if err != nil {
		return err
	}

	// Write to bundle directory (canonical authority alongside plan artifacts).
	bundlePath, err := bundleChangeFilePathForChange(root, st)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(bundlePath), 0o755); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(bundlePath, b, 0o644); err != nil {
		return err
	}
	return nil
}

func decodeChangeStrict(raw []byte, change *model.Change) error {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	// KnownFields(true) rejects truly unknown fields while allowing the
	// unified runtime fields (artifacts, evidence_refs, etc.) which now
	// have proper yaml tags in the Change struct.
	decoder.KnownFields(true)
	return decoder.Decode(change)
}

func decodeAndValidateChange(raw []byte) (model.Change, error) {
	var st model.Change
	if err := decodeChangeStrict(raw, &st); err != nil {
		return model.Change{}, err
	}
	st.Normalize()
	if err := st.Validate(); err != nil {
		return model.Change{}, err
	}
	return st, nil
}

func loadChangeCandidate(path string) (model.Change, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return model.Change{}, err
	}
	return decodeAndValidateChange(b)
}

func validateBundleAuthorityPath(path string) error {
	bundleDir := filepath.Dir(path)
	info, err := os.Stat(bundleDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fs.ErrNotExist
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("bundle path exists and is not a directory: %s", bundleDir)
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return missingBundleAuthorityError(bundleDir)
		}
		return err
	}
	return nil
}

func missingBundleAuthorityError(bundleDir string) error {
	return fmt.Errorf(
		"%w: %s: change.yaml missing in governed bundle; run `slipway repair` to inspect or repair change state files",
		errMissingBundleAuthority,
		bundleDir,
	)
}

// FindActiveChange finds the single active change. Returns ErrNoActiveChange if none,
// ErrMultipleActiveChanges if more than one.
func FindActiveChange(root string) (model.Change, error) {
	changes, err := ListChanges(root)
	if err != nil {
		return model.Change{}, err
	}

	var active []model.Change
	for _, c := range changes {
		if c.Status == model.ChangeStatusActive {
			active = append(active, c)
		}
	}

	if len(active) == 0 {
		return model.Change{}, ErrNoActiveChange
	}
	if len(active) > 1 {
		return model.Change{}, ErrMultipleActiveChanges
	}
	return active[0], nil
}

// FindActiveChangeForWorktree finds the active change bound to the given worktree path.
func FindActiveChangeForWorktree(root, currentWorktreePath string) (model.Change, error) {
	normalizedCurrent, err := NormalizePath(currentWorktreePath)
	if err != nil {
		return model.Change{}, fmt.Errorf("normalize worktree path: %w", err)
	}

	changes, err := ListChanges(root)
	if err != nil {
		return model.Change{}, err
	}

	// Phase 1: Match by worktree path.
	var worktreeMatches []model.Change
	var unboundActive []model.Change
	for _, c := range changes {
		if c.Status != model.ChangeStatusActive {
			continue
		}
		if c.WorktreePath == "" {
			unboundActive = append(unboundActive, c)
			continue
		}
		normalizedChange, err := NormalizePath(c.WorktreePath)
		if err != nil {
			continue
		}
		if normalizedChange == normalizedCurrent {
			worktreeMatches = append(worktreeMatches, c)
		}
	}

	if len(worktreeMatches) == 1 {
		return worktreeMatches[0], nil
	}
	if len(worktreeMatches) > 1 {
		return model.Change{}, ErrMultipleActiveChanges
	}

	// Phase 2: Fall back to unbound active changes.
	if len(unboundActive) == 1 {
		return unboundActive[0], nil
	}
	if len(unboundActive) > 1 {
		return model.Change{}, ErrMultipleActiveChanges
	}

	return model.Change{}, ErrNoActiveChange
}

// ListChanges returns all changes found in the changes directory.
func ListChanges(root string) ([]model.Change, error) {
	slugs, err := discoverActiveChangeSlugs(root)
	if err != nil {
		return nil, err
	}
	changes, _, err := loadChangesForSlugs(root, slugs, false)
	return changes, err
}

type ChangeLoadIssue struct {
	Slug string
	Err  error
}

// ListChangesBestEffortWithIssues returns readable active changes while
// collecting unreadable bundle authority issues for diagnostics/repair flows.
func ListChangesBestEffortWithIssues(root string) ([]model.Change, []ChangeLoadIssue, error) {
	slugs, err := discoverActiveChangeSlugsWithMode(root, true)
	if err != nil {
		return nil, nil, err
	}
	return loadChangesForSlugsWithLoader(slugs, true, func(slug string) (model.Change, error) {
		return LoadChangeForDiagnostics(root, slug)
	})
}

// ListRepoChangesBestEffortWithIssues returns repo-wide authoritative changes
// while preserving unreadable bundle authority errors for diagnostics.
func ListRepoChangesBestEffortWithIssues(root string) ([]model.Change, []ChangeLoadIssue, error) {
	slugs, err := discoverActiveChangeSlugsRegardlessOfVisibility(root, true)
	if err != nil {
		return nil, nil, err
	}
	return loadChangesForSlugsWithLoader(slugs, true, func(slug string) (model.Change, error) {
		return loadChangeRegardlessOfVisibilityForDiagnostics(root, slug)
	})
}

func loadChangesForSlugs(root string, slugs []string, bestEffort bool) ([]model.Change, []ChangeLoadIssue, error) {
	return loadChangesForSlugsWithLoader(slugs, bestEffort, func(slug string) (model.Change, error) {
		return LoadChange(root, slug)
	})
}

func loadChangesForSlugsWithLoader(
	slugs []string,
	bestEffort bool,
	load func(string) (model.Change, error),
) ([]model.Change, []ChangeLoadIssue, error) {
	var changes []model.Change
	var issues []ChangeLoadIssue
	for _, slug := range slugs {
		c, err := load(slug)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			wrapped := fmt.Errorf("load change %q: %w", slug, err)
			if bestEffort {
				issues = append(issues, ChangeLoadIssue{Slug: slug, Err: wrapped})
				continue
			}
			return nil, nil, wrapped
		}
		changes = append(changes, c)
	}

	slices.SortFunc(changes, func(a, b model.Change) int {
		return strings.Compare(a.Slug, b.Slug)
	})

	return changes, issues, nil
}

// listSubdirs returns sorted directory names under dir. Returns nil if dir does not exist.
func listSubdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirs = append(dirs, entry.Name())
	}
	slices.Sort(dirs)
	return dirs, nil
}

func restoreChangeAuthorityIfNeeded(root string, expected model.Change) error {
	expected.Normalize()
	bundlePath, err := bundleChangeFilePathForChange(root, expected)
	if err != nil {
		return err
	}

	raw, err := os.ReadFile(bundlePath)
	if err == nil {
		current, decodeErr := decodeAndValidateChange(raw)
		if decodeErr == nil {
			current.Normalize()
			if reflect.DeepEqual(current, expected) {
				return nil
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return SaveChange(root, expected)
}

func wrapRollbackError(primary error, rollbackErrs ...error) error {
	filtered := make([]error, 0, len(rollbackErrs))
	for _, err := range rollbackErrs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) == 0 {
		return primary
	}
	return fmt.Errorf("%w (rollback failed: %v)", primary, errors.Join(filtered...))
}
