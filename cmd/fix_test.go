package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixJSONSurfacesReviewFindingRepairContract(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		writeTestSubagentConfig(t, root, func(cfg *model.Config) {
			cfg.Subagents.Fix = model.SubagentSlot{
				Type:                model.SubagentTypeNative,
				Name:                "review-repairer",
				SessionInstructions: "Collect all selected reviewer findings before editing files.",
				Timeout:             "40m",
			}
		})

		slug := createGovernedRequest(t, root, levelNonDiscovery, "fix should surface review findings")

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
			Verdict:   model.VerificationVerdictFail,
			Blockers:  []model.ReasonCode{model.NewReasonCode("review_layer_failed", "R1")},
			Timestamp: time.Now().UTC(),
		})

		cmd := commandForRoot(t, root, makeFixCmd())
		cmd.SetArgs([]string{"--json", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view fixView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, string(model.StateS3Review), view.CurrentState)
		assert.Equal(t, "s3-review-repair:"+slug, view.Contract.RepairBatchID)
		assert.True(t, view.Contract.CollectAllSelectedReviewFindingsFirst)
		assert.True(t, view.Contract.RequiresFreshContext)
		assertSubagentDirective(
			t,
			view.Contract.Subagent,
			model.SubagentTypeNative,
			"review-repairer",
			"Collect all selected reviewer findings before editing files.",
			"40m",
			false,
			"allow",
		)
		assert.Contains(t, view.Contract.FindingCollection, "Collect all selected S3 reviewer findings first")
		assert.Contains(t, view.Contract.Dispatch, "Use contract.subagent when present")
		assert.Contains(t, view.Contract.Dispatch, "native fresh-context repair subagent")
		assert.Contains(t, view.Contract.RepairBrief, "One repair brief")
		assert.Equal(t, model.ContextOriginReferencePrefix+model.StageContextFix+"=<repair-subagent-handle>", view.Contract.ContextReference)
		assert.Contains(t, view.Contract.Prohibited, "Do not repair individual review findings before collecting the selected review batch findings.")
		require.NotEmpty(t, view.RepairTargets)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.RepairTargets[0].Reviewer)
		assert.Equal(t, "review_finding", view.RepairTargets[0].Kind)
	})
}

func TestFixRejectsNonReviewState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "fix should reject non review state")

		cmd := commandForRoot(t, root, makeFixCmd())
		cmd.SetArgs([]string{"--json", "--change", slug})
		err := cmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		assert.Equal(t, "fix_state_invalid", cliErr.ErrorCode)
		assert.Equal(t, "slipway plan", cliErr.Details["next_command"])
	})
}

func TestFixStartReexecutionAdvancesWavePlanRunVersionAndReopensS2(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:stale-task-evidence",
		})
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))
		writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
			Verdict:    model.VerificationVerdictFail,
			Blockers:   []model.ReasonCode{model.NewReasonCode("review_layer_failed", "R1")},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
		})

		cmd := commandForRoot(t, root, makeFixCmd())
		cmd.SetArgs([]string{"--json", "--change", slug, "--start-reexecution"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS2Implement, reloaded.CurrentState)

		plan, err := state.LoadWavePlanForChange(root, reloaded)
		require.NoError(t, err)
		assert.Equal(t, 2, plan.RunSummaryVersion)

		_, err = os.Stat(filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json"))
		assert.True(t, os.IsNotExist(err), "starting a fresh execution run must clear stale task evidence")
	})
}
func TestFixStartReexecutionRejectsAdditiveS3Convergence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:preserved-task-evidence",
		})
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` exercise command fixture
  - depends_on: []
  - target_files: ["cmd/lifecycle_commands_test.go"]
  - task_kind: verification
  - covers: [REQ-001]

- [ ] `+"`t-02`"+` review-discovered additive task
  - depends_on: []
  - target_files: ["cmd/fix.go"]
  - task_kind: code
  - covers: [REQ-001]
`)))

		cmd := commandForRoot(t, root, makeFixCmd())
		cmd.SetArgs([]string{"--json", "--change", slug, "--start-reexecution"})
		cmd.SetOut(&bytes.Buffer{})
		err := cmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "fix_start_reexecution_inplace_convergence_available", cliErr.ErrorCode)
		assert.Equal(t, "slipway run", cliErr.Details["remediation_command_hint"])
		assert.Contains(t, cliErr.Details["added_tasks"], "t-02")
		require.NotNil(t, cliErr.Recovery)
		assert.Equal(t, "slipway run", cliErr.Recovery.PrimaryCommand,
			"REQ-005: guarded S3 task-plan amendment reexecution must surface recovery.primary_command=slipway run so JSON clients follow the in-place convergence path")

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS3Review, reloaded.CurrentState)
		plan, err := state.LoadWavePlanForChange(root, reloaded)
		require.NoError(t, err)
		assert.Equal(t, 1, plan.RunSummaryVersion)
		assertTaskEvidenceWritten(t, root, slug, "t-01")
	})
}

func TestFixStartReexecutionRejectsEditedS3Convergence(t *testing.T) {
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:preserved-task-evidence",
		})
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` exercise amended command fixture
  - depends_on: []
  - target_files: ["cmd/fix.go"]
  - task_kind: code
  - covers: [REQ-001]
`)))

		cmd := commandForRoot(t, root, makeFixCmd())
		cmd.SetArgs([]string{"--json", "--change", slug, "--start-reexecution"})
		cmd.SetOut(&bytes.Buffer{})
		err := cmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "fix_start_reexecution_inplace_convergence_available", cliErr.ErrorCode)
		assert.Equal(t, "slipway run", cliErr.Details["remediation_command_hint"])
		assert.Contains(t, cliErr.Details["changed_tasks"], "t-01")
		assert.Empty(t, cliErr.Details["added_tasks"])
		assert.Equal(t, "--discard-prior-evidence", cliErr.Details["override_flag"])
		require.NotNil(t, cliErr.Recovery)
		assert.Equal(t, "slipway run", cliErr.Recovery.PrimaryCommand)

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS3Review, reloaded.CurrentState)
		plan, err := state.LoadWavePlanForChange(root, reloaded)
		require.NoError(t, err)
		assert.Equal(t, 1, plan.RunSummaryVersion)
		assertTaskEvidenceWritten(t, root, slug, "t-01")
	})
}

func TestFixStartReexecutionRejectsTargetFilesOnlyS3Convergence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:preserved-scope-task-evidence",
		})
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` exercise command fixture
  - depends_on: []
  - target_files: ["cmd/lifecycle_commands_test.go", "cmd/fix.go"]
  - task_kind: verification
  - covers: [REQ-001]
`)))

		cmd := commandForRoot(t, root, makeFixCmd())
		cmd.SetArgs([]string{"--json", "--change", slug, "--start-reexecution"})
		cmd.SetOut(&bytes.Buffer{})
		err := cmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "fix_start_reexecution_inplace_convergence_available", cliErr.ErrorCode)
		assert.Equal(t, "slipway run", cliErr.Details["remediation_command_hint"])
		assert.Contains(t, cliErr.Details["changed_tasks"], "t-01")
		require.NotNil(t, cliErr.Recovery)
		assert.Equal(t, "slipway run", cliErr.Recovery.PrimaryCommand)

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS3Review, reloaded.CurrentState)
		plan, err := state.LoadWavePlanForChange(root, reloaded)
		require.NoError(t, err)
		assert.Equal(t, 1, plan.RunSummaryVersion)
		assertTaskEvidenceWritten(t, root, slug, "t-01")
	})
}

func TestFixStartReexecutionDiscardPriorEvidenceOverridesS3ConvergenceGuard(t *testing.T) {
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:discarded-task-evidence",
		})
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` exercise command fixture
  - depends_on: []
  - target_files: ["cmd/lifecycle_commands_test.go"]
  - task_kind: verification
  - covers: [REQ-001]

- [ ] `+"`t-02`"+` review-discovered additive task
  - depends_on: []
  - target_files: ["cmd/fix.go"]
  - task_kind: code
  - covers: [REQ-001]
`)))

		cmd := commandForRoot(t, root, makeFixCmd())
		cmd.SetArgs([]string{"--json", "--change", slug, "--start-reexecution", "--discard-prior-evidence"})
		cmd.SetOut(&bytes.Buffer{})
		require.NoError(t, cmd.Execute())

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS2Implement, reloaded.CurrentState)
		plan, err := state.LoadWavePlanForChange(root, reloaded)
		require.NoError(t, err)
		assert.Equal(t, 2, plan.RunSummaryVersion)
		assertTaskEvidenceNotWritten(t, root, slug, "t-01")
	})
}

func TestFixDiscardPriorEvidenceRequiresStartReexecution(t *testing.T) {
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "discard evidence flag requires reexecution")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeFixCmd())
		cmd.SetArgs([]string{"--json", "--change", slug, "--discard-prior-evidence"})
		cmd.SetOut(&bytes.Buffer{})
		err = cmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "discard_prior_evidence_requires_reexecution", cliErr.ErrorCode)
	})
}
