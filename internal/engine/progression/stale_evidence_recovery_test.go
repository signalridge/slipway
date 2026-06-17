package progression

import (
	"os"
	"path/filepath"
	"strings"
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

func TestStaleEvidenceAuthoritiesUseSelectedReviewSet(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()))

	change := model.NewChange("stale-review-default-selection")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.WorkflowPreset = model.WorkflowPresetStandard
	require.NoError(t, state.SaveChange(root, change))
	writeStaleRecoverySelectionBundle(t, root, change, []string{"cmd/review.go"})

	authorities, err := staleEvidenceAuthorities(root, change, true)
	require.NoError(t, err)
	names := staleEvidenceAuthorityNames(authorities)

	assert.Contains(t, names, SkillSpecComplianceReview)
	assert.Contains(t, names, SkillCodeQualityReview)
	assert.Contains(t, names, SkillIndependentReview)
	assert.NotContains(t, names, SkillSecurityReview)
}

func TestStaleEvidenceAuthoritiesIncludeSelectedSecurityReview(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()))

	change := model.NewChange("stale-review-security-selection")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.WorkflowPreset = model.WorkflowPresetStrict
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	require.NoError(t, state.SaveChange(root, change))
	writeStaleRecoverySelectionBundle(t, root, change, []string{
		"cmd/a.go",
		"cmd/b.go",
		"cmd/c.go",
		"cmd/d.go",
		"cmd/e.go",
	})

	authorities, err := staleEvidenceAuthorities(root, change, true)
	require.NoError(t, err)
	names := staleEvidenceAuthorityNames(authorities)

	assert.Contains(t, names, SkillSpecComplianceReview)
	assert.Contains(t, names, SkillCodeQualityReview)
	assert.Contains(t, names, SkillIndependentReview)
	assert.Contains(t, names, SkillSecurityReview)
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
			nextSkills, _ := ResolveNextSkill(reloaded)
			assert.Equal(t, []string{SkillIntakeClarification}, nextSkills,
				"post-recovery next skill must be intake-clarification")

			// The stale verification record is cleared so the rerun re-stamps fresh.
			_, statErr := os.Stat(filepath.Join(bundleDir, "verification", SkillIntakeClarification+".yaml"))
			assert.Truef(t, os.IsNotExist(statErr),
				"stale intake-clarification verification should be cleared, stat err=%v", statErr)
		})
	}
}

func TestStaleEvidenceRecoveryIgnoresIntakeOpenQuestionResolution(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()))

	change := model.NewChange("issue-238-open-question-resolution")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepResearch
	change.NeedsDiscovery = true
	change.ComplexityLevel = "complex"
	change.WorkflowPreset = model.WorkflowPresetStandard
	require.NoError(t, state.SaveChange(root, change))

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	intentPath := filepath.Join(bundleDir, "intent.md")
	require.NoError(t, os.WriteFile(intentPath, []byte(issue238Intent("- [ ] Which digest boundary owns research resolution?\n")), 0o644))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Date(2026, 6, 16, 1, 0, 0, 0, time.UTC),
		RunVersion: 0,
		References: []string{"intake:pass"},
	}
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, record)
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillIntakeClarification, record, nil))

	resolvedOpenQuestions := "- [x] Which digest boundary owns research resolution?\n" +
		"  Resolved: intake digest owns substantive scope; research owns this checklist state.\n"
	require.NoError(t, os.WriteFile(intentPath, []byte(issue238Intent(resolvedOpenQuestions)), 0o644))

	target, ok, err := StaleEvidenceRecoveryAvailable(root, change, nil)
	require.NoError(t, err)
	assert.Falsef(t, ok, "Open Questions resolution must not reopen stale intake evidence, got target=%+v", target)

	require.NoError(t, os.WriteFile(intentPath, []byte(strings.Replace(
		issue238Intent(resolvedOpenQuestions),
		"Fix issue #238.",
		"Fix issue #238 with revised substantive scope.",
		1,
	)), 0o644))

	target, ok, err = StaleEvidenceRecoveryAvailable(root, change, nil)
	require.NoError(t, err)
	require.True(t, ok, "substantive intent changes must still reopen stale intake evidence")
	assert.Equal(t, SkillIntakeClarification, target.SkillName)
	assert.Equal(t, model.StateS0Intake, target.State)
	assert.Contains(t, model.ReasonSpecs(target.Blockers), "required_skill_stale:intake-clarification:intent.md")
}

func issue238Intent(openQuestions string) string {
	return `# Intent

## Summary
Fix issue #238.

## Complexity Assessment
complex

## Guardrail Domains
Governance lifecycle correctness

## In Scope
- Update intake digest behavior.

## Out of Scope
- Redesign all freshness recovery.

## Constraints
- Preserve fail-closed behavior for substantive scope changes.

## Acceptance Signals
- Regression tests cover Open Questions resolution and substantive changes.

## Open Questions
` + openQuestions + `
## Approved Summary
User confirmed the issue #238 scope.
`
}

func staleEvidenceAuthorityNames(authorities []staleEvidenceAuthority) []string {
	names := make([]string, 0, len(authorities))
	for _, authority := range authorities {
		names = append(names, authority.SkillName)
	}
	return names
}

func writeStaleRecoverySelectionBundle(t *testing.T, root string, change model.Change, targetFiles []string) {
	t.Helper()
	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte("# Intent\n\nReview selection.\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`## Requirements

### Requirement: ReviewSelection
REQ-001: Review selection must be explicit.
`), 0o644))
	tasks := "# Tasks\n\n- [ ] `t-01` selected review proof\n  - target_files: ["
	for i, target := range targetFiles {
		if i > 0 {
			tasks += ", "
		}
		tasks += `"` + target + `"`
	}
	tasks += "]\n  - task_kind: code\n  - covers: [REQ-001]\n"
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(tasks), 0o644))
}
