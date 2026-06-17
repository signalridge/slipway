package progression

import (
	"testing"

	engineskill "github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
)

// singleSkill asserts the resolved skill set is at most one skill and returns
// it (or "") for the single-skill states (S0/S1/S2/S4).
func singleSkill(t *testing.T, skills []string) string {
	t.Helper()
	if len(skills) > 1 {
		t.Fatalf("expected at most one skill, got %v", skills)
	}
	if len(skills) == 0 {
		return ""
	}
	return skills[0]
}

func TestResolveNextSkill_S0Intake_SubSteps(t *testing.T) {
	t.Parallel()
	tests := []struct {
		subStep model.IntakeSubStep
		skill   string
	}{
		{model.IntakeSubStepClarify, SkillIntakeClarification},
		{model.IntakeSubStepResearch, ""},
		{model.IntakeSubStepConfirm, ""},
	}
	for _, tt := range tests {
		change := model.Change{
			CurrentState:  model.StateS0Intake,
			IntakeSubStep: tt.subStep,
		}
		skills, evidenceState := ResolveNextSkill(change)
		name := singleSkill(t, skills)
		if name != tt.skill {
			t.Errorf("ResolveNextSkill(S0_INTAKE, %s) = %q, want %q", tt.subStep, name, tt.skill)
		}
		if tt.skill != "" && evidenceState != string(model.StateS0Intake) {
			t.Errorf("ResolveNextSkill(S0_INTAKE, %s) evidenceState = %q, want %q", tt.subStep, evidenceState, model.StateS0Intake)
		}
	}
}

func TestResolveNextSkill_S1Plan_SubSteps(t *testing.T) {
	t.Parallel()
	tests := []struct {
		subStep model.PlanSubStep
		skill   string
	}{
		{model.PlanSubStepResearch, SkillResearchOrchestration},
		{model.PlanSubStepBundle, ""},
		{model.PlanSubStepAudit, SkillPlanAudit},
		{model.PlanSubStepValidate, ""},
	}
	for _, tt := range tests {
		change := model.Change{
			CurrentState: model.StateS1Plan,
			PlanSubStep:  tt.subStep,
		}
		skills, evidenceState := ResolveNextSkill(change)
		name := singleSkill(t, skills)
		if name != tt.skill {
			t.Errorf("ResolveNextSkill(S1_PLAN, %s) = %q, want %q", tt.subStep, name, tt.skill)
		}
		if tt.skill != "" && evidenceState != string(model.StateS1Plan) {
			t.Errorf("ResolveNextSkill(S1_PLAN, %s) evidenceState = %q, want %q", tt.subStep, evidenceState, model.StateS1Plan)
		}
	}
}

func TestResolveNextSkill_S2Execute(t *testing.T) {
	t.Parallel()

	// Without guardrail domain -> wave-orchestration
	change := model.Change{CurrentState: model.StateS2Execute}
	skills, state := ResolveNextSkill(change)
	name := singleSkill(t, skills)
	if name != SkillWaveOrchestration {
		t.Errorf("expected %s, got %s", SkillWaveOrchestration, name)
	}
	if state != string(model.StateS2Execute) {
		t.Errorf("expected %s, got %s", model.StateS2Execute, state)
	}

	// With guardrail domain -> still wave-orchestration (no kernel-level dispatch override)
	change.GuardrailDomain = model.GuardrailDomainAuthAuthZ
	skills, _ = ResolveNextSkill(change)
	name = singleSkill(t, skills)
	if name != SkillWaveOrchestration {
		t.Errorf("expected %s for guardrail domain, got %s", SkillWaveOrchestration, name)
	}
}

func TestResolveNextSkill_S3Review(t *testing.T) {
	t.Parallel()
	change := model.Change{CurrentState: model.StateS3Review}
	skills, state := ResolveNextSkill(change)
	if state != string(model.StateS3Review) {
		t.Errorf("expected %s, got %s", model.StateS3Review, state)
	}
	assertReviewPair(
		t,
		"default-selection",
		skills,
		[]string{SkillSpecComplianceReview, SkillCodeQualityReview, SkillIndependentReview},
	)
}

func TestResolveNextSkill_S3Review_SelectedSecurityReview(t *testing.T) {
	t.Parallel()

	change := model.Change{CurrentState: model.StateS3Review}
	skills, state := ResolveNextSkillWithReviewSelection(
		change,
		engineskill.ReviewSkillSelection{SecurityReviewSelected: true},
	)
	if state != string(model.StateS3Review) {
		t.Errorf("expected %s, got %s", model.StateS3Review, state)
	}
	assertReviewPair(
		t,
		"security-selected",
		skills,
		[]string{SkillSpecComplianceReview, SkillCodeQualityReview, SkillIndependentReview, SkillSecurityReview},
	)
}

// TestResolveNextSkill_S3Review_ReviewSetIndependentOfEvidence proves the
// routing surface exposes the selected review set at S3 regardless of any
// recorded review evidence: ResolveNextSkill is a pure function of change
// state plus explicit selection input, so recorded verdicts on any review must
// not collapse the parallel set.
func TestResolveNextSkill_S3Review_ReviewSetIndependentOfEvidence(t *testing.T) {
	t.Parallel()

	base := model.Change{CurrentState: model.StateS3Review}
	wantPair := []string{SkillSpecComplianceReview, SkillCodeQualityReview, SkillIndependentReview}

	// No recorded review evidence.
	skills, _ := ResolveNextSkill(base)
	assertReviewPair(t, "no-evidence", skills, wantPair)

	// Even with guardrail/sensitive domain set, the pair is unchanged.
	sensitive := base
	sensitive.GuardrailDomain = model.GuardrailDomainAuthAuthZ
	skills, _ = ResolveNextSkill(sensitive)
	assertReviewPair(t, "sensitive-domain", skills, wantPair)
}

func assertReviewPair(t *testing.T, label string, got, want []string) {
	t.Helper()
	gotSet := map[string]bool{}
	for _, s := range got {
		gotSet[s] = true
	}
	if len(got) != len(want) {
		t.Fatalf("[%s] S3 review set %v, want %v", label, got, want)
	}
	for _, w := range want {
		if !gotSet[w] {
			t.Errorf("[%s] S3 review set %v missing %s", label, got, w)
		}
	}
}

func TestResolveNextSkill_S4Verify(t *testing.T) {
	t.Parallel()
	change := model.Change{CurrentState: model.StateS4Verify}
	skills, state := ResolveNextSkill(change)
	name := singleSkill(t, skills)
	if name != SkillGoalVerification {
		t.Errorf("expected %s, got %s", SkillGoalVerification, name)
	}
	if state != string(model.StateS4Verify) {
		t.Errorf("expected %s, got %s", model.StateS4Verify, state)
	}
}

func TestResolveNextSkill_Discovery_S1Plan_Discovery(t *testing.T) {
	t.Parallel()
	change := model.Change{
		CurrentState:   model.StateS1Plan,
		PlanSubStep:    model.PlanSubStepResearch,
		NeedsDiscovery: true,
		WorktreePath:   "/tmp/worktree",
	}
	skills, state := ResolveNextSkill(change)
	name := singleSkill(t, skills)
	if name != SkillResearchOrchestration {
		t.Errorf("expected %s, got %s", SkillResearchOrchestration, name)
	}
	if state != string(model.StateS1Plan) {
		t.Errorf("expected %s, got %s", model.StateS1Plan, state)
	}
}

func TestResolveNextSkill_S1Plan_Bundle_NoSkill(t *testing.T) {
	t.Parallel()
	for _, needsDiscovery := range []bool{false, true} {
		change := model.Change{
			CurrentState:   model.StateS1Plan,
			PlanSubStep:    model.PlanSubStepBundle,
			NeedsDiscovery: needsDiscovery,
		}
		skills, _ := ResolveNextSkill(change)
		name := singleSkill(t, skills)
		if name != "" {
			t.Errorf("ResolveNextSkill(%v, S1_PLAN/bundle) = %q, want empty", needsDiscovery, name)
		}
	}
}
