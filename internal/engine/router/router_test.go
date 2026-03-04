package router

import (
	"testing"

	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalizeGuardrailDomain(t *testing.T) {
	got := CanonicalizeGuardrailDomain("security/credentials")
	assert.Equal(t, "security_credentials", got)

	got = CanonicalizeGuardrailDomain("financial flows")
	assert.Equal(t, "financial_flows", got)
}

func TestClassifyIntakeNonSpln(t *testing.T) {
	assessment := model.IntakeAssessment{
		IntentType:    "question",
		IsExecutable:  false,
		Confidence:    0.9,
		ChangeTargets: []string{},
	}

	classification, rationale := ClassifyIntake(assessment)
	assert.Equal(t, ClassificationNonSpln, classification)
	assert.Empty(t, rationale)
}

func TestClassifyIntakeExecutable(t *testing.T) {
	assessment := model.IntakeAssessment{
		IntentType:       "executable_change",
		IsExecutable:     true,
		Confidence:       0.9,
		ChangeTargets:    []string{"internal/auth/mw.go"},
		IntendedDelta:    "adjust timeout",
		BlockingUnknowns: []string{"deployment window"},
	}

	classification, rationale := ClassifyIntake(assessment)
	assert.Equal(t, ClassificationExecutable, classification)
	assert.Contains(t, rationale, "execution_unknowns_present")
}

func TestRouteAutoGuardrailToL3(t *testing.T) {
	input := RouteInput{
		Mode: LevelModeAuto,
		IntakeAssessment: model.IntakeAssessment{
			IntentType:    "executable_change",
			IsExecutable:  true,
			Confidence:    0.8,
			ChangeTargets: []string{"internal/auth"},
		},
		Scores:          model.Scores{Novelty: 1, Ambiguity: 1, Impact: 1, Risk: 1, ReversibilityCost: 1},
		GuardrailDomain: "auth/authz",
	}

	result, err := Route(input)
	require.NoError(t, err)
	assert.Equal(t, ClassificationExecutable, result.Classification)
	assert.Equal(t, model.LevelL3, result.Level)
	assert.Equal(t, model.LevelSourceAuto, result.LevelSource)
	assert.Equal(t, "auth_authz", result.RouteSnapshot.GuardrailDomain)
}

func TestRouteAutoHighControlToL2(t *testing.T) {
	input := RouteInput{
		Mode: LevelModeAuto,
		IntakeAssessment: model.IntakeAssessment{
			IntentType:    "executable_change",
			IsExecutable:  true,
			Confidence:    0.8,
			ChangeTargets: []string{"internal/service"},
		},
		Scores: model.Scores{Novelty: 1, Ambiguity: 1, Impact: 3, Risk: 3, ReversibilityCost: 2},
	}
	result, err := Route(input)
	require.NoError(t, err)
	assert.Equal(t, model.LevelL2, result.Level)
}

func TestRouteFixedLevelConflicts(t *testing.T) {
	input := RouteInput{
		Mode:            LevelModeFixed,
		FixedLevel:      model.LevelL1,
		GuardrailDomain: "privacy/PII",
		IntakeAssessment: model.IntakeAssessment{
			IntentType:    "executable_change",
			IsExecutable:  true,
			Confidence:    0.8,
			ChangeTargets: []string{"internal/privacy"},
		},
		Scores: model.Scores{Novelty: 1, Ambiguity: 1, Impact: 1, Risk: 1, ReversibilityCost: 1},
	}
	result, err := Route(input)
	require.NoError(t, err)
	assert.Equal(t, model.LevelL1, result.Level)
	assert.Equal(t, model.LevelSourceUserSelected, result.LevelSource)
	assert.Contains(t, result.RouteSnapshot.BlockingConflicts, "fixed_level_guardrail_conflict")
}

func TestDeriveRouteSignals(t *testing.T) {
	assessment := model.IntakeAssessment{
		IntentType:    "executable_change",
		IsExecutable:  true,
		Confidence:    0.9,
		ChangeTargets: []string{"service-a", "service-b"},
		IntendedDelta: "re-architect auth and session modules across middleware and api boundaries",
		AuxiliarySignals: []string{
			"major_refactor",
		},
	}
	signals := DeriveRouteSignals(assessment, WorkspaceSignals{
		HasInScopeSourceFiles: true,
		ScopeTouchCount:       2,
	})
	assert.True(t, signals.MajorRefactor)
	assert.False(t, signals.NewProject)
}

func TestGenerateRequestV1(t *testing.T) {
	seen := map[string]struct{}{
		"add-auth-timeout":   {},
		"add-auth-timeout-2": {},
	}
	requestID, slug, err := GenerateRequestV1("Add Auth Timeout", func(candidate string) bool {
		_, ok := seen[candidate]
		return ok
	})
	require.NoError(t, err)
	assert.True(t, model.IsUUIDv7(requestID))
	assert.Equal(t, "add-auth-timeout-3", slug)
}
