package status

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildProjectionUsesBundleProgressOutsideExecutionStates(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("plan-progress")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithContext(root, change, change.WorkflowPreset, model.ProjectContext{}))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [x] `+"`task-a`"+` completed
  - target_files: ["cmd/status.go"]
  - wave: 1
  - task_kind: code
- [ ] `+"`task-b`"+` pending
  - target_files: ["cmd/status.go"]
  - wave: 2
  - task_kind: verification
`), 0o644))

	projection, err := BuildProjection(root, change, nil, nil, progression.GovernanceReadiness{}, testStageName)
	require.NoError(t, err)
	require.NotNil(t, projection.Progress)
	assert.Equal(t, 1, projection.Progress.TasksCompleted)
	assert.Equal(t, 2, projection.Progress.TasksTotal)
	assert.Equal(t, 0, projection.Progress.RunSummaryVersion)
}

func TestBuildProjectionKeepsExecutionSummaryProgressInExecutionStates(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("execution-progress")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithContext(root, change, change.WorkflowPreset, model.ProjectContext{}))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [x] `+"`task-checklist-only`"+` should not replace execution summary
  - target_files: ["cmd/status.go"]
  - wave: 1
  - task_kind: code
`), 0o644))

	summary := &model.ExecutionSummary{
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		Tasks: []model.ExecutionTaskSummary{
			{TaskID: "task-a", Verdict: model.TaskVerdictPass},
			{TaskID: "task-b", Verdict: model.TaskVerdictFail},
		},
	}

	projection, err := BuildProjection(root, change, summary, nil, progression.GovernanceReadiness{}, testStageName)
	require.NoError(t, err)
	require.NotNil(t, projection.Progress)
	assert.Equal(t, 1, projection.Progress.TasksCompleted)
	assert.Equal(t, 2, projection.Progress.TasksTotal)
	assert.Equal(t, 1, projection.Progress.RunSummaryVersion)
}

func TestBuildProjectionDoesNotSynthesizeWaveProgressWhenWaveRunsAreMissing(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("missing-wave-runs-progress")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithContext(root, change, change.WorkflowPreset, model.ProjectContext{}))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [x] `+"`task-a`"+` completed first wave
  - depends_on: []
  - target_files: ["cmd/status.go"]
  - wave: 1
  - task_kind: code

- [ ] `+"`task-b`"+` pending second wave
  - depends_on: ["task-a"]
  - target_files: ["cmd/status.go"]
  - wave: 2
  - task_kind: code
`), 0o644))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	summary := &model.ExecutionSummary{
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		Tasks: []model.ExecutionTaskSummary{
			{TaskID: "task-a", Verdict: model.TaskVerdictPass},
		},
	}

	projection, err := BuildProjection(root, change, summary, nil, progression.GovernanceReadiness{}, testStageName)
	require.NoError(t, err)
	require.NotNil(t, projection.Progress)
	assert.Equal(t, 2, projection.Progress.TotalWaves)
	assert.Equal(t, 0, projection.Progress.CurrentWaveIndex)
	assert.Equal(t, 0, projection.Progress.CompletedWaves)
	assert.Nil(t, projection.Progress.WavesByVerdict)
}

func TestBuildProjectionDoesNotLabelCompletedExecutionAsResumableWave(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("completed-wave-progress")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithContext(root, change, change.WorkflowPreset, model.ProjectContext{}))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [x] `+"`task-a`"+` completed first wave
  - depends_on: []
  - target_files: ["cmd/status.go"]
  - wave: 1
  - task_kind: code

- [x] `+"`task-b`"+` completed second wave
  - depends_on: ["task-a"]
  - target_files: ["cmd/status.go"]
  - wave: 2
  - task_kind: code
`), 0o644))
	plan, err := state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	summary := &model.ExecutionSummary{
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		Tasks: []model.ExecutionTaskSummary{
			{TaskID: "task-a", Verdict: model.TaskVerdictPass, CapturedAt: time.Now().UTC().Add(-1 * time.Minute)},
			{TaskID: "task-b", Verdict: model.TaskVerdictPass, CapturedAt: time.Now().UTC()},
		},
	}
	runs, err := state.BuildWaveRuns(plan, summary.RunSummaryVersion, summary.Tasks)
	require.NoError(t, err)
	require.NoError(t, state.SaveWaveRuns(root, change.Slug, summary.RunSummaryVersion, runs))

	projection, err := BuildProjection(root, change, summary, nil, progression.GovernanceReadiness{}, testStageName)
	require.NoError(t, err)
	require.NotNil(t, projection.Progress)
	assert.Equal(t, 2, projection.Progress.TotalWaves)
	assert.Equal(t, 2, projection.Progress.CompletedWaves)
	assert.Equal(t, 0, projection.Progress.CurrentWaveIndex)
	assert.Equal(t, map[string]int{"pass": 2}, projection.Progress.WavesByVerdict)
}

func TestBuildProjectionBuildsEvidenceInventoryAndDiagnostics(t *testing.T) {
	change := model.NewChange("inventory")
	change.CurrentState = model.StateS2Execute

	summary := &model.ExecutionSummary{
		RunSummaryVersion: 7,
		OpenBlockers:      model.ReasonCodesFromSpecs([]string{"task:task-b:lint_failed"}),
		Tasks: []model.ExecutionTaskSummary{
			{TaskID: "task-a", Verdict: model.TaskVerdictPass, EvidenceRef: "artifacts/changes/inventory/evidence/tasks/task-a.json"},
			{TaskID: "task-b", Verdict: model.TaskVerdictFail},
		},
	}
	readiness := progression.GovernanceReadiness{
		Diagnostics: []string{"zeta", "alpha", "alpha"},
	}

	projection, err := BuildProjection(t.TempDir(), change, summary, map[string]string{
		"verification.execution-summary": "artifacts/changes/inventory/verification/execution-summary.yaml",
		"artifact.tasks":                 "artifacts/changes/inventory/tasks.md",
	}, readiness, testStageName)
	require.NoError(t, err)

	require.Len(t, projection.SummaryBlockers, 1)
	assert.Equal(t, []string{"task:task-b:lint_failed"}, model.ReasonSpecs(projection.SummaryBlockers))
	require.Len(t, projection.EvidenceInventory.TaskEvidence, 1)
	assert.Equal(t, "task-a", projection.EvidenceInventory.TaskEvidence[0].Key)
	assert.Equal(t, []EvidenceRef{
		{Key: "artifact.tasks", Path: "artifacts/changes/inventory/tasks.md"},
		{Key: "verification.execution-summary", Path: "artifacts/changes/inventory/verification/execution-summary.yaml"},
	}, projection.EvidenceInventory.NonTaskEvidence)
	assert.Equal(t, []string{"alpha", "zeta"}, projection.Diagnostics)
}

func TestBuildProjectionMapsGateStatusAndRequiredArtifactNodes(t *testing.T) {
	change := model.NewChange("projection")
	change.CurrentState = model.StateS1Plan

	readiness := progression.GovernanceReadiness{
		GateEvaluations: map[gate.GateID]gate.GateEvaluation{
			gate.GatePlan: {
				GateID:      gate.GatePlan,
				Status:      model.GateStatusApproved,
				ReasonCodes: nil,
			},
			gate.GateShip: {
				GateID:      gate.GateShip,
				Status:      model.GateStatusBlocked,
				ReasonCodes: model.ReasonCodesFromSpecs([]string{"ship_check_failed"}),
			},
		},
		ArtifactProjection: &progression.ArtifactProjection{
			Nodes: []progression.ArtifactProjectionNode{
				{Name: "intent.md", State: string(model.ArtifactLifecycleApproved), Ready: true, Required: true},
				{Name: "notes.md", State: string(model.ArtifactLifecycleDraft), Ready: false, Required: false},
			},
		},
	}

	projection, err := BuildProjection(t.TempDir(), change, nil, nil, readiness, testStageName)
	require.NoError(t, err)

	require.Contains(t, projection.GateStatus, "G_plan")
	require.Contains(t, projection.GateStatus, "G_ship")
	assert.Equal(t, model.GateStatusApproved, projection.GateStatus["G_plan"].Status)
	assert.Equal(t, model.GateStatusBlocked, projection.GateStatus["G_ship"].Status)
	require.Len(t, projection.ArtifactDAG, 1)
	assert.Equal(t, "intent.md", projection.ArtifactDAG[0].Name)
}

func testStageName(state model.WorkflowState, intake model.IntakeSubStep, plan model.PlanSubStep) string {
	parts := []string{string(state)}
	if intake != "" {
		parts = append(parts, string(intake))
	}
	if plan != "" {
		parts = append(parts, string(plan))
	}
	return strings.Join(parts, "/")
}

func initGitRepo(t *testing.T, root string) {
	t.Helper()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %s failed: %s", strings.Join(args, " "), string(out))
	}

	run("init", "--initial-branch=main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
}
