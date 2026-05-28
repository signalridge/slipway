package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiagnoseBundleConsistencyAllPresent(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("consistent-bundle")
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(root, "artifacts", "changes", "consistent-bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	// change.yaml is written by SaveChange into the bundle directory.
	// Add tasks.md and assurance.md.
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte("# Assurance\n"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

func TestDiagnoseBundleConsistencyChangeYamlMissing(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("no-change-yaml")

	// Create slug registry dir and bundle dir, but don't write change.yaml via SaveChange.
	require.NoError(t, os.MkdirAll(ChangeDir(root, "no-change-yaml"), 0o755))
	bundleDir := filepath.Join(root, "artifacts", "changes", "no-change-yaml")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "change.yaml missing")
}

func TestDiagnoseBundleConsistencyTasksMissing(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("no-tasks")
	require.NoError(t, SaveChange(root, change))

	// Remove tasks.md — only change.yaml in bundle.
	bundleDir := filepath.Join(root, "artifacts", "changes", "no-tasks")

	result := DiagnoseBundleConsistency(root, change)
	_ = bundleDir
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "tasks.md missing")
}

func TestDiagnoseBundleConsistencyAssuranceMissingWarningPreReview(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("no-assurance-early")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(root, "artifacts", "changes", "no-assurance-early")
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "assurance.md missing")
}

func TestDiagnoseBundleConsistencyAssuranceMissingErrorInReview(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("no-assurance-review")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(root, "artifacts", "changes", "no-assurance-review")
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "assurance.md missing")
}

func TestDiagnoseBundleConsistencyLightPresetDoesNotRequireAssuranceInReview(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("light-no-assurance-review")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(root, "artifacts", "changes", "light-no-assurance-review")
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

func TestDiagnoseBundleConsistencyUsesCanonicalConfigForBoundWorkspace(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	worktreeRoot := t.TempDir()
	require.NoError(t, os.WriteFile(ConfigPath(root), []byte("governance:\n  min_preset: strict\n"), 0o644))
	require.NoError(t, os.WriteFile(ConfigPath(worktreeRoot), []byte("governance:\n  min_preset: light\n"), 0o644))

	change := model.NewChange("worktree-light-no-assurance-review")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(worktreeRoot, "artifacts", "changes", change.Slug)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	assert.Contains(t, result.Errors, "assurance.md missing in governed bundle — required for review/verify/done phase")
	assert.Empty(t, result.Warnings)
}

func TestDiagnoseBundleConsistencyNoBundleNoop(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("no-bundle")

	result := DiagnoseBundleConsistency(root, change)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

func TestRepairMissingConfigCreatesDefault(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	// Remove config so repair triggers.
	os.Remove(ConfigPath(root))

	_, err := RepairCorruptConfig(root, time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	cfg, err := model.LoadConfig(ConfigPath(root))
	require.NoError(t, err)
	assert.Equal(t, model.ArtifactSchemaExpanded, cfg.Defaults.ArtifactSchema)
}

func TestRepairArchivedTerminalStatusSanitizesSiblingWorktreeArchiveInPlace(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	slug := "repair-worktree-archive"
	bundleDir := filepath.Join(worktreeRoot, "artifacts", "changes", slug)
	change := model.NewChange(slug)
	change.WorktreePath = worktreeRoot
	change.Status = model.ChangeStatusDone
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	change.Artifacts = map[string]model.ArtifactState{
		"intent": {
			ID:    "intent",
			Path:  filepath.Join(bundleDir, "intent.md"),
			State: model.ArtifactLifecycleFrozen,
		},
	}
	require.NoError(t, SaveChange(root, change))

	archivedBundleDir := filepath.Join(worktreeRoot, "artifacts", "changes", "archived", slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(archivedBundleDir), 0o755))
	require.NoError(t, os.Rename(bundleDir, archivedBundleDir))
	require.NoError(t, os.MkdirAll(ChangeDir(root, slug), 0o755))

	repaired, err := RepairArchivedTerminalStatus(root, slug)
	require.NoError(t, err)
	assert.True(t, repaired)

	_, err = os.Stat(filepath.Join(archivedBundleDir, "change.yaml"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(root, "artifacts", "changes", "archived", slug))
	assert.True(t, os.IsNotExist(err))

	raw, err := os.ReadFile(filepath.Join(archivedBundleDir, "change.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "worktree_path:")
	assert.NotContains(t, string(raw), worktreeRoot)
	assert.Contains(t, string(raw), "path: intent.md")

	loaded, err := LoadArchivedChange(root, slug)
	require.NoError(t, err)
	assert.Empty(t, loaded.WorktreePath)
	assert.Equal(t, "intent.md", loaded.Artifacts["intent"].Path)

	_, err = os.Stat(ChangeDir(root, slug))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestDiagnoseBundleConsistencyDetectsCorruptChangeYaml(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("corrupt-yaml-diag")
	require.NoError(t, SaveChange(root, change))

	paths, err := ResolveChangePaths(root, change)
	require.NoError(t, err)

	// Write corrupt YAML into the bundle's change.yaml.
	require.NoError(t, os.WriteFile(
		filepath.Join(paths.GovernedBundleDir, "change.yaml"),
		[]byte("slug: [invalid yaml"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	require.NotEmpty(t, result.Errors, "should detect corrupt change.yaml")
	assert.Contains(t, result.Errors[0], "YAML parse error")
}
