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

// SkillRequiresManualAutoBoundary reports selected skills that must not be
// softened by execution.auto even when the change has no guardrail domain.
func SkillRequiresManualAutoBoundary(skillName string) bool {
	return strings.TrimSpace(skillName) == SkillSecurityReview
}
