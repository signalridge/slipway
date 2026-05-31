package progression

import (
	"github.com/signalridge/slipway/internal/model"
)

// ResolveNextSkill determines what skill should run at the given state.
// For S0_INTAKE, it dispatches based on the change's IntakeSubStep.
// For S1_PLAN, it dispatches based on the change's PlanSubStep.
// For S2_EXECUTE, it returns wave-orchestration.
// For S3_REVIEW, it returns spec-compliance-review (then code-quality-review via evidence evaluation).
// For S4_VERIFY, it returns goal-verification (then final-closeout via evidence evaluation).
func ResolveNextSkill(change model.Change) (skillName string, evidenceState string) {
	state := change.CurrentState
	switch state {
	case model.StateS0Intake:
		return resolveS0Intake(change)
	case model.StateS1Plan:
		return resolveS1Plan(change)
	case model.StateS2Execute:
		return resolveS2Execute(change)
	case model.StateS3Review:
		return SkillSpecComplianceReview, string(model.StateS3Review)
	case model.StateS4Verify:
		return SkillGoalVerification, string(model.StateS4Verify)

	default:
		return "", ""
	}
}

// resolveS0Intake dispatches within S0_INTAKE based on IntakeSubStep.
func resolveS0Intake(change model.Change) (string, string) {
	switch change.IntakeSubStep {
	case model.IntakeSubStepClarify:
		return SkillIntakeClarification, string(model.StateS0Intake)
	case model.IntakeSubStepResearch:
		return SkillIntakeClarification, string(model.StateS0Intake)
	case model.IntakeSubStepConfirm:
		// Machine-only step: confirms approved summary presence.
		return "", ""
	default:
		return "", ""
	}
}

// resolveS1Plan dispatches within the S1_PLAN state based on PlanSubStep.
func resolveS1Plan(change model.Change) (string, string) {
	switch change.PlanSubStep {
	case model.PlanSubStepResearch:
		return SkillResearchOrchestration, string(model.StateS1Plan)
	case model.PlanSubStepBundle:
		// Machine-only step: no skill needed.
		return "", ""
	case model.PlanSubStepAudit:
		return SkillPlanAudit, string(model.StateS1Plan)
	case model.PlanSubStepValidate:
		// Machine-only step: no skill needed.
		return "", ""
	default:
		return "", ""
	}
}

// resolveS2Execute returns the execution skill. Discovery changes without a
// bound worktree must complete worktree-preflight first.
func resolveS2Execute(change model.Change) (string, string) {
	if change.NeedsDiscovery && change.WorktreePath == "" {
		return SkillWorktreePreflight, string(model.StateS2Execute)
	}
	return SkillWaveOrchestration, string(model.StateS2Execute)
}
