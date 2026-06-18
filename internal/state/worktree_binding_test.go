package state

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestSaveChangeOmitsWorktreePathFromTrackedBundle is the core acceptance check
// for issue #46: a worktree-bound change must not write the machine-local
// absolute worktree_path into the tracked change.yaml, while keeping the
// portable worktree_branch.
func TestSaveChangeOmitsWorktreePathFromTrackedBundle(t *testing.T) {
	t.Parallel()
	root, worktreeRoot := setupRepoWithWorktree(t)

	change := model.NewChange("omit-worktree-path")
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = worktreeRoot
	change.WorktreeBranch = "feature"
	require.NoError(t, SaveChange(root, change))

	bundlePath := BundleChangeFilePath(worktreeRoot, change.Slug)
	raw, err := os.ReadFile(bundlePath)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "worktree_path:",
		"tracked change.yaml must not persist a machine-local absolute worktree path")
	assert.NotContains(t, string(raw), worktreeRoot,
		"tracked change.yaml must not leak the developer-local worktree path")
	assert.Contains(t, string(raw), "worktree_branch: feature",
		"portable worktree_branch should remain in the tracked bundle")

	// The git-local runtime binding carries the absolute path instead.
	bindingRaw, err := os.ReadFile(WorktreeBindingPath(root, change.Slug))
	require.NoError(t, err)
	assert.Contains(t, string(bindingRaw), "worktree_path:")
}

// TestLoadChangeResolvesBoundWorktreeFromRepoRoot covers `--change <slug>`
// invocations from the repo root: the bound worktree resolves from the git-local
// binding even though the tracked bundle carries no path.
func TestLoadChangeResolvesBoundWorktreeFromRepoRoot(t *testing.T) {
	t.Parallel()
	root, worktreeRoot := setupRepoWithWorktree(t)

	change := model.NewChange("resolve-from-root")
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = worktreeRoot
	change.WorktreeBranch = "feature"
	require.NoError(t, SaveChange(root, change))

	loaded, err := LoadChange(root, change.Slug)
	require.NoError(t, err)
	wantWorktree, err := NormalizePath(worktreeRoot)
	require.NoError(t, err)
	assert.Equal(t, wantWorktree, loaded.WorktreePath)
}

// TestFindActiveChangeFromInsideWorktreeResolves covers commands invoked from
// inside the dedicated worktree.
func TestFindActiveChangeFromInsideWorktreeResolves(t *testing.T) {
	t.Parallel()
	root, worktreeRoot := setupRepoWithWorktree(t)

	change := model.NewChange("resolve-from-worktree")
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = worktreeRoot
	change.WorktreeBranch = "feature"
	require.NoError(t, SaveChange(root, change))

	resolved, err := FindActiveChangeForWorktree(worktreeRoot, worktreeRoot)
	require.NoError(t, err)
	assert.Equal(t, change.Slug, resolved.Slug)
	wantWorktree, err := NormalizePath(worktreeRoot)
	require.NoError(t, err)
	assert.Equal(t, wantWorktree, resolved.WorktreePath)
}

// TestResolutionRecoversWhenBindingMissing proves the design is self-healing:
// with the git-local binding deleted (e.g. a fresh clone, or cleared runtime
// state) the bound worktree is still recovered from the bundle's own location.
func TestResolutionRecoversWhenBindingMissing(t *testing.T) {
	t.Parallel()
	root, worktreeRoot := setupRepoWithWorktree(t)

	change := model.NewChange("recover-binding")
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = worktreeRoot
	change.WorktreeBranch = "feature"
	require.NoError(t, SaveChange(root, change))

	require.NoError(t, os.Remove(WorktreeBindingPath(root, change.Slug)))

	loaded, err := LoadChange(root, change.Slug)
	require.NoError(t, err)
	wantWorktree, err := NormalizePath(worktreeRoot)
	require.NoError(t, err)
	assert.Equal(t, wantWorktree, loaded.WorktreePath,
		"resolution must fall back to the bundle's worktree location when the binding is missing")
}

// TestArchiveRemovesWorktreeBindingAndStripsPath confirms archived records stay
// portable: the binding is cleared and the archived change.yaml carries no path.
func TestArchiveRemovesWorktreeBindingAndStripsPath(t *testing.T) {
	t.Parallel()
	root, worktreeRoot := setupRepoWithWorktree(t)

	change := model.NewChange("archive-strips-path")
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = worktreeRoot
	change.WorktreeBranch = "feature"
	require.NoError(t, SaveChange(root, change))
	require.FileExists(t, WorktreeBindingPath(root, change.Slug))

	archived, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)
	assert.Empty(t, archived.WorktreePath)

	_, statErr := os.Stat(WorktreeBindingPath(root, change.Slug))
	assert.True(t, os.IsNotExist(statErr), "archive must remove the git-local worktree binding")

	archivedPath, err := ArchivedChangeFilePathForRead(root, change.Slug)
	require.NoError(t, err)
	raw, err := os.ReadFile(archivedPath)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "worktree_path:",
		"archived change.yaml must remain portable with no worktree path")
}

// TestTrackedChangeYamlWithWorktreePathIsRejected proves the field is
// fail-closed on read: WorktreePath is yaml:"-", so a tracked change.yaml that
// still carries a worktree_path is rejected by strict decoding rather than
// silently tolerated. There is no migration shim — a stale field must be removed
// (the next clean SaveChange never writes it again).
func TestTrackedChangeYamlWithWorktreePathIsRejected(t *testing.T) {
	t.Parallel()
	root, worktreeRoot := setupRepoWithWorktree(t)

	change := model.NewChange("reject-worktree-path")
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = worktreeRoot
	change.WorktreeBranch = "feature"
	require.NoError(t, SaveChange(root, change))

	// Inject a stale worktree_path into the otherwise-clean tracked bundle.
	bundlePath := BundleChangeFilePath(worktreeRoot, change.Slug)
	raw, err := os.ReadFile(bundlePath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(bundlePath, appendLegacyWorktreePath(raw, worktreeRoot), 0o644))

	_, err = LoadChange(root, change.Slug)
	require.Error(t, err, "a tracked change.yaml carrying worktree_path must be rejected by strict decoding")
	assert.Contains(t, err.Error(), "worktree_path")
}

// TestLoadChangeFailsClosedWhenBindingMissingAndLocationFallbackAmbiguous
// covers the edge where the git-local binding is missing and multiple same-slug
// bundle locations are visible. In that state bundle location is no longer a
// unique authority, so loading must fail closed instead of selecting whichever
// candidate sorts first.
func TestLoadChangeFailsClosedWhenBindingMissingAndLocationFallbackAmbiguous(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	scopeRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(scopeRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(scopeRoot, ".slipway.yaml"), []byte("{}"), 0o644))

	worktreeBase := t.TempDir()
	staleWorktree := filepath.Join(worktreeBase, "aaa-stale")
	owningWorktree := filepath.Join(worktreeBase, "bbb-owner")
	runGit(t, root, "worktree", "add", staleWorktree, "-b", "aaa-stale")
	runGit(t, root, "worktree", "add", owningWorktree, "-b", "bbb-owner")

	staleScopeRoot := filepath.Join(staleWorktree, "services", "billing")
	owningScopeRoot := filepath.Join(owningWorktree, "services", "billing")
	for _, workspace := range []string{staleScopeRoot, owningScopeRoot} {
		require.NoError(t, os.MkdirAll(workspace, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(workspace, ".slipway.yaml"), []byte("{}"), 0o644))
	}
	require.NoError(t, ensureScopeMarkerFile(WorkspaceScopeMarkerPath(staleScopeRoot)))

	change := model.NewChange("ambiguous-location-fallback")
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = owningWorktree
	change.WorktreeBranch = "bbb-owner"
	change.Description = "owning bundle"
	require.NoError(t, SaveChange(scopeRoot, change))
	require.NoError(t, os.Remove(WorktreeBindingPath(scopeRoot, change.Slug)))

	staleCopy := change
	staleCopy.Description = "stale sibling bundle"
	staleRaw, err := yaml.Marshal(staleCopy)
	require.NoError(t, err)
	stalePath := BundleChangeFilePath(staleScopeRoot, change.Slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(stalePath), 0o755))
	require.NoError(t, os.WriteFile(stalePath, staleRaw, 0o644))

	_, err = LoadChange(scopeRoot, change.Slug)
	require.ErrorIs(t, err, errAmbiguousWorktreeBinding)
	assert.Contains(t, err.Error(), change.Slug)
}

func appendLegacyWorktreePath(raw []byte, worktreePath string) []byte {
	return []byte(string(raw) + "worktree_path: " + worktreePath + "\n")
}

// TestTrackedChangeBundlesOmitWorktreePath is a static repository hygiene guard:
// no git-tracked artifacts/changes/**/change.yaml may persist a worktree_path.
func TestTrackedChangeBundlesOmitWorktreePath(t *testing.T) {
	t.Parallel()

	toplevelOut, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("not a git repository: %v", err)
	}
	toplevel := strings.TrimSpace(string(toplevelOut))

	out, err := exec.Command("git", "-C", toplevel, "ls-files", "artifacts/changes").Output()
	require.NoError(t, err)

	checked := 0
	for _, rel := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		rel = strings.TrimSpace(rel)
		if rel == "" || filepath.Base(rel) != "change.yaml" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(toplevel, rel))
		if errors.Is(err, os.ErrNotExist) {
			t.Logf("skipping tracked change.yaml deleted in this worktree: %s", rel)
			continue
		}
		require.NoError(t, err)
		assert.NotContainsf(t, string(raw), "worktree_path:",
			"tracked %s must not persist a machine-local worktree_path", rel)
		checked++
	}
	t.Logf("checked %d tracked change.yaml file(s)", checked)
}
