package progression

import (
	"slices"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCloseoutAssuranceAttestationBlockers covers Layer 1 of issue #47: under
// standard/strict the passing final-closeout record must carry the
// assurance-complete attestation, and its absence is a fail-closed blocker.
func TestCloseoutAssuranceAttestationBlockers(t *testing.T) {
	t.Parallel()

	// Standard/strict, passing closeout record but attestation missing -> blocker.
	missing := map[string]model.VerificationRecord{
		SkillFinalCloseout: {
			Verdict:    model.VerificationVerdictPass,
			References: []string{"closeout:test_suite=pass:5/5"},
		},
	}
	blockers := closeoutAssuranceAttestationBlockers(missing, true)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_assurance_attestation_missing", blockers[0].Code)

	// Attestation present -> no blocker.
	present := map[string]model.VerificationRecord{
		SkillFinalCloseout: {
			Verdict:    model.VerificationVerdictPass,
			References: []string{"closeout:test_suite=pass:5/5", "closeout:assurance_complete=pass"},
		},
	}
	assert.Empty(t, closeoutAssuranceAttestationBlockers(present, true))

	// Assurance optional (assuranceRequired=false, i.e. light preset) never
	// enforces the attestation.
	assert.Empty(t, closeoutAssuranceAttestationBlockers(missing, false))

	// No final-closeout record at all -> same fail-closed blocker. Plain
	// standard does not require final-closeout through ComputeVerificationReadiness,
	// so this Layer 1 check owns the missing-record path.
	blockers = closeoutAssuranceAttestationBlockers(map[string]model.VerificationRecord{}, true)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_assurance_attestation_missing", blockers[0].Code)

	// Surrounding whitespace on the reference is tolerated.
	padded := map[string]model.VerificationRecord{
		SkillFinalCloseout: {
			Verdict:    model.VerificationVerdictPass,
			References: []string{"  closeout:assurance_complete=pass  "},
		},
	}
	assert.Empty(t, closeoutAssuranceAttestationBlockers(padded, true))
}

// TestBuildShipAuthorityAttestationPresetGating guards the two ship-authority
// contract bugs in the Layer 1 wiring:
//  1. The attestation must be gated on the effective preset (required on every
//     standard/strict preset), NOT on CloseoutRefreshRequired — which also trips for
//     light + quality_mode=full (false positive) and is false for a plain
//     standard change (false negative).
//  2. When the attestation is missing, the specific, actionable
//     closeout_assurance_attestation_missing code must surface in the G_ship
//     reasons, not collapse into the generic verification_evidence_missing.
func TestBuildShipAuthorityAttestationPresetGating(t *testing.T) {
	t.Parallel()

	const attestationMissing = "closeout_assurance_attestation_missing"
	hasCode := func(codes []model.ReasonCode) bool {
		return slices.ContainsFunc(codes, func(c model.ReasonCode) bool {
			return c.Code == attestationMissing
		})
	}
	passingGoalVerificationOnly := func() map[string]model.VerificationRecord {
		return map[string]model.VerificationRecord{
			SkillGoalVerification: {
				Verdict: model.VerificationVerdictPass,
			},
		}
	}
	// Passing goal-verification plus a passing final-closeout record that omits
	// the assurance attestation.
	passingGoalAndCloseoutNoAttestation := func() map[string]model.VerificationRecord {
		passing := passingGoalVerificationOnly()
		passing[SkillFinalCloseout] = model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			References: []string{"closeout:test_suite=pass:5/5"},
		}
		return passing
	}

	// Plain standard preset (no quality_mode=full, so CloseoutRefreshRequired is
	// false). Final-closeout is still required for standard ship evidence, and
	// the missing record must produce the Layer 1 blocker rather than only a
	// generic verification failure.
	t.Run("standard requires the attestation even without a closeout record", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		change := model.NewChange("ship-standard-missing-closeout")
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS4Verify
		require.NoError(t, state.SaveChange(root, change))

		policy, err := governance.ResolvePresetPolicy(root, change)
		require.NoError(t, err)
		require.Equal(t, model.WorkflowPresetStandard, policy.EffectivePreset)
		require.False(t, policy.CloseoutRefreshRequired,
			"plain standard must NOT set CloseoutRefreshRequired — standard assurance is a separate final-closeout requirement")

		ship, err := buildShipAuthorityFromReadiness(root, change, GovernanceReadiness{
			ArtifactReadiness: ArtifactReadiness{Ready: true},
			PassingSkills:     passingGoalVerificationOnly(),
			ReviewSurface:     &ReviewAuthority{},
		})
		require.NoError(t, err)
		assert.True(t, hasCode(ship.VerifySkillBlockers),
			"standard missing final-closeout must block as a missing assurance attestation")
		assert.True(t, hasCode(ship.Result.ReasonCodes),
			"the actionable blocker must surface in the G_ship reasons")
	})

	// Plain standard preset (no quality_mode=full, so CloseoutRefreshRequired is
	// false). Assurance is still required on every standard/strict preset, so the
	// attestation is required and the specific blocker must reach Result.ReasonCodes.
	t.Run("standard requires and surfaces the attestation blocker", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		change := model.NewChange("ship-standard-missing-attestation")
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS4Verify
		require.NoError(t, state.SaveChange(root, change))

		policy, err := governance.ResolvePresetPolicy(root, change)
		require.NoError(t, err)
		require.Equal(t, model.WorkflowPresetStandard, policy.EffectivePreset)
		require.False(t, policy.CloseoutRefreshRequired,
			"plain standard must NOT set CloseoutRefreshRequired — the old gate would have skipped enforcement here")

		ship, err := buildShipAuthorityFromReadiness(root, change, GovernanceReadiness{
			ArtifactReadiness: ArtifactReadiness{Ready: true},
			PassingSkills:     passingGoalAndCloseoutNoAttestation(),
			ReviewSurface:     &ReviewAuthority{},
		})
		require.NoError(t, err)
		assert.True(t, hasCode(ship.VerifySkillBlockers),
			"standard closeout missing the attestation must block verification")
		assert.True(t, hasCode(ship.Result.ReasonCodes),
			"the actionable blocker must surface in the G_ship reasons, not only as a side field")
	})

	// Light preset under quality_mode=full sets CloseoutRefreshRequired=true, but
	// assurance.md stays optional for light, so the attestation must NOT be
	// required — gating is on the effective preset, not closeout refresh.
	t.Run("light + quality_mode=full does not require the attestation", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		change := model.NewChange("ship-light-full-no-attestation")
		change.WorkflowPreset = model.WorkflowPresetLight
		change.QualityMode = model.QualityModeFull
		change.CurrentState = model.StateS4Verify
		require.NoError(t, state.SaveChange(root, change))

		policy, err := governance.ResolvePresetPolicy(root, change)
		require.NoError(t, err)
		require.Equal(t, model.WorkflowPresetLight, policy.EffectivePreset)
		require.True(t, policy.CloseoutRefreshRequired,
			"light + full must set CloseoutRefreshRequired — the exact case the old gate mis-blocked")

		ship, err := buildShipAuthorityFromReadiness(root, change, GovernanceReadiness{
			ArtifactReadiness: ArtifactReadiness{Ready: true},
			PassingSkills:     passingGoalAndCloseoutNoAttestation(),
			ReviewSurface:     &ReviewAuthority{},
		})
		require.NoError(t, err)
		assert.False(t, hasCode(ship.VerifySkillBlockers),
			"light keeps assurance optional; no attestation blocker")
		assert.False(t, hasCode(ship.Result.ReasonCodes),
			"light keeps assurance optional; no attestation reason in G_ship")
	})
}
