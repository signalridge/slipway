package progression

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
)

// TestSubagentDispatchAuthorizationBlockerRides pins that the
// subagent_dispatch_authorization_required blocker (emitted for the "unknown"
// host-capability state of a subagent-dispatch skill) rides alongside both the
// host-handoff confirmation (plan-audit / intake-side skill_handoff) and the S3
// review-companion confirmation, so it stays continuable and does not escalate
// to a separate blocked_by_governance hard stop. The "unavailable" first-class
// blocker host_capability_unavailable must NOT ride. (#339 / #369)
func TestSubagentDispatchAuthorizationBlockerRides(t *testing.T) {
	t.Parallel()

	riding := model.NewReasonCode("subagent_dispatch_authorization_required", "plan-audit:subagent")
	if !HostHandoffBlockerCanRide(riding) {
		t.Fatal("subagent_dispatch_authorization_required must ride the host handoff (plan-audit / intake-side skill_handoff)")
	}
	if !ReviewCompanionBlockerCanRide(riding) {
		t.Fatal("subagent_dispatch_authorization_required must ride the S3 review-companion handoff")
	}

	unavailable := model.NewReasonCode("host_capability_unavailable", "independent-review:subagent")
	if HostHandoffBlockerCanRide(unavailable) {
		t.Fatal("host_capability_unavailable (unavailable) must stay a first-class blocker, not ride the host handoff")
	}
	if ReviewCompanionBlockerCanRide(unavailable) {
		t.Fatal("host_capability_unavailable (unavailable) must stay a first-class blocker, not ride the review companion")
	}
}

// TestSecurityReviewDivergesAcrossAutoBoundaries pins the deliberate divergence
// between ReviewCompanionSkillCanCarryBlockers (lists security-review) and
// SkillIsPurePacingAutoSafe (must NOT list it): a security review may carry
// review/closeout companion blockers, but it must always hard-stop under
// execution.auto. A future edit that adds a review skill to one list must not
// silently add security-review to the auto-safe list.
func TestSecurityReviewDivergesAcrossAutoBoundaries(t *testing.T) {
	t.Parallel()

	if !ReviewCompanionSkillCanCarryBlockers(SkillSecurityReview) {
		t.Fatal("security-review must be allowed to carry review/closeout companion blockers")
	}
	if SkillIsPurePacingAutoSafe(SkillSecurityReview) {
		t.Fatal("security-review must never be pure-pacing auto-safe; it must hard-stop under execution.auto")
	}
	if !SkillRequiresManualAutoBoundary(SkillSecurityReview) {
		t.Fatal("security-review must require a manual auto boundary")
	}
}

// TestSkillRequiresManualAutoBoundaryFailsClosed pins the fail-closed contract:
// an unknown non-empty skill requires a manual auto boundary, while a blank or
// whitespace-only name reports false by design (there is no handoff to gate).
func TestSkillRequiresManualAutoBoundaryFailsClosed(t *testing.T) {
	t.Parallel()

	if !SkillRequiresManualAutoBoundary("totally-unknown-future-skill") {
		t.Fatal("unknown non-empty skill must fail closed to a manual auto boundary")
	}
	if SkillRequiresManualAutoBoundary("") {
		t.Fatal("blank skill name must report false (no handoff to gate), not fail closed")
	}
	if SkillRequiresManualAutoBoundary("   ") {
		t.Fatal("whitespace-only skill name must be treated as blank")
	}
}

// TestPurePacingAutoSafeAllowlistMembership pins the current pure-pacing
// allowlist so a silent add or removal is caught. worktree-preflight and
// security-review are intentionally excluded and must keep hard-stopping.
func TestPurePacingAutoSafeAllowlistMembership(t *testing.T) {
	t.Parallel()

	autoSafe := []string{
		SkillResearchOrchestration,
		SkillPlanAudit,
		SkillWaveOrchestration,
		SkillSpecComplianceReview,
		SkillCodeQualityReview,
		SkillIndependentReview,
		SkillShipVerification,
	}
	for _, name := range autoSafe {
		if !SkillIsPurePacingAutoSafe(name) {
			t.Errorf("skill %q must be pure-pacing auto-safe", name)
		}
	}

	// intake-clarification (the fresh approved-summary hard gate, #357) and
	// security-review must hard-stop even under execution.auto, so neither may be
	// pure-pacing auto-safe.
	mustHardStop := []string{
		SkillWorktreePreflight,
		SkillSecurityReview,
		SkillIntakeClarification,
	}
	for _, name := range mustHardStop {
		if SkillIsPurePacingAutoSafe(name) {
			t.Errorf("skill %q must NOT be pure-pacing auto-safe", name)
		}
	}
}
