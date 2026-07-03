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
	ErrNoActiveChange           = errors.New("no active change")
	ErrMultipleActiveChanges    = errors.New("multiple active changes")
	errMissingBundleAuthority   = errors.New("change bundle missing authority file")
	errAmbiguousWorktreeBinding = errors.New("ambiguous worktree binding")
)

// IsMissingBundleAuthority reports whether err means a bundle directory exists
// but its change.yaml authority file is absent.
func IsMissingBundleAuthority(err error) bool {
	return errors.Is(err, errMissingBundleAuthority)
}

type BoundChangeRef struct {
	Slug         string
	WorktreePath string
}

type ChangeBoundElsewhereError struct {
	BoundChanges []BoundChangeRef
}

func (err *ChangeBoundElsewhereError) Error() string {
	if err == nil || len(err.BoundChanges) == 0 {
		return "active changes are bound to other worktrees"
	}
	parts := make([]string, 0, len(err.BoundChanges))
	for _, change := range err.BoundChanges {
		parts = append(parts, fmt.Sprintf("%s -> %s", change.Slug, change.WorktreePath))
	}
	return "active changes are bound to other worktrees: " + strings.Join(parts, ", ")
}

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

// EvidenceTasksDir returns the canonical per-change task evidence directory.
// Individual evidence payloads carry run_summary_version; the path stays flat
// so there is a single runtime evidence surface to inspect.
func EvidenceTasksDir(root, slug string) string {
	return filepath.Join(ChangeDir(root, slug), "evidence", "tasks")
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

func fastBundleCandidates(root, slug string) []bundleCandidate {
	normalizedRoot, err := NormalizePath(root)
	if err != nil {
		normalizedRoot = filepath.Clean(root)
	}

	candidates := []bundleCandidate{{
		WorkspaceRoot: normalizedRoot,
		Path:          BundleChangeFilePath(normalizedRoot, slug),
	}}
	seen := map[string]struct{}{candidates[0].Path: {}}

	binding, ok := readWorktreeBinding(normalizedRoot, slug)
	if !ok {
		return candidates
	}
	workspaceRoot, err := scopeRootInWorkspace(normalizedRoot, binding.WorktreePath)
	if err != nil {
		return candidates
	}
	path := BundleChangeFilePath(workspaceRoot, slug)
	if _, ok := seen[path]; ok {
		return candidates
	}
	return append(candidates, bundleCandidate{
		WorkspaceRoot: workspaceRoot,
		Path:          path,
	})
}

func candidateArchivedBundlePaths(root, slug string) ([]bundleCandidate, error) {
	roots, err := allWorkspaceRoots(root)
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
				hasFiles, emptyErr := orphanBundleDirHasFiles(filepath.Dir(bundlePath))
				if emptyErr != nil {
					return nil, emptyErr
				}
				if !hasFiles {
					if !discovery.HasAuthority && discovery.FirstOrphanDir == "" {
						delete(found, slug)
					}
					continue
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

	_, err := loadArchivedChangeWithCandidate(root, slug)
	if err == nil || errors.Is(err, errMissingBundleAuthority) {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func loadChangeFromCandidates(root string, paths []bundleCandidate) (model.Change, error) {
	change, _, err := loadChangeFromCandidatesWithLoaderAndCandidate(root, paths, loadChangeCandidate)
	return change, err
}

func loadChangeFromCandidatesWithLoader(
	root string,
	paths []bundleCandidate,
	load func(string) (model.Change, error),
) (model.Change, error) {
	change, _, err := loadChangeFromCandidatesWithLoaderAndCandidate(root, paths, load)
	return change, err
}

func loadChangeFromCandidatesWithLoaderAndCandidate(
	root string,
	paths []bundleCandidate,
	load func(string) (model.Change, error),
) (model.Change, bundleCandidate, error) {
	var firstAuthorityErr error
	var locationFallbackChange model.Change
	var locationFallbackCandidate bundleCandidate
	var locationFallbackSet bool
	for _, candidate := range paths {
		change, err := load(candidate.Path)
		if err == nil {
			resolution := worktreeBindingUnresolved
			if !candidate.Archived {
				resolution = HydrateWorktreeBinding(root, candidate.WorkspaceRoot, &change)
			}
			if !changeVisibleFromRoot(root, candidate.WorkspaceRoot, change, candidate.Archived) {
				continue
			}
			if !candidate.Archived && resolution == worktreeBindingFromLocation {
				if locationFallbackSet {
					return model.Change{}, bundleCandidate{}, ambiguousWorktreeBindingError(change.Slug, locationFallbackCandidate, candidate)
				}
				locationFallbackChange = change
				locationFallbackCandidate = candidate
				locationFallbackSet = true
				continue
			}
			return change, candidate, nil
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
				return model.Change{}, bundleCandidate{}, bundleErr
			}
			continue
		}
		return model.Change{}, bundleCandidate{}, err
	}
	if locationFallbackSet {
		return locationFallbackChange, locationFallbackCandidate, nil
	}
	if firstAuthorityErr != nil {
		return model.Change{}, bundleCandidate{}, firstAuthorityErr
	}
	return model.Change{}, bundleCandidate{}, fs.ErrNotExist
}

func ambiguousWorktreeBindingError(slug string, first, second bundleCandidate) error {
	return fmt.Errorf(
		"%w for %q: runtime binding is missing and multiple bundle locations are visible (%s, %s); run `slipway repair` or remove the stale bundle copy",
		errAmbiguousWorktreeBinding,
		slug,
		first.Path,
		second.Path,
	)
}

// LoadChange loads the active change state for the given slug from the canonical bundle paths.
func LoadChange(root, slug string) (model.Change, error) {
	paths, rootsErr := candidateBundlePaths(root, slug)
	if rootsErr != nil {
		return model.Change{}, rootsErr
	}
	return loadChangeFromCandidates(root, paths)
}

// LoadChangeFast loads an active change using the invocation-local authority
// first: the caller's bundle and the git-local worktree binding for slug. It
// falls back to the full workspace scan only when that narrow authority cannot
// establish the change location.
func LoadChangeFast(root, slug string) (model.Change, error) {
	paths := fastBundleCandidates(root, slug)
	change, err := loadChangeFromCandidates(root, paths)
	if err == nil {
		return change, nil
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, errMissingBundleAuthority) {
		return LoadChange(root, slug)
	}
	return model.Change{}, err
}

func loadArchivedChangeWithCandidate(root, slug string) (bundleCandidate, error) {
	paths, err := candidateArchivedBundlePaths(root, slug)
	if err != nil {
		return bundleCandidate{}, err
	}
	_, candidate, err := loadChangeFromCandidatesWithLoaderAndCandidate(root, paths, loadChangeCandidate)
	return candidate, err
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
	ops, err := SaveChangeTransactionOps(root, st)
	if err != nil {
		return err
	}
	return fsutil.ApplyFileTransaction(ops)
}

// SaveChangeTransactionOps returns the file operations needed to persist the
// governed bundle authority and its machine-local worktree binding.
func SaveChangeTransactionOps(root string, st model.Change) ([]fsutil.FileTransactionOp, error) {
	if st.Slug == "" {
		return nil, errors.New("slug is required")
	}

	st.Normalize()
	if err := st.Validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(st.WorktreePath) != "" {
		if err := EnsureWorkspaceScopeMarker(root, st.WorktreePath); err != nil {
			return nil, err
		}
		if err := EnsureWorkspaceScopeConfig(root, st.WorktreePath); err != nil {
			return nil, err
		}
	}

	b, err := yaml.Marshal(st)
	if err != nil {
		return nil, err
	}

	// Write to bundle directory (canonical authority alongside plan artifacts).
	bundlePath, err := bundleChangeFilePathForChange(root, st)
	if err != nil {
		return nil, err
	}

	// Record the machine-local worktree binding in git-local runtime state.
	// The tracked bundle above carries no absolute path (Change.MarshalYAML
	// strips it); this runtime record is the authority for resolving the bound
	// worktree, with the bundle's own location as a self-healing fallback.
	bindingOp, err := worktreeBindingTransactionOp(root, st)
	if err != nil {
		return nil, err
	}
	return []fsutil.FileTransactionOp{
		fsutil.WriteFileTransactionOp(bundlePath, b, 0o644),
		bindingOp,
	}, nil
}

func decodeChangeStrict(raw []byte, change *model.Change) error {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	// KnownFields(true) rejects truly unknown fields while allowing the
	// unified runtime fields (artifacts, evidence_refs, etc.) which now
	// have proper yaml tags in the Change struct.
	decoder.KnownFields(true)
	if err := decoder.Decode(change); err != nil {
		if strings.Contains(err.Error(), "field active_checkpoint not found") {
			return errors.New("unsupported retired change state: active_checkpoint was removed; recover by aborting or recreating the active change without checkpoint state")
		}
		return err
	}
	return nil
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
	b, err := os.ReadFile(path) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
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

// SelectActiveChange resolves the single active change from an already-loaded
// change list. It mirrors FindActiveChange without re-running discovery.
func SelectActiveChange(changes []model.Change) (model.Change, error) {
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

// SelectActiveChangeForWorktree resolves the active change for worktree from an
// already-loaded change list. It mirrors FindActiveChangeForWorktree without
// re-running discovery.
func SelectActiveChangeForWorktree(changes []model.Change, currentWorktreePath string) (model.Change, error) {
	normalizedCurrent, err := NormalizePath(currentWorktreePath)
	if err != nil {
		return model.Change{}, fmt.Errorf("normalize worktree path: %w", err)
	}

	// Phase 1: Match by worktree path.
	var worktreeMatches []model.Change
	var unboundActive []model.Change
	var boundElsewhere []BoundChangeRef
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
		} else {
			boundElsewhere = append(boundElsewhere, BoundChangeRef{Slug: c.Slug, WorktreePath: c.WorktreePath})
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
	if len(boundElsewhere) > 0 {
		slices.SortFunc(boundElsewhere, func(a, b BoundChangeRef) int {
			if a.Slug != b.Slug {
				return strings.Compare(a.Slug, b.Slug)
			}
			return strings.Compare(a.WorktreePath, b.WorktreePath)
		})
		return model.Change{}, &ChangeBoundElsewhereError{BoundChanges: boundElsewhere}
	}

	return model.Change{}, ErrNoActiveChange
}

func RepairEmptyOrphanBundleDirs(root string) ([]string, error) {
	workspaceRoots, err := allWorkspaceRoots(root)
	if err != nil {
		return nil, err
	}
	removed := []string{}
	for _, workspaceRoot := range workspaceRoots {
		entries, err := os.ReadDir(ActiveBundlesDir(workspaceRoot))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == "archived" {
				continue
			}
			bundleDir := filepath.Join(ActiveBundlesDir(workspaceRoot), entry.Name())
			changeYaml := filepath.Join(bundleDir, "change.yaml")
			if _, err := os.Stat(changeYaml); err == nil {
				continue
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
			hasFiles, err := orphanBundleDirHasFiles(bundleDir)
			if err != nil {
				return nil, err
			}
			if hasFiles {
				continue
			}
			if err := os.RemoveAll(bundleDir); err != nil {
				return nil, err
			}
			removed = append(removed, entry.Name())
		}
	}
	slices.Sort(removed)
	return slices.Compact(removed), nil
}

func orphanBundleDirHasFiles(dir string) (bool, error) {
	hasFiles := false
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dir {
			return nil
		}
		if !entry.IsDir() {
			hasFiles = true
		}
		return nil
	})
	return hasFiles, err
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

	raw, err := os.ReadFile(bundlePath) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err == nil {
		current, decodeErr := decodeAndValidateChange(raw)
		if decodeErr == nil {
			current.Normalize()
			expectedOnDisk := expected
			expectedOnDisk.WorktreePath = ""
			if reflect.DeepEqual(current, expectedOnDisk) {
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
