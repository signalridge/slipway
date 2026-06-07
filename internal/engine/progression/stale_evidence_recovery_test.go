package progression

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaleEvidencePlanSubstepOrderingIncludesValidate(t *testing.T) {
	t.Parallel()

	research := staleEvidencePositionFor(model.StateS1Plan, model.PlanSubStepResearch)
	bundle := staleEvidencePositionFor(model.StateS1Plan, model.PlanSubStepBundle)
	audit := staleEvidencePositionFor(model.StateS1Plan, model.PlanSubStepAudit)
	validate := staleEvidencePositionFor(model.StateS1Plan, model.PlanSubStepValidate)
	execute := staleEvidencePositionFor(model.StateS2Execute, model.PlanSubStepNone)

	assert.Negative(t, compareStaleEvidencePosition(research, bundle))
	assert.Negative(t, compareStaleEvidencePosition(bundle, audit))
	assert.Negative(t, compareStaleEvidencePosition(audit, validate))
	assert.Negative(t, compareStaleEvidencePosition(validate, execute))
}

// TestStalePlanSubStepRankDerivesFromPlanSubStepOrder pins the stale-recovery
// ordering to the canonical planSubStepOrder (REQ-003): the forward substeps
// must rank by their planSubStepOrder index, and `validate` must rank strictly
// after the last forward substep. If planSubStepOrder is ever reordered without
// the rank function tracking it (e.g. a regression back to a hand-maintained
// switch), this fails.
func TestStalePlanSubStepRankDerivesFromPlanSubStepOrder(t *testing.T) {
	t.Parallel()

	for i, sub := range planSubStepOrder {
		assert.Equalf(t, i, stalePlanSubStepRank(sub),
			"rank of %s must equal its planSubStepOrder index", sub)
	}
	assert.Greater(t,
		stalePlanSubStepRank(model.PlanSubStepValidate),
		stalePlanSubStepRank(model.PlanSubStepAudit),
		"validate must rank after the terminal forward substep (audit)")
	assert.Equal(t, len(planSubStepOrder), stalePlanSubStepRank(model.PlanSubStepValidate))
	assert.Equal(t, 0, stalePlanSubStepRank(model.PlanSubStepNone))
}

// TestStaleIntakeRecoveryReopensToClarifyFromMachineOnlySubsteps reproduces #90:
// when intent.md changes after intake-clarification passed and the change has
// already moved onto a machine-only S0 substep (research/confirm), mutating
// `AdvanceGoverned` must reopen S0_INTAKE back to the entry substep (clarify) so
// the host is routed to intake-clarification — never stranded on a substep with
// no routable skill, and never requiring a manual digest edit or a second
// recovery command.
func TestStaleIntakeRecoveryReopensToClarifyFromMachineOnlySubsteps(t *testing.T) {
	cases := []struct {
		name          string
		intakeSubStep model.IntakeSubStep
		needsDiscover bool
	}{
		{name: "from_research", intakeSubStep: model.IntakeSubStepResearch, needsDiscover: true},
		{name: "from_confirm", intakeSubStep: model.IntakeSubStepConfirm, needsDiscover: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			require.NoError(t, model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()))

			change := model.NewChange("s0-stale-intake-" + tc.name)
			change.CurrentState = model.StateS0Intake
			change.IntakeSubStep = tc.intakeSubStep
			change.PlanSubStep = model.PlanSubStepNone
			change.NeedsDiscovery = tc.needsDiscover
			change.WorkflowPreset = model.WorkflowPresetStandard
			require.NoError(t, state.SaveChange(root, change))

			bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
			require.NoError(t, os.MkdirAll(bundleDir, 0o755))
			intentPath := filepath.Join(bundleDir, "intent.md")
			require.NoError(t, os.WriteFile(intentPath, []byte("# Intent\n\n## Summary\nOriginal clarified intent.\n"), 0o644))

			// intake-clarification passed and its digest was stamped from the
			// original intent.md content.
			verdictAt := time.Date(2026, 6, 6, 1, 0, 0, 0, time.UTC)
			record := model.VerificationRecord{
				Verdict:    model.VerificationVerdictPass,
				Timestamp:  verdictAt,
				RunVersion: 1,
			}
			writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, record)
			require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillIntakeClarification, record, nil))

			// intent.md changes after the accepted verdict → intake-clarification
			// is now stale.
			require.NoError(t, os.WriteFile(intentPath, []byte("# Intent\n\n## Summary\nIntent changed after clarification.\n"), 0o644))

			summary, err := AdvanceGoverned(root, change.Slug)
			require.NoError(t, err)
			assert.Equal(t, "advanced", summary.Action)
			assert.Equal(t, "stale_evidence_recovery_started", summary.Reason)
			assert.Equal(t, model.StateS0Intake, summary.ToState)
			assert.Equal(t, string(model.IntakeSubStepClarify), summary.ToSubStep,
				"reopen must land on the clarify substep, not the machine-only substep")
			assert.True(t, summary.RecoveryOnly)

			reloaded, err := state.LoadChange(root, change.Slug)
			require.NoError(t, err)
			assert.Equal(t, model.StateS0Intake, reloaded.CurrentState)
			assert.Equal(t, model.IntakeSubStepClarify, reloaded.IntakeSubStep,
				"#90: reopen from %s must reset the intake substep to clarify", tc.intakeSubStep)

			// Read-only resolution after recovery routes back to intake-clarification.
			nextSkill, _ := ResolveNextSkill(reloaded)
			assert.Equal(t, SkillIntakeClarification, nextSkill,
				"post-recovery next skill must be intake-clarification")

			// The stale verification record is cleared so the rerun re-stamps fresh.
			_, statErr := os.Stat(filepath.Join(bundleDir, "verification", SkillIntakeClarification+".yaml"))
			assert.Truef(t, os.IsNotExist(statErr),
				"stale intake-clarification verification should be cleared, stat err=%v", statErr)
		})
	}
}
