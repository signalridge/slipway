package skill

import (
	"testing"
	"time"

	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGovernanceRegistryCompleteness(t *testing.T) {
	registry := GovernanceRegistry()
	require.Len(t, registry, 7)
	assert.True(t, IsGovernanceSkill("intake-analysis"))
	assert.True(t, IsGovernanceSkill("final-closeout"))
	assert.False(t, IsGovernanceSkill("spln-tdd"))
}

func TestRequiredSkillsByLevelAndState(t *testing.T) {
	assert.Nil(t, RequiredSkillsForState(model.LevelL1, model.StateS6RunWaves, false, false))
	assert.Equal(
		t,
		[]string{"intake-analysis"},
		RequiredSkillsForState(model.LevelL1, model.StateS1Analyze, true, false),
	)
	assert.Equal(
		t,
		[]string{"artifact-review"},
		RequiredSkillsForState(model.LevelL2, model.StateS7Review, false, false),
	)
	assert.Equal(
		t,
		[]string{"goal-verification", "final-closeout"},
		RequiredSkillsForState(model.LevelL3, model.StateS8Verify, false, true),
	)
}

func TestValidateGovernanceEvidenceReadinessPass(t *testing.T) {
	reviewerSession, err := NewSessionID()
	require.NoError(t, err)
	implSession, err := NewSessionID()
	require.NoError(t, err)
	require.NotEqual(t, reviewerSession, implSession)

	hash, err := CanonicalInputHash(map[string]any{
		"request_id":          "r1",
		"state":               "S7_REVIEW",
		"run_summary_version": 2,
	})
	require.NoError(t, err)

	record := model.EvidenceRecord{
		RunSummaryVersion: 2,
		SessionID:         reviewerSession,
		SkillName:         "artifact-review",
		Version:           "v1",
		State:             model.StateS7Review,
		Verdict:           model.EvidenceVerdictPass,
		Blockers:          []string{},
		References:        []string{},
		Timestamp:         time.Now().UTC(),
		InputHash:         hash,
	}

	err = ValidateGovernanceEvidenceReadiness(EvidenceReadinessInput{
		Level:                         model.LevelL2,
		Record:                        record,
		LatestFrozenRunSummaryVersion: 2,
		ImplementerBaselineSessionID:  implSession,
	})
	require.NoError(t, err)
}

func TestValidateGovernanceEvidenceReadinessFailsOnReviewerIndependence(t *testing.T) {
	session, err := NewSessionID()
	require.NoError(t, err)
	hash, err := CanonicalInputHash(map[string]any{
		"request_id":          "r1",
		"state":               "S7_REVIEW",
		"run_summary_version": 1,
	})
	require.NoError(t, err)

	record := model.EvidenceRecord{
		RunSummaryVersion: 1,
		SessionID:         session,
		SkillName:         "artifact-review",
		Version:           "v1",
		State:             model.StateS7Review,
		Verdict:           model.EvidenceVerdictPass,
		Blockers:          []string{},
		References:        []string{},
		Timestamp:         time.Now().UTC(),
		InputHash:         hash,
	}

	err = ValidateGovernanceEvidenceReadiness(EvidenceReadinessInput{
		Level:                         model.LevelL2,
		Record:                        record,
		LatestFrozenRunSummaryVersion: 1,
		ImplementerBaselineSessionID:  session,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reviewer session_id must differ")
}

func TestValidateGovernanceEvidenceReadinessFailsOnVersionMismatch(t *testing.T) {
	session, err := NewSessionID()
	require.NoError(t, err)
	impl, err := NewSessionID()
	require.NoError(t, err)
	hash, err := CanonicalInputHash(map[string]any{
		"request_id":          "r1",
		"state":               "S8_VERIFY",
		"run_summary_version": 1,
	})
	require.NoError(t, err)

	record := model.EvidenceRecord{
		RunSummaryVersion: 1,
		SessionID:         session,
		SkillName:         "goal-verification",
		Version:           "v1",
		State:             model.StateS8Verify,
		Verdict:           model.EvidenceVerdictPass,
		Blockers:          []string{},
		References:        []string{},
		Timestamp:         time.Now().UTC(),
		InputHash:         hash,
	}

	err = ValidateGovernanceEvidenceReadiness(EvidenceReadinessInput{
		Level:                         model.LevelL3,
		Record:                        record,
		LatestFrozenRunSummaryVersion: 2,
		ImplementerBaselineSessionID:  impl,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run_summary_version mismatch")
}

func TestCanonicalInputHashReproducibleAndSessionValidation(t *testing.T) {
	payloadA := map[string]any{
		"b": "line1\r\nline2",
		"a": map[string]any{
			"z": 2,
			"y": 1,
		},
	}
	payloadB := map[string]any{
		"a": map[string]any{
			"y": 1,
			"z": 2,
		},
		"b": "line1\nline2",
	}
	hashA, err := CanonicalInputHash(payloadA)
	require.NoError(t, err)
	hashB, err := CanonicalInputHash(payloadB)
	require.NoError(t, err)
	assert.Equal(t, hashA, hashB)

	sessionID, err := NewSessionID()
	require.NoError(t, err)
	assert.True(t, IsValidSessionID(sessionID))
	assert.False(t, IsValidSessionID("not-a-uuidv7"))
}

func TestMitigationTargetMismatchInvalidatesReadiness(t *testing.T) {
	session, err := NewSessionID()
	require.NoError(t, err)

	record := model.EvidenceRecord{
		RunSummaryVersion: 0,
		SessionID:         session,
		SkillName:         "plan-audit",
		Version:           "v1",
		State:             model.StateS5PlanAudit,
		Verdict:           model.EvidenceVerdictPass,
		Blockers:          []string{},
		References:        []string{},
		Timestamp:         time.Now().UTC(),
		MitigationTarget:  "wrong-target",
	}

	err = ValidateGovernanceEvidenceReadiness(EvidenceReadinessInput{
		Level:                         model.LevelL2,
		Record:                        record,
		LatestFrozenRunSummaryVersion: 0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mitigation_target mismatch")
}
