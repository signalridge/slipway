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

	reviewAfterGoal := passingCloseoutReuseRecords(1)
	goalAt = time.Now().UTC()
	reviewAfterGoal[SkillGoalVerification] = model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  goalAt,
		RunVersion: 1,
	}
	reviewAfterGoal[SkillFinalCloseout] = model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  goalAt.Add(3 * time.Second),
		RunVersion: 1,
		References: []string{
			closeoutGoalVerificationReuseReference,
			closeoutGoalVerificationReuseRunVersionPrefix + "1",
			assuranceCompleteReference,
		},
	}
	reviewRecords := closeoutReuseReviewRecords(1, goalAt.Add(time.Second), goalAt.Add(2*time.Second))
	blockers = closeoutGoalVerificationReuseBlockers(root, change, reviewAfterGoal, reviewRecords, summary)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_goal_verification_reuse_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, "latest review evidence")

	closeoutBeforeGoal := passingCloseoutReuseRecords(1)
	goalAt = time.Now().UTC()
	closeoutBeforeGoal[SkillGoalVerification] = model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  goalAt,
		RunVersion: 1,
	}
	closeoutBeforeGoal[SkillFinalCloseout] = model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  goalAt.Add(-time.Second),
		RunVersion: 1,
		References: []string{
			closeoutGoalVerificationReuseReference,
			closeoutGoalVerificationReuseRunVersionPrefix + "1",
			assuranceCompleteReference,
		},
	}
	blockers = closeoutGoalVerificationReuseBlockers(root, change, closeoutBeforeGoal, nil, summary)
	require.Len(t, blockers, 1)
	assert.Equal(t, "closeout_goal_verification_reuse_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, "final-closeout timestamp")

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
