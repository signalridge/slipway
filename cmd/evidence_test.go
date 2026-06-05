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

func TestEvidenceRestampTier0EligibleDryRunThenApply(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "evidence restamp tier0 eligible")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		verdictAt := time.Now().UTC().Add(-time.Hour)
		rec := model.VerificationRecord{
			Verdict:   model.VerificationVerdictPass,
			Blockers:  []model.ReasonCode{},
			Timestamp: verdictAt,
		}
		writeSkillVerification(t, root, slug, progression.SkillPlanAudit, rec)

		// Planning inputs settled before the verdict → Tier-0 eligible.
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		beforeVerdict := verdictAt.Add(-time.Minute)
		for _, rel := range []string{"intent.md", "requirements.md", "research.md", "decision.md", "tasks.md"} {
			p := filepath.Join(bundlePath, rel)
			if _, statErr := os.Stat(p); statErr == nil {
				require.NoError(t, os.Chtimes(p, beforeVerdict, beforeVerdict))
			}
		}

		// --dry-run reports eligible without writing the digest.
		var dryOut bytes.Buffer
		dryCmd := commandForRoot(t, root, makeEvidenceCmd())
		dryCmd.SetOut(&dryOut)
		dryCmd.SetArgs([]string{"restamp", "--skill", progression.SkillPlanAudit, "--dry-run", "--json", "--change", slug})
		require.NoError(t, dryCmd.Execute())
		var dryView evidenceRestampView
		require.NoError(t, json.Unmarshal(dryOut.Bytes(), &dryView))
		assert.True(t, dryView.Eligible)
		assert.False(t, dryView.Stamped)
		assert.True(t, dryView.DryRun)
		if dig, derr := state.LoadOptionalEvidenceDigestsForChange(root, change); derr == nil && dig != nil {
			assert.NotContains(t, dig.Skills, progression.SkillPlanAudit, "dry-run must not write the digest")
		}

		// Without --dry-run the eligible digest is stamped.
		var applyOut bytes.Buffer
		applyCmd := commandForRoot(t, root, makeEvidenceCmd())
		applyCmd.SetOut(&applyOut)
		applyCmd.SetArgs([]string{"restamp", "--skill", progression.SkillPlanAudit, "--json", "--change", slug})
		require.NoError(t, applyCmd.Execute())
		var applyView evidenceRestampView
		require.NoError(t, json.Unmarshal(applyOut.Bytes(), &applyView))
		assert.True(t, applyView.Stamped)
		assert.True(t, applyView.Eligible)

		digests, err := state.LoadEvidenceDigestsForChange(root, change)
		require.NoError(t, err)
		assert.Contains(t, digests.Skills, progression.SkillPlanAudit)
	})
}

func TestEvidenceRestampTier0RefusesInputsChangedAfterVerdict(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "evidence restamp refuses drift")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		verdictAt := time.Now().UTC().Add(-time.Hour)
		rec := model.VerificationRecord{
			Verdict:   model.VerificationVerdictPass,
			Blockers:  []model.ReasonCode{},
			Timestamp: verdictAt,
		}
		writeSkillVerification(t, root, slug, progression.SkillPlanAudit, rec)

		// An input changed after the verdict → not Tier-0 safe.
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		requirementsPath := filepath.Join(bundlePath, "requirements.md")
		require.NoError(t, os.WriteFile(requirementsPath, []byte("# Requirements\nREQ-001 changed after verdict\n"), 0o644))
		afterVerdict := verdictAt.Add(time.Hour)
		require.NoError(t, os.Chtimes(requirementsPath, afterVerdict, afterVerdict))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"restamp", "--skill", progression.SkillPlanAudit, "--json", "--change", slug})
		require.NoError(t, cmd.Execute())
		var view evidenceRestampView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.False(t, view.Eligible)
		assert.False(t, view.Stamped)
		assert.Equal(t, "inputs_changed_after_verdict", view.Reason)
		assert.Contains(t, view.ChangedInputs, "requirements.md")
		assert.Equal(t, progression.SkillPlanAudit, view.RerunSkill)
		assert.Contains(t, view.Message, "Re-run")
	})
}

func TestEvidenceRestampTier0DryRunRefusesUnavailableInputs(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "evidence restamp refuses unavailable inputs")

		writeSkillVerification(t, root, slug, progression.SkillFinalCloseout, model.VerificationRecord{
			Verdict:   model.VerificationVerdictPass,
			Blockers:  []model.ReasonCode{},
			Timestamp: time.Now().UTC(),
		})

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"restamp", "--skill", progression.SkillFinalCloseout, "--dry-run", "--json", "--change", slug})
		require.NoError(t, cmd.Execute())

		var view evidenceRestampView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.False(t, view.Eligible)
		assert.False(t, view.Stamped)
		assert.True(t, view.DryRun)
		assert.Equal(t, progression.EvidenceRestampReasonInputsUnavailable, view.Reason)
		assert.Equal(t, progression.SkillFinalCloseout, view.RerunSkill)
		assert.Contains(t, view.Message, "digest inputs are unavailable")
	})
}

func TestEvidenceRestampRequiresSkill(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		_ = createGovernedRequest(t, root, "L2", "evidence restamp requires skill")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{"restamp", "--json"})
		err := cmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_restamp_skill_required", cliErr.ErrorCode)
	})
}

func TestEvidenceTaskWrongStateInS4RoutesToGoalVerificationAndFinalCloseout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		_, change := createEvidenceTaskFixture(t, root)
		change.CurrentState = model.StateS4Verify
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:wrong-state",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_wrong_state", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Remediation, "goal-verification")
		assert.Contains(t, cliErr.Remediation, "final-closeout")
	})
}
