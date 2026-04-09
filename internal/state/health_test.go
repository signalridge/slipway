package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectHealthReportFindsBrokenConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(ConfigPath(root), []byte("defaults: ["), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)
	require.NotEmpty(t, report.Findings)

	var categories []string
	for _, finding := range report.Findings {
		categories = append(categories, finding.Category)
	}
	assert.Contains(t, categories, "config")
}

func TestCollectHealthReportReportsUnreadableChangeAuthority(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("corrupt-change")
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.WriteFile(BundleChangeFilePath(root, change.Slug), []byte("slug: corrupt-change\ncurrent_state: [\n"), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "bundle_integrity" || finding.Slug != change.Slug {
			continue
		}
		found = true
		assert.Contains(t, finding.Message, "unreadable")
		assert.False(t, finding.Repairable)
	}
	assert.True(t, found, "expected unreadable authority finding")
}

func TestCollectHealthReportReportsUnreadableHiddenWorktreeAuthority(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	branch := "health-hidden-unreadable-branch"
	worktreeRoot := addGitWorktree(t, root, branch)

	change := model.NewChange("hidden-unreadable-change")
	require.NoError(t, PersistScopeWorktreeMetadata(&change, worktreeRoot, branch))
	require.NoError(t, SaveChange(root, change))

	require.NoError(t, os.Remove(ConfigPath(worktreeRoot)))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))
	require.NoError(t, os.WriteFile(BundleChangeFilePath(worktreeRoot, change.Slug), []byte("slug: hidden-unreadable-change\ncurrent_state: [\n"), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "bundle_integrity" || finding.Slug != change.Slug {
			continue
		}
		found = true
		assert.Contains(t, finding.Message, "unreadable")
		assert.False(t, finding.Repairable)
	}
	assert.True(t, found, "expected hidden unreadable authority finding")
}

func TestCollectHealthReportReportsUnreadableExecutionSummary(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("corrupt-execution-summary")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	summaryPath := filepath.Join(VerificationDir(root, change.Slug), ExecutionSummaryFileName)
	require.NoError(t, os.MkdirAll(filepath.Dir(summaryPath), 0o755))
	require.NoError(t, os.WriteFile(summaryPath, []byte("version: ["), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "execution_summary" || finding.Slug != change.Slug {
			continue
		}
		found = true
		assert.Contains(t, finding.Message, "Execution summary")
		assert.False(t, finding.Repairable)
	}
	assert.True(t, found, "expected execution summary integrity finding")
}

func TestCollectHealthReportFindsOrphanBundleDirsAcrossWorktrees(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	worktreeRoot := addGitWorktree(t, root, "health-orphan-branch")
	require.NoError(t, EnsureWorkspaceScopeMarker(root, worktreeRoot))
	orphanDir := filepath.Join(worktreeRoot, "artifacts", "changes", "orphan-worktree-bundle")
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	var orphanReasons []string
	var orphanSlugs []string
	for _, finding := range report.Findings {
		for _, reason := range finding.Reasons {
			if reason.Code == "orphan_bundle_directory" {
				orphanReasons = append(orphanReasons, reason.Detail)
				orphanSlugs = append(orphanSlugs, finding.Slug)
			}
		}
	}
	assert.Contains(t, orphanReasons, "orphan-worktree-bundle")
	assert.Contains(t, orphanSlugs, "orphan-worktree-bundle")
}

func TestCollectHealthReportDeduplicatesMatchingOrphanBundleDirsAcrossWorktrees(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	for _, branch := range []string{"health-orphan-branch-a", "health-orphan-branch-b"} {
		worktreeRoot := addGitWorktree(t, root, branch)
		require.NoError(t, EnsureWorkspaceScopeMarker(root, worktreeRoot))
		orphanDir := filepath.Join(worktreeRoot, "artifacts", "changes", "shared-orphan")
		require.NoError(t, os.MkdirAll(orphanDir, 0o755))
	}

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	count := 0
	for _, finding := range report.Findings {
		for _, reason := range finding.Reasons {
			if reason.Code == "orphan_bundle_directory" && reason.Detail == "shared-orphan" {
				count++
			}
		}
	}
	assert.Equal(t, 1, count)
}

func TestCollectHealthReportFindsHiddenWorktreeOrphanBundleDirs(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	worktreeRoot := addGitWorktree(t, root, "health-hidden-orphan-branch")
	require.NoError(t, EnsureWorkspaceScopeMarker(root, worktreeRoot))
	require.NoError(t, os.Remove(filepath.Join(worktreeRoot, ".slipway.yaml")))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))

	orphanDir := filepath.Join(worktreeRoot, "artifacts", "changes", "hidden-orphan-worktree-bundle")
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	var orphanReasons []string
	for _, finding := range report.Findings {
		for _, reason := range finding.Reasons {
			if reason.Code == "orphan_bundle_directory" {
				orphanReasons = append(orphanReasons, reason.Detail)
			}
		}
	}
	assert.Contains(t, orphanReasons, "hidden-orphan-worktree-bundle")
}

func TestCollectHealthReportMarksInvalidWorktreeBindingNonRepairable(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	change := model.NewChange("invalid-worktree-binding")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.NeedsDiscovery = true
	change.WorktreePath = root
	change.WorktreeBranch = "main"
	require.NoError(t, SaveChange(root, change))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "worktree" || finding.Slug != change.Slug {
			continue
		}
		found = true
		assert.False(t, finding.Repairable, "invalid bound worktree should require explicit operator action")
		assert.Contains(t, finding.Message, "Dedicated worktree binding is invalid")
	}
	assert.True(t, found, "expected worktree integrity finding")
}

func TestCollectHealthReportReportsMissingBoundWorktreeScopeConfig(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	change := model.NewChange("missing-bound-worktree-config")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.NeedsDiscovery = true
	change.WorktreePath = worktreeRoot
	change.WorktreeBranch = "feature"
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.Remove(filepath.Join(worktreeRoot, ".slipway.yaml")))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "worktree" || finding.Slug != change.Slug {
			continue
		}
		for _, reason := range finding.Reasons {
			if reason.Code == "workspace_scope_config_missing" {
				found = true
				assert.True(t, finding.Repairable)
				assert.Contains(t, finding.RepairHint, "slipway repair")
			}
		}
	}
	assert.True(t, found, "expected missing bound-worktree scope config finding")
}

func TestCollectHealthReportFailsWhenWorkspaceDiscoveryFails(t *testing.T) {
	root := createRuntimeLayout(t)
	installFakeGitForStoreTests(t, root, true)

	_, err := CollectHealthReport(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list git worktrees")
}

func TestOrphanBundleSlugsReturnsNonNotExistReadErrors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Dir(ActiveBundlesDir(root)), 0o755))
	require.NoError(t, os.WriteFile(ActiveBundlesDir(root), []byte("not a directory"), 0o644))

	_, err := OrphanBundleSlugs(root)
	require.Error(t, err)
}
