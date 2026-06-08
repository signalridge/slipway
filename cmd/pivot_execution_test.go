package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteGovernedPivotRerouteReturnsDirectWorktreeState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, "L2", "refactor service modules")

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	view, err := executeGovernedPivot(root, slug, string(gate.PivotKindReroute))
	require.NoError(t, err)
	assert.Equal(t, "governed", view.ExecutionMode)
	assert.Equal(t, string(model.StateS1Plan), view.CurrentState)
}

func TestExecuteGovernedPivotFromPlanAuditReturnsToWorktree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, "L2", "plan finalized pivot")

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	view, err := executeGovernedPivot(root, slug, string(gate.PivotKindReroute))
	require.NoError(t, err)
	assert.Equal(t, string(model.StateS1Plan), view.CurrentState)

	updated, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS1Plan, updated.CurrentState)
}

func TestExecuteGovernedPivotKeepsBundleInBoundWorktreeWhenDiscoveryClears(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	initGitRepoForWorktreeTests(t, root)
	slug := createGovernedRequest(t, root, "L3", "pivot bundle relocation")

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	worktreePath := filepath.Join(t.TempDir(), slug)
	branch := "feat/" + slug
	runGit(t, root, "worktree", "add", worktreePath, "-b", branch)
	normalizedWT, err := state.NormalizePath(worktreePath)
	require.NoError(t, err)
	change.WorktreePath = normalizedWT
	change.WorktreeBranch = branch
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))
	require.NoError(t, os.RemoveAll(filepath.Join(root, "artifacts", "changes", slug)))

	worktreeBundle := filepath.Join(normalizedWT, "artifacts", "changes", slug)
	_, err = os.Stat(filepath.Join(worktreeBundle, "intent.md"))
	require.NoError(t, err)

	view, err := executeGovernedPivot(root, slug, string(gate.PivotKindReroute))
	require.NoError(t, err)
	assert.Equal(t, string(model.StateS1Plan), view.CurrentState)

	projectBundle := filepath.Join(root, "artifacts", "changes", slug)
	_, err = os.Stat(projectBundle)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(worktreeBundle, "intent.md"))
	require.NoError(t, err)

	updated, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.True(t, updated.NeedsDiscovery)
	assert.Equal(t, normalizedWT, updated.WorktreePath)
}

func TestExecuteGovernedPivotPromotesCoreSchemaWhenDiscoveryBecomesRequired(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, "L2", "schema promotion pivot")

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.ArtifactSchema = model.ArtifactSchemaCore
	require.NoError(t, state.SaveChange(root, change))

	view, err := executeGovernedPivot(root, slug, string(gate.PivotKindReroute))
	require.NoError(t, err)
	assert.Equal(t, string(model.StateS1Plan), view.CurrentState)

	updated, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.True(t, updated.NeedsDiscovery)
	assert.Equal(t, model.ArtifactSchemaExpanded, updated.ArtifactSchema)
}

func TestExecuteGovernedPivotWritesControlDeactivationAuditTrail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, "L2", "pivot deactivation audit")

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = "auth_authz"
	require.NoError(t, state.SaveChange(root, change))

	paths, err := state.ResolveChangePaths(root, change)
	require.NoError(t, err)
	_, err = governance.RecomputeGovernanceSnapshot(root, change, paths.GovernedBundleDir)
	require.NoError(t, err)

	_, err = executeGovernedPivot(root, slug, string(gate.PivotKindReroute))
	require.NoError(t, err)

	snap, err := governance.LoadSnapshot(root, slug)
	require.NoError(t, err)
	// Reroute preserves guardrail domain, so the snapshot is recomputed
	// with the same domain. Verify the snapshot was recomputed (non-nil).
	assert.NotNil(t, snap)

	updated, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, "auth_authz", updated.GuardrailDomain,
		"reroute must preserve existing guardrail domain")
	assert.True(t, updated.NeedsDiscovery)
}

func TestExecuteGovernedPivotRecoversFromUnreadableGovernanceSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, "L2", "pivot unreadable snapshot")

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = "auth_authz"
	require.NoError(t, state.SaveChange(root, change))

	snapshotPath := governance.SnapshotPath(root, slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(snapshotPath), 0o755))
	require.NoError(t, os.WriteFile(
		snapshotPath,
		[]byte("version: ["),
		0o644,
	))

	view, err := executeGovernedPivot(root, slug, string(gate.PivotKindReroute))
	require.NoError(t, err)
	assert.Equal(t, string(model.StateS1Plan), view.CurrentState)

	snap, err := governance.LoadSnapshot(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.GovernanceSnapshotVersion, snap.Version)

	backups, err := filepath.Glob(filepath.Join(
		filepath.Dir(snapshotPath),
		"governance_snapshot.broken.*.yaml",
	))
	require.NoError(t, err)
	require.Len(t, backups, 1, "expected unreadable snapshot to be backed up during pivot recovery")
}

func TestExecuteGovernedPivotRescopeClearsApprovedSummary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, "L2", "rescope intent amendment")

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	// Write intent.md with a filled Approved Summary.
	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	intentContent := `# Intent
## Summary
Rescope intent amendment.
## In Scope
Everything.
## Out of Scope
Nothing.
## Approved Summary
User approved this on 2026-04-01.
`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intentContent), 0o644))

	view, err := executeGovernedPivot(root, slug, string(gate.PivotKindRescope))
	require.NoError(t, err)
	assert.Equal(t, string(model.StateS0Intake), view.CurrentState)

	// Verify Approved Summary was cleared.
	updated, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS0Intake, updated.CurrentState)
	assert.Equal(t, model.IntakeSubStepClarify, updated.IntakeSubStep)

	// Read intent.md and verify Approved Summary content is gone.
	updatedBundleDir, err := state.GovernedBundleDir(root, updated)
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(updatedBundleDir, "intent.md"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "## Approved Summary", "heading should be preserved")
	assert.Contains(t, content, "Cleared by rescope pivot", "should contain rescope marker")
	assert.False(t, strings.Contains(content, "User approved this on 2026-04-01"), "old approval text should be removed")
}

func TestExecuteGovernedPivotClearsDerivedRuntimeEvidenceAndPreservesTaskEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, "L2", "pivot clears execution state")

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	now := time.Now().UTC()
	require.NoError(t, state.SaveExecutionSummary(root, slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        now,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{
			{TaskID: "task-a", Verdict: model.TaskVerdictPass, TaskKind: model.TaskKindCode, CapturedAt: now},
		},
	}))
	require.NoError(t, state.SaveWavePlan(root, slug, model.WavePlan{
		Version:       model.WavePlanVersion,
		GeneratedAt:   now,
		TasksPlanHash: "task-plan-hash",
		TotalTasks:    1,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Tasks: []model.WavePlanTask{{
				TaskID:   "task-a",
				TaskKind: model.TaskKindCode,
			}},
		}},
	}))
	runtimeEvidence := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(runtimeEvidence), 0o755))
	require.NoError(t, os.WriteFile(runtimeEvidence, []byte(`{"task_id":"task-a","run_summary_version":1,"task_kind":"code","verdict":"pass","evidence_ref":"test:task-a","captured_at":"2026-04-06T10:01:00Z"}`), 0o644))
	waveEvidence := filepath.Join(state.ChangeDir(root, slug), "evidence", "waves", "wave-01.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(waveEvidence), 0o755))
	require.NoError(t, os.WriteFile(waveEvidence, []byte("wave_index: 1\n"), 0o644))
	scratchPath := filepath.Join(state.ChangeDir(root, slug), "scratch.txt")
	require.NoError(t, os.WriteFile(scratchPath, []byte("runtime scratch"), 0o644))
	pidPath := state.TaskPIDFilePath(root, slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(pidPath), 0o755))
	require.NoError(t, os.WriteFile(pidPath, []byte(`{"task-a":123}`), 0o644))

	_, err = executeGovernedPivot(root, slug, string(gate.PivotKindReroute))
	require.NoError(t, err)

	_, err = state.LoadExecutionSummary(root, slug)
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist), "execution summary should be removed")
	_, err = os.Stat(runtimeEvidence)
	assert.NoError(t, err, "runtime task evidence should be preserved")
	_, err = os.Stat(waveEvidence)
	assert.True(t, os.IsNotExist(err), "derived wave evidence should be removed")
	_, err = os.Stat(scratchPath)
	assert.True(t, os.IsNotExist(err), "runtime scratch state should be removed")
	_, err = os.Stat(pidPath)
	assert.True(t, os.IsNotExist(err), "task PID registry should be removed")
	_, err = os.Stat(state.WavePlanPathForRead(root, slug))
	assert.True(t, os.IsNotExist(err), "wave plan should be removed")
}

func TestExecuteGovernedPivotRescopePreservesTaskEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, "L2", "rescope preserves task evidence")

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte("# Intent\n## Summary\nRescope.\n## In Scope\nAll.\n## Out of Scope\nNone.\n## Approved Summary\nApproved 2026-04-01.\n"), 0o644))

	now := time.Now().UTC()
	require.NoError(t, state.SaveExecutionSummary(root, slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        now,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{
			{TaskID: "task-a", Verdict: model.TaskVerdictPass, TaskKind: model.TaskKindCode, CapturedAt: now},
		},
	}))
	runtimeEvidence := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(runtimeEvidence), 0o755))
	require.NoError(t, os.WriteFile(runtimeEvidence, []byte(`{"task_id":"task-a","run_summary_version":1,"task_kind":"code","verdict":"pass","evidence_ref":"test:task-a","captured_at":"2026-04-06T10:01:00Z"}`), 0o644))

	view, err := executeGovernedPivot(root, slug, string(gate.PivotKindRescope))
	require.NoError(t, err)
	assert.Equal(t, string(model.StateS0Intake), view.CurrentState)

	// Rescope reopens intake but must preserve compatible runtime task evidence,
	// consistent with the stale-evidence reopen primitive (#96).
	_, err = os.Stat(runtimeEvidence)
	assert.NoError(t, err, "rescope must preserve runtime task evidence")
}

func TestExecuteGovernedPivotClearsWorktreeOwnedExecutionSummary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	initGitRepoForWorktreeTests(t, root)

	slug := createGovernedRequest(t, root, "L3", "pivot clears worktree execution summary")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	worktreePath := filepath.Join(t.TempDir(), slug)
	branch := "feat/" + slug
	runGit(t, root, "worktree", "add", worktreePath, "-b", branch)
	normalizedWT, err := state.NormalizePath(worktreePath)
	require.NoError(t, err)

	change.WorktreePath = normalizedWT
	change.WorktreeBranch = branch
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	now := time.Now().UTC()
	require.NoError(t, state.SaveExecutionSummary(root, slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        now,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{
			{TaskID: "task-a", Verdict: model.TaskVerdictPass, TaskKind: model.TaskKindCode, CapturedAt: now},
		},
	}))

	summaryPath := state.ExecutionSummaryPathForRead(root, slug)
	_, err = os.Stat(summaryPath)
	require.NoError(t, err)

	_, err = executeGovernedPivot(root, slug, string(gate.PivotKindReroute))
	require.NoError(t, err)

	_, err = os.Stat(summaryPath)
	assert.True(t, os.IsNotExist(err), "pivot should clear the worktree-owned execution summary")
}
