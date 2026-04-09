package progression

import (
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateRequiredSkills_NoRegistry(t *testing.T) {
	t.Parallel()
	change := model.NewChange("req-1")
	_, _, err := EvaluateRequiredSkillsForChange("/tmp/nonexistent", change, model.StateS1Plan, 0, false)
	// Either an error or empty results is acceptable; just shouldn't panic.
	_ = err
}

func TestEvaluateRequiredSkills_MissingEvidenceReturnsBlocker(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	change := model.NewChange("missing-evidence")
	passing, blockers, err := EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(passing) != 0 {
		t.Fatalf("expected no passing skills, got %v", passing)
	}
	found := false
	for _, blocker := range blockers {
		if blocker == "required_skill_missing:plan-audit" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected required_skill_missing:plan-audit, got %v", blockers)
	}
}

func TestEvaluateRequiredSkills_PassingEvidenceIsReturned(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "passing-evidence"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS1Plan
	require.NoError(t, state.SaveChange(root, change))

	rec := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
	}
	writeVerificationForTest(t, root, slug, SkillPlanAudit, rec)

	passing, blockers, err := EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blockers) != 0 {
		t.Fatalf("expected no blockers, got %v", blockers)
	}
	if !passing[SkillPlanAudit].IsPassing() {
		t.Fatalf("expected passing plan-audit verification, got %v", passing)
	}
}

func TestEvaluateRequiredSkills_FailsClosedWhenRunSummaryBoundSkillHasNoSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "run-summary-missing"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	rec := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 7,
	}
	writeVerificationForTest(t, root, slug, SkillSpecComplianceReview, rec)

	passing, blockers, err := EvaluateRequiredSkillsForChange(root, change, model.StateS3Review, 0, false)
	require.NoError(t, err)
	assert.Empty(t, passing)
	assert.Contains(t, blockers, "required_skill_not_ready:spec-compliance-review:run_summary_missing")
}

func TestEvaluateRequiredSkillsForChange_RequiresExplicitPlanSubStep(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "explicit-plan-substep"

	change := model.NewChange(slug)
	change.CurrentState = model.StateS1Plan
	change.NeedsDiscovery = true
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	rec := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
	}
	writeVerificationForTest(t, root, slug, SkillPlanAudit, rec)

	_, blockersWithoutExplicitSubStep, err := EvaluateRequiredSkillsForChange(
		root,
		change,
		model.StateS1Plan,
		0,
		false,
	)
	require.NoError(t, err)
	assert.Contains(t, blockersWithoutExplicitSubStep, "required_skill_missing:research-orchestration")

	_, blockersWithExplicitSubStep, err := EvaluateRequiredSkillsForChange(
		root,
		change,
		model.StateS1Plan,
		0,
		false,
		model.PlanSubStepAudit,
	)
	require.NoError(t, err)
	assert.NotContains(t, blockersWithExplicitSubStep, "required_skill_missing:research-orchestration")
}

func TestExtractHighRiskChecks_FromReferences(t *testing.T) {
	t.Parallel()
	passingSkills := map[string]model.VerificationRecord{
		"test-skill": {
			Verdict: model.VerificationVerdictPass,
			References: []string{
				"check:auth_authz.safety_baseline=pass",
				"check:security_credentials.safety_baseline=fail",
			},
		},
	}
	checks := ExtractHighRiskChecks(passingSkills)
	if !checks["auth_authz.safety_baseline"] {
		t.Error("expected auth_authz.safety_baseline to be true")
	}
	if checks["security_credentials.safety_baseline"] {
		t.Error("expected security_credentials.safety_baseline to be false")
	}
}

func TestParseHighRiskCheckReference_Formats(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		wantID   string
		wantPass bool
		wantOK   bool
	}{
		{"", "", false, false},
		{"check:auth_authz.safety_baseline=pass", "auth_authz.safety_baseline", true, true},
		{"check:security_credentials.safety_baseline=fail", "security_credentials.safety_baseline", false, true},
		{"high_risk_check:auth_authz.safety_baseline:pass", "auth_authz.safety_baseline", true, true},
		{"bogus_check_id=pass", "", false, false},
	}
	for _, tt := range tests {
		id, pass, ok := ParseHighRiskCheckReference(tt.input)
		if ok != tt.wantOK || id != tt.wantID || pass != tt.wantPass {
			t.Errorf("ParseHighRiskCheckReference(%q) = (%q, %v, %v), want (%q, %v, %v)",
				tt.input, id, pass, ok, tt.wantID, tt.wantPass, tt.wantOK)
		}
	}
}

func TestReasonCodeFromBlockerSpecUsesCanonicalCodeEnvelope(t *testing.T) {
	t.Parallel()

	reason := model.ReasonCodeFromSpec("required_skill_not_ready:plan-audit:run summary drift")
	assert.Equal(t, "required_skill_not_ready", reason.Code)
	assert.Equal(t, model.ReasonSeverityError, reason.Severity)
	assert.Equal(t, "plan-audit:run summary drift", reason.Detail)
}
