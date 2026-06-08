package gate

import (
	"slices"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
)

type GateID string

const (
	GateScope GateID = "G_scope"
	GatePlan  GateID = "G_plan"
	GatePivot GateID = "G_pivot"
	GateShip  GateID = "G_ship"
)

type PivotKind string

const (
	PivotKindReroute PivotKind = "reroute"
	PivotKindRescope PivotKind = "rescope"
)

type GateEvaluation struct {
	GateID      GateID             `json:"gate_id"`
	Status      model.GateStatus   `json:"status"`
	ReasonCodes []model.ReasonCode `json:"reason_codes,omitempty"`
}

func EvaluateGScope(
	change model.Change,
	researchContent string,
	discoveryEvidenceOK bool,
	worktreeValidationReasons []model.ReasonCode,
	researchArtifactReasons ...model.ReasonCode,
) GateEvaluation {
	reasonCodes := []model.ReasonCode{}
	if change.NeedsDiscovery {
		if !discoveryEvidenceOK {
			reasonCodes = append(reasonCodes, model.NewReasonCode("missing_discovery_evidence", ""))
		}

		if len(researchArtifactReasons) > 0 {
			reasonCodes = append(reasonCodes, researchArtifactReasons...)
		} else if blockers := artifact.ResearchStructureBlockers(researchContent); len(blockers) > 0 {
			reasonCodes = append(reasonCodes, blockers...)
		}

		reasonCodes = append(reasonCodes, worktreeValidationReasons...)
	}

	reasonCodes = model.NormalizeReasonCodes(reasonCodes)
	status := model.GateStatusApproved
	if len(reasonCodes) > 0 {
		status = model.GateStatusBlocked
	}
	return GateEvaluation{
		GateID:      GateScope,
		Status:      status,
		ReasonCodes: reasonCodes,
	}
}

func EvaluateGPlan(bundleReady bool, planAuditPass bool, blockers []model.ReasonCode) GateEvaluation {
	reasonCodes := []model.ReasonCode{}
	if !bundleReady {
		reasonCodes = append(reasonCodes, model.NewReasonCode("artifact_not_ready", ""))
	}
	if !planAuditPass {
		reasonCodes = append(reasonCodes, model.NewReasonCode("plan_audit_failed", ""))
	}
	reasonCodes = append(reasonCodes, blockers...)

	reasonCodes = model.NormalizeReasonCodes(reasonCodes)
	status := model.GateStatusApproved
	if len(reasonCodes) > 0 {
		status = model.GateStatusBlocked
	}
	return GateEvaluation{
		GateID:      GatePlan,
		Status:      status,
		ReasonCodes: reasonCodes,
	}
}

func EvaluateGPivot(kind PivotKind, approved bool, state model.WorkflowState) GateEvaluation {
	reasonCodes := []model.ReasonCode{}
	if kind != PivotKindReroute && kind != PivotKindRescope {
		reasonCodes = append(reasonCodes, model.NewReasonCode("invalid_pivot_kind", string(kind)))
	}
	if !approved {
		reasonCodes = append(reasonCodes, model.NewReasonCode("pivot_not_approved", ""))
	}
	switch kind {
	case PivotKindReroute:
		switch state {
		case model.StateS1Plan, model.StateS2Execute, model.StateS3Review, model.StateS4Verify:
			// OK: reroute is available from planning through verification.
		default:
			reasonCodes = append(reasonCodes, model.NewReasonCode("pivot_state_invalid", string(state)))
		}
	case PivotKindRescope:
		if state != model.StateS2Execute {
			reasonCodes = append(reasonCodes, model.NewReasonCode("rescope_state_invalid", string(state)))
		}
	}

	reasonCodes = model.NormalizeReasonCodes(reasonCodes)
	status := model.GateStatusApproved
	if len(reasonCodes) > 0 {
		status = model.GateStatusBlocked
	}
	return GateEvaluation{
		GateID:      GatePivot,
		Status:      status,
		ReasonCodes: reasonCodes,
	}
}

func EvaluateGShip(
	change model.Change,
	artifactReady bool,
	verificationReady bool,
	manifestR0Valid bool,
	unresolvedBlockers []model.ReasonCode,
	highRiskChecks map[string]bool,
) GateEvaluation {
	reasonCodes := []model.ReasonCode{}
	if !artifactReady {
		reasonCodes = append(reasonCodes, model.NewReasonCode("artifact_not_ready", ""))
	}
	if !verificationReady {
		reasonCodes = append(reasonCodes, model.NewReasonCode("verification_evidence_missing", ""))
	}
	if !manifestR0Valid {
		reasonCodes = append(reasonCodes, model.NewReasonCode("manifest_r0_invalid", ""))
	}
	reasonCodes = append(reasonCodes, unresolvedBlockers...)

	hrReasons := evaluateHighRiskChecksWithType(change.GuardrailDomain, highRiskChecks)
	reasonCodes = append(reasonCodes, hrReasons...)

	reasonCodes = model.NormalizeReasonCodes(reasonCodes)
	status := model.GateStatusApproved
	if len(reasonCodes) > 0 {
		status = model.GateStatusBlocked
	}
	return GateEvaluation{
		GateID:      GateShip,
		Status:      status,
		ReasonCodes: reasonCodes,
	}
}

var highRiskCatalog = map[string][]string{
	model.GuardrailDomainAuthAuthZ:            {model.GuardrailDomainAuthAuthZ + ".safety_baseline"},
	model.GuardrailDomainSecurityCredentials:  {model.GuardrailDomainSecurityCredentials + ".safety_baseline"},
	model.GuardrailDomainPrivacyPII:           {model.GuardrailDomainPrivacyPII + ".safety_baseline"},
	model.GuardrailDomainFinancialFlows:       {model.GuardrailDomainFinancialFlows + ".safety_baseline"},
	model.GuardrailDomainSchemaDataMigration:  {model.GuardrailDomainSchemaDataMigration + ".safety_baseline"},
	model.GuardrailDomainIrreversibleOps:      {model.GuardrailDomainIrreversibleOps + ".safety_baseline"},
	model.GuardrailDomainExternalAPIContracts: {model.GuardrailDomainExternalAPIContracts + ".safety_baseline"},
}

func RequiredHighRiskChecks(domain string) []string {
	if domain == "" {
		return nil
	}
	checks := append([]string(nil), highRiskCatalog[domain]...)
	slices.Sort(checks)
	return checks
}

func evaluateHighRiskChecksWithType(domain string, checkResults map[string]bool) []model.ReasonCode {
	required := RequiredHighRiskChecks(domain)
	if len(required) == 0 {
		return nil
	}
	reasons := []model.ReasonCode{}
	for _, checkID := range required {
		passed, exists := checkResults[checkID]
		if !exists {
			reasons = append(reasons, model.NewReasonCode("high_risk_check_missing", checkID))
			continue
		}
		if !passed {
			reasons = append(reasons, model.NewReasonCode("high_risk_check_failed", checkID))
		}
	}
	return model.NormalizeReasonCodes(reasons)
}

func IsRegisteredCheckID(checkID string) bool {
	for _, checks := range highRiskCatalog {
		for _, c := range checks {
			if c == checkID {
				return true
			}
		}
	}
	return false
}
