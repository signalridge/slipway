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

func TestWriteRepairTextDoesNotReportNoopWhenUnrepairedDriftExists(t *testing.T) {
	var out bytes.Buffer

	err := writeRepairText(&out, repairSummary{
		UnrepairedDrift: []repairDriftFinding{{
			Reason:     state.StalePlanningEvidenceBlockerToken,
			Target:     "artifacts/changes/demo/tasks.md",
			NextAction: "regenerate wave plan from the current tasks.md",
		}},
	})

	require.NoError(t, err)
	text := out.String()
	assert.Contains(t, text, "Unrepaired drift:")
	assert.Contains(t, text, state.StalePlanningEvidenceBlockerToken)
	assert.NotContains(t, text, "No repairs were needed")
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

func TestRepairDoesNotCleanFreshAtomicTempArtifacts(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		tmpDir := filepath.Join(root, ".git", "slipway", "locks", "changes")
		require.NoError(t, os.MkdirAll(tmpDir, 0o755))
		freshTemp := filepath.Join(tmpDir, ".tmp-demo.lock.meta-fresh")
		staleTemp := filepath.Join(tmpDir, ".tmp-demo.lock.meta-stale")
		require.NoError(t, os.WriteFile(freshTemp, []byte("fresh"), 0o644))
		require.NoError(t, os.WriteFile(staleTemp, []byte("stale"), 0o644))
		old := time.Now().Add(-5 * time.Minute)
		require.NoError(t, os.Chtimes(staleTemp, old, old))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		hasCleanedSuffix := func(suffix string) bool {
			for _, cleaned := range summary.CleanedAtomicTemps {
				if strings.HasSuffix(cleaned, suffix) {
					return true
				}
			}
			return false
		}
		staleSuffix := filepath.Join(".git", "slipway", "locks", "changes", ".tmp-demo.lock.meta-stale")
		freshSuffix := filepath.Join(".git", "slipway", "locks", "changes", ".tmp-demo.lock.meta-fresh")
		assert.True(t, hasCleanedSuffix(staleSuffix))
		assert.False(t, hasCleanedSuffix(freshSuffix))

		_, err := os.Stat(freshTemp)
		require.NoError(t, err)
		_, err = os.Stat(staleTemp)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
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
		require.NoError(t, os.WriteFile(filepath.Join(root, "artifacts", "changes", "orphan-dir", "intent.md"), []byte("# orphan\n"), 0o644))

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

func TestRepairRemovesEmptyOrphanBundleDirectory(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		emptyResidue := filepath.Join(root, "artifacts", "changes", "empty-residue")
		require.NoError(t, os.MkdirAll(filepath.Join(emptyResidue, "verification"), 0o755))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.Contains(t, summary.RemovedEmptyOrphanBundles, "empty-residue")
		assert.Contains(t, summary.AppliedRepairs, repairAppliedFinding{Kind: "empty_orphan_bundle", Target: "empty-residue"})
		assert.Empty(t, summary.NonRepairableFindings)
		require.NoDirExists(t, emptyResidue)
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

func TestRepairRebuildsUnreadableExecutionSummaryWithoutResidualDrift(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair should converge unreadable execution summary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` rebuild unreadable execution summary
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))
		writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
		})
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"changed_files": []string{"cmd/repair.go"},
			"target_files":  []string{"cmd/repair.go"},
		})

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
		assert.Contains(t, summary.MaterializedWavePlans, slug)
		assert.Contains(t, summary.RebuiltExecutionSummaries, slug)
		assert.Contains(t, summary.AppliedRepairs, repairAppliedFinding{Kind: "materialized_wave_plan", Target: slug})
		assert.Contains(t, summary.AppliedRepairs, repairAppliedFinding{Kind: "rebuilt_execution_summary", Target: slug})

		for _, finding := range summary.NonRepairableFindings {
			assert.False(t,
				strings.Contains(finding, slug) && strings.Contains(finding, "execution summary unreadable"),
				"rebuilt summary must not leave unreadable finding: %s", finding,
			)
		}
		for _, drift := range summary.UnrepairedDrift {
			assert.False(t,
				strings.Contains(drift.Target, slug) && strings.Contains(drift.Reason, "execution summary unreadable"),
				"rebuilt summary must not leave unreadable drift: %+v", drift,
			)
		}

		rebuilt, err := state.LoadExecutionSummary(root, slug)
		require.NoError(t, err)
		assert.True(t, state.ExecutionSummaryReady(&rebuilt))
	})
}

func TestBuildUnrepairedDriftFindingsKeepsActionableTargets(t *testing.T) {
	t.Parallel()

	drift := buildUnrepairedDriftFindings([]string{
		"bundle directory exists without change.yaml: orphan-dir",
		"multiple active changes require operator intervention",
		"demo: execution summary unreadable: bad yaml",
	})

	require.Len(t, drift, 3)
	assert.Contains(t, drift, repairDriftFinding{
		Target:     filepath.ToSlash(filepath.Join("artifacts", "changes", "orphan-dir")),
		Reason:     "bundle directory exists without change.yaml",
		NextAction: "repair or replace the authoritative change.yaml before continuing",
	})
	assert.Contains(t, drift, repairDriftFinding{
		Target:     "workspace",
		Reason:     "multiple active changes require operator intervention",
		NextAction: "inspect the named artifact and rerun the owning Slipway command after correction",
	})
	assert.Contains(t, drift, repairDriftFinding{
		Target:     "demo",
		Reason:     "execution summary unreadable: bad yaml",
		NextAction: "regenerate execution-summary.yaml from current wave-backed task evidence",
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
		assert.Contains(t, summary.RecoveredWaveRuns, slug)
		assert.Contains(t, summary.ClearedCheckpoints, slug)
		assert.Contains(t, summary.AppliedRepairs, repairAppliedFinding{Kind: "materialized_wave_plan", Target: slug})
		assert.Contains(t, summary.AppliedRepairs, repairAppliedFinding{Kind: "recovered_wave_run", Target: slug})
		assert.Contains(t, summary.AppliedRepairs, repairAppliedFinding{Kind: "cleared_checkpoint", Target: slug})

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

func TestRepairClearsStaleCheckpointWhenExecutionSummaryUnreadable(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair should clear stale checkpoint despite unreadable summary")
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

- [ ] `+"`t-01`"+` clear stale checkpoint despite unreadable summary
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))

		// Unreadable summary with no recoverable evidence: the summary cannot be
		// rebuilt, so repair must still clear the stale checkpoint instead of
		// leaving execution wedged behind an unreadable summary.
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

		assert.Contains(t, summary.ClearedCheckpoints, slug)
		assert.Contains(t, summary.AppliedRepairs, repairAppliedFinding{Kind: "cleared_checkpoint", Target: slug})

		unreadableReported := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, slug) && strings.Contains(finding, "execution summary unreadable") {
				unreadableReported = true
				break
			}
		}
		assert.True(t, unreadableReported, "unreadable summary must still be reported alongside the checkpoint repair")

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Nil(t, change.ActiveCheckpoint, "stale checkpoint must be cleared even when the summary is unreadable")
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
		require.NoError(t, os.WriteFile(filepath.Join(state.WaveEvidenceDir(root, slug), "wave-01.yaml"), []byte("wave_index: [\n"), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.Contains(t, summary.MaterializedWavePlans, slug)
		assert.Contains(t, summary.RecoveredWaveRuns, slug)

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

func TestRepairReportsMalformedTaskEvidenceWithoutFailing(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair reports malformed task evidence")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` report malformed task evidence
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		require.NoError(t, os.MkdirAll(state.EvidenceTasksDir(root, slug), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(state.EvidenceTasksDir(root, slug), "broken.json"), []byte("{"), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		found := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, slug) &&
				strings.Contains(finding, "task evidence unreadable") &&
				strings.Contains(finding, "broken.json") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected repair to report malformed task evidence without failing")
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

		evidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
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
		assert.NotContains(t, summary.RecoveredWaveRuns, slug)
		assert.NotContains(t, summary.ClearedCheckpoints, slug)
		assert.NotContains(t, summary.PrunedTaskEvidence, filepath.ToSlash(filepath.Join(slug, "t-01.json")))

		foundBlocked := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, slug) && strings.Contains(finding, "wave plan repair blocked") {
				foundBlocked = true
				break
			}
		}
		assert.True(t, foundBlocked, "expected repair summary to report blocked wave-plan reconstruction")
		require.NotEmpty(t, summary.UnrepairedDrift)
		foundWavePlanDrift := false
		for _, drift := range summary.UnrepairedDrift {
			if drift.Target == slug && strings.Contains(drift.Reason, "wave plan repair blocked") {
				foundWavePlanDrift = true
				assert.Contains(t, drift.NextAction, "regenerate or rescope")
				break
			}
		}
		assert.True(t, foundWavePlanDrift, "expected unrepaired drift to keep the wave-plan repair blocker")

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

func TestRepairRebuildsReadyButStaleExecutionSummaryDrift(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair reports stale ready execution summary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` rebuild first stale task
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
- [ ] `+"`t-02`"+` rebuild second stale task
  - wave: 1
  - depends_on: []
  - target_files: ["docs/commands.md"]
  - task_kind: code
`)))
		writePassingWaveEvidence(t, root, slug, 1)

		runtimeCapturedAt := time.Now().UTC()
		summaryCapturedAt := runtimeCapturedAt.Add(time.Hour)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_id":     "t-01",
			"captured_at": runtimeCapturedAt.Format(time.RFC3339Nano),
		})
		writeTaskEvidenceFile(t, root, slug, 1, "t-02", map[string]any{
			"task_id":     "t-02",
			"captured_at": runtimeCapturedAt.Format(time.RFC3339Nano),
		})
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        summaryCapturedAt,
			OverallVerdict:    model.ExecutionVerdictPass,
			CompletedTasks:    []string{"t-01", "t-02"},
			Tasks: []model.ExecutionTaskSummary{{
				TaskID:          "t-01",
				Verdict:         model.TaskVerdictPass,
				TaskKind:        model.TaskKindCode,
				ChangedFiles:    []string{"cmd/repair.go"},
				CapturedAt:      summaryCapturedAt,
				FreshnessInputs: state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-01"),
			}, {
				TaskID:          "t-02",
				Verdict:         model.TaskVerdictPass,
				TaskKind:        model.TaskKindCode,
				ChangedFiles:    []string{"docs/commands.md"},
				CapturedAt:      summaryCapturedAt,
				FreshnessInputs: state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-02"),
			}},
		})

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		assert.Contains(t, summary.RebuiltExecutionSummaries, slug)
		for _, drift := range summary.UnrepairedDrift {
			assert.False(t, strings.Contains(drift.Target, slug), "rebuilt stale summary must not remain unrepaired: %+v", drift)
		}
		require.Contains(t, summary.PathAuthority, slug)
		taskEvidencePath := summary.PathAuthority[slug].TaskEvidencePath
		assert.True(t, filepath.IsAbs(taskEvidencePath))
		assert.True(t, strings.HasSuffix(taskEvidencePath, "/.git/slipway/runtime/changes/"+slug+"/evidence/tasks"), taskEvidencePath)

		rebuilt, err := state.LoadExecutionSummary(root, slug)
		require.NoError(t, err)
		require.Len(t, rebuilt.Tasks, 2)
		for _, task := range rebuilt.Tasks {
			assert.True(t, runtimeCapturedAt.Equal(task.CapturedAt), "task %s captured_at was not rebuilt from runtime evidence", task.TaskID)
		}
	})
}

func TestRepairDoesNotRewriteReadyButStaleExecutionSummaryWhenTaskEvidenceInvalid(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair leaves invalid task evidence alone")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` keep valid stale task
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
- [ ] `+"`t-02`"+` leave invalid task evidence unrepaired
  - wave: 1
  - depends_on: []
  - target_files: ["docs/commands.md"]
  - task_kind: code
`)))
		writePassingWaveEvidence(t, root, slug, 1)

		runtimeCapturedAt := time.Now().UTC()
		summaryCapturedAt := runtimeCapturedAt.Add(time.Hour)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_id":     "t-01",
			"captured_at": runtimeCapturedAt.Format(time.RFC3339Nano),
		})
		writeTaskEvidenceFile(t, root, slug, 1, "t-02", map[string]any{
			"task_id":     "t-02",
			"task_kind":   "not-a-task-kind",
			"captured_at": runtimeCapturedAt.Format(time.RFC3339Nano),
		})
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        summaryCapturedAt,
			OverallVerdict:    model.ExecutionVerdictPass,
			CompletedTasks:    []string{"t-01", "t-02"},
			Tasks: []model.ExecutionTaskSummary{{
				TaskID:          "t-01",
				Verdict:         model.TaskVerdictPass,
				TaskKind:        model.TaskKindCode,
				ChangedFiles:    []string{"cmd/repair.go"},
				CapturedAt:      summaryCapturedAt,
				FreshnessInputs: state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-01"),
			}, {
				TaskID:          "t-02",
				Verdict:         model.TaskVerdictPass,
				TaskKind:        model.TaskKindCode,
				ChangedFiles:    []string{"docs/commands.md"},
				CapturedAt:      summaryCapturedAt,
				FreshnessInputs: state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-02"),
			}},
		})
		summaryPath := executionSummaryPathForTest(root, slug)
		before, err := os.ReadFile(summaryPath)
		require.NoError(t, err)

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.NotContains(t, summary.RebuiltExecutionSummaries, slug)

		foundInvalidEvidence := false
		for _, drift := range summary.UnrepairedDrift {
			if drift.Target == slug && strings.Contains(drift.Reason, "task_evidence_invalid:t-02.json") {
				foundInvalidEvidence = true
				assert.Contains(t, drift.NextAction, "execution-summary.yaml")
			}
		}
		assert.True(t, foundInvalidEvidence, "expected invalid task evidence to remain unrepaired")

		after, err := os.ReadFile(summaryPath)
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after), "repair must not rewrite execution-summary.yaml when task evidence is invalid")
	})
}

func TestRepairDoesNotRebuildWhenPlanningEvidenceIsStale(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair leaves stale planning evidence alone")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "requirements.md", []byte(`# Requirements
### Requirement: Original
REQ-001: Original requirement.
`)))
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` preserve planning drift
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))
		writePassingWaveEvidence(t, root, slug, 1)

		capturedAt := time.Now().UTC()
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"captured_at": capturedAt.Format(time.RFC3339Nano),
		})
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        capturedAt,
			OverallVerdict:    model.ExecutionVerdictPass,
			CompletedTasks:    []string{"t-01"},
			Tasks: []model.ExecutionTaskSummary{{
				TaskID:          "t-01",
				Verdict:         model.TaskVerdictPass,
				TaskKind:        model.TaskKindCode,
				ChangedFiles:    []string{"cmd/repair.go"},
				CapturedAt:      capturedAt,
				FreshnessInputs: state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-01"),
			}},
		})

		requirementsPath := filepath.Join(bundlePath, "requirements.md")
		stalePlanningAt := capturedAt.Add(time.Hour)
		require.NoError(t, os.Chtimes(requirementsPath, stalePlanningAt, stalePlanningAt))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.NotContains(t, summary.RebuiltExecutionSummaries, slug)

		foundPlanningDrift := false
		for _, drift := range summary.UnrepairedDrift {
			if strings.Contains(drift.Target, "requirements.md") &&
				strings.Contains(drift.Reason, state.StalePlanningEvidenceBlockerToken) {
				foundPlanningDrift = true
				assert.Contains(t, drift.NextAction, "plan-audit")
			}
		}
		assert.True(t, foundPlanningDrift, "expected stale planning evidence to remain unrepaired")
	})
}

func TestRepairPathAuthorityUsesLinkedWorktreeInvocationWorkspace(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "repair path authority linked worktree")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		worktreeRoot := filepath.Join(root, ".worktrees", slug)
		branch := "feat/" + slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")

		bound := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&bound, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, change, bound))
		require.NoError(t, state.SaveChange(root, bound))

		runtimeCapturedAt := time.Now().UTC()
		summaryCapturedAt := runtimeCapturedAt.Add(time.Hour)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"captured_at": runtimeCapturedAt.Format(time.RFC3339Nano),
		})
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        summaryCapturedAt,
			OverallVerdict:    model.ExecutionVerdictPass,
			CompletedTasks:    []string{"t-01"},
			Tasks: []model.ExecutionTaskSummary{{
				TaskID:          "t-01",
				Verdict:         model.TaskVerdictPass,
				TaskKind:        model.TaskKindCode,
				ChangedFiles:    []string{"cmd/repair.go"},
				CapturedAt:      summaryCapturedAt,
				FreshnessInputs: state.ExpectedExecutionTaskFreshnessInputs(bound, 1, "t-01"),
			}},
		})

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(worktreeRoot))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		require.Contains(t, summary.PathAuthority, slug)
		require.NotNil(t, summary.PathAuthority[slug])
		assert.Equal(t, state.DisplayPath(root, worktreeRoot), summary.PathAuthority[slug].InvocationWorkspacePath)
		assert.Equal(t, state.DisplayPath(root, worktreeRoot), summary.PathAuthority[slug].BoundWorkspacePath)
	})
}
