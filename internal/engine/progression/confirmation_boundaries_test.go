package progression

import "testing"

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
		SkillIntakeClarification,
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

	mustHardStop := []string{
		SkillWorktreePreflight,
		SkillSecurityReview,
	}
	for _, name := range mustHardStop {
		if SkillIsPurePacingAutoSafe(name) {
			t.Errorf("skill %q must NOT be pure-pacing auto-safe", name)
		}
	}
}
