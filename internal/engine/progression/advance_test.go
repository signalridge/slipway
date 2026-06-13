package progression

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

func hasAdvanceReasonCode(reasons []model.ReasonCode, code string) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func hasAdvanceReasonDetail(reasons []model.ReasonCode, code, detail string) bool {
	for _, reason := range reasons {
		if reason.Code == code && reason.Detail == detail {
			return true
		}
	}
	return false
}

func hasSideEffect(sideEffects []SideEffect, kind string) bool {
	for _, sideEffect := range sideEffects {
		if sideEffect.Kind == kind {
			return true
		}
	}
	return false
}

func TestAdvance_NoChangeFile(t *testing.T) {
	t.Parallel()
	_, err := Advance("/tmp/nonexistent", "bogus-slug")
	if err == nil {
		t.Fatal("expected error for missing change file")
	}
}

func TestAdvance_DispatchS1Plan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create minimal config so governed advance can load it.
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("admission-test")
	// NewChange defaults to S1_PLAN/bundle.
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	// Should dispatch to AdvanceGoverned for S1_PLAN routing.
	_, advErr := Advance(root, "admission-test")
	// We don't assert specific success/failure here — just that it didn't
	// return a "not found" error, confirming dispatch happened.
	if advErr != nil && errors.Is(advErr, os.ErrNotExist) {
		t.Fatalf("unexpected not-exist error after saving change: %v", advErr)
	}
}

func TestAdvance_DispatchGoverned(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	change := model.NewChange("governed-test")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	// Should dispatch to AdvanceGoverned (L2 path).
	_, advErr := Advance(root, "governed-test")
	// We don't assert specific success/failure here — just that it didn't
	// return a "not found" error, confirming dispatch happened.
	if advErr != nil && errors.Is(advErr, os.ErrNotExist) {
		t.Fatalf("unexpected not-exist error after saving change: %v", advErr)
	}
}

func TestComputeNextGovernedState_NoNextState(t *testing.T) {
	t.Parallel()
	change := model.Change{
		CurrentState: model.StateDone,
	}
	_, err := ComputeNextGovernedState(change)
	if err == nil {
		t.Fatal("expected error for no next state from DONE")
	}
	if !errors.Is(err, ErrNoNextState) {
		t.Fatalf("expected ErrNoNextState, got %v", err)
	}
}

func TestComputeNextGovernedState_Valid(t *testing.T) {
	t.Parallel()
	change := model.Change{
		CurrentState: model.StateS1Plan,
		PlanSubStep:  model.PlanSubStepAudit,
	}
	next, err := ComputeNextGovernedState(change)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next == "" {
		t.Fatal("expected non-empty next state")
	}
}

func TestCheckGateWithIteration_MissingEvidence(t *testing.T) {
	t.Parallel()
	change := model.Change{
		Slug: "test-slug",
	}
	passingSkills := map[string]model.VerificationRecord{}
	result := CheckGateWithIteration("/tmp/nonexistent", change, passingSkills, 3)
	if !result.Blocked {
		t.Fatal("expected blocked when plan audit evidence is missing")
	}
	found := false
	for _, b := range result.Blockers {
		if b.Code == "plan_audit_evidence_missing" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected plan_audit_evidence_missing blocker, got %v", result.Blockers)
	}
	if change.PlanAuditIterations != 0 {
		t.Fatalf("expected CheckGateWithIteration to keep input unchanged, got %d", change.PlanAuditIterations)
	}
	sideEffects, err := ApplyPlanGateResult(&change, result)
	if err != nil {
		t.Fatalf("apply plan gate result: %v", err)
	}
	if change.PlanAuditIterations != 1 {
		t.Fatalf("expected PlanAuditIterations=1, got %d", change.PlanAuditIterations)
	}
	if len(sideEffects) == 0 {
		t.Fatal("expected explicit side effects when applying plan gate result")
	}
	if strings.TrimSpace(change.EvidenceRefs[planAuditLastCheckerFeedbackKey]) == "" {
		t.Fatal("expected checker feedback to be recorded in evidence refs")
	}
}

func TestCheckGateWithIterationDetectsStalledFeedback(t *testing.T) {
	t.Parallel()
	change := model.Change{
		Slug: "stalled-plan-audit",
	}
	passingSkills := map[string]model.VerificationRecord{}

	first := CheckGateWithIteration("/tmp/nonexistent", change, passingSkills, 3)
	sideEffects, err := ApplyPlanGateResult(&change, first)
	if err != nil {
		t.Fatalf("apply first result: %v", err)
	}
	if len(sideEffects) == 0 {
		t.Fatal("expected side effects after first failed audit")
	}

	second := CheckGateWithIteration("/tmp/nonexistent", change, passingSkills, 3)
	if !second.Stalled {
		t.Fatal("expected second unchanged failed audit to be marked stalled")
	}
	if !hasAdvanceReasonCode(second.Blockers, "plan_audit_stalled") {
		t.Fatalf("expected plan_audit_stalled blocker, got %v", second.Blockers)
	}
	if !hasAdvanceReasonCode(second.Blockers, "plan_checker_loop_terminated") {
		t.Fatalf("expected loop termination blocker, got %v", second.Blockers)
	}
	expectedRecovery := "consider `slipway pivot --reroute` or manual plan revision"
	if !hasAdvanceReasonDetail(second.Blockers, "plan_audit_budget_exhausted", expectedRecovery) {
		t.Fatalf("expected valid reroute recovery detail, got %v", second.Blockers)
	}
	for _, blocker := range second.Blockers {
		if strings.Contains(blocker.Detail, "--kind") {
			t.Fatalf("blocker detail must not recommend invalid pivot --kind syntax: %v", second.Blockers)
		}
	}
	if !hasAdvanceReasonDetail(second.Blockers, "plan_audit_iteration", "2/3") {
		t.Fatalf("expected second iteration detail, got %v", second.Blockers)
	}
}

func TestAdvanceGoverned_BlocksWhenBundleMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "bundle-missing"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	summary, err := AdvanceGoverned(root, slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked summary, got %+v", summary)
	}
	if len(summary.Blockers) == 0 {
		t.Fatalf("expected missing bundle blockers, got %+v", summary)
	}
}

func TestAdvanceGovernedWritesLifecycleEvent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("advance-event-log")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "simple"
	change.WorkflowPreset = model.WorkflowPresetLight
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	intent := "# Intent\n\n## Summary\nTest\n\n## In Scope\nAdd logging\n\n## Out of Scope\nNothing\n\n## Acceptance Signals\nTests pass\n"
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intent), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Now().UTC(),
		RunVersion: 0,
	})

	summary, err := AdvanceGoverned(root, change.Slug, AdvanceOptions{Command: "run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "advanced" {
		t.Fatalf("expected advanced, got %+v", summary)
	}

	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	events, err := state.ReadLifecycleEvents(root, reloaded)
	if err != nil {
		t.Fatalf("read lifecycle events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected transition and skill evidence lifecycle events, got %d: %+v", len(events), events)
	}
	event := events[0]
	if event.EventType != "state.transitioned" {
		t.Fatalf("expected state.transitioned event, got %q", event.EventType)
	}
	if event.Command != "run" {
		t.Fatalf("expected run command, got %q", event.Command)
	}
	if event.BeforeState != model.StateS0Intake || event.AfterState != model.StateS0Intake {
		t.Fatalf("expected S0 substep transition, got before=%s after=%s", event.BeforeState, event.AfterState)
	}
	if event.BeforeSubStep != string(model.IntakeSubStepClarify) || event.AfterSubStep != string(model.IntakeSubStepConfirm) {
		t.Fatalf("unexpected substeps: before=%q after=%q", event.BeforeSubStep, event.AfterSubStep)
	}
	evidenceEvent := events[1]
	if evidenceEvent.EventType != "skill.evidence_recorded" {
		t.Fatalf("expected skill.evidence_recorded event, got %q", evidenceEvent.EventType)
	}
	if evidenceEvent.SkillID != SkillIntakeClarification {
		t.Fatalf("expected intake skill evidence event, got %q", evidenceEvent.SkillID)
	}
	if evidenceEvent.EvidenceRefs[SkillIntakeClarification] == "" {
		t.Fatalf("expected evidence ref for %s, got %+v", SkillIntakeClarification, evidenceEvent.EvidenceRefs)
	}
	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	if err != nil {
		t.Fatalf("load evidence digests: %v", err)
	}
	if digests.Skills[SkillIntakeClarification].Inputs["intent.md"] == "" {
		t.Fatalf("expected intake-clarification digest for intent.md, got %+v", digests.Skills[SkillIntakeClarification])
	}
}

func TestAdvanceGoverned_UsesLightPlanAuditBudget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("light-plan-budget")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	// Manually create bundle files (light preset excludes assurance.md).
	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	for _, file := range []string{"intent.md", "requirements.md", "decision.md", "tasks.md"} {
		if err := os.WriteFile(filepath.Join(bundleDir, file), []byte("# "+file+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}
	evidenceAt := time.Now().UTC()
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  evidenceAt,
		RunVersion: 0,
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked summary, got %+v", summary)
	}
	found := false
	for _, blocker := range summary.Blockers {
		if blocker.Code == "plan_audit_iteration" && blocker.Detail == "1/2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected light preset to use 2-step plan audit budget, got %+v", summary.Blockers)
	}
	if summary.Reason != "plan_audit_feedback_recorded" {
		t.Fatalf("expected structured blocked reason, got %q", summary.Reason)
	}
	if summary.FromSubStep != string(model.PlanSubStepAudit) || summary.ToSubStep != string(model.PlanSubStepAudit) {
		t.Fatalf("expected audit substep to remain explicit, got from=%q to=%q", summary.FromSubStep, summary.ToSubStep)
	}
	if len(summary.SideEffects) == 0 {
		t.Fatalf("expected side effects for plan audit feedback write, got %+v", summary)
	}

	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.PlanAuditIterations != 1 {
		t.Fatalf("expected persisted plan audit iteration, got %d", reloaded.PlanAuditIterations)
	}
	if strings.TrimSpace(reloaded.EvidenceRefs[planAuditLastCheckerFeedbackKey]) == "" {
		t.Fatal("expected persisted plan checker feedback")
	}
}

func TestAdvanceIntake_ClarifyBlocksOnMissingSections(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("intake-clarify-test")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "simple"
	change.WorkflowPreset = model.WorkflowPresetLight
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	// Create intent.md with only a summary (missing In Scope, Acceptance Signals)
	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte("# Intent\n\n## Summary\nTest change\n"), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}

	// Provide skill evidence so the test reaches section validation
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Now().UTC(),
		RunVersion: 0,
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked, got %+v", summary)
	}
	found := false
	for _, b := range summary.Blockers {
		if b.Code == "intake_clarification_incomplete" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected intake_clarification_incomplete blocker, got %v", summary.Blockers)
	}
}

func TestAdvanceIntake_ClarifyToConfirm(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("intake-confirm-test")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "simple"
	change.WorkflowPreset = model.WorkflowPresetLight
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	intent := "# Intent\n\n## Summary\nTest\n\n## In Scope\nAdd logging\n\n## Out of Scope\nNothing\n\n## Acceptance Signals\nTests pass\n"
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intent), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}

	// Write intake-clarification evidence so skill check passes.
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Now().UTC(),
		RunVersion: 0,
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "advanced" {
		t.Fatalf("expected advanced, got %+v", summary)
	}
	if !strings.Contains(summary.Message, "confirm") {
		t.Fatalf("expected advance to confirm, got message: %s", summary.Message)
	}
	if summary.ToSubStep != string(model.IntakeSubStepConfirm) {
		t.Fatalf("expected ToSubStep=%s, got %s", model.IntakeSubStepConfirm, summary.ToSubStep)
	}
	if summary.Reason == "" {
		t.Fatal("expected non-empty Reason for structured advance")
	}

	// Reload and verify substep changed
	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.IntakeSubStep != model.IntakeSubStepConfirm {
		t.Fatalf("expected IntakeSubStepConfirm, got %s", reloaded.IntakeSubStep)
	}
}

func TestAdvanceIntake_OpenQuestionsUseChecklistStructure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		questions  string
		wantSub    model.IntakeSubStep
		wantReason string
	}{
		{
			name: "explicit none advances to confirm",
			questions: `## Open Questions
(none)
`,
			wantSub:    model.IntakeSubStepConfirm,
			wantReason: "clarification_complete",
		},
		{
			name: "explicit none bullet advances to confirm",
			questions: `## Open Questions
- None.
`,
			wantSub:    model.IntakeSubStepConfirm,
			wantReason: "clarification_complete",
		},
		{
			name: "resolved checklist advances to confirm",
			questions: `## Open Questions
- [x] Installer path resolved by research.
`,
			wantSub:    model.IntakeSubStepConfirm,
			wantReason: "clarification_complete",
		},
		{
			name: "unchecked checklist advances to research",
			questions: `## Open Questions
- [ ] Which installer path should be documented?
`,
			wantSub:    model.IntakeSubStepResearch,
			wantReason: "open_questions_detected",
		},
		{
			// Contract: only an unchecked `- [ ]` blocks. A bare bullet is
			// documentation, so intake advances; promoting it to a real question is
			// the intake-clarification skill's responsibility, not the engine's.
			name: "plain bullet is documentation, advances to confirm",
			questions: `## Open Questions
- Which docs build command should be used?
`,
			wantSub:    model.IntakeSubStepConfirm,
			wantReason: "clarification_complete",
		},
		{
			name: "plain prose advances to confirm",
			questions: `## Open Questions
Need to decide which adapter layout should be documented.
`,
			wantSub:    model.IntakeSubStepConfirm,
			wantReason: "clarification_complete",
		},
		{
			// Regression for #104: a sentinel followed by an explanatory clause is
			// prose, not an open question, and must advance instead of detouring to
			// research.
			name: "sentinel prose advances to confirm (#104)",
			questions: `## Open Questions
None requiring research — the page model is already specified.
`,
			wantSub:    model.IntakeSubStepConfirm,
			wantReason: "clarification_complete",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
				t.Fatalf("save config: %v", err)
			}

			change := model.NewChange("open-questions-" + strings.ReplaceAll(tt.name, " ", "-"))
			change.CurrentState = model.StateS0Intake
			change.IntakeSubStep = model.IntakeSubStepClarify
			change.PlanSubStep = model.PlanSubStepNone
			change.ComplexityLevel = "complex"
			change.WorkflowPreset = model.WorkflowPresetStandard
			if err := state.SaveChange(root, change); err != nil {
				t.Fatalf("save change: %v", err)
			}

			bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
			if err := os.MkdirAll(bundleDir, 0o755); err != nil {
				t.Fatalf("mkdir bundle: %v", err)
			}
			intent := `# Intent

## Summary
Test

## In Scope
Add docs

## Out of Scope
Runtime changes

## Acceptance Signals
Docs build

` + tt.questions
			if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intent), 0o644); err != nil {
				t.Fatalf("write intent.md: %v", err)
			}

			writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, model.VerificationRecord{
				Verdict:    model.VerificationVerdictPass,
				Timestamp:  time.Now().UTC(),
				RunVersion: 0,
			})

			summary, err := AdvanceGoverned(root, change.Slug)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if summary.Action != "advanced" {
				t.Fatalf("expected advanced, got %+v", summary)
			}
			if summary.ToSubStep != string(tt.wantSub) {
				t.Fatalf("expected ToSubStep=%s, got %s", tt.wantSub, summary.ToSubStep)
			}
			if summary.Reason != tt.wantReason {
				t.Fatalf("expected Reason=%s, got %s", tt.wantReason, summary.Reason)
			}
		})
	}
}

// Routing out of clarify on a blocking open question must not be silent (#104):
// the note names the specific unchecked entry and the resolve escape hatch.
func TestOpenQuestionsRoutingNoteNamesEntryAndEscapeHatch(t *testing.T) {
	t.Parallel()

	note := openQuestionsRoutingNote("## Open Questions\n- [ ] Which token TTL?\n")
	if !strings.Contains(note, "Which token TTL?") {
		t.Fatalf("note should name the blocking entry, got %q", note)
	}
	if !strings.Contains(note, "- [x]") {
		t.Fatalf("note should give the resolve escape hatch, got %q", note)
	}
	if got := openQuestionsRoutingNote("## Open Questions\n(none)\n"); got != "" {
		t.Fatalf("note should be empty when nothing blocks, got %q", got)
	}
}

func TestAdvanceIntakeResearchDiscoveryEntersS1ResearchAndClearsStaleEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("s0-research-enters-s1")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepResearch
	change.PlanSubStep = model.PlanSubStepNone
	change.NeedsDiscovery = true
	change.ComplexityLevel = "complex"
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	intent := `# Intent

## Summary
Test S0 research handoff.

## In Scope
Clarify discovery before planning.

## Out of Scope
Skip direct execution.

## Acceptance Signals
S1 research must require fresh research evidence.

## Open Questions
- [ ] Which implementation path should be selected?
`
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intent), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}
	// Reaching S0_INTAKE/research implies intake clarification already passed;
	// the machine-only research advance stays fail-closed on that evidence.
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Now().UTC(),
		RunVersion: 0,
		References: []string{"s0:clarify"},
	})
	writeVerificationForTest(t, root, change.Slug, SkillResearchOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  change.CreatedAt,
		RunVersion: 0,
		References: []string{"s0:stale"},
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "advanced" {
		t.Fatalf("expected advanced, got %+v", summary)
	}
	if summary.ToState != model.StateS1Plan || summary.ToSubStep != string(model.PlanSubStepResearch) {
		t.Fatalf("expected S1_PLAN/research, got state=%s substep=%s", summary.ToState, summary.ToSubStep)
	}
	if summary.Reason != "open_questions_require_discovery" {
		t.Fatalf("expected open_questions_require_discovery, got %s", summary.Reason)
	}
	if !hasSideEffect(summary.SideEffects, "cleared_verification") {
		t.Fatalf("expected stale research verification to be cleared, got %+v", summary.SideEffects)
	}
	if !hasSideEffect(summary.SideEffects, "deferred_research_authoring") {
		t.Fatalf("expected research artifact authoring to be deferred, got %+v", summary.SideEffects)
	}
	if _, err := os.Stat(filepath.Join(bundleDir, "research.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected research.md to remain absent until research-orchestration authors it, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(bundleDir, "verification", SkillResearchOrchestration+".yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected stale research verification to be removed, err=%v", err)
	}

	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.CurrentState != model.StateS1Plan || reloaded.PlanSubStep != model.PlanSubStepResearch {
		t.Fatalf("expected reloaded S1_PLAN/research, got state=%s substep=%s", reloaded.CurrentState, reloaded.PlanSubStep)
	}
	if reloaded.IntakeSubStep != model.IntakeSubStepNone {
		t.Fatalf("expected intake substep cleared, got %s", reloaded.IntakeSubStep)
	}

	blocked, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected second advance error: %v", err)
	}
	if blocked.Action != "blocked" {
		t.Fatalf("expected missing S1 research evidence to block, got %+v", blocked)
	}
	if !hasAdvanceReasonDetail(blocked.Blockers, "required_skill_missing", SkillResearchOrchestration) {
		t.Fatalf("expected missing research-orchestration blocker, got %+v", blocked.Blockers)
	}
}

func TestAdvanceIntakeResearchBlocksWhenIntakeClarificationEvidenceMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("s0-research-missing-intake-evidence")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepResearch
	change.PlanSubStep = model.PlanSubStepNone
	change.NeedsDiscovery = true
	change.ComplexityLevel = "complex"
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	intent := `# Intent

## Summary
Test S0 research fail-closed on missing intake evidence.

## In Scope
Clarify discovery before planning.

## Out of Scope
Skip direct execution.

## Acceptance Signals
Advance must block without intake-clarification evidence.

## Open Questions
- [ ] Which implementation path should be selected?
`
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intent), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}
	// Intentionally omit intake-clarification evidence: the machine-only research
	// advance must not bypass the intake gate even though no skill is surfaced.

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected missing intake-clarification evidence to block, got %+v", summary)
	}
	if !hasAdvanceReasonDetail(summary.Blockers, "required_skill_missing", SkillIntakeClarification) {
		t.Fatalf("expected missing intake-clarification blocker, got %+v", summary.Blockers)
	}

	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.CurrentState != model.StateS0Intake || reloaded.IntakeSubStep != model.IntakeSubStepResearch {
		t.Fatalf("expected change to remain at S0_INTAKE/research, got state=%s substep=%s", reloaded.CurrentState, reloaded.IntakeSubStep)
	}
}

func TestSectionNonEmptyPrefersCanonicalIntentSectionOverSummarySourceDocument(t *testing.T) {
	t.Parallel()

	content := `# Intent

## Summary
Source document excerpt.

## Acceptance Signals
Source document says verification exists.

## Guardrail Domains
auth_authz

## In Scope
Update session timeout handling.

## Out of Scope
No session store changes.

## Acceptance Signals
<!-- What verifiable signals indicate completion -->
`

	if sectionNonEmpty(content, "## Acceptance Signals") {
		t.Fatal("expected canonical empty Acceptance Signals section to win over the earlier summary copy")
	}
}

func TestAdvanceGoverned_PresetPendingBlocksBeforeIntakeAdvance(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("preset-pending-intake")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "critical"
	change.SuggestedWorkflowPreset = model.WorkflowPresetStrict
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	intent := "# Intent\n\n## Summary\nTest\n\n## In Scope\nAdd logging\n\n## Out of Scope\nNothing\n\n## Acceptance Signals\nTests pass\n"
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intent), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  change.CreatedAt,
		RunVersion: 0,
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked, got %+v", summary)
	}
	if len(summary.Blockers) != 1 || summary.Blockers[0].Code != "preset_confirmation_required" {
		t.Fatalf("expected preset_confirmation_required blocker, got %v", summary.Blockers)
	}

	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.CurrentState != model.StateS0Intake {
		t.Fatalf("expected CurrentState=S0_INTAKE, got %s", reloaded.CurrentState)
	}
	if reloaded.IntakeSubStep != model.IntakeSubStepClarify {
		t.Fatalf("expected IntakeSubStepClarify, got %s", reloaded.IntakeSubStep)
	}
}

func TestAdvanceGoverned_SyncDoesNotRewriteUnchangedChangeAuthority(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("sync-no-change-rewrite")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.WorkflowPreset = model.WorkflowPresetLight
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	if err := artifact.ScaffoldGovernedBundleForChange(root, change, ""); err != nil {
		t.Fatalf("scaffold bundle: %v", err)
	}
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		t.Fatalf("bundle dir: %v", err)
	}
	// requirements.md/decision.md are authored by the host skill, not scaffolded
	// (issue #119); write real ones so the lifecycle does not fail closed on
	// missing_required_artifact before reaching the task-evidence check.
	if err := os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

### Requirement: Change authority
REQ-001: Sync MUST NOT rewrite unchanged change authority during advance.
`), 0o644); err != nil {
		t.Fatalf("write requirements.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision

## Alternatives Considered
- A: Rewrite change.yaml on every sync.
- B: Skip the write when authority is unchanged.

## Selected Approach
Use B.

## Interfaces and Data Flow
Advance reconciles change authority and only persists it on a real change.

## Rollout and Rollback
Limited to the sync write path and revertible directly.

## Risk
Low; an unchanged authority simply skips the persist.
`), 0o644); err != nil {
		t.Fatalf("write decision.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`task-a`"+` preserve change authority
  - target_files: ["cmd/next.go"]
  - task_kind: code
`), 0o644); err != nil {
		t.Fatalf("write tasks.md: %v", err)
	}
	if _, err := state.MaterializeWavePlan(root, change); err != nil {
		t.Fatalf("materialize wave plan: %v", err)
	}

	changePath := state.BundleChangeFilePath(root, change.Slug)
	before := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	if err := os.Chtimes(changePath, before, before); err != nil {
		t.Fatalf("chtimes change.yaml: %v", err)
	}

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  before.Add(time.Minute),
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, change.Slug, SkillWaveOrchestration, record)

	taskEvidence := []byte(`{
  "task_id": "task-a",
  "run_summary_version": 1,
  "task_kind": "code",
  "verdict": "fail",
  "blockers": [],
  "evidence_ref": "test:task-a",
  "captured_at": "2026-04-06T10:01:00Z",
  "freshness_inputs": {
    "change_id": "sync-no-change-rewrite",
    "run_summary_version": 1,
    "task_id": "task-a"
  }
}`)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, change.Slug), "task-a.json")
	if err := os.MkdirAll(filepath.Dir(taskPath), 0o755); err != nil {
		t.Fatalf("mkdir task dir: %v", err)
	}
	if err := os.WriteFile(taskPath, taskEvidence, 0o644); err != nil {
		t.Fatalf("write task evidence: %v", err)
	}

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked summary, got %+v", summary)
	}
	if !hasAdvanceReasonDetail(summary.Blockers, "non_pass_task", "task-a") {
		t.Fatalf("expected non_pass_task blocker, got %v", summary.Blockers)
	}

	info, err := os.Stat(changePath)
	if err != nil {
		t.Fatalf("stat change.yaml: %v", err)
	}
	if !info.ModTime().Equal(before) {
		t.Fatalf("expected unchanged change.yaml mtime %s, got %s", before, info.ModTime())
	}

	if _, err := state.LoadExecutionSummary(root, change.Slug); err != nil {
		t.Fatalf("load execution summary: %v", err)
	}
}

func TestAdvanceGoverned_S2PassesWithCompleteRuntimeTaskEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("s2-complete-runtime-evidence")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.WorkflowPreset = model.WorkflowPresetLight
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	if err := artifact.ScaffoldGovernedBundleForChange(root, change, ""); err != nil {
		t.Fatalf("scaffold bundle: %v", err)
	}
	// requirements.md/decision.md are authored by the host skill, not scaffolded
	// (issue #119); write real ones so S2 does not fail closed on
	// missing_required_artifact before reaching the task-evidence check.
	s2BundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		t.Fatalf("bundle dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s2BundleDir, "requirements.md"), []byte(`# Requirements

### Requirement: Runtime task evidence
REQ-001: S2 MUST advance to S3 when every wave task carries complete passing runtime evidence.

#### Scenario: Complete runtime evidence advances
GIVEN all wave tasks have fresh passing evidence
WHEN governed advance runs at S2
THEN the change advances to S3.
`), 0o644); err != nil {
		t.Fatalf("write requirements.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s2BundleDir, "decision.md"), []byte(`# Decision

## Alternatives Considered
- A: Require manual recovery before advancing.
- B: Advance directly when runtime evidence is complete.

## Selected Approach
Use B.

## Interfaces and Data Flow
S2 advance reads per-task runtime evidence and wave-orchestration verification.

## Rollout and Rollback
Limited to the S2 advance path and revertible directly.

## Risk
Low; incomplete or stale evidence still blocks the advance.
`), 0o644); err != nil {
		t.Fatalf("write decision.md: %v", err)
	}

	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [ ] `+"`t-01`"+` add regression test
  - target_files: ["internal/engine/progression/advance_test.go"]
  - task_kind: test

- [ ] `+"`t-02`"+` implement change
  - depends_on: [t-01]
  - target_files: ["internal/engine/progression/advance_governed.go"]
  - task_kind: code

- [ ] `+"`t-03`"+` verify behavior
  - depends_on: [t-02]
  - target_files: ["internal/engine/progression"]
  - task_kind: verification
`)

	base := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	taskEvidence := []struct {
		taskID       string
		taskKind     model.TaskKind
		changedFiles []string
		targetFiles  []string
		capturedAt   time.Time
	}{
		{
			taskID:       "t-01",
			taskKind:     model.TaskKindTest,
			changedFiles: []string{"internal/engine/progression/advance_test.go"},
			targetFiles:  []string{"internal/engine/progression/advance_test.go"},
			capturedAt:   base.Add(time.Minute),
		},
		{
			taskID:       "t-02",
			taskKind:     model.TaskKindCode,
			changedFiles: []string{"internal/engine/progression/advance_governed.go"},
			targetFiles:  []string{"internal/engine/progression/advance_governed.go"},
			capturedAt:   base.Add(2 * time.Minute),
		},
		{
			taskID:      "t-03",
			taskKind:    model.TaskKindVerification,
			targetFiles: []string{"internal/engine/progression"},
			capturedAt:  base.Add(3 * time.Minute),
		},
	}
	for _, task := range taskEvidence {
		payload := map[string]any{
			"task_id":             task.taskID,
			"run_summary_version": 1,
			"task_kind":           task.taskKind,
			"verdict":             model.TaskVerdictPass,
			"changed_files":       task.changedFiles,
			"target_files":        task.targetFiles,
			"evidence_ref":        "test:" + task.taskID,
			"captured_at":         task.capturedAt.Format(time.RFC3339Nano),
			"freshness_inputs": expectedTaskFreshnessInputsForWavePlan(
				t,
				root,
				change,
				1,
				task.taskID,
			),
		}
		raw, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s task evidence: %v", task.taskID, err)
		}
		taskPath := filepath.Join(state.EvidenceTasksDir(root, change.Slug), task.taskID+".json")
		if err := os.MkdirAll(filepath.Dir(taskPath), 0o755); err != nil {
			t.Fatalf("mkdir task dir: %v", err)
		}
		if err := os.WriteFile(taskPath, raw, 0o644); err != nil {
			t.Fatalf("write %s task evidence: %v", task.taskID, err)
		}
	}
	writeVerificationForTest(t, root, change.Slug, SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  base.Add(4 * time.Minute),
		RunVersion: 1,
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "advanced" || summary.ToState != model.StateS3Review {
		t.Fatalf("expected S2 to advance to S3 without stale recovery, got %+v", summary)
	}
	if _, err := state.LoadExecutionSummary(root, change.Slug); err != nil {
		t.Fatalf("load execution summary: %v", err)
	}
	if _, err := state.LoadVerification(root, change.Slug, SkillWaveOrchestration); err != nil {
		t.Fatalf("load wave-orchestration verification: %v", err)
	}
}

func TestAdvanceGoverned_AppliesWorktreePreflightBeforeRequiredActionBlockers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitRepoForValidationTests(t, root)
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("worktree-preflight-before-actions")
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.WorkflowPreset = model.WorkflowPresetStrict
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	if err := artifact.ScaffoldGovernedBundleForChange(root, change, model.WorkflowPresetStrict); err != nil {
		t.Fatalf("scaffold bundle: %v", err)
	}
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		t.Fatalf("bundle dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

### Requirement: Worktree preflight
REQ-001: The change MUST consume worktree preflight evidence before required-action blockers deadlock execution.
`), 0o644); err != nil {
		t.Fatalf("write requirements: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision

## Alternatives Considered
- A: Block before reading evidence.
- B: Consume preflight evidence before action blockers.

## Selected Approach
Use B.

## Interfaces and Data Flow
S2 execution reads worktree-preflight verification and persists worktree metadata.

## Rollout and Rollback
The change is limited to S2 preflight ordering and can be reverted directly.

## Risk
Low; failed or missing evidence still returns the existing metadata blocker.
`), 0o644); err != nil {
		t.Fatalf("write decision: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "research.md"), []byte(`# Research

## Alternatives Considered
- A: Keep existing ordering.
- B: Consume worktree evidence before required-action blockers.

## Unknowns
- None.

## Assumptions
- Worktree preflight evidence is already validated by DeriveWorktreeBlockers.

## Canonical References
- internal/engine/progression/advance_governed.go
`), 0o644); err != nil {
		t.Fatalf("write research: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` exercise worktree preflight ordering
  - depends_on: []
  - target_files: ["a.go", "b.go", "c.go", "d.go", "e.go", "f.go", "g.go", "h.go", "i.go", "j.go", "k.go"]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}

	worktreeRoot := filepath.Join(t.TempDir(), change.Slug)
	branch := "feat/" + change.Slug
	runGitForValidationTests(t, root, "worktree", "add", worktreeRoot, "-b", branch)
	writeVerificationForTest(t, root, change.Slug, SkillWorktreePreflight, model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Timestamp: time.Now().UTC(),
		References: []string{
			"worktree_path:" + worktreeRoot,
			"worktree_branch:" + branch,
			"baseline_verify_cmd:go test ./...",
		},
	})

	summary, err := AdvanceGoverned(root, change.Slug, AdvanceOptions{Command: "run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasAdvanceReasonCode(summary.Blockers, state.WorktreeReasonMetadataRequired) {
		t.Fatalf("worktree preflight evidence was not consumed before required-action blockers: %+v", summary.Blockers)
	}
	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("load change: %v", err)
	}
	if reloaded.WorktreePath == "" || reloaded.WorktreeBranch != branch {
		t.Fatalf("expected worktree metadata to be persisted, got path=%q branch=%q", reloaded.WorktreePath, reloaded.WorktreeBranch)
	}
}

func TestAdvanceIntake_ConfirmToS1Plan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("intake-to-plan-test")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepConfirm
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "simple"
	change.WorkflowPreset = model.WorkflowPresetLight
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	intent := "# Intent\n\n## Summary\nTest\n\n## Approved Summary\nConfirmed: add logging to service layer\n"
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intent), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "advanced" {
		t.Fatalf("expected advanced, got %+v", summary)
	}
	if summary.ToState != model.StateS1Plan {
		t.Fatalf("expected transition to S1_PLAN, got %s", summary.ToState)
	}

	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.CurrentState != model.StateS1Plan {
		t.Fatalf("expected S1_PLAN, got %s", reloaded.CurrentState)
	}
	if reloaded.IntakeSubStep != model.IntakeSubStepNone {
		t.Fatalf("expected IntakeSubStep cleared, got %s", reloaded.IntakeSubStep)
	}
}

func TestAdvanceIntake_ConfirmToS1PlanResearchDefersResearchAuthoring(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("intake-to-research-artifact")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepConfirm
	change.PlanSubStep = model.PlanSubStepNone
	change.NeedsDiscovery = true
	change.ComplexityLevel = "complex"
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	intent := "# Intent\n\n## Summary\nTest\n\n## Approved Summary\nConfirmed: add governed workflow fixes\n"
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intent), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}
	writeVerificationForTest(t, root, change.Slug, SkillResearchOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  change.CreatedAt,
		RunVersion: 0,
		References: []string{"s0-confirm:stale"},
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "advanced" {
		t.Fatalf("expected advanced, got %+v", summary)
	}
	if summary.ToState != model.StateS1Plan || summary.ToSubStep != string(model.PlanSubStepResearch) {
		t.Fatalf("expected transition to S1_PLAN/research, got state=%s substep=%s", summary.ToState, summary.ToSubStep)
	}
	if !hasSideEffect(summary.SideEffects, "deferred_research_authoring") {
		t.Fatalf("expected deferred_research_authoring side effect, got %+v", summary.SideEffects)
	}
	if !hasSideEffect(summary.SideEffects, "cleared_verification") {
		t.Fatalf("expected stale research verification to be cleared, got %+v", summary.SideEffects)
	}
	if _, err := os.Stat(filepath.Join(bundleDir, "research.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected research.md to remain absent until research-orchestration authors it, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(bundleDir, "verification", SkillResearchOrchestration+".yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected stale research verification to be removed, err=%v", err)
	}

	blocked, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected second advance error: %v", err)
	}
	if blocked.Action != "blocked" {
		t.Fatalf("expected missing S1 research evidence to block, got %+v", blocked)
	}
	if !hasAdvanceReasonDetail(blocked.Blockers, "required_skill_missing", SkillResearchOrchestration) {
		t.Fatalf("expected missing research-orchestration blocker, got %+v", blocked.Blockers)
	}
}

func TestAdvanceGoverned_GScopeDoesNotBlockOnEmptyWorktreeAtS1Research(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("gscope-worktree-test")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepResearch
	change.IntakeSubStep = model.IntakeSubStepNone
	change.NeedsDiscovery = true
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	// WorktreePath intentionally empty — worktree is not yet created at S1_PLAN.
	change.WorktreePath = ""
	change.WorktreeBranch = ""
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	// Write core bundle artifacts so bundle check passes.
	for _, file := range []string{"intent.md", "requirements.md", "tasks.md", "assurance.md", "decision.md"} {
		if err := os.WriteFile(filepath.Join(bundleDir, file), []byte("# "+file+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}
	// Write a valid research.md so G_scope structure check passes.
	research := `## Alternatives Considered
### Option A
Approach A

### Selected Direction
Use Option A

## Unknowns
None critical.

## Assumptions
Standard env.

## Canonical References
Internal docs.
`
	if err := os.WriteFile(filepath.Join(bundleDir, "research.md"), []byte(research), 0o644); err != nil {
		t.Fatalf("write research.md: %v", err)
	}

	// Write research-orchestration evidence so skill check passes.
	writeVerificationForTest(t, root, change.Slug, SkillResearchOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  change.CreatedAt,
		RunVersion: 0,
	})

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// G_scope should NOT block due to empty worktree; worktree gate is at S2_EXECUTE.
	// The change should advance from S1_PLAN/research to S1_PLAN/bundle.
	if summary.Action == "blocked" {
		for _, b := range summary.Blockers {
			if strings.Contains(b.Code, "worktree") || strings.Contains(b.Code, "dedicated_worktree") {
				t.Fatalf("G_scope must not block on missing worktree at S1_PLAN/research: %v", summary.Blockers)
			}
		}
	}
}

func TestEvaluateScopeGateReportsMissingResearchArtifact(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("missing-research-artifact")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepResearch
	change.IntakeSubStep = model.IntakeSubStepNone
	change.NeedsDiscovery = true
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	for _, file := range []string{"intent.md", "requirements.md", "tasks.md", "assurance.md", "decision.md"} {
		if err := os.WriteFile(filepath.Join(bundleDir, file), []byte("# "+file+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}

	evaluation, err := EvaluateScopeGate(root, change, map[string]model.VerificationRecord{
		SkillResearchOrchestration: {
			Verdict:    model.VerificationVerdictPass,
			Timestamp:  change.CreatedAt,
			RunVersion: 0,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluation.Status != model.GateStatusBlocked {
		t.Fatalf("expected missing research.md to block, got %+v", evaluation)
	}
	if !hasAdvanceReasonDetail(evaluation.ReasonCodes, "missing_required_artifact", "research.md") {
		t.Fatalf("expected missing_required_artifact:research.md blocker, got %+v", evaluation.ReasonCodes)
	}
	for _, blocker := range evaluation.ReasonCodes {
		if blocker.Code == "research_structure_invalid" {
			t.Fatalf("missing research.md must not be reported as a structure error: %+v", evaluation.ReasonCodes)
		}
	}
}

func TestAdvanceGoverned_BlocksWhenAssuranceContractInvalidAtReview(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	change := model.NewChange("assurance-invalid")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.ArtifactSchema = model.ArtifactSchemaCore
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	// Manually create bundle files (core schema: intent, requirements, tasks, assurance).
	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	for _, file := range []string{"intent.md", "requirements.md", "tasks.md"} {
		if err := os.WriteFile(filepath.Join(bundleDir, file), []byte("# "+file+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}

	// Write an incomplete assurance.md to trigger the structure validation blocker.
	if err := os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte("## Scope Summary\nIncomplete\n"), 0o644); err != nil {
		t.Fatalf("write assurance.md: %v", err)
	}

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked summary, got %+v", summary)
	}
	found := false
	for _, blocker := range summary.Blockers {
		if blocker.Code == "assurance_structure_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected assurance structure blocker, got %+v", summary.Blockers)
	}
}

func TestAdvanceGoverned_BlocksWhenExecutionSummaryHasSummaryLevelBlockers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("summary-level-blockers")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	for _, file := range []string{"intent.md", "requirements.md", "decision.md", "tasks.md"} {
		if err := os.WriteFile(filepath.Join(bundleDir, file), []byte("# "+file+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}

	if err := state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        change.CreatedAt.Add(time.Second),
		OverallVerdict:    model.ExecutionVerdictFail,
		OpenBlockers:      []model.ReasonCode{model.NewReasonCode("session_isolation_warning", "session_id=abc:shared_by=task-a,task-b")},
	}); err != nil {
		t.Fatalf("save execution summary: %v", err)
	}

	summary, err := AdvanceGoverned(root, change.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected blocked summary, got %+v", summary)
	}
	if !hasAdvanceReasonDetail(summary.Blockers, "session_isolation_warning", "session_id=abc:shared_by=task-a,task-b") {
		t.Fatalf("expected summary-level execution blocker, got %+v", summary.Blockers)
	}
}
