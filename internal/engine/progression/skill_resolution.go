package progression

import (
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
)

// ResolveNextSkill determines what skills should run at the given state.
//
// It returns a skill set rather than a single skill: most states route to a
// single skill, but S3_REVIEW dispatches BOTH spec-compliance-review and
// code-quality-review as a concurrent peer pair. The returned slice is empty
// for machine-only steps and unknown states. evidenceState is the workflow
// state the resulting evidence is recorded against.
//
//   - For S0_INTAKE, it dispatches based on the change's IntakeSubStep.
//   - For S1_PLAN, it dispatches based on the change's PlanSubStep.
//   - For S2_IMPLEMENT, it returns wave-orchestration.
//   - For S3_REVIEW, it returns the workflow-profile-filtered selected review
//     peer set (all run concurrently; none precedes another).
func ResolveNextSkill(change model.Change) (skillNames []string, evidenceState string) {
	return ResolveNextSkillWithReviewSelection(change, skill.ReviewSkillSelection{})
}

func ResolveNextSkillWithReviewSelection(
	change model.Change,
	reviewSelection skill.ReviewSkillSelection,
) (skillNames []string, evidenceState string) {
	state := change.CurrentState
	switch state {
	case model.StateS0Intake:
		return resolveS0Intake(change)
	case model.StateS1Plan:
		return resolveS1Plan(change)
	case model.StateS2Implement:
		return resolveS2Implement(change)
	case model.StateS3Review:
		// Parallel review set: all selected peer reviews dispatch concurrently
		// and are unordered. Per-skill evidence evaluation then routes S3
		// closeout authorities.
		return skill.SelectedReviewSkillsForWorkflowProfile(reviewSelection, change.EffectiveWorkflowProfile()), string(model.StateS3Review)
	default:
		return nil, ""
	}
}

// PrimaryNextSkill returns the conventional single primary skill for the given
// state, selecting the first member of the resolved skill set. It exists for
// callers that genuinely need exactly one authority skill (e.g. stale-evidence
// ordering); the routing surface itself must use ResolveNextSkill so S3 exposes
// the selected review set. The primary at S3_REVIEW is spec-compliance-review by
// convention; selected reviews remain unordered peers for gating.
func PrimaryNextSkill(change model.Change) (skillName string, evidenceState string) {
	return PrimaryNextSkillWithReviewSelection(change, skill.ReviewSkillSelection{})
}

func PrimaryNextSkillWithReviewSelection(
	change model.Change,
	reviewSelection skill.ReviewSkillSelection,
) (skillName string, evidenceState string) {
	skills, evidenceState := ResolveNextSkillWithReviewSelection(change, reviewSelection)
	if len(skills) == 0 {
		return "", evidenceState
	}
	return skills[0], evidenceState
}

func ReviewSkillSelectionFromControls(activeControls []model.ControlActivation) skill.ReviewSkillSelection {
	for _, ctrl := range activeControls {
		if ctrl.ControlID == model.ControlSecurityReview && ctrl.Active {
			return skill.ReviewSkillSelection{SecurityReviewSelected: true}
		}
	}
	return skill.ReviewSkillSelection{}
}

// resolveS0Intake dispatches within S0_INTAKE based on IntakeSubStep.
func resolveS0Intake(change model.Change) ([]string, string) {
	switch change.IntakeSubStep {
	case model.IntakeSubStepClarify:
		return []string{SkillIntakeClarification}, string(model.StateS0Intake)
	case model.IntakeSubStepResearch:
		// Machine-only step: advances discovery-scoped intake into S1_PLAN/research.
		return nil, ""
	case model.IntakeSubStepConfirm:
		// Machine-only step: confirms approved summary presence.
		return nil, ""
	default:
		return nil, ""
	}
}

// resolveS1Plan dispatches within the S1_PLAN state based on PlanSubStep.
func resolveS1Plan(change model.Change) ([]string, string) {
	switch change.PlanSubStep {
	case model.PlanSubStepResearch:
		return []string{SkillResearchOrchestration}, string(model.StateS1Plan)
	case model.PlanSubStepBundle:
		// Machine-only step: no skill needed.
		return nil, ""
	case model.PlanSubStepAudit:
		return []string{SkillPlanAudit}, string(model.StateS1Plan)
	case model.PlanSubStepValidate:
		// Machine-only step: no skill needed.
		return nil, ""
	default:
		return nil, ""
	}
}

// resolveS2Implement returns the execution skill. Discovery changes without a
// bound worktree must complete worktree-preflight first.
func resolveS2Implement(change model.Change) ([]string, string) {
	if change.NeedsDiscovery && change.WorktreePath == "" {
		return []string{SkillWorktreePreflight}, string(model.StateS2Implement)
	}
	return []string{SkillWaveOrchestration}, string(model.StateS2Implement)
}
