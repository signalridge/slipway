package progression

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
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

func TestCloseoutGoalVerificationReuseBlockers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("ship-closeout-reuse")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.CurrentState = model.StateS4Verify
	require.NoError(t, state.SaveChange(root, change))

	capturedAt := time.Now().UTC().Add(-time.Minute)
	summary := closeoutReuseExecutionSummary(change, 1, capturedAt)
	passing := passingCloseoutReuseRecords(1)
	assert.Empty(t, closeoutGoalVerificationReuseBlockers(root, change, passing, nil, summary))

	mismatchedGoal := passingCloseoutReuseRecords(1)
	mismatchedGoal[SkillGoalVerification] = model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 2,
	}
	blockers := closeoutGoalVerificationReuseBlockers(root, change, mismatchedGoal, nil, summary)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_goal_verification_reuse_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, "goal-verification run_version mismatch")

	missingRunReference := passingCloseoutReuseRecords(1)
	missingRunReference[SkillFinalCloseout] = model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
		References: []string{
			closeoutGoalVerificationReuseReference,
			assuranceCompleteReference,
		},
	}
	blockers = closeoutGoalVerificationReuseBlockers(root, change, missingRunReference, nil, summary)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_goal_verification_reuse_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, closeoutGoalVerificationReuseRunVersionPrefix)

	executionAfterGoal := passingCloseoutReuseRecords(1)
	goalAt := time.Now().UTC().Add(-2 * time.Minute)
	executionAfterGoal[SkillGoalVerification] = model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  goalAt,
		RunVersion: 1,
	}
	executionAfterGoal[SkillFinalCloseout] = model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  goalAt.Add(time.Minute),
		RunVersion: 1,
		References: []string{
			closeoutGoalVerificationReuseReference,
			closeoutGoalVerificationReuseRunVersionPrefix + "1",
			assuranceCompleteReference,
		},
	}
	executionAfterGoalSummary := closeoutReuseExecutionSummary(change, 1, goalAt.Add(time.Hour))
	blockers = closeoutGoalVerificationReuseBlockers(root, change, executionAfterGoal, nil, executionAfterGoalSummary)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_goal_verification_reuse_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, "latest execution evidence")

	// The review<=goal and closeout>=goal ordering halves have moved out of this
	// opt-in reuse gate into the always-on closeoutChainOrderBlockers invariant;
	// they are asserted under closeout_chain_order_invalid in
	// TestCloseoutChainOrderBlockers, not here.

	changedContent := passingCloseoutReuseRecords(1)
	changedGoalAt := changedContent[SkillGoalVerification].Timestamp.UTC()
	targetRel := "internal/reuse-target.go"
	targetPath := filepath.Join(root, targetRel)
	require.NoError(t, os.MkdirAll(filepath.Dir(targetPath), 0o755))
	require.NoError(t, os.WriteFile(targetPath, []byte("package internal\n"), 0o644))
	contentSummary := closeoutReuseExecutionSummaryWithFiles(change, 1, changedGoalAt.Add(-time.Minute), nil, []string{targetRel})
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillGoalVerification, changedContent[SkillGoalVerification], contentSummary))
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillFinalCloseout, changedContent[SkillFinalCloseout], contentSummary))
	require.NoError(t, os.WriteFile(targetPath, []byte("package internal\nconst changed = true\n"), 0o644))
	blockers = closeoutGoalVerificationReuseBlockers(root, change, changedContent, nil, contentSummary)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_goal_verification_reuse_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, targetRel)

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n\n- [ ] `task-a` updated plan\n"), 0o644))
	staleSummary := closeoutReuseExecutionSummary(change, 1, time.Now().UTC().Add(-time.Hour))
	staleSummary.TasksPlanHash = "previous-task-plan-hash"
	blockers = closeoutGoalVerificationReuseBlockers(root, change, passing, nil, staleSummary)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_goal_verification_reuse_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, "freshness must be fresh")
}

func TestBuildShipAuthoritySurfacesCloseoutReuseBlocker(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("ship-closeout-reuse-blocked")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.CurrentState = model.StateS4Verify
	require.NoError(t, state.SaveChange(root, change))

	passing := passingCloseoutReuseRecords(1)
	passing[SkillGoalVerification] = model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 2,
	}
	ship, err := buildShipAuthorityFromReadiness(root, change, GovernanceReadiness{
		ExecutionSummary:  closeoutReuseExecutionSummary(change, 1, time.Now().UTC().Add(time.Minute)),
		ArtifactReadiness: ArtifactReadiness{Ready: true},
		PassingSkills:     passing,
		ReviewSurface:     &ReviewAuthority{},
	})
	require.NoError(t, err)
	assert.True(t, hasAdvanceReasonCode(ship.VerifySkillBlockers, "closeout_goal_verification_reuse_invalid"))
	assert.True(t, hasAdvanceReasonCode(ship.Result.ReasonCodes, "closeout_goal_verification_reuse_invalid"))
}

func TestCloseoutGoalVerificationReuseInvalidBlockerRoutesS4Recovery(t *testing.T) {
	t.Parallel()

	blocker := closeoutGoalVerificationReuseInvalidBlocker("assurance.md changed after reused proof")
	assert.Contains(t, blocker.Detail, "goal-verification")
	assert.Contains(t, blocker.Detail, "final-closeout")
	assert.Contains(t, blocker.Detail, "assurance.md")
}

// TestCloseoutReviewerIndependenceBlockers covers the P1 presence facet
// (REQ-001): under standard/strict the passing final-closeout record must carry
// closeout:reviewer_independence=pass; absence fails closed, light is advisory.
func TestCloseoutReviewerIndependenceBlockers(t *testing.T) {
	t.Parallel()

	missing := map[string]model.VerificationRecord{
		SkillFinalCloseout: {
			Verdict:    model.VerificationVerdictPass,
			References: []string{"closeout:assurance_complete=pass"},
		},
	}
	blockers := closeoutReviewerIndependenceBlockers(missing, true)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_reviewer_independence_missing", blockers[0].Code)

	present := map[string]model.VerificationRecord{
		SkillFinalCloseout: {
			Verdict:    model.VerificationVerdictPass,
			References: []string{"  closeout:reviewer_independence=pass  "},
		},
	}
	assert.Empty(t, closeoutReviewerIndependenceBlockers(present, true))

	// No final-closeout record at all -> same fail-closed blocker.
	blockers = closeoutReviewerIndependenceBlockers(map[string]model.VerificationRecord{}, true)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_reviewer_independence_missing", blockers[0].Code)

	// Light preset (required=false) is advisory: never blocks.
	assert.Empty(t, closeoutReviewerIndependenceBlockers(missing, false))
}

// TestCloseoutChainOrderBlockers covers the always-on P1 ordering invariant
// (REQ-001): closeout >= goal >= max(selected review evidence) under its own
// distinct closeout_chain_order_invalid code, advisory on light.
func TestCloseoutChainOrderBlockers(t *testing.T) {
	t.Parallel()

	selectedReviewers := engineskill.SelectedReviewSkills(engineskill.ReviewSkillSelection{})
	selectedReviewersWithSecurity := engineskill.SelectedReviewSkills(engineskill.ReviewSkillSelection{SecurityReviewSelected: true})
	goalAt := time.Now().UTC()
	inOrder := map[string]model.VerificationRecord{
		SkillGoalVerification: {
			Verdict:   model.VerificationVerdictPass,
			Timestamp: goalAt,
		},
		SkillFinalCloseout: {
			Verdict:   model.VerificationVerdictPass,
			Timestamp: goalAt.Add(time.Second),
		},
	}
	reviewsBeforeGoal := closeoutReuseReviewRecords(1, goalAt.Add(-2*time.Second), goalAt.Add(-time.Second))
	assert.Empty(t, closeoutChainOrderBlockers(inOrder, reviewsBeforeGoal, selectedReviewers, true),
		"in-order chain with reviews before goal and goal before closeout must pass")

	// review after goal -> blocker.
	reviewsAfterGoal := closeoutReuseReviewRecords(1, goalAt.Add(time.Second), goalAt.Add(2*time.Second))
	blockers := closeoutChainOrderBlockers(inOrder, reviewsAfterGoal, selectedReviewers, true)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_chain_order_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, "review evidence")

	// Every selected reviewer must be ordered before goal-verification, not only
	// the historical spec/code pair.
	selectedReviewsAfterGoal := closeoutReuseReviewRecords(1, goalAt.Add(-2*time.Second), goalAt.Add(-time.Second))
	selectedReviewsAfterGoal[SkillIndependentReview] = model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Timestamp: goalAt.Add(time.Second),
	}
	selectedReviewsAfterGoal[SkillSecurityReview] = model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Timestamp: goalAt.Add(2 * time.Second),
	}
	blockers = closeoutChainOrderBlockers(inOrder, selectedReviewsAfterGoal, selectedReviewers, true)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_chain_order_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, SkillIndependentReview)

	unselectedSecurityAfterGoal := closeoutReuseReviewRecords(1, goalAt.Add(-2*time.Second), goalAt.Add(-time.Second))
	unselectedSecurityAfterGoal[SkillSecurityReview] = model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Timestamp: goalAt.Add(time.Second),
	}
	assert.Empty(t, closeoutChainOrderBlockers(inOrder, unselectedSecurityAfterGoal, selectedReviewers, true),
		"security-review evidence is silent when the security control did not select it")

	blockers = closeoutChainOrderBlockers(inOrder, unselectedSecurityAfterGoal, selectedReviewersWithSecurity, true)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_chain_order_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, SkillSecurityReview)

	// closeout before goal -> blocker.
	closeoutBeforeGoal := map[string]model.VerificationRecord{
		SkillGoalVerification: {
			Verdict:   model.VerificationVerdictPass,
			Timestamp: goalAt,
		},
		SkillFinalCloseout: {
			Verdict:   model.VerificationVerdictPass,
			Timestamp: goalAt.Add(-time.Second),
		},
	}
	blockers = closeoutChainOrderBlockers(closeoutBeforeGoal, nil, selectedReviewers, true)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_chain_order_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, "final-closeout must not predate")

	// Genuinely-absent goal: nothing to compare, no blocker (owned elsewhere).
	assert.Empty(t, closeoutChainOrderBlockers(map[string]model.VerificationRecord{}, reviewsAfterGoal, selectedReviewers, true))

	// Light preset (required=false) is advisory even on an out-of-order chain.
	assert.Empty(t, closeoutChainOrderBlockers(closeoutBeforeGoal, reviewsAfterGoal, selectedReviewers, false))
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

func hasReasonCodeDetail(codes []model.ReasonCode, code, detail string) bool {
	return slices.ContainsFunc(codes, func(c model.ReasonCode) bool {
		return c.Code == code && c.Detail == detail
	})
}

// countReasonCode counts how many entries in codes carry the given reason code.
func countReasonCode(codes []model.ReasonCode, code string) int {
	n := 0
	for _, c := range codes {
		if c.Code == code {
			n++
		}
	}
	return n
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
	shipStages := crossStageContextShipStagesForSelectedSkills(selectedReviewers)
	shipStagesWithSecurity := crossStageContextShipStagesForSelectedSkills(selectedReviewersWithSecurity)

	t.Run("review-owned spec/code edge does not re-fire at ship", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		change := model.NewChange("ship-lattice-no-refire")
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS4Verify
		require.NoError(t, state.SaveChange(root, change))

		// spec and code share a handle (a review-owned collision), but goal and
		// closeout are distinct. The ship gate owns only goal/closeout, so the
		// spec<->code edge must NOT fire here.
		merged := map[string]model.VerificationRecord{
			SkillSpecComplianceReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "shared-review")},
			},
			SkillCodeQualityReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "shared-review")},
			},
			SkillGoalVerification: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextGoal, "handle-goal")},
			},
			SkillFinalCloseout: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextCloseout, "handle-closeout")},
			},
		}
		blockers := crossStageContextDistinctBlockers(root, change, merged, shipStages, crossStageContextOwnedShipStages, true)
		assert.Empty(t, blockers, "spec<->code is review-owned and must not re-fire at ship")
	})

	t.Run("review-owned selected security edge does not re-fire at ship", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		change := model.NewChange("ship-lattice-security-no-refire")
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS4Verify
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
			SkillGoalVerification: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextGoal, "handle-goal")},
			},
			SkillFinalCloseout: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextCloseout, "handle-closeout")},
			},
		}
		blockers := crossStageContextDistinctBlockers(root, change, merged, shipStagesWithSecurity, crossStageContextOwnedShipStages, true)
		assert.Empty(t, blockers, "selected security peer collisions are review-owned and must not re-fire at ship")
	})

	t.Run("ship-owned goal/spec edge fires and never emits executor_agent_missing", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		change := model.NewChange("ship-lattice-goal-collision")
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS4Verify
		require.NoError(t, state.SaveChange(root, change))

		// goal shares a handle with spec -> a goal-owned edge fires.
		merged := map[string]model.VerificationRecord{
			SkillSpecComplianceReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "shared-goal-spec")},
			},
			SkillCodeQualityReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "handle-code")},
			},
			SkillGoalVerification: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextGoal, "shared-goal-spec")},
			},
			SkillFinalCloseout: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextCloseout, "handle-closeout")},
			},
		}
		blockers := crossStageContextDistinctBlockers(root, change, merged, shipStages, crossStageContextOwnedShipStages, true)
		require.Equal(t, 1, countReasonCode(blockers, "cross_stage_context_not_distinct"))
		// lexical order: goal < spec-compliance-review.
		assert.Equal(t, model.StageContextGoal+"|"+SkillSpecComplianceReview, blockers[0].Detail)
		assert.False(t, hasReasonCode(blockers, "executor_agent_missing"),
			"the distinct-context lattice must never emit executor_agent_missing")
	})

	t.Run("selected security participates at ship and unselected security stays silent", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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
				References: []string{contextOriginRef(model.StageContextReview, "shared-goal-security")},
			},
		}
		verifyPassing := map[string]model.VerificationRecord{
			SkillGoalVerification: {
				Verdict:    model.VerificationVerdictPass,
				Timestamp:  time.Now().UTC(),
				References: []string{contextOriginRef(model.StageContextGoal, "shared-goal-security")},
			},
			SkillFinalCloseout: {
				Verdict: model.VerificationVerdictPass,
				References: []string{
					contextOriginRef(model.StageContextCloseout, "handle-closeout"),
					assuranceCompleteReference,
					closeoutReviewerIndependenceReference,
				},
			},
		}
		wantDetail := model.StageContextGoal + "|" + SkillSecurityReview

		selectedChange := model.NewChange("ship-lattice-security-selected")
		selectedChange.WorkflowPreset = model.WorkflowPresetStandard
		selectedChange.CurrentState = model.StateS4Verify
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
		assert.True(t,
			hasReasonCodeDetail(selectedShip.VerifySkillBlockers, "cross_stage_context_not_distinct", wantDetail),
			"selected security-review must participate in ship-owned goal edges")
		assert.True(t,
			hasReasonCodeDetail(selectedShip.Result.ReasonCodes, "cross_stage_context_not_distinct", wantDetail),
			"selected security-review ship blocker must reach G_ship reasons")

		unselectedChange := model.NewChange("ship-lattice-security-unselected")
		unselectedChange.WorkflowPreset = model.WorkflowPresetStandard
		unselectedChange.CurrentState = model.StateS4Verify
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

	t.Run("ship blocker is dual-surfaced and advisory on light", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		initGitWorkspaceForReadinessOptimizationTests(t, root)
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		change := model.NewChange("ship-lattice-dual-surface")
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.CurrentState = model.StateS4Verify
		require.NoError(t, state.SaveChange(root, change))

		// Build a passing set where goal and closeout collide (a ship-owned edge),
		// carried through both the review surface (spec/code) and verify surface
		// (goal/closeout).
		reviewPassing := map[string]model.VerificationRecord{
			SkillSpecComplianceReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "handle-spec")},
			},
			SkillCodeQualityReview: {
				Verdict:    model.VerificationVerdictPass,
				References: []string{contextOriginRef(model.StageContextReview, "handle-code")},
			},
		}
		verifyPassing := map[string]model.VerificationRecord{
			SkillGoalVerification: {
				Verdict:    model.VerificationVerdictPass,
				Timestamp:  time.Now().UTC(),
				References: []string{contextOriginRef(model.StageContextGoal, "shared-go-close")},
			},
			SkillFinalCloseout: {
				Verdict: model.VerificationVerdictPass,
				References: []string{
					contextOriginRef(model.StageContextCloseout, "shared-go-close"),
					assuranceCompleteReference,
					closeoutReviewerIndependenceReference,
				},
			},
		}

		ship, err := buildShipAuthorityFromReadiness(root, change, GovernanceReadiness{
			ArtifactReadiness: ArtifactReadiness{Ready: true},
			PassingSkills:     verifyPassing,
			ReviewSurface:     &ReviewAuthority{PassingSkills: reviewPassing},
		})
		require.NoError(t, err)
		assert.True(t, hasReasonCode(ship.VerifySkillBlockers, "cross_stage_context_not_distinct"),
			"ship-owned goal<->closeout collision must surface in VerifySkillBlockers")
		assert.True(t, hasReasonCode(ship.Result.ReasonCodes, "cross_stage_context_not_distinct"),
			"the actionable lattice blocker must reach G_ship reasons (dual-surfaced into unresolved)")

		// Light preset: same colliding records, advisory (no blocker).
		lightChange := model.NewChange("ship-lattice-light")
		lightChange.WorkflowPreset = model.WorkflowPresetLight
		lightChange.CurrentState = model.StateS4Verify
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

func closeoutReuseExecutionSummary(change model.Change, runVersion int, capturedAt time.Time) *model.ExecutionSummary {
	return closeoutReuseExecutionSummaryWithFiles(change, runVersion, capturedAt, nil, nil)
}

func closeoutReuseExecutionSummaryWithFiles(
	change model.Change,
	runVersion int,
	capturedAt time.Time,
	changedFiles []string,
	targetFiles []string,
) *model.ExecutionSummary {
	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: runVersion,
		CapturedAt:        capturedAt.UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01"},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:       "t-01",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindCode,
				ChangedFiles: append([]string(nil), changedFiles...),
				TargetFiles:  append([]string(nil), targetFiles...),
				EvidenceRef:  "test:t-01",
				CapturedAt:   capturedAt.UTC(),
			},
		},
	}
	state.ApplyExecutionSummaryFreshnessInputs(&summary, change)
	summary.SyncDerivedFields()
	return &summary
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

func passingCloseoutReuseRecords(runVersion int) map[string]model.VerificationRecord {
	now := time.Now().UTC()
	return map[string]model.VerificationRecord{
		SkillGoalVerification: {
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  now,
			RunVersion: runVersion,
		},
		SkillFinalCloseout: {
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  now.Add(time.Second),
			RunVersion: runVersion,
			References: []string{
				closeoutGoalVerificationReuseReference,
				closeoutGoalVerificationReuseRunVersionPrefix + strconv.Itoa(runVersion),
				assuranceCompleteReference,
			},
		},
	}
}
