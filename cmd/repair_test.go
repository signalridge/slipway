package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepairRejectsPlainGitRepoWithoutSlipwayMarkers(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644))

		err := makeRepairCmd().Execute()
		require.Error(t, err)
		assert.ErrorIs(t, err, fsutil.ErrProjectRootNotFound)

		_, statErr := os.Stat(state.ConfigPath(root))
		require.Error(t, statErr)
		assert.ErrorIs(t, statErr, os.ErrNotExist)
	})
}

func TestRepairRestoresMissingBoundWorktreeScopeMetadata(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair restores missing bound worktree scope metadata")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		worktreeRoot := filepath.Join(t.TempDir(), slug)
		branch := "feat/" + slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")

		bound := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&bound, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, change, bound))
		require.NoError(t, state.SaveChange(root, bound))

		require.NoError(t, os.Remove(state.ConfigPath(worktreeRoot)))
		require.NoError(t, os.Remove(state.WorkspaceScopeMarkerPath(worktreeRoot)))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		_, err = os.Stat(state.ConfigPath(worktreeRoot))
		require.NoError(t, err)
		_, err = os.Stat(state.WorkspaceScopeMarkerPath(worktreeRoot))
		require.NoError(t, err)
		assert.Contains(t, summary.WorktreeScopeRepairs, slug)
	})
}

func TestRepairRejectsEmptyDirectoryWithoutSlipwayMarkers(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		err := makeRepairCmd().Execute()
		require.Error(t, err)
		assert.ErrorIs(t, err, fsutil.ErrProjectRootNotFound)

		_, statErr := os.Stat(state.ConfigPath(root))
		require.Error(t, statErr)
		assert.ErrorIs(t, statErr, os.ErrNotExist)
	})
}

func TestRepairRecoversMissingConfigWhenGitScopedRuntimeMarkerExists(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git", "slipway", "runtime"), 0o755))

		require.NoError(t, makeRepairCmd().Execute())

		cfg, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		assert.Equal(t, model.ArtifactSchemaExpanded, cfg.Defaults.ArtifactSchema)
	})
}

func TestRepairRecoversMissingConfigFromNestedDirectoryWhenRootMarkersExist(t *testing.T) {
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	nested := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.MkdirAll(state.GitRuntimeDir(root), 0o755))

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(nested))
	defer func() {
		_ = os.Chdir(previousWD)
	}()

	require.NoError(t, makeRepairCmd().Execute())

	cfg, err := model.LoadConfig(state.ConfigPath(root))
	require.NoError(t, err)
	assert.Equal(t, model.ArtifactSchemaExpanded, cfg.Defaults.ArtifactSchema)

	_, statErr := os.Stat(state.ConfigPath(nested))
	require.Error(t, statErr)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestRepairRecoversMissingConfigAfterInitWorkspace(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		runGit(t, root, "init")
		runGit(t, root, "commit", "--allow-empty", "-m", "init")

		initTestWorkspace(t, root)
		require.NoError(t, os.Remove(state.ConfigPath(root)))

		require.NoError(t, makeRepairCmd().Execute())

		cfg, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		assert.Equal(t, model.ArtifactSchemaExpanded, cfg.Defaults.ArtifactSchema)
	})
}

func TestRepairRepairsInterruptedArchivedBundleResidue(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := "interrupted-archive"
		change := model.NewChange(slug)
		change.Status = model.ChangeStatusActive
		change.CurrentState = model.StateDone
		change.PlanSubStep = model.PlanSubStepNone
		change.Artifacts = map[string]model.ArtifactState{
			"intent": {ID: "intent", State: model.ArtifactLifecycleDraft},
		}
		require.NoError(t, state.SaveChange(root, change))

		activeBundleDir := filepath.Dir(state.BundleChangeFilePath(root, slug))
		archivedBundleDir := filepath.Join(state.ArchivedBundlesDir(root), slug)
		require.NoError(t, os.MkdirAll(filepath.Dir(archivedBundleDir), 0o755))
		require.NoError(t, os.Rename(activeBundleDir, archivedBundleDir))

		require.NoError(t, os.MkdirAll(state.ChangeDir(root, slug), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Dir(state.TaskPIDFilePath(root, slug)), 0o755))
		require.NoError(t, os.WriteFile(state.TaskPIDFilePath(root, slug), []byte("[]"), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Dir(state.GovernanceSnapshotCachePath(root, slug)), 0o755))
		require.NoError(t, os.WriteFile(state.GovernanceSnapshotCachePath(root, slug), []byte("version: 1\n"), 0o644))

		require.NoError(t, makeRepairCmd().Execute())

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeStatusDone, archived.Status)
		assert.Equal(t, model.ArtifactLifecycleFrozen, archived.Artifacts["intent"].State)

		_, err = os.Stat(state.ChangeDir(root, slug))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))

		_, err = os.Stat(filepath.Dir(state.TaskPIDFilePath(root, slug)))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))

		_, err = os.Stat(filepath.Dir(state.GovernanceSnapshotCachePath(root, slug)))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestRepairReportsOrphanBundleDirectoryFinding(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", "orphan-dir"), 0o755))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		found := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, "orphan-dir") && strings.Contains(finding, "change.yaml") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected orphan bundle finding in repair summary")
	})
}

func TestRepairReportsUnreadableChangeAuthorityFinding(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		change := model.NewChange("corrupt-change")
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, os.WriteFile(state.BundleChangeFilePath(root, change.Slug), []byte("slug: corrupt-change\ncurrent_state: [\n"), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		found := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, "corrupt-change") && strings.Contains(finding, "unreadable") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected unreadable change authority finding in repair summary")
	})
}

func TestRepairReportsHiddenUnreadableChangeAuthorityFinding(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair reports hidden unreadable authority")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		worktreeRoot := filepath.Join(t.TempDir(), slug)
		branch := "feat/" + slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")

		bound := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&bound, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, change, bound))
		require.NoError(t, state.SaveChange(root, bound))

		require.NoError(t, os.Remove(state.ConfigPath(worktreeRoot)))
		require.NoError(t, os.Remove(state.WorkspaceScopeMarkerPath(worktreeRoot)))
		require.NoError(t, os.WriteFile(state.BundleChangeFilePath(worktreeRoot, slug), []byte("slug: hidden\ncurrent_state: [\n"), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		found := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, slug) && strings.Contains(finding, "change authority unreadable") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected hidden unreadable change authority finding in repair summary")
	})
}

func TestRepairReportsUnreadableExecutionSummaryFinding(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair reports unreadable execution summary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		summaryPath := executionSummaryPathForTest(root, slug)
		require.NoError(t, os.MkdirAll(filepath.Dir(summaryPath), 0o755))
		require.NoError(t, os.WriteFile(summaryPath, []byte("version: ["), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		found := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, slug) && strings.Contains(finding, "execution summary unreadable") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected execution summary finding in repair summary")
	})
}

func TestRepairMaterializesWavePlanRecoversWaveRunsAndClearsStaleCheckpoint(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair should recover wave execution artifacts")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:    "t-01",
			PausedWaveIndex: 1,
			PausedAt:        time.Now().UTC().Add(-10 * time.Minute),
			CheckpointType:  string(model.CheckpointHumanVerify),
		}
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` recover wave execution state
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.Contains(t, summary.MaterializedWavePlans, slug)
		assert.Contains(t, summary.RecoveredWaveRuns, slug+"@rv1")
		assert.Contains(t, summary.ClearedCheckpoints, slug)

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		_, err = state.LoadWavePlanForChange(root, change)
		require.NoError(t, err)
		runs, err := state.LoadWaveRuns(root, slug, 1)
		require.NoError(t, err)
		require.Len(t, runs, 1)

		assert.Nil(t, change.ActiveCheckpoint)
	})
}

func TestRepairRebuildsUnreadableWavePlanAndWaveRuns(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair should rebuild unreadable wave artifacts")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` rebuild unreadable wave artifacts
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		materializeWaveExecutionForSummary(t, root, slug)

		require.NoError(t, os.WriteFile(state.WavePlanPathForRead(root, slug), []byte("version: [\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(state.WaveEvidenceDir(root, slug, 1), "wave-01.yaml"), []byte("wave_index: [\n"), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.Contains(t, summary.MaterializedWavePlans, slug)
		assert.Contains(t, summary.RecoveredWaveRuns, slug+"@rv1")

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		plan, err := state.LoadWavePlanForChange(root, change)
		require.NoError(t, err)
		assert.Equal(t, model.WavePlanVersion, plan.Version)

		runs, err := state.LoadWaveRuns(root, slug, 1)
		require.NoError(t, err)
		require.Len(t, runs, 1)
		assert.Equal(t, model.WaveVerdictPass, runs[0].Verdict)
	})
}

func TestRepairDoesNotRewriteHistoricalExecutionStateWhenTasksDrifted(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair should not rewrite drifted historical execution")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:    "t-01",
			PausedWaveIndex: 1,
			PausedAt:        time.Now().UTC(),
			CheckpointType:  string(model.CheckpointHumanVerify),
		}
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` historical executed task
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_id":  "t-01",
			"verdict":  "pass",
			"evidence": "historical",
		})

		tasksPath := filepath.Join(bundlePath, "tasks.md")
		updatedAt := time.Now().UTC().Add(2 * time.Second)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-02`"+` replacement task after drift
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))
		require.NoError(t, os.Chtimes(tasksPath, updatedAt, updatedAt))

		evidencePath := filepath.Join(state.EvidenceTasksDir(root, slug, 1), "t-01.json")
		_, err = os.Stat(evidencePath)
		require.NoError(t, err)

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		assert.NotContains(t, summary.MaterializedWavePlans, slug)
		assert.NotContains(t, summary.RecoveredWaveRuns, slug+"@rv1")
		assert.NotContains(t, summary.ClearedCheckpoints, slug)
		assert.NotContains(t, summary.PrunedTaskEvidence, filepath.ToSlash(filepath.Join(slug, "rv1", "t-01.json")))

		foundBlocked := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, slug) && strings.Contains(finding, "wave plan repair blocked") {
				foundBlocked = true
				break
			}
		}
		assert.True(t, foundBlocked, "expected repair summary to report blocked wave-plan reconstruction")

		_, err = os.Stat(evidencePath)
		require.NoError(t, err, "historical task evidence must be preserved")

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		_, err = state.LoadWavePlanForChange(root, change)
		require.Error(t, err, "repair must not materialize a replacement wave-plan when current tasks drifted")

		require.NotNil(t, change.ActiveCheckpoint, "repair must preserve the historical checkpoint when reconstruction is blocked")
		assert.Equal(t, "t-01", change.ActiveCheckpoint.PausedTaskID)
	})
}
