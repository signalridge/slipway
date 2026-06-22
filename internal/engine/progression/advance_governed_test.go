package progression

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// planAuditRecordWithOrigins builds a passing plan-audit verification record
// carrying the given plan_origin / audit_origin context-origin handles. An
// empty handle is omitted so the resulting record exercises the missing-handle
// path.
func planAuditRecordWithOrigins(planHandle, auditHandle string) model.VerificationRecord {
	refs := []string{}
	if planHandle != "" {
		refs = append(refs, model.PlanOriginReferencePrefix+planHandle)
	}
	if auditHandle != "" {
		refs = append(refs, model.AuditOriginReferencePrefix+auditHandle)
	}
	return model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		References: refs,
	}
}

// TestEvaluatePlanGate_PlanAuditSelfAuditEdge proves the local plan-audit
// self-audit edge: the plan-audit record must carry a well-formed plan_origin
// AND audit_origin, with audit_origin != plan_origin. Missing either or equal
// fails closed at error severity with plan_audit_origin_invalid on
// standard/strict, and is advisory (no blocker) on light.
//
// EvaluatePlanGate may still report unrelated bundle/checklist blockers for a
// bare temp root, so each subtest asserts specifically on the presence or
// absence of the plan_audit_origin_invalid edge rather than overall gate
// status.
func TestEvaluatePlanGate_PlanAuditSelfAuditEdge(t *testing.T) {
	t.Parallel()

	standardPolicy := governance.PresetPolicy{EffectivePreset: model.WorkflowPresetStandard}
	lightPolicy := governance.PresetPolicy{EffectivePreset: model.WorkflowPresetLight}

	t.Run("distinct plan and audit origins pass the self-audit edge", func(t *testing.T) {
		t.Parallel()
		change := model.Change{Slug: "plan-audit-distinct"}
		passingSkills := map[string]model.VerificationRecord{
			SkillPlanAudit: planAuditRecordWithOrigins("author-ctx", "auditor-ctx"),
		}

		eval := EvaluatePlanGate("/tmp/nonexistent-plan-gate-distinct", change, passingSkills, standardPolicy)
		if hasAdvanceReasonCode(eval.ReasonCodes, "plan_audit_origin_invalid") {
			t.Fatalf("distinct plan/audit origins must NOT raise plan_audit_origin_invalid, got %v", eval.ReasonCodes)
		}
	})

	t.Run("equal plan and audit origins fail closed as a blocker", func(t *testing.T) {
		t.Parallel()
		change := model.Change{Slug: "plan-audit-equal"}
		passingSkills := map[string]model.VerificationRecord{
			SkillPlanAudit: planAuditRecordWithOrigins("same-ctx", "same-ctx"),
		}

		eval := EvaluatePlanGate("/tmp/nonexistent-plan-gate-equal", change, passingSkills, standardPolicy)
		assertPlanAuditOriginBlocker(t, eval)
	})

	t.Run("missing audit_origin fails closed as a blocker", func(t *testing.T) {
		t.Parallel()
		change := model.Change{Slug: "plan-audit-missing"}
		passingSkills := map[string]model.VerificationRecord{
			SkillPlanAudit: planAuditRecordWithOrigins("author-ctx", ""),
		}

		eval := EvaluatePlanGate("/tmp/nonexistent-plan-gate-missing", change, passingSkills, standardPolicy)
		assertPlanAuditOriginBlocker(t, eval)
	})

	t.Run("light preset keeps the self-audit edge advisory", func(t *testing.T) {
		t.Parallel()
		change := model.Change{Slug: "plan-audit-light"}
		passingSkills := map[string]model.VerificationRecord{
			// Equal origins would be a blocker on standard/strict; light must
			// not raise the error blocker.
			SkillPlanAudit: planAuditRecordWithOrigins("same-ctx", "same-ctx"),
		}

		eval := EvaluatePlanGate("/tmp/nonexistent-plan-gate-light", change, passingSkills, lightPolicy)
		if hasAdvanceReasonCode(eval.ReasonCodes, "plan_audit_origin_invalid") {
			t.Fatalf("light preset must keep plan_audit_origin_invalid advisory (no blocker), got %v", eval.ReasonCodes)
		}
	})
}

// assertPlanAuditOriginBlocker requires the self-audit edge to be present as an
// error-severity blocker that flips the gate to blocked.
func assertPlanAuditOriginBlocker(t *testing.T, eval gate.GateEvaluation) {
	t.Helper()
	if eval.Status != model.GateStatusBlocked {
		t.Fatalf("expected G_plan blocked when the self-audit edge fails, got status %q", eval.Status)
	}
	for _, reason := range eval.ReasonCodes {
		if reason.Code != "plan_audit_origin_invalid" {
			continue
		}
		if reason.Severity != model.ReasonSeverityError {
			t.Fatalf("plan_audit_origin_invalid must be error severity on standard/strict, got %q", reason.Severity)
		}
		return
	}
	t.Fatalf("expected a plan_audit_origin_invalid blocker, got %v", eval.ReasonCodes)
}

// hasLifecycleReason reports whether any lifecycle event carries the reason.
func hasLifecycleReason(events []state.LifecycleEvent, reason string) bool {
	for _, event := range events {
		if event.Reason == reason {
			return true
		}
	}
	return false
}

// writeAutoPresetIntent writes a complete intake intent.md so an S0_INTAKE
// change can advance past intake section validation once the preset is
// confirmed.
func writeAutoPresetIntent(t *testing.T, root, slug string) {
	t.Helper()
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	intent := "# Intent\n\n## Summary\nTest\n\n## In Scope\nAdd logging\n\n## Out of Scope\nNothing\n\n## Acceptance Signals\nTests pass\n"
	if err := os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(intent), 0o644); err != nil {
		t.Fatalf("write intent.md: %v", err)
	}
}

// TestAutoPresetConfirmTarget_UpgradeOnly proves the pure upgrade-only predicate:
// it confirms to the effective preset when that rank is >= the pending
// suggestion, and refuses (ok=false) any downgrade target so the engine never
// auto-downgrades.
func TestAutoPresetConfirmTarget_UpgradeOnly(t *testing.T) {
	t.Parallel()

	t.Run("effective rank equal to suggested confirms", func(t *testing.T) {
		t.Parallel()
		change := model.Change{SuggestedWorkflowPreset: model.WorkflowPresetStandard}
		policy := governance.PresetPolicy{EffectivePreset: model.WorkflowPresetStandard}
		target, ok := autoPresetConfirmTarget(change, policy)
		if !ok || target != model.WorkflowPresetStandard {
			t.Fatalf("expected confirm to standard, got target=%q ok=%v", target, ok)
		}
	})

	t.Run("effective rank above suggested confirms (forced upgrade)", func(t *testing.T) {
		t.Parallel()
		change := model.Change{SuggestedWorkflowPreset: model.WorkflowPresetLight}
		policy := governance.PresetPolicy{EffectivePreset: model.WorkflowPresetStrict}
		target, ok := autoPresetConfirmTarget(change, policy)
		if !ok || target != model.WorkflowPresetStrict {
			t.Fatalf("expected forced upgrade to strict, got target=%q ok=%v", target, ok)
		}
	})

	t.Run("downgrade target is refused (never auto-downgrade)", func(t *testing.T) {
		t.Parallel()
		change := model.Change{SuggestedWorkflowPreset: model.WorkflowPresetStrict}
		policy := governance.PresetPolicy{EffectivePreset: model.WorkflowPresetLight}
		if target, ok := autoPresetConfirmTarget(change, policy); ok {
			t.Fatalf("downgrade must be refused, got target=%q ok=%v", target, ok)
		}
	})

	t.Run("nothing pending returns ok=false", func(t *testing.T) {
		t.Parallel()
		change := model.Change{WorkflowPreset: model.WorkflowPresetStandard}
		policy := governance.PresetPolicy{EffectivePreset: model.WorkflowPresetStrict}
		if _, ok := autoPresetConfirmTarget(change, policy); ok {
			t.Fatal("expected ok=false when no confirmation is pending")
		}
	})
}

// TestAdvanceGoverned_AutoConfirmsPendingPresetAndContinues covers REQ-003:
// under --auto a pending preset whose suggested/effective rank is >= current is
// auto-confirmed to the suggested/effective preset, a distinct
// auto_preset_confirmed lifecycle event is recorded, and advancement proceeds
// WITHOUT a preset_confirmation_required blocker.
func TestAdvanceGoverned_AutoConfirmsPendingPresetAndContinues(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("auto-preset-confirm")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "critical"
	change.WorkflowPreset = ""
	change.SuggestedWorkflowPreset = model.WorkflowPresetStrict
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	writeAutoPresetIntent(t, root, change.Slug)
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Now().UTC(),
		RunVersion: 0,
	})

	summary, err := AdvanceGoverned(root, change.Slug, AdvanceOptions{Auto: true, Command: "run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action == "blocked" {
		if hasAdvanceReasonCode(summary.Blockers, "preset_confirmation_required") {
			t.Fatalf("auto must not surface a preset confirmation blocker, got %+v", summary.Blockers)
		}
	}
	if summary.Action != "advanced" {
		t.Fatalf("expected advancement to continue after auto-confirm, got %+v", summary)
	}

	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.WorkflowPreset != model.WorkflowPresetStrict {
		t.Fatalf("expected confirmed preset=strict, got %q", reloaded.WorkflowPreset)
	}
	if reloaded.SuggestedWorkflowPreset != "" {
		t.Fatalf("expected pending suggestion cleared, got %q", reloaded.SuggestedWorkflowPreset)
	}

	events, err := state.ReadLifecycleEvents(root, reloaded)
	if err != nil {
		t.Fatalf("read lifecycle events: %v", err)
	}
	if !hasLifecycleReason(events, "auto_preset_confirmed") {
		t.Fatalf("expected an auto_preset_confirmed lifecycle event, got %+v", events)
	}
}

// TestAdvanceGoverned_AutoConfirmsPendingPresetInGuardrailDomain is finding #3's
// regression pin: upgrade-only preset auto-confirm is intentionally
// domain-independent. A change in a GUARDRAIL/sensitive domain whose pending
// suggested preset is an upgrade (rank >= current) STILL auto-confirms under
// --auto — the guardrail domain gates pure-pacing confirmation softening
// (review_batch / skill_handoff), NOT the upgrade-only preset confirm.
// Auto-confirm only mutates the preset/suggested-preset fields;
// it must not forge or clear evidence.
func TestAdvanceGoverned_AutoConfirmsPendingPresetInGuardrailDomain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("auto-preset-guardrail-confirm")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "critical"
	change.GuardrailDomain = model.GuardrailDomainAuthAuthZ
	change.WorkflowPreset = ""
	change.SuggestedWorkflowPreset = model.WorkflowPresetStrict
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	writeAutoPresetIntent(t, root, change.Slug)
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Now().UTC(),
		RunVersion: 0,
	})

	policy := governance.PresetPolicy{EffectivePreset: model.WorkflowPresetStrict}
	confirmed, err := autoConfirmPendingPreset(root, &change, true, policy, "run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !confirmed {
		t.Fatal("expected upgrade-only preset auto-confirm to remain domain-independent (guardrail domain present)")
	}
	if change.WorkflowPreset != model.WorkflowPresetStrict {
		t.Fatalf("expected confirmed preset=strict, got %q", change.WorkflowPreset)
	}
	if change.SuggestedWorkflowPreset != "" {
		t.Fatalf("expected pending suggestion cleared, got %q", change.SuggestedWorkflowPreset)
	}

	// The auto-confirm must not forge or clear the intake evidence: the
	// pre-existing intake-clarification record is still present and untouched.
	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.GuardrailDomain != model.GuardrailDomainAuthAuthZ {
		t.Fatalf("auto-confirm must not mutate the guardrail domain, got %q", reloaded.GuardrailDomain)
	}
	rec, err := state.LoadVerification(root, reloaded.Slug, SkillIntakeClarification)
	if err != nil {
		t.Fatalf("read intake verification: %v", err)
	}
	if rec.Verdict != model.VerificationVerdictPass {
		t.Fatalf("auto-confirm must not alter recorded evidence, got verdict %q", rec.Verdict)
	}

	events, err := state.ReadLifecycleEvents(root, reloaded)
	if err != nil {
		t.Fatalf("read lifecycle events: %v", err)
	}
	if !hasLifecycleReason(events, "auto_preset_confirmed") {
		t.Fatalf("expected an auto_preset_confirmed lifecycle event, got %+v", events)
	}
}

func TestAutoConfirmPendingPresetRollsBackWhenLifecycleEventAppendFails(t *testing.T) {
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("auto-preset-event-rollback")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.WorkflowPreset = ""
	change.SuggestedWorkflowPreset = model.WorkflowPresetStrict
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	writeAutoPresetIntent(t, root, change.Slug)

	appendErr := errors.New("append lifecycle event failed")
	originalAppend := appendLifecycleEvent
	appendLifecycleEvent = func(string, model.Change, state.LifecycleEvent) (state.LifecycleEvent, error) {
		return state.LifecycleEvent{}, appendErr
	}
	defer func() {
		appendLifecycleEvent = originalAppend
	}()

	confirmed, err := autoConfirmPendingPreset(
		root,
		&change,
		true,
		governance.PresetPolicy{EffectivePreset: model.WorkflowPresetStrict},
		"run",
	)
	if !errors.Is(err, appendErr) {
		t.Fatalf("expected append error, got %v", err)
	}
	if confirmed {
		t.Fatal("expected failed auto-confirm when lifecycle event append fails")
	}
	if change.WorkflowPreset != "" || change.SuggestedWorkflowPreset != model.WorkflowPresetStrict {
		t.Fatalf("expected in-memory preset restored to pending, got preset=%q suggested=%q",
			change.WorkflowPreset, change.SuggestedWorkflowPreset)
	}

	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.WorkflowPreset != "" || reloaded.SuggestedWorkflowPreset != model.WorkflowPresetStrict {
		t.Fatalf("expected persisted preset restored to pending, got preset=%q suggested=%q",
			reloaded.WorkflowPreset, reloaded.SuggestedWorkflowPreset)
	}

	events, err := state.ReadLifecycleEvents(root, reloaded)
	if err != nil {
		t.Fatalf("read lifecycle events: %v", err)
	}
	if hasLifecycleReason(events, "auto_preset_confirmed") {
		t.Fatalf("failed auto-confirm must not record auto_preset_confirmed, got %+v", events)
	}
}

// TestAdvanceGoverned_AutoForcesUpgradeViaMinPreset covers REQ-006: under --auto
// the suggested preset is confirmed UPGRADE-ONLY to the project-forced effective
// preset (here min_preset=strict forces a light suggestion up to strict). Auto
// never lands below the forced control level.
func TestAdvanceGoverned_AutoForcesUpgradeViaMinPreset(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := model.DefaultConfig()
	cfg.Governance.MinPreset = model.WorkflowPresetStrict
	if err := model.SaveConfig(state.ConfigPath(root), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("auto-preset-min-upgrade")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "simple"
	change.WorkflowPreset = ""
	change.SuggestedWorkflowPreset = model.WorkflowPresetLight
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	writeAutoPresetIntent(t, root, change.Slug)
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Now().UTC(),
		RunVersion: 0,
	})

	if _, err := AdvanceGoverned(root, change.Slug, AdvanceOptions{Auto: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.WorkflowPreset != model.WorkflowPresetStrict {
		t.Fatalf("expected min_preset to force confirmed preset up to strict, got %q", reloaded.WorkflowPreset)
	}
}

// TestAdvanceGoverned_AutoDoesNotSkipEvidenceGate covers REQ-006: under --auto a
// missing required skill evidence still blocks advancement. Auto only
// auto-confirms the preset; it never weakens an evidence gate.
func TestAdvanceGoverned_AutoDoesNotSkipEvidenceGate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("auto-preset-evidence-gate")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "critical"
	change.WorkflowPreset = ""
	change.SuggestedWorkflowPreset = model.WorkflowPresetStrict
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	// Intentionally omit intake-clarification evidence: auto-confirming the
	// preset must not bypass the intake skill evidence gate.
	writeAutoPresetIntent(t, root, change.Slug)

	summary, err := AdvanceGoverned(root, change.Slug, AdvanceOptions{Auto: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Action != "blocked" {
		t.Fatalf("expected missing intake evidence to still block under auto, got %+v", summary)
	}
	if !hasAdvanceReasonDetail(summary.Blockers, "required_skill_missing", SkillIntakeClarification) {
		t.Fatalf("expected required_skill_missing for intake-clarification, got %+v", summary.Blockers)
	}

	// The preset was auto-confirmed (upgrade-only) but the evidence gate still
	// fail-closed: the change did not leave S0_INTAKE/clarify.
	reloaded, err := state.LoadChange(root, change.Slug)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if reloaded.WorkflowPreset != model.WorkflowPresetStrict {
		t.Fatalf("expected auto-confirmed strict preset, got %q", reloaded.WorkflowPreset)
	}
	if reloaded.CurrentState != model.StateS0Intake || reloaded.IntakeSubStep != model.IntakeSubStepClarify {
		t.Fatalf("expected change held at S0_INTAKE/clarify by the evidence gate, got state=%s substep=%s",
			reloaded.CurrentState, reloaded.IntakeSubStep)
	}
}

// TestAdvanceGoverned_AutoOffPresetPendingUnchanged is a regression pin: with
// auto OFF, a pending preset still hard-stops with preset_confirmation_required
// exactly as before, and the change is not mutated.
func TestAdvanceGoverned_AutoOffPresetPendingUnchanged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	change := model.NewChange("auto-off-preset-pending")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepClarify
	change.PlanSubStep = model.PlanSubStepNone
	change.ComplexityLevel = "critical"
	change.WorkflowPreset = ""
	change.SuggestedWorkflowPreset = model.WorkflowPresetStrict
	if err := state.SaveChange(root, change); err != nil {
		t.Fatalf("save change: %v", err)
	}
	writeAutoPresetIntent(t, root, change.Slug)
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
	if reloaded.WorkflowPreset.IsValid() {
		t.Fatalf("expected preset to remain pending (unconfirmed), got %q", reloaded.WorkflowPreset)
	}
	if reloaded.SuggestedWorkflowPreset != model.WorkflowPresetStrict {
		t.Fatalf("expected pending suggestion preserved, got %q", reloaded.SuggestedWorkflowPreset)
	}
}
