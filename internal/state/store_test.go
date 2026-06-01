package state

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSaveLoadChangeRoundTrip(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	st := model.NewChange("round-trip")
	st.CurrentState = model.StateS1Plan
	st.PlanSubStep = model.PlanSubStepResearch

	require.NoError(t, SaveChange(root, st))

	loaded, err := LoadChange(root, "round-trip")
	require.NoError(t, err)
	assert.Equal(t, model.ChangeVersion, loaded.Version)
	assert.Equal(t, st.Slug, loaded.Slug)
	assert.Equal(t, st.CurrentState, loaded.CurrentState)
	assert.NotNil(t, loaded.EvidenceRefs)
}

func TestSaveChangeQualityModeRoundTrip(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	st := model.NewChange("quality-round-trip")
	st.QualityMode = model.QualityModeDiscuss

	require.NoError(t, SaveChange(root, st))

	loaded, err := LoadChange(root, "quality-round-trip")
	require.NoError(t, err)
	assert.Equal(t, model.QualityModeDiscuss, loaded.QualityMode)
}

func TestSaveChangeWorkflowPresetRoundTrip(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	st := model.NewChange("preset-round-trip")
	st.WorkflowPreset = model.WorkflowPresetStrict
	st.SuggestedWorkflowPreset = model.WorkflowPresetLight

	require.NoError(t, SaveChange(root, st))

	loaded, err := LoadChange(root, "preset-round-trip")
	require.NoError(t, err)
	assert.Equal(t, model.WorkflowPresetStrict, loaded.WorkflowPreset)
	assert.Equal(t, model.WorkflowPresetLight, loaded.SuggestedWorkflowPreset)
}

func TestSaveLoadChangeSlugRoundTrip(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	st := model.NewChange("my-change")
	require.NoError(t, SaveChange(root, st))

	loaded, err := LoadChange(root, "my-change")
	require.NoError(t, err)
	assert.Equal(t, st.Slug, loaded.Slug)
	assert.NotNil(t, loaded.EvidenceRefs)
}

func TestSaveChangePersistsRuntimeFieldsInChangeAuthority(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	change := model.NewChange("runtime-unified")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.Artifacts["intent"] = model.ArtifactState{ID: "intent", State: model.ArtifactLifecycleApproved}
	change.EvidenceRefs["plan-audit"] = "artifacts/changes/runtime-unified/verification/plan-audit.yaml"
	change.LastAutoPassedStates = []model.AutoPassedState{{
		State:  model.StateS3Review,
		Reason: "no_blocking_review_obligations",
	}}
	change.ReviewIntentDriftFailures = 2
	change.InterruptedExecutionAt = time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)

	require.NoError(t, SaveChange(root, change))

	// Runtime fields must now be IN change.yaml.
	changeRaw, err := os.ReadFile(BundleChangeFilePath(root, change.Slug))
	require.NoError(t, err)
	assert.Contains(t, string(changeRaw), "version: 1")
	assert.Contains(t, string(changeRaw), "artifacts:")
	assert.Contains(t, string(changeRaw), "evidence_refs:")
	assert.Contains(t, string(changeRaw), "last_auto_passed_states:")
	assert.Contains(t, string(changeRaw), "review_intent_drift_failures:")
	assert.Contains(t, string(changeRaw), "interrupted_execution_at:")

	loaded, err := LoadChange(root, change.Slug)
	require.NoError(t, err)
	assert.Equal(t, change.Artifacts, loaded.Artifacts)
	assert.Equal(t, change.EvidenceRefs, loaded.EvidenceRefs)
	assert.Equal(t, change.LastAutoPassedStates, loaded.LastAutoPassedStates)
	assert.Equal(t, change.ReviewIntentDriftFailures, loaded.ReviewIntentDriftFailures)
	assert.True(t, change.InterruptedExecutionAt.Equal(loaded.InterruptedExecutionAt))
}

func TestLoadChangeRejectsArtifactLevelVersion(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	slug := "artifact-level-version"
	path := BundleChangeFilePath(root, slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
slug: artifact-level-version
status: active
current_state: S1_PLAN
plan_substep: bundle
artifacts:
  intent:
    version: 1
    id: intent
    state: draft
`), 0o644))

	_, err := LoadChange(root, slug)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field version not found in type model.ArtifactState")
}

func TestRestoreChangeAuthorityIfNeededAcceptsExistingAuthority(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission semantics differ on Windows")
	}

	root := createRuntimeLayout(t)
	change := model.NewChange("restore-equal-sidecar")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.Artifacts["intent"] = model.ArtifactState{ID: "intent", State: model.ArtifactLifecycleApproved}
	change.EvidenceRefs["plan-audit"] = "artifacts/changes/restore-equal-sidecar/verification/plan-audit.yaml"
	require.NoError(t, SaveChange(root, change))

	expected, err := LoadChange(root, change.Slug)
	require.NoError(t, err)

	bundleDir := filepath.Dir(BundleChangeFilePath(root, change.Slug))
	require.NoError(t, os.Chmod(bundleDir, 0o555))
	t.Cleanup(func() {
		_ = os.Chmod(bundleDir, 0o755)
	})

	require.NoError(t, restoreChangeAuthorityIfNeeded(root, expected))
}

func TestSaveChangeRejectsInvalidRuntimeStateWithoutPersistingAuthority(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	change := model.NewChange("runtime-invalid")
	change.Description = "before"
	require.NoError(t, SaveChange(root, change))

	change.Description = "after"
	change.ReviewIntentDriftFailures = -1
	err := SaveChange(root, change)
	require.Error(t, err)

	loaded, loadErr := LoadChange(root, change.Slug)
	require.NoError(t, loadErr)
	assert.Equal(t, "before", loaded.Description)
	assert.Zero(t, loaded.ReviewIntentDriftFailures)
}

func TestLoadChangeRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	st := model.NewChange("strict-load")
	require.NoError(t, SaveChange(root, st))

	path := BundleChangeFilePath(root, "strict-load")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	raw = append(raw, []byte("\nentry_surface: quick\n")...)
	require.NoError(t, os.WriteFile(path, raw, 0o644))

	_, err = LoadChange(root, "strict-load")
	require.Error(t, err, "unknown fields must be rejected by KnownFields(true)")
}

func TestFindActiveChangeSingle(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	st := model.NewChange("single-active")
	require.NoError(t, SaveChange(root, st))

	resolved, err := FindActiveChange(root)
	require.NoError(t, err)
	assert.Equal(t, "single-active", resolved.Slug)
}

func TestFindActiveChangeNoActive(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	_, err := FindActiveChange(root)
	require.ErrorIs(t, err, ErrNoActiveChange)
}

func TestFindActiveChangeMultipleReturnsError(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	a := model.NewChange("change-a")
	require.NoError(t, SaveChange(root, a))

	b := model.NewChange("change-b")
	require.NoError(t, SaveChange(root, b))

	_, err := FindActiveChange(root)
	require.ErrorIs(t, err, ErrMultipleActiveChanges)
}

func TestListChangesDiagnostics(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	st := model.NewChange("diag-change")
	require.NoError(t, SaveChange(root, st))

	changes, err := ListChanges(root)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "diag-change", changes[0].Slug)
}

func TestFindActiveChangeForWorktreeSingleMatch(t *testing.T) {
	t.Parallel()
	root := createRuntimeRepoLayout(t)
	worktreeDir := addGitWorktree(t, root, "wt-match-branch")

	ch := model.NewChange("wt-match")
	ch.WorktreePath = worktreeDir
	require.NoError(t, SaveChange(root, ch))

	resolved, err := FindActiveChangeForWorktree(root, worktreeDir)
	require.NoError(t, err)
	assert.Equal(t, "wt-match", resolved.Slug)
}

func TestFindActiveChangeForWorktreeZeroMatchFallsBackToUnbound(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	queryDir := t.TempDir()

	ch := model.NewChange("unbound-fallback")
	// No WorktreePath set — unbound active change.
	require.NoError(t, SaveChange(root, ch))

	resolved, err := FindActiveChangeForWorktree(root, queryDir)
	require.NoError(t, err)
	assert.Equal(t, "unbound-fallback", resolved.Slug)
}

func TestFindActiveChangeForWorktreeMultipleMatchReturnsError(t *testing.T) {
	t.Parallel()
	root := createRuntimeRepoLayout(t)
	worktreeDir := addGitWorktree(t, root, "shared-worktree-branch")

	ch1 := model.NewChange("change-a")
	ch1.WorktreePath = worktreeDir
	require.NoError(t, SaveChange(root, ch1))

	ch2 := model.NewChange("change-b")
	ch2.WorktreePath = worktreeDir
	require.NoError(t, SaveChange(root, ch2))

	_, err := FindActiveChangeForWorktree(root, worktreeDir)
	require.ErrorIs(t, err, ErrMultipleActiveChanges)
}

func TestFindActiveChangeForWorktreeUnboundFallback(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	queryDir := t.TempDir()

	// Create a change without WorktreePath (pre-S1_PLAN).
	ch := model.NewChange("unbound-change")
	// WorktreePath intentionally left empty.
	require.NoError(t, SaveChange(root, ch))

	resolved, err := FindActiveChangeForWorktree(root, queryDir)
	require.NoError(t, err)
	assert.Equal(t, "unbound-change", resolved.Slug)
}

func TestFindActiveChangeForWorktreeNoMatchNoFallback(t *testing.T) {
	t.Parallel()
	root := createRuntimeRepoLayout(t)
	queryDir := t.TempDir()

	ch1 := model.NewChange("other-a")
	ch1.WorktreePath = addGitWorktree(t, root, "other-a-branch")
	require.NoError(t, SaveChange(root, ch1))

	ch2 := model.NewChange("other-b")
	ch2.WorktreePath = addGitWorktree(t, root, "other-b-branch")
	require.NoError(t, SaveChange(root, ch2))

	_, err := FindActiveChangeForWorktree(root, queryDir)
	var boundErr *ChangeBoundElsewhereError
	require.ErrorAs(t, err, &boundErr)
	require.Len(t, boundErr.BoundChanges, 2)
	assert.Equal(t, "other-a", boundErr.BoundChanges[0].Slug)
	ch1Worktree, err := NormalizePath(ch1.WorktreePath)
	require.NoError(t, err)
	assert.Equal(t, ch1Worktree, boundErr.BoundChanges[0].WorktreePath)
	assert.Equal(t, "other-b", boundErr.BoundChanges[1].Slug)
	ch2Worktree, err := NormalizePath(ch2.WorktreePath)
	require.NoError(t, err)
	assert.Equal(t, ch2Worktree, boundErr.BoundChanges[1].WorktreePath)
}

func TestTwoConcurrentChangesDifferentWorktreesCoexist(t *testing.T) {
	t.Parallel()
	root := createRuntimeRepoLayout(t)

	// Create two changes with different worktree paths.
	worktreeA := addGitWorktree(t, root, "change-alpha-branch")
	chA := model.NewChange("change-alpha")
	chA.WorktreePath = worktreeA
	chA.CurrentState = model.StateS1Plan
	chA.PlanSubStep = model.PlanSubStepBundle
	require.NoError(t, SaveChange(root, chA))

	worktreeB := addGitWorktree(t, root, "change-beta-branch")
	chB := model.NewChange("change-beta")
	chB.WorktreePath = worktreeB
	chB.CurrentState = model.StateS1Plan
	chB.PlanSubStep = model.PlanSubStepBundle
	require.NoError(t, SaveChange(root, chB))

	// ListChanges must return both changes.
	changes, err := ListChanges(root)
	require.NoError(t, err)
	require.Len(t, changes, 2)

	// FindActiveChangeForWorktree with path A returns change A.
	resolvedA, err := FindActiveChangeForWorktree(root, worktreeA)
	require.NoError(t, err)
	assert.Equal(t, "change-alpha", resolvedA.Slug)

	// FindActiveChangeForWorktree with path B returns change B.
	resolvedB, err := FindActiveChangeForWorktree(root, worktreeB)
	require.NoError(t, err)
	assert.Equal(t, "change-beta", resolvedB.Slug)

	// Advance change A to S1_PLAN/bundle, save, verify B is unaffected.
	chA.CurrentState = model.StateS1Plan
	chA.PlanSubStep = model.PlanSubStepBundle
	require.NoError(t, SaveChange(root, chA))

	loadedA, err := LoadChange(root, "change-alpha")
	require.NoError(t, err)
	assert.Equal(t, model.StateS1Plan, loadedA.CurrentState)
	assert.Equal(t, model.PlanSubStepBundle, loadedA.PlanSubStep)

	loadedB, err := LoadChange(root, "change-beta")
	require.NoError(t, err)
	assert.Equal(t, model.StateS1Plan, loadedB.CurrentState, "advancing A must not affect B")
	assert.Equal(t, model.PlanSubStepBundle, loadedB.PlanSubStep, "advancing A must not affect B")
}

func TestListChangesMultipleSummary(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	// Create two changes with distinct slugs.
	chA := model.NewChange("multi-a")
	require.NoError(t, SaveChange(root, chA))

	chB := model.NewChange("multi-b")
	require.NoError(t, SaveChange(root, chB))

	changes, err := ListChanges(root)
	require.NoError(t, err)
	require.Len(t, changes, 2)

	// Collect returned slugs and verify both are present.
	gotSlugs := map[string]bool{}
	for _, c := range changes {
		gotSlugs[c.Slug] = true
	}
	assert.True(t, gotSlugs["multi-a"], "expected change A in listed changes")
	assert.True(t, gotSlugs["multi-b"], "expected change B in listed changes")
}

func TestSaveChangeWritesToBundlePath(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	change := model.NewChange("bundle-write")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	// change.yaml should exist in bundle directory.
	bundlePath := filepath.Join(root, "artifacts", "changes", "bundle-write", "change.yaml")
	raw, err := os.ReadFile(bundlePath)
	require.NoError(t, err)

	var loaded model.Change
	require.NoError(t, yaml.Unmarshal(raw, &loaded))
	assert.Equal(t, model.StateS2Execute, loaded.CurrentState)
	assert.Equal(t, "bundle-write", loaded.Slug)
	assert.Equal(t, "HEAD", loaded.BaseRef)

	_, err = os.Stat(ChangeDir(root, "bundle-write"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestListChangesDiscoversBundleWithoutSidecarDir(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	change := model.NewChange("bundle-discovery")
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.RemoveAll(ChangeDir(root, change.Slug)))

	changes, err := ListChanges(root)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, change.Slug, changes[0].Slug)
}

func TestLoadChangeFindsWorktreeBundleWithoutRegistryMirror(t *testing.T) {
	t.Parallel()
	root, worktreeRoot := setupRepoWithWorktree(t)

	change := model.NewChange("worktree-bundle")
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	require.NoError(t, os.RemoveAll(ChangeDir(root, change.Slug)))

	loaded, err := LoadChange(root, "worktree-bundle")
	require.NoError(t, err)
	assert.Equal(t, model.StateS2Execute, loaded.CurrentState)
	// ChangeDir removal also drops the git-local worktree binding, so this
	// exercises the location-inference fallback, which resolves to the canonical
	// (symlink-resolved) worktree root.
	wantWorktree, err := NormalizePath(worktreeRoot)
	require.NoError(t, err)
	assert.Equal(t, wantWorktree, loaded.WorktreePath)
}

func TestLoadChangeSkipsMarkerlessSiblingWorktreeAtRepoRoot(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	worktreeRoot := addGitWorktree(t, root, "markerless-root-worktree")
	require.NoError(t, os.Remove(filepath.Join(worktreeRoot, ".slipway.yaml")))
	require.NoError(t, ensureScopeMarkerFile(WorkspaceScopeMarkerPath(worktreeRoot)))

	staleChange := model.NewChange("ghost-root")
	staleChange.CurrentState = model.StateS2Execute
	staleChange.PlanSubStep = model.PlanSubStepNone
	staleBundlePath := BundleChangeFilePath(worktreeRoot, staleChange.Slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(staleBundlePath), 0o755))
	raw, err := yaml.Marshal(staleChange)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(staleBundlePath, raw, 0o644))

	changes, err := ListChanges(root)
	require.NoError(t, err)
	assert.Empty(t, changes, "markerless sibling worktree bundles must be invisible at repo root")

	_, err = LoadChange(root, staleChange.Slug)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestLoadChangeFindsNestedScopeBundleInsideWorktree(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	scopeRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(scopeRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(scopeRoot, ".slipway.yaml"), []byte("{}"), 0o644))

	worktreeRoot := addGitWorktree(t, root, "nested-scope-worktree")
	worktreeScopeRoot := filepath.Join(worktreeRoot, "services", "billing")
	require.NoError(t, os.MkdirAll(worktreeScopeRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(worktreeScopeRoot, ".slipway.yaml"), []byte("{}"), 0o644))

	change := model.NewChange("nested-worktree-bundle")
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(scopeRoot, change))

	require.NoError(t, os.RemoveAll(ChangeDir(scopeRoot, change.Slug)))

	loaded, err := LoadChange(scopeRoot, change.Slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS2Execute, loaded.CurrentState)
	// ChangeDir removal also drops the git-local worktree binding, so this
	// exercises the location-inference fallback, which resolves to the canonical
	// (symlink-resolved) worktree root.
	wantWorktree, err := NormalizePath(worktreeRoot)
	require.NoError(t, err)
	assert.Equal(t, wantWorktree, loaded.WorktreePath)

	_, err = os.Stat(filepath.Join(worktreeRoot, "services", "billing", "artifacts", "changes", change.Slug, "change.yaml"))
	require.NoError(t, err)
}

func TestLoadChangeSkipsSiblingBundleWhoseAuthorityPointsAtDifferentWorktree(t *testing.T) {
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

	owningScopeRoot := filepath.Join(owningWorktree, "services", "billing")
	require.NoError(t, os.MkdirAll(owningScopeRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(owningScopeRoot, ".slipway.yaml"), []byte("{}"), 0o644))
	staleScopeRoot := filepath.Join(staleWorktree, "services", "billing")
	require.NoError(t, os.MkdirAll(staleScopeRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staleScopeRoot, ".slipway.yaml"), []byte("{}"), 0o644))
	require.NoError(t, ensureScopeMarkerFile(WorkspaceScopeMarkerPath(staleScopeRoot)))

	change := model.NewChange("cross-worktree-authority")
	change.WorktreePath = owningWorktree
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.Description = "owning bundle"
	require.NoError(t, SaveChange(scopeRoot, change))

	staleCopy := change
	staleCopy.Description = "stale sibling bundle"
	raw, err := yaml.Marshal(staleCopy)
	require.NoError(t, err)
	staleBundlePath := BundleChangeFilePath(staleScopeRoot, change.Slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(staleBundlePath), 0o755))
	require.NoError(t, os.WriteFile(staleBundlePath, raw, 0o644))

	loaded, err := LoadChange(scopeRoot, change.Slug)
	require.NoError(t, err)
	assert.Equal(t, "owning bundle", loaded.Description)
	wantWorktree, err := NormalizePath(owningWorktree)
	require.NoError(t, err)
	assert.Equal(t, wantWorktree, loaded.WorktreePath)
}

func TestLoadChangeSkipsLocalStaleBundleWhenAuthorityOwnedBySiblingWorktree(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)

	change := model.NewChange("local-stale-bundle")
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.Description = "owning bundle"
	require.NoError(t, SaveChange(root, change))

	staleCopy := change
	staleCopy.Description = "stale local bundle"
	raw, err := yaml.Marshal(staleCopy)
	require.NoError(t, err)

	localBundlePath := BundleChangeFilePath(root, change.Slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(localBundlePath), 0o755))
	require.NoError(t, os.WriteFile(localBundlePath, raw, 0o644))

	loaded, err := LoadChange(root, change.Slug)
	require.NoError(t, err)
	assert.Equal(t, "owning bundle", loaded.Description)
	wantWorktree, err := NormalizePath(worktreeRoot)
	require.NoError(t, err)
	assert.Equal(t, wantWorktree, loaded.WorktreePath)
}

func TestLoadChangeDoesNotReadRegistryOnlyMirror(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	change := model.NewChange("registry-only")
	dir := ChangeDir(root, change.Slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	b, err := yaml.Marshal(change)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "change.yaml"), b, 0o644))

	_, err = LoadChange(root, change.Slug)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestLoadChangeReportsMissingBundleAuthorityFile(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	bundleDir := filepath.Join(root, "artifacts", "changes", "broken-change")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	_, err := LoadChange(root, "broken-change")
	require.Error(t, err)
	assert.ErrorIs(t, err, errMissingBundleAuthority)
	assert.False(t, errors.Is(err, os.ErrNotExist))
	assert.Contains(t, err.Error(), "slipway repair")
}

func TestChangeSlugExistsTreatsOrphanBundleAsReserved(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", "broken-change"), 0o755))

	exists, err := ChangeSlugExists(root, "broken-change")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestChangeSlugExistsTreatsHiddenSiblingWorktreeAuthorityAsReserved(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	change := model.NewChange("hidden-worktree-reserved")
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	require.NoError(t, os.Remove(filepath.Join(worktreeRoot, ".slipway.yaml")))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))

	exists, err := ChangeSlugExists(root, change.Slug)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestChangeSlugExistsTreatsWorktreeArchiveFromWorktreeChangeAsReserved(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	slug := "worktree-archive-reserved"
	change := model.NewChange(slug)
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	_, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	require.NoError(t, os.Remove(filepath.Join(worktreeRoot, ".slipway.yaml")))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))

	exists, err := ChangeSlugExists(root, slug)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestListChangesForCreateGuardIncludesHiddenSiblingWorktreeAuthority(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	change := model.NewChange("hidden-worktree-guard")
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	require.NoError(t, os.Remove(filepath.Join(worktreeRoot, ".slipway.yaml")))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))

	visible, err := ListChanges(root)
	require.NoError(t, err)
	assert.Empty(t, visible)

	guarded, err := ListChangesForCreateGuard(root)
	require.NoError(t, err)
	require.Len(t, guarded, 1)
	assert.Equal(t, change.Slug, guarded[0].Slug)
}

func TestLoadChangeFallsBackToSiblingWorktreeAuthorityWhenLocalBundleDirIsOrphaned(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	change := model.NewChange("worktree-authority")
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	localBundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(localBundleDir, 0o755))

	loaded, err := LoadChange(root, change.Slug)
	require.NoError(t, err)
	wantWorktree, err := NormalizePath(worktreeRoot)
	require.NoError(t, err)
	assert.Equal(t, wantWorktree, loaded.WorktreePath)
}

func TestListChangesSkipsSiblingWorktreeWithoutConfigEvenWhenBoundWorktreeMatches(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	change := model.NewChange("hidden-sibling-config")
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	require.NoError(t, os.Remove(filepath.Join(worktreeRoot, ".slipway.yaml")))

	changes, err := ListChanges(root)
	require.NoError(t, err)
	assert.Empty(t, changes, "sibling worktree without canonical config must not stay visible")

	_, err = LoadChange(root, change.Slug)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestListChangesReportsOrphanBundleDirectoryAsIntegrityError(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	// Create a bundle directory without change.yaml (orphan).
	orphanDir := filepath.Join(root, "artifacts", "changes", "orphan-dir")
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(orphanDir, "intent.md"), []byte("# orphan\n"), 0o644))

	// Also create a valid change so we can verify orphan integrity is not hidden
	// just because another bundle is still readable.
	validChange := model.NewChange("valid-change")
	require.NoError(t, SaveChange(root, validChange))

	_, err := ListChanges(root)
	require.Error(t, err)
	assert.ErrorIs(t, err, errMissingBundleAuthority)
	normalizedOrphanDir, normalizeErr := NormalizePath(orphanDir)
	require.NoError(t, normalizeErr)
	assert.Contains(t, err.Error(), normalizedOrphanDir)
}

func TestListChangesIgnoresEmptyOrphanBundleDirectory(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	emptyResidue := filepath.Join(root, "artifacts", "changes", "empty-residue", "verification")
	require.NoError(t, os.MkdirAll(emptyResidue, 0o755))
	validChange := model.NewChange("valid-change")
	require.NoError(t, SaveChange(root, validChange))

	changes, err := ListChanges(root)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "valid-change", changes[0].Slug)
}

func TestSaveChangeDoesNotCreateUnusedSidecarDir(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	change := model.NewChange("registry-check")
	require.NoError(t, SaveChange(root, change))

	_, err := os.Stat(ChangeDir(root, "registry-check"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestListChangesFindsNestedScopeBundleInsideWorktree(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	scopeRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(scopeRoot, 0o755))
	// Both scope roots must have the project marker for cross-worktree discovery.
	require.NoError(t, os.WriteFile(filepath.Join(scopeRoot, ".slipway.yaml"), []byte("{}"), 0o644))

	worktreeRoot := addGitWorktree(t, root, "nested-scope-list-worktree")
	worktreeScopeRoot := filepath.Join(worktreeRoot, "services", "billing")
	require.NoError(t, os.MkdirAll(worktreeScopeRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(worktreeScopeRoot, ".slipway.yaml"), []byte("{}"), 0o644))

	change := model.NewChange("nested-worktree-list")
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(scopeRoot, change))

	changes, err := ListChanges(scopeRoot)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, change.Slug, changes[0].Slug)
	wantWorktree, err := NormalizePath(worktreeRoot)
	require.NoError(t, err)
	assert.Equal(t, wantWorktree, changes[0].WorktreePath)
}

func TestListChangesSkipsStaleSiblingWorktreeScopeMarkerWithoutConfig(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	scopeRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(scopeRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(scopeRoot, ".slipway.yaml"), []byte("{}"), 0o644))

	worktreeRoot := addGitWorktree(t, root, "stale-scope-worktree")
	worktreeScopeRoot := filepath.Join(worktreeRoot, "services", "billing")
	require.NoError(t, os.MkdirAll(worktreeScopeRoot, 0o755))
	require.NoError(t, ensureScopeMarkerFile(WorkspaceScopeMarkerPath(worktreeScopeRoot)))
	// Intentionally NO .slipway.yaml in the sibling worktree scope.

	// Manually place a stale bundle in the markerless worktree scope.
	staleChange := model.NewChange("ghost")
	staleChange.CurrentState = model.StateS2Execute
	staleChange.PlanSubStep = model.PlanSubStepNone
	staleBundlePath := BundleChangeFilePath(worktreeScopeRoot, "ghost")
	require.NoError(t, os.MkdirAll(filepath.Dir(staleBundlePath), 0o755))
	raw, err := yaml.Marshal(staleChange)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(staleBundlePath, raw, 0o644))

	// The stale bundle must NOT be visible from the main scope.
	changes, err := ListChanges(scopeRoot)
	require.NoError(t, err)
	assert.Empty(t, changes, "markerless sibling worktree bundles must be invisible")

	// LoadChange must also fail for the stale slug.
	_, err = LoadChange(scopeRoot, "ghost")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestListChangesBestEffortSkipsUnreadableChangeBundle(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)

	good := model.NewChange("good-change")
	require.NoError(t, SaveChange(root, good))

	bad := model.NewChange("bad-change")
	require.NoError(t, SaveChange(root, bad))
	require.NoError(t, os.WriteFile(BundleChangeFilePath(root, bad.Slug), []byte("slug: bad-change\ncurrent_state: [\n"), 0o644))

	changes, issues, err := ListChangesBestEffortWithIssues(root)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, good.Slug, changes[0].Slug)
	require.Len(t, issues, 1)
	assert.Equal(t, bad.Slug, issues[0].Slug)
	assert.ErrorContains(t, issues[0].Err, "load change")

}

func TestLoadChangeReturnsWorktreeEnumerationErrorWhenGitWorkspaceLookupMisses(t *testing.T) {
	root := createRuntimeLayout(t)
	installFakeGitForStoreTests(t, root, true)

	_, err := LoadChange(root, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list git worktrees")
}

func TestLoadChangeFailsClosedWhenWorktreeEnumerationErrorsDespiteLocalBundle(t *testing.T) {
	root := createRuntimeLayout(t)

	change := model.NewChange("existing")
	require.NoError(t, SaveChange(root, change))

	installFakeGitForStoreTests(t, root, true)

	_, err := LoadChange(root, change.Slug)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list git worktrees")
}

func TestListChangesIgnoresWorktreeEnumerationFailureOutsideGitWorkspace(t *testing.T) {
	root := createRuntimeLayout(t)
	installFakeGitForStoreTests(t, root, false)

	changes, err := ListChanges(root)
	require.NoError(t, err)
	assert.Empty(t, changes)
}

func TestChangeVisibleFromRootOnlyAllowsEmptyWorktreeForArchivedCandidates(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	otherWorkspace := t.TempDir()
	change := model.NewChange("portable-archive")
	change.Status = model.ChangeStatusDone

	assert.False(t, changeVisibleFromRoot(root, otherWorkspace, change, false))
	assert.True(t, changeVisibleFromRoot(root, otherWorkspace, change, true))
	assert.True(t, changeVisibleFromRoot(root, root, change, false))
}

func createRuntimeLayout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(ChangesDir(root), 0o755))
	return root
}

func createRuntimeRepoLayout(t *testing.T) string {
	t.Helper()
	root := createRuntimeLayout(t)
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".slipway.yaml"), []byte("defaults:\n  artifact_schema: expanded\n"), 0o644))
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "init")
	return root
}

func saveActiveChangeForTest(t *testing.T, root, slug string) model.Change {
	t.Helper()
	change := model.NewChange(slug)
	require.NoError(t, SaveChange(root, change))
	return change
}

func addGitWorktree(t *testing.T, repoRoot, branch string) string {
	t.Helper()
	worktreePath := filepath.Join(t.TempDir(), branch)
	runGit(t, repoRoot, "worktree", "add", worktreePath, "-b", branch)
	return worktreePath
}

func installFakeGitForStoreTests(t *testing.T, root string, treatAsGitWorkspace bool) {
	t.Helper()

	fakeBinDir := t.TempDir()
	scriptPath := filepath.Join(fakeBinDir, "git")
	repoResponse := "echo 'not a git repository' >&2\nexit 128\n"
	if treatAsGitWorkspace {
		repoResponse = "printf '%s\\n' \"" + root + "\"\nexit 0\n"
	}
	script := "#!/bin/sh\n" +
		"args=\"$*\"\n" +
		"case \"$args\" in\n" +
		"  *\"worktree list --porcelain\"*)\n" +
		"    echo 'worktree metadata broken' >&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"  *\"rev-parse --show-toplevel\"*)\n" +
		"    " + repoResponse +
		"    ;;\n" +
		"  *)\n" +
		"    echo \"unexpected git invocation: $args\" >&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"esac\n"
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755))
	if runtime.GOOS == "windows" {
		wrapperPath := filepath.Join(fakeBinDir, "git.bat")
		wrapper := "@echo off\r\nbash \"%~dp0git\" %*\r\nexit /b %ERRORLEVEL%\r\n"
		require.NoError(t, os.WriteFile(wrapperPath, []byte(wrapper), 0o755))
	}
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
