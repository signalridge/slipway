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

func TestEvidenceSkillRecordsPlanAuditVerification(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "evidence skill records plan audit")
		change := setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepAudit)

		notesPath := filepath.Join(root, "artifacts", "changes", slug, "verification", "plan-audit-notes.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(notesPath), 0o755))
		require.NoError(t, os.WriteFile(notesPath, []byte("Plan audit passed.\n"), 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillPlanAudit,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "plan-audit:pass",
			"--notes-file", notesPath,
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		expectedPath := state.DisplayPath(
			root,
			filepath.Join(root, "artifacts", "changes", slug, "verification", "plan-audit.yaml"),
		)
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, progression.SkillPlanAudit, view.SkillName)
		assert.Equal(t, model.VerificationVerdictPass, view.Verdict)
		assert.Equal(t, 0, view.RunVersion)
		assert.Equal(t, expectedPath, view.Path)
		assert.True(t, view.Recorded)

		rec, err := state.LoadVerification(root, slug, progression.SkillPlanAudit)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictPass, rec.Verdict)
		assert.Empty(t, rec.Blockers)
		assert.False(t, rec.Timestamp.IsZero())
		assert.Equal(t, 0, rec.RunVersion)
		assert.Equal(t, []string{"plan-audit:pass"}, rec.References)
		assert.Equal(t, "Plan audit passed.", rec.Notes)

		digests, err := state.LoadEvidenceDigestsForChange(root, change)
		require.NoError(t, err)
		require.Contains(t, digests.Skills, progression.SkillPlanAudit)
		assert.NotEmpty(t, digests.Skills[progression.SkillPlanAudit].Inputs["tasks.md"])

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, change.CurrentState, reloaded.CurrentState)
		assert.Equal(t, expectedPath, reloaded.EvidenceRefs[progression.SkillPlanAudit])

		events, err := state.ReadLifecycleEvents(root, reloaded)
		require.NoError(t, err)
		require.NotEmpty(t, events)
		assert.Equal(t, "skill.evidence_recorded", events[len(events)-1].EventType)
		assert.Equal(t, "recorded", events[len(events)-1].Result)
	})
}

func TestEvidenceSkillFailOverwritesPlanAuditAndPrunesDigest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "evidence skill fail prunes digest")
		change := setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepAudit)

		passCmd := commandForRoot(t, root, makeEvidenceCmd())
		passCmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillPlanAudit,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "plan-audit:pass",
			"--notes", "Plan audit passed.",
		})
		require.NoError(t, passCmd.Execute())

		digests, err := state.LoadEvidenceDigestsForChange(root, change)
		require.NoError(t, err)
		require.Contains(t, digests.Skills, progression.SkillPlanAudit)

		failCmd := commandForRoot(t, root, makeEvidenceCmd())
		failCmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillPlanAudit,
			"--verdict", model.VerificationVerdictFail,
			"--blocker", "plan_audit_failed",
			"--notes", "Plan audit now fails.",
		})
		require.NoError(t, failCmd.Execute())

		rec, err := state.LoadVerification(root, slug, progression.SkillPlanAudit)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictFail, rec.Verdict)

		digests, err = state.LoadEvidenceDigestsForChange(root, change)
		require.NoError(t, err)
		assert.NotContains(t, digests.Skills, progression.SkillPlanAudit)
	})
}

func TestEvidenceSkillRejectsCodeQualityBeforeSpecCompliance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "evidence skill rejects review ordering")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillCodeQualityReview,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "code-quality:pass",
			"--notes", "Quality review passed.",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_predecessor_required", cliErr.ErrorCode)
		assert.Equal(t, progression.SkillCodeQualityReview, cliErr.Details["skill"])
		assert.Equal(t, progression.SkillSpecComplianceReview, cliErr.Details["required_first"])

		_, err := os.Stat(filepath.Join(state.VerificationDir(root, slug), progression.SkillCodeQualityReview+".yaml"))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestEvidenceSkillRejectsFinalCloseoutBeforeGoalVerification(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "evidence skill rejects closeout ordering")
		setEvidenceSkillChangeState(t, root, slug, model.StateS4Verify, model.PlanSubStepNone)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillFinalCloseout,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "closeout:pass",
			"--notes", "Closeout passed.",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_predecessor_required", cliErr.ErrorCode)
		assert.Equal(t, progression.SkillFinalCloseout, cliErr.Details["skill"])
		assert.Equal(t, progression.SkillGoalVerification, cliErr.Details["required_first"])

		_, err := os.Stat(filepath.Join(state.VerificationDir(root, slug), progression.SkillFinalCloseout+".yaml"))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestEvidenceSkillRejectsRunSummaryBoundWithoutExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "evidence skill requires execution summary")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "spec-compliance:pass",
			"--notes", "Review passed.",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_run_summary_missing", cliErr.ErrorCode)
		assert.Equal(t, slug, cliErr.Slug)
		assert.Equal(t, progression.SkillSpecComplianceReview, cliErr.Details["skill"])

		_, err := os.Stat(filepath.Join(state.VerificationDir(root, slug), progression.SkillSpecComplianceReview+".yaml"))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestEvidenceSkillRecordsWaveOrchestrationFromRuntimeTaskEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		capturedAt := time.Now().UTC().Add(-time.Minute)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "go test ./cmd -run TestEvidenceSkillRecordsWaveOrchestrationFromRuntimeTaskEvidence",
			"captured_at":   capturedAt.Format(time.RFC3339Nano),
		})

		summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		require.NoError(t, err)
		require.Nil(t, summary)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "wave-orchestration:pass",
			"--notes", "Wave orchestration passed.",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		expectedPath := state.DisplayPath(
			root,
			filepath.Join(root, "artifacts", "changes", slug, "verification", "wave-orchestration.yaml"),
		)
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, progression.SkillWaveOrchestration, view.SkillName)
		assert.Equal(t, model.VerificationVerdictPass, view.Verdict)
		assert.Equal(t, 1, view.RunVersion)
		assert.Equal(t, expectedPath, view.Path)
		assert.True(t, view.Recorded)

		rec, err := state.LoadVerification(root, slug, progression.SkillWaveOrchestration)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictPass, rec.Verdict)
		assert.Equal(t, 1, rec.RunVersion)
		assert.Equal(t, []string{"wave-orchestration:pass"}, rec.References)
		assert.Equal(t, "Wave orchestration passed.", rec.Notes)

		digests, err := state.LoadEvidenceDigestsForChange(root, change)
		require.NoError(t, err)
		require.Contains(t, digests.Skills, progression.SkillWaveOrchestration)
		assert.Contains(t, digests.Skills[progression.SkillWaveOrchestration].Inputs, "wave-plan.yaml")
		assert.Contains(t, digests.Skills[progression.SkillWaveOrchestration].Inputs, "runtime_task_evidence")
		assert.NotContains(t, digests.Skills[progression.SkillWaveOrchestration].Inputs, "execution-summary.yaml")

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS2Execute, reloaded.CurrentState)
		assert.Equal(t, expectedPath, reloaded.EvidenceRefs[progression.SkillWaveOrchestration])
	})
}

func TestEvidenceSkillRejectsWrongState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "evidence skill wrong state")
		setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepAudit)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", model.VerificationVerdictPass,
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_wrong_state", cliErr.ErrorCode)
		assert.Equal(t, progression.SkillSpecComplianceReview, cliErr.Details["skill"])
		assert.Equal(t, string(model.StateS3Review), cliErr.Details["required_state"])
		assert.Equal(t, string(model.StateS1Plan), cliErr.Details["current_state"])
	})
}

func TestEvidenceSkillRejectsNotesConflict(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "evidence skill notes conflict")
		setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepAudit)

		notesPath := filepath.Join(root, "notes.md")
		require.NoError(t, os.WriteFile(notesPath, []byte("file notes\n"), 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillPlanAudit,
			"--verdict", model.VerificationVerdictPass,
			"--notes", "inline notes",
			"--notes-file", notesPath,
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_notes_conflict", cliErr.ErrorCode)
	})
}

func setEvidenceSkillChangeState(
	t *testing.T,
	root string,
	slug string,
	workflowState model.WorkflowState,
	planSubStep model.PlanSubStep,
) model.Change {
	t.Helper()

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = workflowState
	change.PlanSubStep = planSubStep
	if workflowState != model.StateS1Plan {
		change.PlanSubStep = model.PlanSubStepNone
	}
	require.NoError(t, state.SaveChange(root, change))
	return change
}
