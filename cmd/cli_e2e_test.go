package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeJSONMap(t *testing.T, raw string) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))
	return payload
}

func TestCLIEndToEndDiagnosticsAndCodebaseMapFlow(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		stdout, stderr, err := runRootCommand([]string{"status", "--json"})
		require.Error(t, err)
		assert.Empty(t, stdout)
		errPayload := decodeJSONMap(t, stderr)
		assert.Equal(t, "runtime_failure", errPayload["error_code"])

		stdout, stderr, err = runRootCommand([]string{"init", "--tools", "none"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		assert.Contains(t, stdout, "initialized slipway workspace")

		stdout, stderr, err = runRootCommand([]string{"status", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		statusPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "diagnostics", statusPayload["execution_mode"])

		stdout, stderr, err = runRootCommand([]string{"health", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		healthPayload := decodeJSONMap(t, stdout)
		findings, ok := healthPayload["findings"].([]any)
		require.True(t, ok)
		require.Len(t, findings, 1)

		stdout, stderr, err = runRootCommand([]string{"stats", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		statsPayload := decodeJSONMap(t, stdout)
		codebaseMap, ok := statsPayload["codebase_map"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "missing", codebaseMap["freshness"])

		stdout, stderr, err = runRootCommand([]string{"codebase-map"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		assert.Contains(t, stdout, "Created:")

		stdout, stderr, err = runRootCommand([]string{"health", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		healthPayload = decodeJSONMap(t, stdout)
		assert.Equal(t, "diagnostics", healthPayload["execution_mode"])
		_, hasFindings := healthPayload["findings"]
		assert.False(t, hasFindings)

		stdout, stderr, err = runRootCommand([]string{"stats", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		statsPayload = decodeJSONMap(t, stdout)
		codebaseMap, ok = statsPayload["codebase_map"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "fresh", codebaseMap["freshness"])
	})
}

func TestCLIEndToEndGovernedLifecycleBlockersAndCancel(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		_, _, err := runRootCommand([]string{"init", "--tools", "none"})
		require.NoError(t, err)

		stdout, stderr, err := runRootCommand([]string{"new", "--json", "--preset", "standard", "fix status output"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		createPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "governed", createPayload["mode"])
		assert.Equal(t, "S0_INTAKE", createPayload["current_state"])

		stdout, stderr, err = runRootCommand([]string{"validate", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		validatePayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "S0_INTAKE", validatePayload["current_state"])

		stdout, stderr, err = runRootCommand([]string{"next", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		nextPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "S0_INTAKE", nextPayload["current_state"])

		stdout, stderr, err = runRootCommand([]string{"validate-requirements", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		validateRequirementsPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, true, validateRequirementsPayload["valid"])

		stdout, stderr, err = runRootCommand([]string{"checkpoint", "--json", "--task-id", "task-01", "--type", "human_verify"})
		require.Error(t, err)
		assert.Empty(t, stdout)
		checkpointPayload := decodeJSONMap(t, stderr)
		assert.Equal(t, "checkpoint_wrong_state", checkpointPayload["error_code"])

		stdout, stderr, err = runRootCommand([]string{"pivot", "--json", "--rescope"})
		require.Error(t, err)
		assert.Empty(t, stdout)
		pivotPayload := decodeJSONMap(t, stderr)
		assert.Equal(t, "pivot_state_invalid", pivotPayload["error_code"])

		stdout, stderr, err = runRootCommand([]string{"review", "--json"})
		require.Error(t, err)
		assert.Empty(t, stdout)
		reviewPayload := decodeJSONMap(t, stderr)
		assert.Equal(t, "review_state_invalid", reviewPayload["error_code"])

		stdout, stderr, err = runRootCommand([]string{"done", "--json"})
		require.Error(t, err)
		assert.Empty(t, stdout)
		donePayload := decodeJSONMap(t, stderr)
		assert.Equal(t, "not_done_ready", donePayload["error_code"])

		stdout, stderr, err = runRootCommand([]string{"cancel", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		cancelPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "cancelled", cancelPayload["status"])
		assert.Equal(t, true, cancelPayload["archived"])

		stdout, stderr, err = runRootCommand([]string{"status", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		statusPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "diagnostics", statusPayload["execution_mode"])
	})
}

func TestCLIEndToEndRunBlocksOnNextGovernanceSkill(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L3", "run should stop at research orchestration")

		stdout, stderr, err := runRootCommand([]string{"run", "--json", "--change", slug})
		require.NoError(t, err)
		assert.Empty(t, stderr)

		runPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "S1_PLAN", runPayload["current_state"])

		nextSkill, ok := runPayload["next_skill"].(map[string]any)
		require.True(t, ok, "expected next_skill in run output")
		assert.Equal(t, "research-orchestration", nextSkill["name"])
		assert.Equal(t, "slipway-researcher", nextSkill["agent_hint"])
	})
}

func TestCLIEndToEndRunResumeResponseFlow(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "run resume-response e2e")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:    "task-02",
			PausedWaveIndex: 2,
			CheckpointType:  "human_verify",
		}
		require.NoError(t, state.SaveChange(root, change))
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`task-01`"+` first wave
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` checkpointed second wave
  - depends_on: ["task-01"]
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		stdout, stderr, err := runRootCommand([]string{"run", "--json", "--resume-response", "verified ok", "--change", slug})
		require.NoError(t, err)
		assert.Empty(t, stderr)

		runPayload := decodeJSONMap(t, stdout)
		inputContext, ok := runPayload["input_context"].(map[string]any)
		require.True(t, ok, "expected input_context in run output")
		resumeCheckpoint, ok := inputContext["resume_checkpoint"].(map[string]any)
		require.True(t, ok, "expected resume_checkpoint in run output")
		assert.Equal(t, "task-02", resumeCheckpoint["paused_task_id"])
		assert.Equal(t, float64(2), resumeCheckpoint["paused_wave_index"])
		assert.Equal(t, "verified ok", resumeCheckpoint["user_response_payload"])

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Nil(t, after.ActiveCheckpoint, "run --resume-response should consume the active checkpoint")
	})
}

func TestCLIEndToEndAbortThenRunResumeFlow(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "abort then resume e2e")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave before abort
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` continue second wave after abort
  - depends_on: ["task-01"]
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		materializeWaveExecutionForSummary(t, root, slug)

		stdout, stderr, err := runRootCommand([]string{"abort", "--json", "--change", slug})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		abortPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "S2_EXECUTE", abortPayload["current_state"])

		stdout, stderr, err = runRootCommand([]string{"run", "--json", "--change", slug})
		require.Error(t, err)
		assert.Empty(t, stdout)
		errPayload := decodeJSONMap(t, stderr)
		assert.Equal(t, "resume_required", errPayload["error_code"])

		stdout, stderr, err = runRootCommand([]string{"run", "--json", "--resume", "--change", slug})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		runPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "S2_EXECUTE", runPayload["current_state"])
		nextSkill, ok := runPayload["next_skill"].(map[string]any)
		require.True(t, ok, "expected next_skill in resumed run output")
		assert.Equal(t, "wave-orchestration", nextSkill["name"])

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.True(t, after.InterruptedExecutionAt.IsZero(), "run --resume should clear interrupted execution marker")
	})
}

func TestCLIEndToEndNewRepairAndCancelFlow(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		_, _, err := runRootCommand([]string{"init", "--tools", "none"})
		require.NoError(t, err)

		stdout, stderr, err := runRootCommand([]string{"new", "--json", "--discuss", "--full", "Refine status messaging"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		newPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "governed", newPayload["mode"])
		assert.Equal(t, "full", newPayload["quality_mode"])

		stdout, stderr, err = runRootCommand([]string{"repair", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		repairPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, false, repairPayload["stale_lock_cleaned"])

		stdout, stderr, err = runRootCommand([]string{"cancel", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		cancelPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "cancelled", cancelPayload["status"])
	})
}

func TestCLIEndToEndSuccessfulValidateRequirementsChecksRequirements(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "sync e2e positive path")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		bundleDir, err := state.GovernedBundleDir(root, change)
		require.NoError(t, err)

		// Write requirements.md flat in bundle.
		reqContent := "# Requirements\n\n### Requirement: Token Auth\nREQ-001: The system MUST support token-based authentication.\n"
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(reqContent), 0o644))

		stdout, stderr, err := runRootCommand([]string{"validate-requirements", "--json", "--change", slug})
		require.NoError(t, err)
		assert.Empty(t, stderr)

		view := validateRequirementsView{}
		require.NoError(t, json.Unmarshal([]byte(stdout), &view))
		assert.True(t, view.Valid)
		assert.Equal(t, slug, view.Slug)
		assert.Contains(t, view.Message, "validated")

		// Verify no published directory is created.
		_, err = os.Stat(filepath.Join(root, "artifacts", "requirements", slug))
		assert.True(t, os.IsNotExist(err))
	})
}

func TestCLIEndToEndSuccessfulCheckpointAtS5(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "checkpoint e2e positive path")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Advance to S5_RUN_WAVES.
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		plan, err := state.MaterializeWavePlan(root, change)
		require.NoError(t, err)
		require.NotEmpty(t, plan.Waves)
		require.NotEmpty(t, plan.Waves[0].Tasks)
		taskID := plan.Waves[0].Tasks[0].TaskID

		out := bytes.NewBuffer(nil)
		cmd := makeCheckpointCmd()
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{"--json", "--task-id", taskID, "--type", "human_verify"})
		require.NoError(t, cmd.Execute())

		var view checkpointView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.True(t, view.Set)
		assert.Equal(t, taskID, view.PausedTaskID)
		assert.Equal(t, "human_verify", view.CheckpointType)

		// Verify persisted.
		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		require.NotNil(t, change.ActiveCheckpoint)
		assert.Equal(t, taskID, change.ActiveCheckpoint.PausedTaskID)
	})
}

func TestCLIEndToEndSuccessfulReviewPassAtS7(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "review e2e positive path")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Advance to S7 with persisted execution summary.
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		change.Artifacts = map[string]model.ArtifactState{}
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

		// Write spec with a single requirement.
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		specPath := artifact.ResolveArtifactPath(bundlePath, slug, "requirements.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(specPath), 0o755))
		require.NoError(t, os.WriteFile(specPath, []byte("## Requirements\n\n### Requirement: ReviewE2E\n\nREQ-001: The system MUST support review from the CLI.\n"), 0o644))

		// Write tasks.md covering that requirement.
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte("# Tasks\n\n- [ ] `t-01` implement review e2e\n  - depends_on: []\n  - target_files: [\"cmd/review.go\"]\n  - task_kind: code\n  - covers: [REQ-001]\n"), 0o644))

		// Write passing evidence pack and execution summary AFTER all artifacts
		// so that evidence timestamps post-date artifact modifications.
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		materializeWaveExecutionForSummary(t, root, slug)

		out := bytes.NewBuffer(nil)
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		cmd.SetOut(out)
		cmd.SetErr(out)
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "pass", view.Verdict)
		assert.Equal(t, string(model.StateS4Verify), view.CurrentState)
	})
}

func TestCLIEndToEndSuccessfulDoneArchive(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "done e2e positive path")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		markChangeReadyForDone(t, root, &change)
		writeAssuranceMD(t, root, slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		out := bytes.NewBuffer(nil)
		cmd := makeDoneCmd()
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{"--json"})
		require.NoError(t, cmd.Execute())

		var view doneView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, "done", view.Status)
		assert.True(t, view.Archived)

		// Verify change is archived.
		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeStatusDone, archived.Status)
		assert.Equal(t, model.StateDone, archived.CurrentState)
	})
}

func TestCLIEndToEndValidateRequirementsAfterRequestNext(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		// Create governed change via helper (request + scaffold + advance to S1).
		slug := createGovernedRequest(t, root, "L2", "implement token rotation")

		// Write requirements in the change bundle (flat).
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		bundleDir, err := state.GovernedBundleDir(root, change)
		require.NoError(t, err)
		reqContent := "# Requirements\n\n## Requirements\n\n### Requirement: Token Rotation\nREQ-001: The system MUST rotate tokens on schedule.\n"
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(reqContent), 0o644))

		validateOut := bytes.NewBuffer(nil)
		validateCmd := makeValidateRequirementsCmd()
		validateCmd.SetOut(validateOut)
		validateCmd.SetErr(validateOut)
		validateCmd.SetArgs([]string{"--json", "--change", slug})
		require.NoError(t, validateCmd.Execute())

		realView := validateRequirementsView{}
		require.NoError(t, json.Unmarshal(validateOut.Bytes(), &realView))
		assert.True(t, realView.Valid)
		assert.Contains(t, realView.Message, "validated")

		// No published directory should exist.
		_, err = os.Stat(filepath.Join(root, "artifacts", "requirements", slug))
		assert.True(t, os.IsNotExist(err))
	})
}

func TestMachineReadableCommandsExposeJSONFlag(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  func() *cobra.Command
	}{
		{name: "validate", cmd: makeValidateCmd},
		{name: "review", cmd: makeReviewCmd},
		{name: "done", cmd: makeDoneCmd},
		{name: "cancel", cmd: makeCancelCmd},
		{name: "repair", cmd: makeRepairCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := tc.cmd()
			assert.NotNil(t, command.Flags().Lookup("json"))
		})
	}
}
