package progression

import (
	"testing"

	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
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
