package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runNextDiagnostics drives `next --json --diagnostics` for the given change and
// returns the decoded view. --diagnostics selects the full view that carries
// agent constraints (allowed_operations / required_outputs).
func runNextDiagnostics(t *testing.T, root, slug string) nextView {
	t.Helper()
	cmd := commandForRoot(t, root, makeNextCmd())
	cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())

	var view nextView
	require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
	return view
}

// TestNextBundleHandoffDoesNotAdvertiseEvidenceBeforeAudit asserts that at
// S1_PLAN/bundle the plan-audit handoff does NOT advertise write_evidence /
// evidence_record. `slipway evidence skill --skill plan-audit` fails closed with
// evidence_skill_wrong_plan_substep until the substep advances to audit, so
// advertising evidence recording at bundle is a dead-end handoff. The
// authoritative next action is to run the lifecycle into S1_PLAN/audit.
//
// issue #229
func TestNextBundleHandoffDoesNotAdvertiseEvidenceBeforeAudit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelDiscovery, "plan-audit bundle handoff")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.PlanSubStep = model.PlanSubStepBundle
		require.NoError(t, state.SaveChange(root, change))

		view := runNextDiagnostics(t, root, slug)

		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillPlanAudit, view.NextSkill.Name)

		require.NotNil(t, view.Constraints)
		assert.NotContains(t, view.Constraints.AllowedOperations, "write_evidence",
			"bundle handoff must not advertise write_evidence before the substep advances to audit")
		assert.NotContains(t, view.Constraints.RequiredOutputs, "evidence_record",
			"bundle handoff must not require an evidence_record the evidence command rejects at bundle")

		warnings := strings.Join(view.Warnings, "\n")
		assert.Contains(t, warnings, "S1_PLAN/bundle")
		assert.Contains(t, warnings, "slipway run", "warning must point to running into S1_PLAN/audit")
		assert.Contains(t, warnings, "S1_PLAN/audit")
		assert.NotContains(t, warnings, "write plan-audit evidence",
			"warning must not offer recording evidence as a direct action at bundle")
	})
}

// TestNextS1PlanAuditSurfacesSubagentDelegationAcrossCapabilityStates pins the
// host subagent-delegation contract for the S1 plan-audit handoff (#339).
// plan-audit REQUIRES dispatching an independent auditor subagent, but it is not
// a catalog-registered skill, so the contract comes from the built-in
// subagent-dispatch lever. "unknown" stays continuable on the skill_handoff
// boundary while riding a subagent_dispatch_authorization_required prerequisite
// with an enriched, named-fallback next_action; "unavailable" fails closed as a
// first-class host_capability_unavailable blocker; an explicit fallback clears it.
func TestNextS1PlanAuditSurfacesSubagentDelegationAcrossCapabilityStates(t *testing.T) {
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelDiscovery, "plan-audit subagent delegation")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.PlanSubStep = model.PlanSubStepBundle
		require.NoError(t, state.SaveChange(root, change))

		t.Setenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS", "")

		// unknown: host declared nothing -> continuable skill_handoff with prerequisite.
		t.Setenv("SLIPWAY_HOST_CAPABILITIES", "")
		unknown := runNextDiagnostics(t, root, slug)
		require.NotNil(t, unknown.NextSkill)
		assert.Equal(t, progression.SkillPlanAudit, unknown.NextSkill.Name)
		unknownCap := requireHostCapabilityForSkill(t, unknown.HostCapabilities, progression.SkillPlanAudit)
		assert.Equal(t, "unknown", unknownCap.Availability)
		assert.False(t, unknownCap.FallbackSelected)
		unknownSpecs := model.ReasonSpecs(unknown.Blockers)
		assert.Contains(t, unknownSpecs, "subagent_dispatch_authorization_required:plan-audit:subagent")
		assert.NotContains(t, unknownSpecs, "host_capability_unavailable:plan-audit:subagent")
		assert.Equal(t, "skill_handoff:plan-audit", unknown.ConfirmationRequirement.Reason)
		assert.Contains(t, unknown.ConfirmationRequirement.NextAction, "Host subagent delegation is a prerequisite")
		assert.Contains(t, unknown.ConfirmationRequirement.NextAction, "same_context_degraded")

		// unavailable: host declared other capabilities but not subagent.
		t.Setenv("SLIPWAY_HOST_CAPABILITIES", "none")
		unavailable := runNextDiagnostics(t, root, slug)
		unavailableCap := requireHostCapabilityForSkill(t, unavailable.HostCapabilities, progression.SkillPlanAudit)
		assert.Equal(t, "unavailable", unavailableCap.Availability)
		unavailableSpecs := model.ReasonSpecs(unavailable.Blockers)
		assert.Contains(t, unavailableSpecs, "host_capability_unavailable:plan-audit:subagent")
		assert.NotContains(t, unavailableSpecs, "subagent_dispatch_authorization_required:plan-audit:subagent")
		assert.Equal(t, "blocked_by_governance", unavailable.ConfirmationRequirement.Reason)

		// unavailable + named fallback clears the blocker and restores the handoff.
		t.Setenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS", "manual_plan_audit")
		fallback := runNextDiagnostics(t, root, slug)
		fallbackCap := requireHostCapabilityForSkill(t, fallback.HostCapabilities, progression.SkillPlanAudit)
		assert.True(t, fallbackCap.FallbackSelected)
		assert.Equal(t, "manual_plan_audit", fallbackCap.FallbackMode)
		fallbackSpecs := model.ReasonSpecs(fallback.Blockers)
		assert.NotContains(t, fallbackSpecs, "host_capability_unavailable:plan-audit:subagent")
		assert.NotContains(t, fallbackSpecs, "subagent_dispatch_authorization_required:plan-audit:subagent")
		assert.Equal(t, "skill_handoff:plan-audit", fallback.ConfirmationRequirement.Reason)
	})
}

func requireHostCapabilityForSkill(t *testing.T, capabilities []hostCapabilityView, skillName string) hostCapabilityView {
	t.Helper()
	for _, capability := range capabilities {
		if capability.SkillName == skillName {
			return capability
		}
	}
	t.Fatalf("host capability for %q not found in %+v", skillName, capabilities)
	return hostCapabilityView{}
}

// TestNextAuditHandoffStillAdvertisesEvidence asserts the audit-substep behavior
// is unchanged: at S1_PLAN/audit, where `slipway evidence skill --skill
// plan-audit` is accepted, the handoff still advertises write_evidence and
// requires the evidence_record output.
//
// issue #229
func TestNextAuditHandoffStillAdvertisesEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelDiscovery, "plan-audit audit handoff")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		view := runNextDiagnostics(t, root, slug)

		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillPlanAudit, view.NextSkill.Name)

		require.NotNil(t, view.Constraints)
		assert.Contains(t, view.Constraints.AllowedOperations, "write_evidence",
			"audit handoff must still advertise write_evidence where evidence recording is accepted")
		assert.Contains(t, view.Constraints.RequiredOutputs, "evidence_record",
			"audit handoff must still require the evidence_record output")
	})
}
