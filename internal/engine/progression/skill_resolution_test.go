package progression

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
)

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
		name, evidenceState := ResolveNextSkill(change)
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
		name, evidenceState := ResolveNextSkill(change)
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
	name, state := ResolveNextSkill(change)
	if name != SkillWaveOrchestration {
		t.Errorf("expected %s, got %s", SkillWaveOrchestration, name)
	}
	if state != string(model.StateS2Execute) {
		t.Errorf("expected %s, got %s", model.StateS2Execute, state)
	}

	// With guardrail domain -> still wave-orchestration (no kernel-level dispatch override)
	change.GuardrailDomain = model.GuardrailDomainAuthAuthZ
	name, _ = ResolveNextSkill(change)
	if name != SkillWaveOrchestration {
		t.Errorf("expected %s for guardrail domain, got %s", SkillWaveOrchestration, name)
	}
}

func TestResolveNextSkill_S3Review(t *testing.T) {
	t.Parallel()
	change := model.Change{CurrentState: model.StateS3Review}
	name, state := ResolveNextSkill(change)
	if name != SkillSpecComplianceReview {
		t.Errorf("expected %s, got %s", SkillSpecComplianceReview, name)
	}
	if state != string(model.StateS3Review) {
		t.Errorf("expected %s, got %s", model.StateS3Review, state)
	}
}

func TestResolveNextSkill_S4Verify(t *testing.T) {
	t.Parallel()
	change := model.Change{CurrentState: model.StateS4Verify}
	name, state := ResolveNextSkill(change)
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
	name, state := ResolveNextSkill(change)
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
		name, _ := ResolveNextSkill(change)
		if name != "" {
			t.Errorf("ResolveNextSkill(%v, S1_PLAN/bundle) = %q, want empty", needsDiscovery, name)
		}
	}
}
