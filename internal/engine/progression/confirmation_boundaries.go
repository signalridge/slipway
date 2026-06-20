package progression

import (
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

// IsRequiredSkillBlockerCode reports whether a reason code represents missing,
// stale, failing, or otherwise not-ready required skill evidence.
func IsRequiredSkillBlockerCode(code string) bool {
	switch strings.TrimSpace(code) {
	case "required_skill_missing",
		"required_skill_not_ready",
		"required_skill_not_passed",
		"required_skill_blockers_present",
		"required_skill_stale":
		return true
	default:
		return false
	}
}

// HostHandoffBlockerCanRide reports blockers that are the reason for the host
// handoff itself rather than a separate governance stop.
func HostHandoffBlockerCanRide(reason model.ReasonCode) bool {
	code := strings.TrimSpace(reason.Code)
	return IsRequiredSkillBlockerCode(code) || code == "review_alignment_required"
}

// ReviewCompanionBlockerCanRide reports governance blockers that are satisfied
// by running the selected review or closeout handoff that carries them.
func ReviewCompanionBlockerCanRide(reason model.ReasonCode) bool {
	switch strings.TrimSpace(reason.Code) {
	case "governance_action_required",
		"closeout_assurance_attestation_missing",
		"closeout_reviewer_independence_missing",
		"context_origin_handle_invalid",
		"high_risk_check_missing",
		"verification_evidence_missing":
		return true
	default:
		return false
	}
}

// ReviewCompanionSkillCanCarryBlockers reports skills whose handoff may carry
// review/closeout companion blockers without turning them into a separate stop.
//
// SkillSecurityReview is intentionally listed here but is deliberately ABSENT
// from SkillIsPurePacingAutoSafe: a security review may carry companion blockers
// on its handoff, yet it must always hard-stop under execution.auto. Keep that
// divergence intact when editing either list; it is pinned by
// TestSecurityReviewDivergesAcrossAutoBoundaries.
func ReviewCompanionSkillCanCarryBlockers(skillName string) bool {
	switch strings.TrimSpace(skillName) {
	case SkillSpecComplianceReview,
		SkillCodeQualityReview,
		SkillIndependentReview,
		SkillGoalVerification,
		SkillSecurityReview,
		SkillFinalCloseout:
		return true
	default:
		return false
	}
}

// SkillIsPurePacingAutoSafe reports skills whose handoff is only a pacing
// boundary and may be softened by execution.auto for non-guardrail changes.
//
// SkillSecurityReview is deliberately omitted: even outside a guardrail domain a
// security review must hard-stop under auto, so it must never appear here even
// though ReviewCompanionSkillCanCarryBlockers does list it. This divergence is
// pinned by TestSecurityReviewDivergesAcrossAutoBoundaries.
func SkillIsPurePacingAutoSafe(skillName string) bool {
	switch strings.TrimSpace(skillName) {
	case SkillIntakeClarification,
		SkillResearchOrchestration,
		SkillPlanAudit,
		SkillWaveOrchestration,
		SkillSpecComplianceReview,
		SkillCodeQualityReview,
		SkillIndependentReview,
		SkillGoalVerification,
		SkillFinalCloseout:
		return true
	default:
		return false
	}
}

// SkillRequiresManualAutoBoundary reports skills that must not be softened by
// execution.auto even when the change has no guardrail domain. Unknown non-empty
// skill names fail closed until explicitly classified as pure-pacing auto-safe.
//
// A blank name reports false by design, not as a fail-open gap: an empty skill
// name means there is no skill handoff to gate, so there is nothing for auto to
// soften. ReviewBatch skill names and NextSkill.Name are always populated, so
// they never reach the blank case. NextSkill.BlockingName may be blank, but when
// it is, nextSkillRequiresManualAutoBoundary still also checks the always-populated
// Name, so a blank name never opens a softening path.
func SkillRequiresManualAutoBoundary(skillName string) bool {
	name := strings.TrimSpace(skillName)
	return name != "" && !SkillIsPurePacingAutoSafe(name)
}
