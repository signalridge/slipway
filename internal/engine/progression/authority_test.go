package progression

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/governance"
	engineskill "github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShipAssuranceAttestationBlockers covers the re-homed assurance attestation:
// under standard/strict the passing ship-verification record must carry the
// assurance-complete attestation, and its absence is a fail-closed blocker.
func TestShipAssuranceAttestationBlockers(t *testing.T) {
	t.Parallel()

	// Standard/strict, passing ship record but attestation missing -> blocker.
	missing := map[string]model.VerificationRecord{
		SkillShipVerification: {
			Verdict:    model.VerificationVerdictPass,
			References: []string{"closeout:test_suite=pass:5/5"},
		},
	}
	blockers := shipAssuranceAttestationBlockers(missing, true)
	require.Len(t, blockers, 1)
	assert.Equal(t, "ship_verification_assurance_attestation_missing", blockers[0].Code)

	// Attestation present -> no blocker.
	present := map[string]model.VerificationRecord{
		SkillShipVerification: {
			Verdict:    model.VerificationVerdictPass,
			References: []string{"closeout:test_suite=pass:5/5", "closeout:assurance_complete=pass"},
		},
	}
	assert.Empty(t, shipAssuranceAttestationBlockers(present, true))

	// Assurance optional (assuranceRequired=false, i.e. light preset) never
	// enforces the attestation.
	assert.Empty(t, shipAssuranceAttestationBlockers(missing, false))

	// No ship-verification record at all -> same fail-closed blocker.
	blockers = shipAssuranceAttestationBlockers(map[string]model.VerificationRecord{}, true)
	require.Len(t, blockers, 1)
	assert.Equal(t, "ship_verification_assurance_attestation_missing", blockers[0].Code)

	// Surrounding whitespace on the reference is tolerated.
	padded := map[string]model.VerificationRecord{
		SkillShipVerification: {
			Verdict:    model.VerificationVerdictPass,
			References: []string{"  closeout:assurance_complete=pass  "},
		},
	}
	assert.Empty(t, shipAssuranceAttestationBlockers(padded, true))
}

// TestBuildShipAuthorityAttestationPresetGating guards the two ship-authority
// contract bugs in the attestation wiring:
//  1. The attestation must be gated on the effective preset (required on every
//     standard/strict preset), NOT on CloseoutRefreshRequired — which also trips for
//     light + quality_mode=full (false positive) and is false for a plain
//     standard change (false negative).
//  2. When the attestation is missing, the specific, actionable
//     ship_verification_assurance_attestation_missing code must surface in the
//     G_ship reasons, not collapse into the generic ship_verification_evidence_missing.
func TestBuildShipAuthorityAttestationPresetGating(t *testing.T) {
	t.Parallel()

	const attestationMissing = "ship_verification_assurance_attestation_missing"
	hasCode := func(codes []model.ReasonCode) bool {
		return slices.ContainsFunc(codes, func(c model.ReasonCode) bool {
			return c.Code == attestationMissing
		})
	}
	// A passing ship-verification record that omits the assurance attestation.
	passingShipNoAttestation := func() map[string]model.VerificationRecord {
		return map[string]model.VerificationRecord{
			SkillShipVerification: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{"closeout:test_suite=pass:5/5"},
			},
		}
	}

	// Plain standard preset (no quality_mode=full, so CloseoutRefreshRequired is
	// false). ship-verification is required for standard ship evidence, and a
	// missing record must produce the attestation blocker rather than only a
	// generic verification failure.
	t.Run("standard requires the attestation even without a ship record", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		change := model.NewChange("ship-standard-missing-ship-record")
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		policy, err := governance.ResolvePresetPolicy(root, change)
		require.NoError(t, err)
		require.Equal(t, model.WorkflowPresetStandard, policy.EffectivePreset)
		require.False(t, policy.CloseoutRefreshRequired,
			"plain standard must NOT set CloseoutRefreshRequired — standard assurance is a separate ship-verification requirement")

		ship, err := buildShipAuthorityFromReadiness(root, change, GovernanceReadiness{
			ArtifactReadiness: ArtifactReadiness{Ready: true},
			PassingSkills:     map[string]model.VerificationRecord{},
			ReviewSurface:     &ReviewAuthority{},
		})
		require.NoError(t, err)
		assert.True(t, hasCode(ship.VerifySkillBlockers),
			"standard missing ship-verification must block as a missing assurance attestation")
		assert.True(t, hasCode(ship.Result.ReasonCodes),
			"the actionable blocker must surface in the G_ship reasons")
	})

	// Plain standard preset: assurance is required on every standard/strict preset,
	// so a passing-but-unattested ship record blocks and the specific code must
	// reach Result.ReasonCodes.
	t.Run("standard requires and surfaces the attestation blocker", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		change := model.NewChange("ship-standard-missing-attestation")
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		policy, err := governance.ResolvePresetPolicy(root, change)
		require.NoError(t, err)
		require.Equal(t, model.WorkflowPresetStandard, policy.EffectivePreset)
		require.False(t, policy.CloseoutRefreshRequired,
			"plain standard must NOT set CloseoutRefreshRequired — the old gate would have skipped enforcement here")

		ship, err := buildShipAuthorityFromReadiness(root, change, GovernanceReadiness{
			ArtifactReadiness: ArtifactReadiness{Ready: true},
			PassingSkills:     passingShipNoAttestation(),
			ReviewSurface:     &ReviewAuthority{},
		})
		require.NoError(t, err)
		assert.True(t, hasCode(ship.VerifySkillBlockers),
			"standard ship record missing the attestation must block verification")
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
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		policy, err := governance.ResolvePresetPolicy(root, change)
		require.NoError(t, err)
		require.Equal(t, model.WorkflowPresetLight, policy.EffectivePreset)
		require.True(t, policy.CloseoutRefreshRequired,
			"light + full must set CloseoutRefreshRequired — the exact case the old gate mis-blocked")

		ship, err := buildShipAuthorityFromReadiness(root, change, GovernanceReadiness{
			ArtifactReadiness: ArtifactReadiness{Ready: true},
			PassingSkills:     passingShipNoAttestation(),
			ReviewSurface:     &ReviewAuthority{},
		})
		require.NoError(t, err)
		assert.False(t, hasCode(ship.VerifySkillBlockers),
			"light keeps assurance optional; no attestation blocker")
		assert.False(t, hasCode(ship.Result.ReasonCodes),
			"light keeps assurance optional; no attestation reason in G_ship")
	})
}

// TestShipReviewerIndependenceBlockers covers the re-homed independence presence
// facet: under standard/strict the passing ship-verification record must carry
// closeout:reviewer_independence=pass; absence fails closed, light is advisory.
func TestShipReviewerIndependenceBlockers(t *testing.T) {
	t.Parallel()

	missing := map[string]model.VerificationRecord{
		SkillShipVerification: {
			Verdict:    model.VerificationVerdictPass,
			References: []string{"closeout:assurance_complete=pass"},
		},
	}
	blockers := shipReviewerIndependenceBlockers(missing, true)
	require.Len(t, blockers, 1)
	assert.Equal(t, "ship_verification_reviewer_independence_missing", blockers[0].Code)

	present := map[string]model.VerificationRecord{
		SkillShipVerification: {
			Verdict:    model.VerificationVerdictPass,
			References: []string{"  closeout:reviewer_independence=pass  "},
		},
	}
	assert.Empty(t, shipReviewerIndependenceBlockers(present, true))

	// No ship-verification record at all -> same fail-closed blocker.
	blockers = shipReviewerIndependenceBlockers(map[string]model.VerificationRecord{}, true)
	require.Len(t, blockers, 1)
	assert.Equal(t, "ship_verification_reviewer_independence_missing", blockers[0].Code)

	// Light preset (required=false) is advisory: never blocks.
	assert.Empty(t, shipReviewerIndependenceBlockers(missing, false))
}

// TestShipReviewSetOrderingBlockers covers the single retained S3 ordering
// invariant: ship-verification must be stamped at or after every selected review
// peer (spec/code/independent/security). A peer stamped after ship-verification is
// a fail-closed ship_verification_ordering_invalid blocker, enforced on every
// preset (no light advisory carveout — the invariant is causal validity, not a
// quality attestation).
func TestShipReviewSetOrderingBlockers(t *testing.T) {
	t.Parallel()

	selectedReviewers := engineskill.SelectedReviewSkills(engineskill.ReviewSkillSelection{})
	selectedReviewersWithSecurity := engineskill.SelectedReviewSkills(engineskill.ReviewSkillSelection{SecurityReviewSelected: true})
	shipAt := time.Now().UTC()
	shipPassing := map[string]model.VerificationRecord{
		SkillShipVerification: {
			Verdict:   model.VerificationVerdictPass,
			Timestamp: shipAt,
		},
	}

	// All selected reviewers before ship-verification -> no blocker.
	reviewsBeforeShip := closeoutReuseReviewRecords(1, shipAt.Add(-2*time.Second), shipAt.Add(-time.Second))
	assert.Empty(t, shipReviewSetOrderingBlockers(shipPassing, reviewsBeforeShip, selectedReviewers),
		"reviews stamped before ship-verification must pass")

	// Boundary: a selected reviewer stamped at the EXACT ship-verification time
	// must pass. The invariant is ship >= review (compared with After(), not a
	// strict >), so an equal stamp is in order. Pins the >= boundary against a
	// regression to a strict After()/Before() that would block equal timestamps.
	reviewsAtShip := closeoutReuseReviewRecords(1, shipAt, shipAt)
	assert.Empty(t, shipReviewSetOrderingBlockers(shipPassing, reviewsAtShip, selectedReviewers),
		"a reviewer stamped at the exact ship-verification time must pass (ship >= review)")

	// A selected reviewer stamped after ship-verification -> blocker.
	reviewsAfterShip := closeoutReuseReviewRecords(1, shipAt.Add(time.Second), shipAt.Add(2*time.Second))
	blockers := shipReviewSetOrderingBlockers(shipPassing, reviewsAfterShip, selectedReviewers)
	require.Len(t, blockers, 1)
	assert.Equal(t, "ship_verification_ordering_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, "selected reviewer evidence")

	// Every selected reviewer must be ordered before ship-verification, not only
	// the historical spec/code pair.
	independentAfterShip := closeoutReuseReviewRecords(1, shipAt.Add(-2*time.Second), shipAt.Add(-time.Second))
	independentAfterShip[SkillIndependentReview] = model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Timestamp: shipAt.Add(2 * time.Second),
	}
	blockers = shipReviewSetOrderingBlockers(shipPassing, independentAfterShip, selectedReviewers)
	require.Len(t, blockers, 1)
	assert.Equal(t, "ship_verification_ordering_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, SkillIndependentReview)

	// Unselected security evidence after ship is silent unless the control selected it.
	unselectedSecurityAfterShip := closeoutReuseReviewRecords(1, shipAt.Add(-2*time.Second), shipAt.Add(-time.Second))
	unselectedSecurityAfterShip[SkillSecurityReview] = model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Timestamp: shipAt.Add(2 * time.Second),
	}
	assert.Empty(t, shipReviewSetOrderingBlockers(shipPassing, unselectedSecurityAfterShip, selectedReviewers),
		"security-review evidence is silent when the security control did not select it")

	blockers = shipReviewSetOrderingBlockers(shipPassing, unselectedSecurityAfterShip, selectedReviewersWithSecurity)
	require.Len(t, blockers, 1)
	assert.Equal(t, "ship_verification_ordering_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, SkillSecurityReview)

	// Genuinely-absent ship record: nothing to compare, no blocker (owned elsewhere).
	assert.Empty(t, shipReviewSetOrderingBlockers(map[string]model.VerificationRecord{}, reviewsAfterShip, selectedReviewers))

	// Always-on: the ordering invariant has NO preset carveout. The same
	// out-of-order chain that blocks above must still block here — this is the
	// regression guard against re-introducing a light advisory bypass, which would
	// let a terminal ship verdict pass without having observed the final review
	// evidence.
	alwaysOnBlockers := shipReviewSetOrderingBlockers(shipPassing, reviewsAfterShip, selectedReviewers)
	require.Len(t, alwaysOnBlockers, 1)
	assert.Equal(t, "ship_verification_ordering_invalid", alwaysOnBlockers[0].Code)
}

// TestExtractShipVerificationHighRiskChecksScopesToShipRecord pins REQ-005's
// ownership: the guardrail SAST baseline that satisfies G_ship is read ONLY from
// the ship-verification record. A review peer carrying the same high-risk
// reference must NOT satisfy the gate, closing the fail-open path where any
// passing peer could vouch for the safety baseline.
func TestExtractShipVerificationHighRiskChecksScopesToShipRecord(t *testing.T) {
	t.Parallel()

	const baseline = "auth_authz.safety_baseline"
	ref := "high_risk_check:" + baseline + "=pass"

	// A review peer carrying the SAST token does not satisfy the ship-owned check.
	peerOnly := map[string]model.VerificationRecord{
		SkillIndependentReview: {Verdict: model.VerificationVerdictPass, References: []string{ref}},
	}
	assert.Empty(t, extractShipVerificationHighRiskChecks(peerOnly),
		"a review peer's high-risk reference must not satisfy the ship-owned guardrail check")

	// The same token on the ship-verification record is honored.
	shipScoped := map[string]model.VerificationRecord{
		SkillIndependentReview: {Verdict: model.VerificationVerdictPass, References: []string{ref}},
		SkillShipVerification:  {Verdict: model.VerificationVerdictPass, References: []string{ref}},
	}
	checks := extractShipVerificationHighRiskChecks(shipScoped)
	pass, ok := checks[baseline]
	assert.True(t, ok, "ship-verification's own high-risk reference must be extracted")
	assert.True(t, pass)

	// No ship record -> no checks (G_ship stays blocked with high_risk_check_missing).
	assert.Empty(t, extractShipVerificationHighRiskChecks(map[string]model.VerificationRecord{}))
}

// contextOriginRef builds a per-stage context-origin handle reference token.
func contextOriginRef(stage, handle string) string {
	return model.ContextOriginReferencePrefix + stage + "=" + handle
}

// reviewContextRecords returns spec + code passing records carrying the given
// per-stage context-origin handles.
func reviewContextRecords(specHandle, codeHandle string) map[string]model.VerificationRecord {
	records := map[string]model.VerificationRecord{}
	if specHandle != "" {
		records[SkillSpecComplianceReview] = model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			References: []string{contextOriginRef(model.StageContextReview, specHandle)},
		}
	}
	if codeHandle != "" {
		records[SkillCodeQualityReview] = model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			References: []string{contextOriginRef(model.StageContextReview, codeHandle)},
		}
	}
	return records
}

// reviewSkillContextRecords returns passing reviewer records carrying the
// selected-reviewer context-origin handle shape.
func reviewSkillContextRecords(handles map[string]string) map[string]model.VerificationRecord {
	records := make(map[string]model.VerificationRecord, len(handles))
	for skillName, handle := range handles {
		records[skillName] = model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			References: []string{contextOriginRef(model.StageContextReview, handle)},
		}
	}
	return records
}

// hasReasonCode reports whether codes contains the given reason code.
func hasReasonCode(codes []model.ReasonCode, code string) bool {
	return slices.ContainsFunc(codes, func(c model.ReasonCode) bool { return c.Code == code })
}

func TestCrossStageContextDistinctBlockersUsesSelectedReviewSkillParticipants(t *testing.T) {
	t.Parallel()

	newRoot := func(t *testing.T, slug string) (string, model.Change) {
		t.Helper()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		change := model.NewChange(slug)
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))
		return root, change
	}

	selectedReviewers := []string{SkillSpecComplianceReview, SkillCodeQualityReview, SkillIndependentReview}
	reviewStages := crossStageContextReviewStagesForSelectedSkills(selectedReviewers)
	ownedReview := crossStageContextOwnedReviewStagesForSelectedSkills(selectedReviewers)

	t.Run("same-handle selected reviewers collide by skill name", func(t *testing.T) {
		t.Parallel()
		root, change := newRoot(t, "lattice-selected-reviewer-collision")
		records := reviewSkillContextRecords(map[string]string{
			SkillSpecComplianceReview: "shared-reviewer",
			SkillCodeQualityReview:    "shared-reviewer",
			SkillIndependentReview:    "independent-reviewer",
		})

		blockers := crossStageContextDistinctBlockers(root, change, records, reviewStages, ownedReview, true)
		require.Len(t, blockers, 1)
		assert.Equal(t, "cross_stage_context_not_distinct", blockers[0].Code)
		assert.Equal(t, SkillCodeQualityReview+"|"+SkillSpecComplianceReview, blockers[0].Detail)
	})

	t.Run("missing selected review handle fails closed", func(t *testing.T) {
		t.Parallel()
		root, change := newRoot(t, "lattice-selected-reviewer-missing-handle")
		records := reviewSkillContextRecords(map[string]string{
			SkillSpecComplianceReview: "ctx-spec-reviewer",
			SkillCodeQualityReview:    "ctx-code-reviewer",
		})
		records[SkillIndependentReview] = model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			References: []string{"independent:review=pass"},
		}

		blockers := crossStageContextDistinctBlockers(root, change, records, reviewStages, ownedReview, true)
		require.Len(t, blockers, 1)
		assert.Equal(t, "context_origin_handle_invalid", blockers[0].Code)
		assert.Contains(t, blockers[0].Detail, SkillIndependentReview)
		assert.Contains(t, blockers[0].Detail, model.StageContextReview)
	})

	t.Run("unselected security review evidence is ignored", func(t *testing.T) {
		t.Parallel()
		root, change := newRoot(t, "lattice-unselected-security-review")
		records := reviewSkillContextRecords(map[string]string{
			SkillSpecComplianceReview: "ctx-spec-reviewer",
			SkillCodeQualityReview:    "ctx-code-reviewer",
			SkillIndependentReview:    "ctx-independent-reviewer",
			// Security is present in the available passing records, but it is not
			// in selectedReviewers. Its colliding handle must not create a lattice
			// endpoint unless the security-review control selected it.
			SkillSecurityReview: "ctx-spec-reviewer",
		})

		assert.Empty(t, crossStageContextDistinctBlockers(root, change, records, reviewStages, ownedReview, true))
	})
}

func TestSelectedReviewContextOriginInvalidTargetsOnlySameSelectedPassingSkill(t *testing.T) {
	t.Parallel()

	authority := ReviewAuthority{
		SelectedReviewSkills: []string{SkillSpecComplianceReview, SkillCodeQualityReview},
		PassingSkills: map[string]model.VerificationRecord{
			SkillSpecComplianceReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{"layer:R0=pass"},
			},
			SkillCodeQualityReview: {
				Verdict: model.VerificationVerdictPass,
				References: []string{
					contextOriginRef(model.StageContextReview, "ctx-code-reviewer"),
				},
			},
			SkillSecurityReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{"security-review:pass"},
			},
		},
		Blockers: []model.ReasonCode{
			selectedReviewContextOriginInvalidBlocker(SkillSpecComplianceReview),
			selectedReviewContextOriginInvalidBlocker(SkillSecurityReview),
		},
	}

	assert.True(t, selectedReviewContextOriginInvalid(authority, SkillSpecComplianceReview))
	assert.False(t, selectedReviewContextOriginInvalid(authority, SkillCodeQualityReview),
		"valid current selected-review evidence must not be replaceable")
	assert.False(t, selectedReviewContextOriginInvalid(authority, SkillSecurityReview),
		"unselected review evidence must not be replaceable through the selected-review repair path")

	noMatchingBlocker := authority
	noMatchingBlocker.Blockers = []model.ReasonCode{selectedReviewContextOriginInvalidBlocker(SkillCodeQualityReview)}
	assert.False(t, selectedReviewContextOriginInvalid(noMatchingBlocker, SkillSpecComplianceReview),
		"the invalid-context blocker must target the same selected skill")

	malformed := authority
	malformed.PassingSkills = map[string]model.VerificationRecord{
		SkillSpecComplianceReview: {
			Verdict: model.VerificationVerdictPass,
			References: []string{
				contextOriginRef(model.StageContextReview, "ctx-a"),
				contextOriginRef(model.StageContextReview, "ctx-b"),
			},
		},
	}
	malformed.Blockers = []model.ReasonCode{selectedReviewContextOriginInvalidBlocker(SkillSpecComplianceReview)}
	assert.True(t, selectedReviewContextOriginInvalid(malformed, SkillSpecComplianceReview),
		"malformed duplicate review handles are recoverable through the same narrow repair path")
}

func TestReviewAuthoritySelectedPassingSkillsIgnoreUnselectedSecurityEvidenceOnDisk(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
	require.NoError(t, os.WriteFile(filepath.Join(root, "tracked.go"), []byte("package main\n"), 0o644))

	change := model.NewChange("review-authority-unselected-security-disk")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.CurrentState = model.StateS3Review
	require.NoError(t, state.SaveChange(root, change))
	writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})
	summary.Tasks[0].ChangedFiles = []string{"tracked.go"}
	summary.SyncDerivedFields()
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	selectedRecords := reviewSkillContextRecords(map[string]string{
		SkillSpecComplianceReview: "ctx-spec-reviewer",
		SkillCodeQualityReview:    "ctx-code-reviewer",
		SkillIndependentReview:    "ctx-independent-reviewer",
	})
	for skillName, record := range selectedRecords {
		record.RunVersion = 1
		record.Timestamp = time.Date(2026, 6, 17, 8, 0, 0, 0, time.UTC)
		writeVerificationForTest(t, root, change.Slug, skillName, record)
		require.NoError(t, StampEvidenceDigestForSkill(root, change, skillName, record, summary))
		selectedRecords[skillName] = record
	}

	// A security-review file exists on disk and even collides with spec, but the
	// security-review control is not selected, so it must not become a passing
	// skill or lattice participant.
	writeVerificationForTest(t, root, change.Slug, SkillSecurityReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		RunVersion: 1,
		Timestamp:  time.Date(2026, 6, 17, 8, 0, 0, 0, time.UTC),
		References: []string{contextOriginRef(model.StageContextReview, "ctx-spec-reviewer")},
	})

	// ship-verification is the always-required terminal S3 skill; record a passing
	// one so the required-skill set is satisfied and the test isolates reviewer
	// selection behavior.
	shipRecord := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		RunVersion: 1,
		Timestamp:  time.Date(2026, 6, 17, 8, 0, 0, 0, time.UTC),
		References: []string{shipAssuranceCompleteReference, shipReviewerIndependenceReference},
	}
	writeVerificationForTest(t, root, change.Slug, SkillShipVerification, shipRecord)
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillShipVerification, shipRecord, summary))

	passingSkills, skillBlockers, err := EvaluateRequiredSkillsForChangeWithReviewSelection(
		root,
		change,
		model.StateS3Review,
		1,
		false,
		engineskill.ReviewSkillSelection{},
	)
	require.NoError(t, err)
	require.Empty(t, skillBlockers)
	assert.Contains(t, passingSkills, SkillSpecComplianceReview)
	assert.Contains(t, passingSkills, SkillCodeQualityReview)
	assert.Contains(t, passingSkills, SkillIndependentReview)
	assert.Contains(t, passingSkills, SkillShipVerification)
	assert.NotContains(t, passingSkills, SkillSecurityReview)

	selectedReviewers := []string{SkillSpecComplianceReview, SkillCodeQualityReview, SkillIndependentReview}
	assert.Empty(t, crossStageContextDistinctBlockers(
		root,
		change,
		passingSkills,
		crossStageContextReviewStagesForSelectedSkills(selectedReviewers),
		crossStageContextOwnedReviewStagesForSelectedSkills(selectedReviewers),
		true,
	))
}

func TestReviewAuthorityDocsProfileIgnoresUnselectedCodeQualityEvidenceOnDisk(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
	require.NoError(t, os.WriteFile(filepath.Join(root, "tracked.go"), []byte("package main\n"), 0o644))

	change := model.NewChange("review-authority-docs-unselected-code")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.WorkflowProfile = model.WorkflowProfileDocs
	change.CurrentState = model.StateS3Review
	require.NoError(t, state.SaveChange(root, change))
	writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})
	summary.Tasks[0].ChangedFiles = []string{"tracked.go"}
	summary.SyncDerivedFields()
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	selectedRecords := reviewSkillContextRecords(map[string]string{
		SkillSpecComplianceReview: "ctx-spec-reviewer",
		SkillIndependentReview:    "ctx-independent-reviewer",
	})
	for skillName, record := range selectedRecords {
		record.RunVersion = 1
		record.Timestamp = time.Date(2026, 6, 17, 8, 0, 0, 0, time.UTC)
		if skillName == SkillSpecComplianceReview {
			record.References = append(record.References, "layer:R0=pass")
		}
		writeVerificationForTest(t, root, change.Slug, skillName, record)
		require.NoError(t, StampEvidenceDigestForSkill(root, change, skillName, record, summary))
		selectedRecords[skillName] = record
	}

	writeVerificationForTest(t, root, change.Slug, SkillCodeQualityReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		RunVersion: 1,
		Timestamp:  time.Date(2026, 6, 17, 8, 0, 0, 0, time.UTC),
		References: []string{
			"layer:IR1=pass",
			contextOriginRef(model.StageContextReview, "ctx-stale-code-reviewer"),
		},
	})

	authority, err := EvaluateReviewAuthority(root, change)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		SkillSpecComplianceReview,
		SkillIndependentReview,
	}, authority.SelectedReviewSkills)
	assert.Contains(t, authority.PassingSkills, SkillSpecComplianceReview)
	assert.Contains(t, authority.PassingSkills, SkillIndependentReview)
	assert.NotContains(t, authority.PassingSkills, SkillCodeQualityReview)
	assert.NotContains(t, strings.Join(model.ReasonSpecs(authority.Blockers), "\n"), SkillCodeQualityReview)
}

// TestCrossStageContextDistinctBlockers covers the generalized P2 distinct-context
// lattice (REQ-002) at the review seam: pass-with-distinct, a colliding pair named
// in earlier|later detail, a single-stage handle equal to a member of the executor
// handle set, a present-passing record missing its handle (-> context_origin_handle_invalid),
// an absent record (silent), and advisory-on-light.
func TestCrossStageContextDistinctBlockers(t *testing.T) {
	t.Parallel()

	newRoot := func(t *testing.T, slug string) (string, model.Change) {
		t.Helper()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		change := model.NewChange(slug)
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))
		return root, change
	}
	selectedReviewers := []string{SkillSpecComplianceReview, SkillCodeQualityReview, SkillIndependentReview}
	reviewStages := crossStageContextReviewStagesForSelectedSkills(selectedReviewers)
	ownedReview := crossStageContextOwnedReviewStagesForSelectedSkills(selectedReviewers)

	t.Run("pass with distinct spec and code handles", func(t *testing.T) {
		t.Parallel()
		root, change := newRoot(t, "lattice-distinct")
		records := reviewContextRecords("handle-spec", "handle-code")
		assert.Empty(t, crossStageContextDistinctBlockers(root, change, records, reviewStages, ownedReview, true))
	})

	t.Run("collision names the earlier|later pair", func(t *testing.T) {
		t.Parallel()
		root, change := newRoot(t, "lattice-collision")
		records := reviewContextRecords("shared", "shared")
		blockers := crossStageContextDistinctBlockers(root, change, records, reviewStages, ownedReview, true)
		require.Len(t, blockers, 1)
		assert.Equal(t, "cross_stage_context_not_distinct", blockers[0].Code)
		// spec and code reviewers are keyed by selected skill name; lexical order
		// is code-quality-review < spec-compliance-review.
		assert.Equal(t, SkillCodeQualityReview+"|"+SkillSpecComplianceReview, blockers[0].Detail)
	})

	t.Run("single-stage handle inside the executor set collides", func(t *testing.T) {
		t.Parallel()
		root, change := newRoot(t, "lattice-executor-set")
		// Wave-orchestration record stamps an executor handle equal to the spec
		// handle, so the spec<->executor edge collides.
		writeVerificationForTest(t, root, change.Slug, SkillWaveOrchestration, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
			References: []string{
				model.WaveExecutorAgentReferencePrefix + "1:task=t-01:handle-spec",
			},
		})
		records := reviewContextRecords("handle-spec", "handle-code")
		blockers := crossStageContextDistinctBlockers(root, change, records, reviewStages, ownedReview, true)
		require.Len(t, blockers, 1)
		assert.Equal(t, "cross_stage_context_not_distinct", blockers[0].Code)
		// lexical order: executor < spec-compliance-review.
		assert.Equal(t, model.StageContextExecutor+"|"+SkillSpecComplianceReview, blockers[0].Detail)
	})

	t.Run("recorded fix handle collides with reviewer handle", func(t *testing.T) {
		t.Parallel()
		root, change := newRoot(t, "lattice-fix-context")
		records := reviewContextRecords("handle-spec", "handle-code")
		spec := records[SkillSpecComplianceReview]
		spec.References = append(spec.References, contextOriginRef(model.StageContextFix, "handle-spec"))
		records[SkillSpecComplianceReview] = spec

		blockers := crossStageContextDistinctBlockers(root, change, records, reviewStages, ownedReview, true)
		require.Len(t, blockers, 1)
		assert.Equal(t, "cross_stage_context_not_distinct", blockers[0].Code)
		assert.Equal(t, model.StageContextFix+"|"+SkillSpecComplianceReview, blockers[0].Detail)
	})

	t.Run("multiple distinct fix handles emit no reviewer-missing blocker and union into the fix set", func(t *testing.T) {
		t.Parallel()
		root, change := newRoot(t, "lattice-multi-fix-context")
		// A selected/passing reviewer carries one valid review handle PLUS two
		// distinct fix handles. The now-multi-valued fix stage must not poison the
		// record parse, so the reviewer's review handle still resolves (no
		// context_origin_handle_invalid reviewer-missing blocker), and every fix
		// handle must land in the fix participant set.
		records := reviewContextRecords("handle-spec", "handle-code")
		spec := records[SkillSpecComplianceReview]
		spec.References = append(spec.References,
			contextOriginRef(model.StageContextFix, "fix-handle-a"),
			contextOriginRef(model.StageContextFix, "fix-handle-b"),
		)
		records[SkillSpecComplianceReview] = spec

		// (a) No reviewer-missing blocker: the review handles are distinct and the
		// fix handles collide with nothing, so the lattice is clean.
		blockers := crossStageContextDistinctBlockers(root, change, records, reviewStages, ownedReview, true)
		assert.Empty(t, blockers)
		assert.False(t, hasReasonCode(blockers, "context_origin_handle_invalid"),
			"multi-fix records must not emit the false reviewer-missing blocker")

		// (b) The fix participant HandleSet must contain BOTH fix handles across the
		// selected reviewers.
		participants, invalid := crossStageContextParticipants(root, change, records, reviewStages)
		require.Empty(t, invalid)
		fixParticipant, ok := participants[model.StageContextFix]
		require.True(t, ok, "fix participant must be present when any reviewer records a fix handle")
		_, hasA := fixParticipant.HandleSet["fix-handle-a"]
		_, hasB := fixParticipant.HandleSet["fix-handle-b"]
		assert.True(t, hasA, "fix-handle-a must be unioned into the fix participant set")
		assert.True(t, hasB, "fix-handle-b must be unioned into the fix participant set")
		assert.Len(t, fixParticipant.HandleSet, 2, "exactly the two distinct fix handles are collected")
	})

	t.Run("fix handles union across multiple selected reviewers", func(t *testing.T) {
		t.Parallel()
		root, change := newRoot(t, "lattice-multi-reviewer-fix-context")
		// Distinct fix handles recorded on DIFFERENT selected reviewers must all
		// flow into one shared fix participant set, not just the first reviewer's.
		records := reviewContextRecords("handle-spec", "handle-code")
		spec := records[SkillSpecComplianceReview]
		spec.References = append(spec.References, contextOriginRef(model.StageContextFix, "fix-from-spec"))
		records[SkillSpecComplianceReview] = spec
		code := records[SkillCodeQualityReview]
		code.References = append(code.References, contextOriginRef(model.StageContextFix, "fix-from-code"))
		records[SkillCodeQualityReview] = code

		participants, invalid := crossStageContextParticipants(root, change, records, reviewStages)
		require.Empty(t, invalid)
		fixParticipant, ok := participants[model.StageContextFix]
		require.True(t, ok)
		_, hasSpecFix := fixParticipant.HandleSet["fix-from-spec"]
		_, hasCodeFix := fixParticipant.HandleSet["fix-from-code"]
		assert.True(t, hasSpecFix, "the spec reviewer's fix handle must be in the union")
		assert.True(t, hasCodeFix, "the code reviewer's fix handle must be in the union")
		assert.Len(t, fixParticipant.HandleSet, 2)
	})

	t.Run("present-passing record missing its handle fails closed", func(t *testing.T) {
		t.Parallel()
		root, change := newRoot(t, "lattice-missing-handle")
		records := reviewContextRecords("handle-spec", "")
		// code-quality-review present and passing but carries no context-origin handle.
		records[SkillCodeQualityReview] = model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			References: []string{"layer:IR1=pass"},
		}
		blockers := crossStageContextDistinctBlockers(root, change, records, reviewStages, ownedReview, true)
		require.Len(t, blockers, 1)
		assert.Equal(t, "context_origin_handle_invalid", blockers[0].Code)
		assert.Contains(t, blockers[0].Detail, SkillCodeQualityReview)
	})

	t.Run("absent records are silent", func(t *testing.T) {
		t.Parallel()
		root, change := newRoot(t, "lattice-absent")
		// Only spec present; code/executor/audit_origin absent -> no participant, no
		// blocker (absence owned by the required-skill-missing gate).
		records := reviewContextRecords("handle-spec", "")
		assert.Empty(t, crossStageContextDistinctBlockers(root, change, records, reviewStages, ownedReview, true))
	})

	t.Run("advisory on light", func(t *testing.T) {
		t.Parallel()
		root, change := newRoot(t, "lattice-light")
		records := reviewContextRecords("shared", "shared")
		assert.Empty(t, crossStageContextDistinctBlockers(root, change, records, reviewStages, ownedReview, false),
			"light preset is advisory; a collision must not block")
	})
}

// TestShipCrossStageContextNoDoubleFire proves the ship gate owns only the
// goal/closeout edges: a spec<->code collision (a review-owned edge) does NOT
// re-fire at ship, while a goal<->spec collision (a ship-owned edge) does. It
// also confirms the ship lattice does not double-fire with the executor_agent
// gate (this gate never emits executor_agent_missing) and that its blocker is
// dual-surfaced into both VerifySkillBlockers and Result.ReasonCodes.
func TestShipCrossStageContextNoDoubleFire(t *testing.T) {
	t.Parallel()

	selectedReviewers := []string{SkillSpecComplianceReview, SkillCodeQualityReview, SkillIndependentReview}
	selectedReviewersWithSecurity := engineskill.SelectedReviewSkills(engineskill.ReviewSkillSelection{SecurityReviewSelected: true})
	shipStages := crossStageContextReviewStagesForSelectedSkills(selectedReviewers)
	shipStagesWithSecurity := crossStageContextReviewStagesForSelectedSkills(selectedReviewersWithSecurity)

	t.Run("review-owned spec/code edge does not re-fire at ship", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		change := model.NewChange("ship-lattice-no-refire")
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		// spec and code share a handle (a review-owned collision). The ship gate
		// no longer adds goal/closeout participants, so the spec<->code edge must
		// NOT fire through the ship-owned (empty) owned-stage set here.
		merged := map[string]model.VerificationRecord{
			SkillSpecComplianceReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "shared-review")},
			},
			SkillCodeQualityReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "shared-review")},
			},
		}
		blockers := crossStageContextDistinctBlockers(root, change, merged, shipStages, map[string]struct{}{}, true)
		assert.Empty(t, blockers, "spec<->code is review-owned and must not re-fire at ship")
	})

	t.Run("review-owned selected security edge does not re-fire at ship", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		change := model.NewChange("ship-lattice-security-no-refire")
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		merged := map[string]model.VerificationRecord{
			SkillSpecComplianceReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "shared-security-review")},
			},
			SkillCodeQualityReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "handle-code")},
			},
			SkillSecurityReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "shared-security-review")},
			},
		}
		blockers := crossStageContextDistinctBlockers(root, change, merged, shipStagesWithSecurity, map[string]struct{}{}, true)
		assert.Empty(t, blockers, "selected security peer collisions are review-owned and must not re-fire at ship")
	})

	t.Run("ship gate never emits executor_agent_missing", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		change := model.NewChange("ship-lattice-executor")
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		merged := map[string]model.VerificationRecord{
			SkillSpecComplianceReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "handle-spec")},
			},
			SkillCodeQualityReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "handle-code")},
			},
		}
		blockers := crossStageContextDistinctBlockers(root, change, merged, shipStages, map[string]struct{}{}, true)
		assert.Empty(t, blockers)
		assert.False(t, hasReasonCode(blockers, "executor_agent_missing"),
			"the distinct-context lattice must never emit executor_agent_missing")
	})

	t.Run("selected security folded review edge stays silent at ship", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		// A passing review set with a security collision plus a passing
		// ship-verification record carrying both re-homed attestations. The ship
		// gate owns no context-origin lattice edges, so the security collision must
		// not surface at ship.
		reviewPassing := map[string]model.VerificationRecord{
			SkillSpecComplianceReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "handle-spec")},
			},
			SkillCodeQualityReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "handle-code")},
			},
			SkillIndependentReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "handle-independent")},
			},
			SkillSecurityReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "shared-spec-security")},
			},
		}
		verifyPassing := map[string]model.VerificationRecord{
			SkillShipVerification: passingShipVerificationRecord(time.Now().UTC()),
		}
		selectedChange := model.NewChange("ship-lattice-security-selected")
		selectedChange.WorkflowPreset = model.WorkflowPresetStandard
		selectedChange.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, selectedChange))
		selectedShip, err := buildShipAuthorityFromReadiness(root, selectedChange, GovernanceReadiness{
			ArtifactReadiness: ArtifactReadiness{Ready: true},
			PassingSkills:     verifyPassing,
			ReviewSurface: &ReviewAuthority{
				PassingSkills:        reviewPassing,
				SelectedReviewSkills: selectedReviewersWithSecurity,
			},
		})
		require.NoError(t, err)
		assert.False(t,
			hasReasonCode(selectedShip.VerifySkillBlockers, "cross_stage_context_not_distinct"),
			"selected security-review folded review edges must not re-fire at ship")
		assert.False(t,
			hasReasonCode(selectedShip.Result.ReasonCodes, "cross_stage_context_not_distinct"),
			"selected security-review folded review edges must not reach G_ship reasons")

		unselectedChange := model.NewChange("ship-lattice-security-unselected")
		unselectedChange.WorkflowPreset = model.WorkflowPresetStandard
		unselectedChange.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, unselectedChange))
		unselectedShip, err := buildShipAuthorityFromReadiness(root, unselectedChange, GovernanceReadiness{
			ArtifactReadiness: ArtifactReadiness{Ready: true},
			PassingSkills:     verifyPassing,
			ReviewSurface: &ReviewAuthority{
				PassingSkills:        reviewPassing,
				SelectedReviewSkills: selectedReviewers,
			},
		})
		require.NoError(t, err)
		assert.False(t,
			hasReasonCode(unselectedShip.VerifySkillBlockers, "cross_stage_context_not_distinct"),
			"unselected security-review evidence must not become a ship lattice participant")
	})

	t.Run("ship no longer owns context lattice and is advisory on light", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		change := model.NewChange("ship-lattice-dual-surface")
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		// Build a passing set where the review peers carry colliding handles, while
		// ship-verification carries the re-homed attestations. The merged ship gate
		// owns no context-origin lattice edges, so the collision stays silent here.
		reviewPassing := map[string]model.VerificationRecord{
			SkillSpecComplianceReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "shared-spec-code")},
			},
			SkillCodeQualityReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "shared-spec-code")},
			},
		}
		verifyPassing := map[string]model.VerificationRecord{
			SkillShipVerification: passingShipVerificationRecord(time.Now().UTC()),
		}

		ship, err := buildShipAuthorityFromReadiness(root, change, GovernanceReadiness{
			ArtifactReadiness: ArtifactReadiness{Ready: true},
			PassingSkills:     verifyPassing,
			ReviewSurface:     &ReviewAuthority{PassingSkills: reviewPassing},
		})
		require.NoError(t, err)
		assert.False(t, hasReasonCode(ship.VerifySkillBlockers, "cross_stage_context_not_distinct"),
			"ship no longer owns context-origin lattice edges")
		assert.False(t, hasReasonCode(ship.Result.ReasonCodes, "cross_stage_context_not_distinct"),
			"ship-owned context lattice blockers must not reach G_ship reasons after S3 folding")

		// Light preset: same colliding records, advisory (no blocker).
		lightChange := model.NewChange("ship-lattice-light")
		lightChange.WorkflowPreset = model.WorkflowPresetLight
		lightChange.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, lightChange))
		lightShip, err := buildShipAuthorityFromReadiness(root, lightChange, GovernanceReadiness{
			ArtifactReadiness: ArtifactReadiness{Ready: true},
			PassingSkills:     verifyPassing,
			ReviewSurface:     &ReviewAuthority{PassingSkills: reviewPassing},
		})
		require.NoError(t, err)
		assert.False(t, hasReasonCode(lightShip.VerifySkillBlockers, "cross_stage_context_not_distinct"),
			"light preset keeps the lattice advisory")
	})
}

// passingShipVerificationRecord returns a passing ship-verification record
// carrying both re-homed standard/strict attestations, stamped at the given time.
func passingShipVerificationRecord(at time.Time) model.VerificationRecord {
	return model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: at.UTC(),
		References: []string{
			shipAssuranceCompleteReference,
			shipReviewerIndependenceReference,
		},
	}
}

func closeoutReuseReviewRecords(runVersion int, specTimestamp time.Time, codeTimestamp time.Time) map[string]model.VerificationRecord {
	return map[string]model.VerificationRecord{
		SkillSpecComplianceReview: {
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  specTimestamp.UTC(),
			RunVersion: runVersion,
			References: []string{"layer:R0=pass"},
		},
		SkillCodeQualityReview: {
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  codeTimestamp.UTC(),
			RunVersion: runVersion,
			References: []string{"layer:IR1=pass"},
		},
	}
}
