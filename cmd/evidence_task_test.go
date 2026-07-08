package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
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

// TestS3AddedTaskDriftRoutesShipRecoveryToInPlaceConvergence is the end-to-end
// guard for #427: when a task is added to tasks.md at a ship-ready S3_REVIEW, the
// materialized wave plan has not absorbed it yet. Recovery must name `slipway run`
// as the primary in-place convergence action, preserve the present-but-stale ship
// record as ship_verification_evidence_stale, and route premature `evidence task`
// attempts to run convergence before recording the added task's evidence.
func TestS3AddedTaskDriftRoutesShipRecoveryToInPlaceConvergence(t *testing.T) {
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "s3 added task drift reexec")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		ch, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		base, err := progression.EvaluateShipAuthority(root, ch)
		require.NoError(t, err)
		require.Equal(t, model.GateStatusApproved, base.Result.Status,
			"baseline ship authority must be clean before the tasks.md edit")

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` verify ship readiness parity
  - depends_on: []
  - target_files: ["cmd/done.go"]
  - task_kind: verification
  - covers: [REQ-001]

- [ ] `+"`t-07`"+` review-driven repair
  - depends_on: []
  - target_files: ["cmd/validate.go"]
  - task_kind: code
  - covers: [REQ-001]
`)))

		ch, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		ship, err := progression.EvaluateShipAuthority(root, ch)
		require.NoError(t, err)
		specs := model.ReasonSpecs(ship.Result.ReasonCodes)
		assert.Equal(t, model.GateStatusBlocked, ship.Result.Status)
		assert.Contains(t, specs, "s3_task_plan_drift_requires_inplace_convergence:t-07",
			"the added-task drift root must be named for the in-place convergence route")
		assert.Contains(t, specs, "ship_verification_evidence_stale",
			"a present-but-stale ship record must be honest, not reported missing")
		assert.NotContains(t, specs, "ship_verification_evidence_missing")

		recovery := model.BuildRecovery(ship.Result.ReasonCodes)
		require.NotNil(t, recovery)
		assert.Equal(t, "slipway run", recovery.PrimaryCommand,
			"in-place convergence must be the primary recovery step over the stale-skill symptoms")

		ec := commandForRoot(t, root, makeEvidenceCmd())
		ec.SetArgs([]string{
			"task", "--json",
			"--task-id", "t-07",
			"--verdict", "pass",
			"--evidence-ref", "test:t-07",
			"--changed-file", "cmd/validate.go",
			"--change", slug,
		})
		var eb bytes.Buffer
		ec.SetOut(&eb)
		ec.SetErr(&eb)
		execErr := ec.Execute()
		require.Error(t, execErr)
		cliErr := asCLIError(execErr)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_unknown", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Remediation, "slipway run",
			"the added-task evidence precondition must name the in-place convergence route")
		assert.Equal(t, "slipway run", cliErr.Details["remediation_command_hint"])
		require.NotNil(t, cliErr.Recovery)
		assert.Equal(t, "slipway run", cliErr.Recovery.PrimaryCommand,
			"REQ-005: premature added-task evidence must surface recovery.primary_command=slipway run so JSON clients follow the in-place convergence route")
	})
}

func TestEvidenceTaskRecordsRuntimeEvidenceAndBuildsExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		capturedAt := time.Now().UTC()
		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:evidence-task",
			// Must stay within the fixture wave-plan's target_files
			// (cmd/lifecycle_commands_test.go) so the Scope Contract passes; an
			// out-of-scope changed_file now resolves through the scope-contract
			// repair gate rather than a backward lifecycle mutation.
			"--changed-file", "cmd/lifecycle_commands_test.go",
			"--target-file", "cmd/lifecycle_commands_test.go",
			"--captured-at", capturedAt.Format(time.RFC3339Nano),
			"--session-id", "session-a",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceTaskView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		wavePlan, err := state.LoadWavePlanForChange(root, change)
		require.NoError(t, err)
		expectedFreshnessInputs := state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-01", wavePlan.TasksPlanHash)
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, "t-01", view.TaskID)
		assert.True(t, view.Recorded)
		require.NotNil(t, view.InvocationRoute)
		assert.Equal(t, "unbound_active", view.InvocationRoute.Kind)
		assert.Equal(t, slug, view.InvocationRoute.ChangeSlug)
		assert.True(t, view.InvocationRoute.LocalLifecycleExecutionAllowed)
		assert.True(t, view.InvocationRoute.EffectiveLifecycleExecutionAllowed)
		assert.True(t, view.FreshnessInputs.Equal(expectedFreshnessInputs))

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		raw, err := os.ReadFile(taskEvidencePath)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(raw, &payload))
		assert.NotContains(t, payload, "input_hash")
		assert.Contains(t, view.Path, ".git/slipway/runtime/changes/"+slug+"/evidence/tasks/t-01.json")

		task, parsedAt, sessionID, err := progression.ParseTaskEvidence(root, taskEvidencePath, 1)
		require.NoError(t, err)
		assert.Equal(t, "t-01", task.TaskID)
		assert.Equal(t, model.TaskVerdictPass, task.Verdict)
		assert.Equal(t, model.TaskKindVerification, task.TaskKind)
		assert.Equal(t, []string{"cmd/lifecycle_commands_test.go"}, task.ChangedFiles)
		assert.Equal(t, []string{"cmd/lifecycle_commands_test.go"}, task.TargetFiles)
		assert.True(t, capturedAt.Equal(parsedAt))
		assert.Equal(t, "session-a", sessionID)
		assert.True(t, task.FreshnessInputs.Equal(expectedFreshnessInputs))

		writeSkillVerification(t, root, slug, progression.SkillWaveOrchestration, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  capturedAt.Add(time.Second),
			RunVersion: 1,
			References: []string{"task:evidence:t-01"},
		})
		_, err = buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{AutoSkipEvidence: true, Command: "run"})
		require.NoError(t, err)

		summary, err := state.LoadExecutionSummary(root, slug)
		require.NoError(t, err)
		require.Len(t, summary.Tasks, 1)
		assert.Equal(t, "t-01", summary.Tasks[0].TaskID)
		assert.Equal(t, model.TaskVerdictPass, summary.Tasks[0].Verdict)
		assert.True(t, summary.Tasks[0].FreshnessInputs.Equal(expectedFreshnessInputs))
	})
}

func TestEvidenceTaskManualRejectsNoOpJustificationWithChangedFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createMultiTaskEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "code",
			"--verdict", "pass",
			"--evidence-ref", "test:contradiction",
			"--changed-file", "cmd/evidence.go",
			"--target-file", "cmd/evidence.go",
			"--no-op-justification", "must not combine with changed files",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		// The manual path rejects the contradiction before any file is written,
		// so the caller sees a usage error rather than a post-write state defect.
		assert.Equal(t, "evidence_task_no_op_justification_conflict", cliErr.ErrorCode)
		assertTaskEvidenceNotWritten(t, root, slug, "t-01")
	})
}

func TestEvidenceTaskManualRejectsUnjustifiedNoOpCodeTask(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createMultiTaskEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "code",
			"--verdict", "pass",
			"--evidence-ref", "test:manual-no-op-missing",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		// The host-owned manual surface must fail closed at record time, not
		// defer missing change proof to a later scope-contract flag.
		assert.Equal(t, "evidence_task_changed_file_required", cliErr.ErrorCode)
		assertTaskEvidenceNotWritten(t, root, slug, "t-01")
	})
}

func TestEvidenceTaskManualRecordsJustifiedNoOpCodeTask(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createMultiTaskEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "code",
			"--verdict", "pass",
			"--evidence-ref", "test:manual-no-op",
			"--no-op-justification", "honest investigation found no safe behavior-preserving change",
		})
		cmd.SetOut(&bytes.Buffer{})
		require.NoError(t, cmd.Execute())
		assertTaskEvidenceWritten(t, root, slug, "t-01")

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		raw, err := os.ReadFile(taskEvidencePath)
		require.NoError(t, err)
		var persisted struct {
			NoOpJustification string `json:"no_op_justification"`
		}
		require.NoError(t, json.Unmarshal(raw, &persisted))
		assert.Equal(t, "honest investigation found no safe behavior-preserving change", persisted.NoOpJustification)
	})
}

func TestEvidenceTaskManualRejectsNoOpJustificationOutsideEnvelope(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createMultiTaskEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		// t-02 is a test-kind task; a no_op_justification is only valid on a code
		// task, so the manual gate rejects it fail-closed before any file is written.
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-02",
			"--run-summary-version", "1",
			"--task-kind", "test",
			"--verdict", "pass",
			"--evidence-ref", "test:manual-out-of-envelope",
			"--no-op-justification", "no safe behavior-preserving change exists",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_no_op_justification_invalid", cliErr.ErrorCode)
		assertTaskEvidenceNotWritten(t, root, slug, "t-02")
	})
}

func TestEvidenceSkillWaveOrchestrationRejectsMixedTaskEvidenceDespiteStaleExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
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

		fixCmd := commandForRoot(t, root, makeFixCmd())
		fixCmd.SetArgs([]string{"--json", "--change", slug, "--start-reexecution"})
		fixCmd.SetOut(&bytes.Buffer{})
		require.NoError(t, fixCmd.Execute())

		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:stale-run-one",
		})
		writeTaskEvidenceFile(t, root, slug, 2, "t-02", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:active-run-two",
		})

		waveCmd := commandForRoot(t, root, makeEvidenceCmd())
		waveCmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes", "must not stamp stale run one",
		})
		cliErr := asCLIError(waveCmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_task_evidence_run_summary_ambiguous", cliErr.ErrorCode)

		rec, err := state.LoadVerification(root, slug, progression.SkillWaveOrchestration)
		require.NoError(t, err)
		assert.Equal(t, 1, rec.RunVersion)
		assert.NotContains(t, rec.Notes, "must not stamp stale run one")
	})
}

func TestEvidenceSkillWaveOrchestrationRejectsInvalidTaskEvidenceDespiteStaleExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writeTaskEvidenceFile(t, root, slug, 2, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:invalid-freshness",
			"freshness_inputs": map[string]any{
				"change_id":           slug,
				"run_summary_version": 2,
				"task_id":             "wrong-task",
			},
		})

		waveCmd := commandForRoot(t, root, makeEvidenceCmd())
		waveCmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes", "must not hide invalid task evidence behind stale summary",
		})
		cliErr := asCLIError(waveCmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_task_evidence_invalid", cliErr.ErrorCode)
		assert.Equal(t, 2, cliErr.Details["run_summary_version"])

		rec, err := state.LoadVerification(root, slug, progression.SkillWaveOrchestration)
		require.Error(t, err)
		assert.True(t, errors.Is(err, os.ErrNotExist))
		assert.Equal(t, model.VerificationRecord{}, rec)
	})
}

func TestEvidenceSkillWaveOrchestrationUsesNewTaskEvidenceInsteadOfStaleExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)
		writePassingExecutionSummary(t, root, slug, 2, "t-01")
		writeTaskEvidenceFile(t, root, slug, 3, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:active-run-three",
		})

		waveCmd := commandForRoot(t, root, makeEvidenceCmd())
		waveCmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes", "stamp active run three",
		})
		var out bytes.Buffer
		waveCmd.SetOut(&out)
		require.NoError(t, waveCmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, 3, view.RunVersion)

		rec, err := state.LoadVerification(root, slug, progression.SkillWaveOrchestration)
		require.NoError(t, err)
		assert.Equal(t, 3, rec.RunVersion)
		assert.Equal(t, "stamp active run three", rec.Notes)
	})
}

func TestEvidenceSkillRecordsCLIStampedVerificationAndDigest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writePassingExecutionSummary(t, root, slug, 2, "t-01")
		writePassingWaveEvidence(t, root, slug, 2)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		notesPath := filepath.Join(root, "review-notes.md")
		require.NoError(t, os.WriteFile(notesPath, []byte("review notes from disk\n"), 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--json",
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", "pass",
			"--reference", "layer:R0=pass",
			"--reference", "scope_contract:pass",
			"--reference", model.ContextOriginReferencePrefix + model.StageContextReview + "=cli-stamped-spec-review",
			"--reference", "dim:decision_soundness=pass:.slipway.yaml",
			"--reference", "dim:consistency=pass:.slipway.yaml",
			"--notes-file", "review-notes.md",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.Skill)
		assert.Equal(t, 2, view.RunVersion)
		assert.True(t, view.Recorded)
		assert.True(t, view.Stamped)
		assert.Contains(t, view.Path, "verification/"+progression.SkillSpecComplianceReview+".yaml")

		rec, err := state.LoadVerification(root, slug, progression.SkillSpecComplianceReview)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictPass, rec.Verdict)
		assert.Equal(t, 2, rec.RunVersion)
		assert.False(t, rec.Timestamp.IsZero())
		assert.Equal(t, "review notes from disk", rec.Notes)
		assert.Equal(t, []string{
			model.ContextOriginReferencePrefix + model.StageContextReview + "=cli-stamped-spec-review",
			"dim:consistency=pass:.slipway.yaml",
			"dim:decision_soundness=pass:.slipway.yaml",
			"layer:R0=pass",
			"scope_contract:pass",
		}, rec.References)

		digests, err := state.LoadOptionalEvidenceDigestsForChange(root, change)
		require.NoError(t, err)
		require.NotNil(t, digests)
		assert.Contains(t, digests.Skills, progression.SkillSpecComplianceReview)
	})
}

func TestEvidenceSkillRejectsUnknownSkillWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--skill", "../escape",
			"--verdict", "pass",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_invalid", cliErr.ErrorCode)

		_, statErr := os.Stat(state.VerificationFilePath(root, slug, "../escape"))
		require.Error(t, statErr)
	})
}

func TestEvidenceSkillRejectsRunSummaryBoundSkillWithoutExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "skill evidence command")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", "pass",
		})
		err = cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_run_summary_missing", cliErr.ErrorCode)
	})
}

func TestEvidenceSkillRecordsWaveOrchestrationBeforeExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		taskCmd := commandForRoot(t, root, makeEvidenceCmd())
		taskCmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:wave-bootstrap-task",
			"--changed-file", "cmd/lifecycle_commands_test.go",
			"--target-file", "cmd/lifecycle_commands_test.go",
		})
		require.NoError(t, taskCmd.Execute())

		summary, err := state.LoadOptionalExecutionSummary(root, slug)
		require.NoError(t, err)
		require.Nil(t, summary)

		notesPath := filepath.Join(root, "wave-notes.md")
		require.NoError(t, os.WriteFile(notesPath, []byte("wave evidence from task ledger\n"), 0o644))
		skillCmd := commandForRoot(t, root, makeEvidenceCmd())
		skillCmd.SetArgs([]string{
			"skill",
			"--json",
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes-file", "wave-notes.md",
		})
		var out bytes.Buffer
		skillCmd.SetOut(&out)
		require.NoError(t, skillCmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, progression.SkillWaveOrchestration, view.Skill)
		assert.Equal(t, 1, view.RunVersion)
		assert.True(t, view.Recorded)
		assert.True(t, view.Stamped)

		rec, err := state.LoadVerification(root, slug, progression.SkillWaveOrchestration)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictPass, rec.Verdict)
		assert.Equal(t, 1, rec.RunVersion)
		assert.Equal(t, "wave evidence from task ledger", rec.Notes)

		digests, err := state.LoadOptionalEvidenceDigestsForChange(root, model.NewChange(slug))
		require.NoError(t, err)
		require.NotNil(t, digests)
		assert.Contains(t, digests.Skills, progression.SkillWaveOrchestration)
	})
}

func TestEvidenceSkillRejectsWrongWorkflowStateWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS2Implement
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", "pass",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_wrong_state", cliErr.ErrorCode)

		_, statErr := os.Stat(state.VerificationFilePath(root, slug, progression.SkillSpecComplianceReview))
		require.Error(t, statErr)
	})
}

func TestEvidenceTaskRejectsUnsafeTaskID(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "../escape",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:unsafe",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_id_invalid", cliErr.ErrorCode)
	})
}

func TestNormalizeEvidencePathsUsesPublicSlashPaths(t *testing.T) {
	t.Parallel()

	got, err := normalizeEvidencePaths([]string{`cmd\run.go`, "cmd/run.go"})
	require.NoError(t, err)
	assert.Equal(t, []string{"cmd/run.go"}, got)
}

func TestNormalizeEvidencePathsRejectsWindowsAbsolutePath(t *testing.T) {
	t.Parallel()

	_, err := normalizeEvidencePaths([]string{`C:\tmp\file.go`})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace-relative")
}

func TestEvidenceTaskRejectsInvalidVerdictWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "maybe",
			"--evidence-ref", "test:invalid-verdict",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_verdict_invalid", cliErr.ErrorCode)

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		_, statErr := os.Stat(taskEvidencePath)
		require.Error(t, statErr)
		assert.True(t, os.IsNotExist(statErr))
	})
}

func TestEvidenceTaskRejectsFutureCapturedAtWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:future-captured-at",
			"--captured-at", time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano),
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_captured_at_invalid", cliErr.ErrorCode)

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		_, statErr := os.Stat(taskEvidencePath)
		require.Error(t, statErr)
		assert.True(t, os.IsNotExist(statErr))
	})
}

func TestEvidenceTaskRejectsRunVersionMismatchWhenWaveEvidenceExists(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)
		writePassingWaveEvidence(t, root, slug, 2)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:wrong-run-version",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_run_summary_version_mismatch", cliErr.ErrorCode)
		assert.Equal(t, 2, cliErr.Details["expected"])
		assert.Equal(t, 1, cliErr.Details["got"])

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		_, statErr := os.Stat(taskEvidencePath)
		require.Error(t, statErr)
		assert.True(t, os.IsNotExist(statErr))
	})
}

func TestEvidenceTaskRejectsNonWorkspaceRelativePathsWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		flag      string
		value     string
		errorCode string
	}{
		{
			name:      "changed file parent traversal",
			flag:      "--changed-file",
			value:     "../escape.go",
			errorCode: "evidence_task_changed_file_invalid",
		},
		{
			name:      "target file absolute path",
			flag:      "--target-file",
			value:     "/tmp/escape.go",
			errorCode: "evidence_task_target_file_invalid",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			withCommandWorkspace(t, root, func() {
				initTestWorkspace(t, root)
				slug, _ := createEvidenceTaskFixture(t, root)

				cmd := commandForRoot(t, root, makeEvidenceCmd())
				cmd.SetArgs([]string{
					"task",
					"--task-id", "t-01",
					"--run-summary-version", "1",
					"--task-kind", "verification",
					"--verdict", "pass",
					"--evidence-ref", "test:invalid-path",
					tt.flag, tt.value,
				})
				err := cmd.Execute()
				cliErr := asCLIError(err)
				require.NotNil(t, cliErr)
				assert.Equal(t, tt.errorCode, cliErr.ErrorCode)

				taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
				_, statErr := os.Stat(taskEvidencePath)
				require.Error(t, statErr)
				assert.True(t, os.IsNotExist(statErr))
			})
		})
	}
}

func assertTaskEvidenceWritten(t *testing.T, root, slug, taskID string) {
	t.Helper()

	taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), taskID+".json")
	_, statErr := os.Stat(taskEvidencePath)
	require.NoError(t, statErr)
}

func assertTaskEvidenceNotWritten(t *testing.T, root, slug, taskID string) {
	t.Helper()

	taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), taskID+".json")
	_, statErr := os.Stat(taskEvidencePath)
	require.Error(t, statErr)
	assert.True(t, os.IsNotExist(statErr))
}

// corruptWavePlanCache overwrites the engine-owned wave-plan.yaml cache with
// view-only fields (wave_count/advisories) that the persisted schema rejects
// under KnownFields(true), so loadCurrentWavePlanForCommand fails closed with
// state.ErrWavePlanCacheUnreadable.
func corruptWavePlanCache(t *testing.T, root, slug string) {
	t.Helper()
	cachePath := state.WavePlanPathForRead(root, slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o755))
	require.NoError(t, os.WriteFile(cachePath, []byte("wave_count: 1\nadvisories: [\"narrow\"]\nwaves: []\n"), 0o644))
}

// assertWavePlanCacheUnreadableError asserts an evidence command translated a
// corrupt engine-owned cache into the canonical wave_plan_unreadable recovery
// story instead of misdirecting the user to edit tasks.md.
func assertWavePlanCacheUnreadableError(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "wave_plan_unreadable", cliErr.ErrorCode,
		"a corrupt engine-owned cache must surface as wave_plan_unreadable, not a tasks.md-derivation failure")
	assert.Contains(t, cliErr.Remediation, "wave-plan.yaml")
	assert.Contains(t, cliErr.Remediation, "slipway repair")
	assert.Contains(t, cliErr.Remediation, "must not be hand-edited",
		"cache-unreadable remediation must describe the cache as engine-owned / not hand-editable")
	assert.NotContains(t, cliErr.Remediation, "Fix tasks.md",
		"cache-unreadable remediation must not tell the user to edit tasks.md")
}

// TestEvidenceTaskInteractiveCacheUnreadableNamesCacheNotTasks covers the
// `slipway evidence task --task-id` surface: a corrupt engine-owned cache must
// route to the cache + `slipway repair` recovery, not the tasks.md remediation.
func TestEvidenceTaskInteractiveCacheUnreadableNamesCacheNotTasks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)
		corruptWavePlanCache(t, root, slug)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:evidence-task",
			"--changed-file", "cmd/lifecycle_commands_test.go",
			"--target-file", "cmd/lifecycle_commands_test.go",
		})
		assertWavePlanCacheUnreadableError(t, cmd.Execute())
	})
}

// TestEvidenceSkillWaveOrchestrationCacheUnreadableNamesCacheNotTasks covers the
// `slipway evidence skill wave-orchestration` surface: task evidence is recorded
// while the cache is valid, then the cache is corrupted before recording
// wave-orchestration evidence.
func TestEvidenceSkillWaveOrchestrationCacheUnreadableNamesCacheNotTasks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		taskCmd := commandForRoot(t, root, makeEvidenceCmd())
		taskCmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:wave-bootstrap-task",
			"--changed-file", "cmd/lifecycle_commands_test.go",
			"--target-file", "cmd/lifecycle_commands_test.go",
		})
		require.NoError(t, taskCmd.Execute())

		// Corrupt the cache only after valid task evidence exists, so the
		// wave-orchestration run-version derivation reaches the wave-plan load.
		corruptWavePlanCache(t, root, slug)

		notesPath := filepath.Join(root, "wave-notes.md")
		require.NoError(t, os.WriteFile(notesPath, []byte("wave evidence from task ledger\n"), 0o644))
		skillCmd := commandForRoot(t, root, makeEvidenceCmd())
		skillCmd.SetArgs([]string{
			"skill",
			"--json",
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes-file", "wave-notes.md",
		})
		assertWavePlanCacheUnreadableError(t, skillCmd.Execute())
	})
}

func createEvidenceTaskFixture(t *testing.T, root string) (string, model.Change) {
	t.Helper()

	slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence task command")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	return slug, change
}

func createMultiTaskEvidenceTaskFixture(t *testing.T, root string) (string, model.Change) {
	t.Helper()

	slug, change := createEvidenceTaskFixture(t, root)
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` harden result file loading
  - depends_on: []
  - target_files: ["cmd/evidence.go"]
  - task_kind: code
  - covers: [REQ-001]

- [ ] `+"`t-02`"+` cover result file reexecution
  - depends_on: []
  - target_files: ["cmd/evidence_task_test.go"]
  - task_kind: test
  - covers: [REQ-001]
`)))
	_, err := state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	return slug, change
}
