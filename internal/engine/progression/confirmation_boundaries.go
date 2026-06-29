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

// IsTerminalShipGateRequiredSkillBlocker reports a generic required-skill blocker
// whose subject is the terminal ship-verification gate. Per REQ-001
// ship-verification is NOT a selected review peer: its missing/stale/blocked
// state is owned by `slipway done` / EvaluateShipAuthority, not by review
// convergence. The review verdict uses this to drop the redundant requiredness.
func IsTerminalShipGateRequiredSkillBlocker(reason model.ReasonCode) bool {
	return IsRequiredSkillBlockerCode(reason.Code) &&
		model.ParseBlocker(reason).Subject == SkillShipVerification
}

// DropTerminalShipGateRequiredSkillBlockers returns blockers without the terminal
// ship-verification gate's generic required-skill entries. `slipway review` uses
// it so review convergence does not fail on the terminal gate, which
// `slipway done` / EvaluateShipAuthority still owns fail-closed. See REQ-001.
func DropTerminalShipGateRequiredSkillBlockers(blockers []model.ReasonCode) []model.ReasonCode {
	if len(blockers) == 0 {
		return blockers
	}
	out := make([]model.ReasonCode, 0, len(blockers))
	for _, blocker := range blockers {
		if IsTerminalShipGateRequiredSkillBlocker(blocker) {
			continue
		}
		out = append(out, blocker)
	}
	return out
}

// IsShipVerificationHandoffBlockerCode reports G_ship gate blockers that a fresh
// ship-verification handoff resolves. `next` uses this so a malformed-but-passing
// ship record (verdict pass yet missing an attestation/ordering/high-risk token)
// still routes back to ship-verification: the generic required-skill check sees a
// passing record, but these gate blockers stay active until ship-verification is
// re-recorded.
func IsShipVerificationHandoffBlockerCode(code string) bool {
	switch strings.TrimSpace(code) {
	case "ship_verification_assurance_attestation_missing",
		"ship_verification_reviewer_independence_missing",
		"ship_verification_evidence_missing",
		"ship_verification_ordering_invalid",
		"high_risk_check_missing":
		return true
	default:
		return false
	}
}

// HostHandoffBlockerCanRide reports blockers that are the reason for the host
// handoff itself rather than a separate governance stop.
//
// subagent_dispatch_authorization_required rides here: it names the host
// subagent-delegation prerequisite for the handoff skill itself (plan-audit at
// S1, an intake-side handoff) when the host has not declared subagent capability
// available. It stays continuable so the handoff next_action can carry the
// prerequisite plus the named fallback, rather than escalating to a separate
// blocked_by_governance stop. The first-class host_capability_unavailable
// blocker (the host explicitly declared subagent unavailable) deliberately does
// NOT ride.
func HostHandoffBlockerCanRide(reason model.ReasonCode) bool {
	code := strings.TrimSpace(reason.Code)
	return IsRequiredSkillBlockerCode(code) ||
		code == "review_alignment_required" ||
		code == "subagent_dispatch_authorization_required"
}

// ReviewCompanionBlockerCanRide reports governance blockers that are satisfied
// by running the selected review or ship-verification handoff that carries them.
func ReviewCompanionBlockerCanRide(reason model.ReasonCode) bool {
	switch strings.TrimSpace(reason.Code) {
	case "governance_action_required",
		"ship_verification_assurance_attestation_missing",
		"ship_verification_reviewer_independence_missing",
		"ship_verification_evidence_missing",
		"ship_verification_ordering_invalid",
		"context_origin_handle_invalid",
		"subagent_dispatch_authorization_required",
		"high_risk_check_missing":
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
		SkillSecurityReview,
		SkillShipVerification:
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
		SkillShipVerification:
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
