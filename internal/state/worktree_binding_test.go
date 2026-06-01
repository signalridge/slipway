package state

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSaveChangeOmitsWorktreePathFromTrackedBundle is the core acceptance check
// for issue #46: a worktree-bound change must not write the machine-local
// absolute worktree_path into the tracked change.yaml, while keeping the
// portable worktree_branch.
func TestSaveChangeOmitsWorktreePathFromTrackedBundle(t *testing.T) {
	t.Parallel()
	root, worktreeRoot := setupRepoWithWorktree(t)

	change := model.NewChange("omit-worktree-path")
	change.CurrentState = model.StateS2Execute
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
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = worktreeRoot
	change.WorktreeBranch = "feature"
	require.NoError(t, SaveChange(root, change))

	loaded, err := LoadChange(root, change.Slug)
	require.NoError(t, err)
	assert.Equal(t, worktreeRoot, loaded.WorktreePath)
}

// TestFindActiveChangeFromInsideWorktreeResolves covers commands invoked from
// inside the dedicated worktree.
func TestFindActiveChangeFromInsideWorktreeResolves(t *testing.T) {
	t.Parallel()
	root, worktreeRoot := setupRepoWithWorktree(t)

	change := model.NewChange("resolve-from-worktree")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = worktreeRoot
	change.WorktreeBranch = "feature"
	require.NoError(t, SaveChange(root, change))

	resolved, err := FindActiveChangeForWorktree(worktreeRoot, worktreeRoot)
	require.NoError(t, err)
	assert.Equal(t, change.Slug, resolved.Slug)
	assert.Equal(t, worktreeRoot, resolved.WorktreePath)
}

// TestResolutionRecoversWhenBindingMissing proves the design is self-healing:
// with the git-local binding deleted (e.g. a fresh clone, or cleared runtime
// state) the bound worktree is still recovered from the bundle's own location.
func TestResolutionRecoversWhenBindingMissing(t *testing.T) {
	t.Parallel()
	root, worktreeRoot := setupRepoWithWorktree(t)

	change := model.NewChange("recover-binding")
	change.CurrentState = model.StateS2Execute
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
	change.CurrentState = model.StateS2Execute
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

// TestLegacyChangeYamlWithWorktreePathStillResolves proves an existing active
// change whose tracked change.yaml still carries a legacy worktree_path is not
// rejected by strict decoding and still resolves its bound worktree — the legacy
// value is ignored in favour of the location-based binding, and the field ages
// out on the next save (Change.MarshalYAML drops it).
func TestLegacyChangeYamlWithWorktreePathStillResolves(t *testing.T) {
	t.Parallel()
	root, worktreeRoot := setupRepoWithWorktree(t)

	change := model.NewChange("legacy-worktree-path")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = worktreeRoot
	change.WorktreeBranch = "feature"
	require.NoError(t, SaveChange(root, change))

	// Rewrite the tracked bundle in the legacy shape (carrying worktree_path) and
	// drop the git-local binding, so resolution must tolerate the legacy field
	// and recover the worktree from the bundle location.
	bundlePath := BundleChangeFilePath(worktreeRoot, change.Slug)
	raw, err := os.ReadFile(bundlePath)
	require.NoError(t, err)
	legacy := string(raw) + "worktree_path: " + worktreeRoot + "\n"
	require.NoError(t, os.WriteFile(bundlePath, []byte(legacy), 0o644))
	require.NoError(t, os.Remove(WorktreeBindingPath(root, change.Slug)))

	loaded, err := LoadChange(root, change.Slug)
	require.NoError(t, err, "a legacy change.yaml carrying worktree_path must still decode")
	wantWorktree, err := NormalizePath(worktreeRoot)
	require.NoError(t, err)
	assert.Equal(t, wantWorktree, loaded.WorktreePath)
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
		require.NoError(t, err)
		assert.NotContainsf(t, string(raw), "worktree_path:",
			"tracked %s must not persist a machine-local worktree_path", rel)
		checked++
	}
	t.Logf("checked %d tracked change.yaml file(s)", checked)
}
