package cmd

import (
	"testing"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeriveConfirmationRequirementAutoKeepsIntakeClarificationHardStop pins #357
// across execution.auto: the intake approved-summary is a FRESH HARD GATE BY
// DESIGN, so the intake-clarification handoff must NOT be softened by auto. A
// softened boundary would report PriorAuthorizationSufficient=true /
// FreshConfirmationRequired=false while the next_action prose still declares the
// gate fresh and non-delegable — a self-contradiction that lets a prior broad
// "continue" satisfy the intake gate. intake-clarification therefore diverges
// from the pure-pacing auto-safe set exactly like security-review.
func TestDeriveConfirmationRequirementAutoKeepsIntakeClarificationHardStop(t *testing.T) {
	t.Parallel()

	t.Run("skill_handoff to intake-clarification stays hard_stop under auto", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: "",
			NextSkill:       &nextSkillView{Name: progression.SkillIntakeClarification},
		}

		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient, "prior broad authorization must not satisfy the intake gate under auto")
		assert.True(t, req.FreshConfirmationRequired, "intake approved-summary stays a fresh hard gate under auto")
		assert.Equal(t, "skill_handoff:"+progression.SkillIntakeClarification, req.Reason)
		assert.Contains(t, req.NextAction, "FRESH HARD GATE BY DESIGN")
	})

	t.Run("intake-clarification blocking name stays hard_stop under auto", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: "",
			NextSkill: &nextSkillView{
				Name:         progression.SkillResearchOrchestration,
				BlockingName: progression.SkillIntakeClarification,
			},
		}

		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient)
		assert.True(t, req.FreshConfirmationRequired)
		assert.Equal(t, "skill_handoff:"+progression.SkillIntakeClarification, req.Reason)
	})
}

// TestValidateS3ShipVerificationSurfacesSubagentDelegationAcrossCapabilityStates
// pins #369 for the validate surface: once the selected S3 review peers have
// passing evidence but the terminal ship-verification gate still owes fresh
// evidence, `slipway validate` must evaluate the ship-verification
// host-capability contract too — exactly as next/run already do via
// nextS3ShipAuthoritySkill. Without this, validate/recovery dead-ends after the
// peers pass: it omits host_capabilities and never names the subagent-delegation
// prerequisite (unknown) or fails closed (unavailable) for the terminal gate.
func TestValidateS3ShipVerificationSurfacesSubagentDelegationAcrossCapabilityStates(t *testing.T) {
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "validate ship verification subagent delegation")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	writeShipReadyGovernedBundle(t, root, change)
	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writePassingReviewEvidencePack(t, root, slug, 1)

	const subagentBlocker = "subagent_dispatch_authorization_required:ship-verification:subagent"
	const unavailableBlocker = "host_capability_unavailable:ship-verification:subagent"

	t.Setenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS", "")

	// unknown: host declared nothing -> continuable prerequisite for the terminal gate.
	t.Setenv("SLIPWAY_HOST_CAPABILITIES", "")
	unknown, err := buildValidateViewForSlugWithReadContext(newStateReadContext(root), slug)
	require.NoError(t, err)
	unknownCap := requireHostCapabilityForSkill(t, unknown.HostCapabilities, progression.SkillShipVerification)
	assert.Equal(t, "unknown", unknownCap.Availability)
	assert.False(t, unknownCap.FallbackSelected)
	unknownSpecs := model.ReasonSpecs(unknown.Blockers)
	assert.Contains(t, unknownSpecs, subagentBlocker)
	assert.NotContains(t, unknownSpecs, unavailableBlocker)

	// unavailable: host declared other capabilities but not subagent -> fails closed.
	t.Setenv("SLIPWAY_HOST_CAPABILITIES", "none")
	unavailable, err := buildValidateViewForSlugWithReadContext(newStateReadContext(root), slug)
	require.NoError(t, err)
	unavailableCap := requireHostCapabilityForSkill(t, unavailable.HostCapabilities, progression.SkillShipVerification)
	assert.Equal(t, "unavailable", unavailableCap.Availability)
	unavailableSpecs := model.ReasonSpecs(unavailable.Blockers)
	assert.Contains(t, unavailableSpecs, unavailableBlocker)
	assert.NotContains(t, unavailableSpecs, subagentBlocker)

	// available: declared subagent -> no new blocker.
	t.Setenv("SLIPWAY_HOST_CAPABILITIES", "subagent")
	available, err := buildValidateViewForSlugWithReadContext(newStateReadContext(root), slug)
	require.NoError(t, err)
	availableCap := requireHostCapabilityForSkill(t, available.HostCapabilities, progression.SkillShipVerification)
	assert.Equal(t, "available", availableCap.Availability)
	availableSpecs := model.ReasonSpecs(available.Blockers)
	assert.NotContains(t, availableSpecs, unavailableBlocker)
	assert.NotContains(t, availableSpecs, subagentBlocker)
}
