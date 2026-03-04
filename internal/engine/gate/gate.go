package gate

import (
	"fmt"
	"slices"

	"github.com/signalridge/speclane/internal/engine/artifact"
	"github.com/signalridge/speclane/internal/model"
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
	GateID  GateID           `json:"gate_id"`
	Status  model.GateStatus `json:"status"`
	Reasons []string         `json:"reasons,omitempty"`
}

func MandatoryGatesForLevel(level model.Level) []GateID {
	return MandatoryGatesForLevelWithPivot(level, false)
}

func MandatoryGatesForLevelWithPivot(level model.Level, pivoting bool) []GateID {
	switch level {
	case model.LevelL1:
		if pivoting {
			return []GateID{GatePivot}
		}
		return nil
	case model.LevelL2:
		gates := []GateID{GatePlan, GateShip}
		if pivoting {
			gates = append(gates, GatePivot)
		}
		return gates
	case model.LevelL3:
		gates := []GateID{GateScope, GatePlan, GateShip}
		if pivoting {
			gates = append(gates, GatePivot)
		}
		return gates
	default:
		return nil
	}
}

func MapGateDecision(decision model.GateDecision, reasons []string) (model.GateStatus, error) {
	switch decision {
	case model.GateDecisionApprove:
		return model.GateStatusApproved, nil
	case model.GateDecisionReject:
		if len(reasons) == 0 {
			return "", fmt.Errorf("reject decision requires reasons")
		}
		return model.GateStatusBlocked, nil
	case model.GateDecisionConditionalApprove:
		if len(reasons) == 0 {
			return "", fmt.Errorf("conditional_approve requires reasons")
		}
		return model.GateStatusPending, nil
	default:
		return "", fmt.Errorf("invalid gate decision: %q", decision)
	}
}

func EvaluateGScope(
	change model.ChangeState,
	exploreContent string,
	discoveryEvidenceOK bool,
	scopeEvidenceOK bool,
	worktreeValidationReasons []string,
) GateEvaluation {
	reasons := []string{}
	if change.Level == model.LevelL3 {
		if !discoveryEvidenceOK {
			reasons = append(reasons, "missing_discovery_evidence")
		}
		if !scopeEvidenceOK {
			reasons = append(reasons, "missing_scope_confirmation_evidence")
		}
		if change.WorktreePath == "" {
			reasons = append(reasons, "missing_worktree_path")
		}
		if change.WorktreeBranch == "" {
			reasons = append(reasons, "missing_worktree_branch")
		}
		reasons = append(reasons, worktreeValidationReasons...)
		if err := artifact.ValidateExploreStructure(exploreContent); err != nil {
			reasons = append(reasons, "explore_structure_invalid:"+err.Error())
		}
	}

	status := model.GateStatusApproved
	if len(reasons) > 0 {
		status = model.GateStatusBlocked
	}
	return GateEvaluation{GateID: GateScope, Status: status, Reasons: reasons}
}

func EvaluateGPlan(bundleReady bool, planAuditPass bool, blockers []string) GateEvaluation {
	reasons := []string{}
	if !bundleReady {
		reasons = append(reasons, "artifact_not_ready")
	}
	if !planAuditPass {
		reasons = append(reasons, "plan_audit_failed")
	}
	reasons = append(reasons, blockers...)

	status := model.GateStatusApproved
	if len(reasons) > 0 {
		status = model.GateStatusBlocked
	}
	return GateEvaluation{GateID: GatePlan, Status: status, Reasons: reasons}
}

func EvaluateGPivot(kind PivotKind, approved bool, state model.WorkflowState, level model.Level) GateEvaluation {
	reasons := []string{}
	if kind != PivotKindReroute && kind != PivotKindRescope {
		reasons = append(reasons, "invalid_pivot_kind")
	}
	if !approved {
		reasons = append(reasons, "pivot_not_approved")
	}
	if kind == PivotKindRescope {
		if state != model.StateS6RunWaves {
			reasons = append(reasons, "rescope_requires_s6_state")
		}
		if level != model.LevelL2 && level != model.LevelL3 {
			reasons = append(reasons, "rescope_requires_governed_level")
		}
		if !approved {
			reasons = append(reasons, "rescope_requires_approved_pivot")
		}
	}

	status := model.GateStatusApproved
	if len(reasons) > 0 {
		status = model.GateStatusBlocked
	}
	return GateEvaluation{GateID: GatePivot, Status: status, Reasons: uniqueReasons(reasons)}
}

func EvaluateGShip(
	change model.ChangeState,
	artifactReady bool,
	verificationReady bool,
	manifestR0Valid bool,
	unresolvedBlockers []string,
	highRiskChecks map[string]bool,
) GateEvaluation {
	reasons := []string{}
	if !artifactReady {
		reasons = append(reasons, "artifact_not_ready")
	}
	if !verificationReady {
		reasons = append(reasons, "verification_evidence_missing")
	}
	if !manifestR0Valid {
		reasons = append(reasons, "manifest_r0_invalid")
	}
	reasons = append(reasons, unresolvedBlockers...)

	reasons = append(reasons, EvaluateHighRiskChecks(change.RouteSnapshot.GuardrailDomain, highRiskChecks)...)

	status := model.GateStatusApproved
	if len(reasons) > 0 {
		status = model.GateStatusBlocked
	}
	return GateEvaluation{GateID: GateShip, Status: status, Reasons: uniqueReasons(reasons)}
}

var highRiskCatalog = map[string][]string{
	"auth_authz":              {"auth_authz.safety_baseline"},
	"security_credentials":    {"security_credentials.safety_baseline"},
	"privacy_pii":             {"privacy_pii.safety_baseline"},
	"financial_flows":         {"financial_flows.safety_baseline"},
	"schema_data_migration":   {"schema_data_migration.safety_baseline"},
	"irreversible_operations": {"irreversible_operations.safety_baseline"},
	"external_api_contracts":  {"external_api_contracts.safety_baseline"},
}

func RequiredHighRiskChecks(domain string) []string {
	if domain == "" {
		return nil
	}
	checks := append([]string(nil), highRiskCatalog[domain]...)
	slices.Sort(checks)
	return checks
}

func EvaluateHighRiskChecks(domain string, checkResults map[string]bool) []string {
	required := RequiredHighRiskChecks(domain)
	if len(required) == 0 {
		return nil
	}
	reasons := []string{}
	for _, checkID := range required {
		passed, exists := checkResults[checkID]
		if !exists {
			reasons = append(reasons, "high_risk_check_missing:"+checkID)
			continue
		}
		if !passed {
			reasons = append(reasons, "high_risk_check_failed:"+checkID)
		}
	}
	return reasons
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

func uniqueReasons(reasons []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if _, ok := seen[reason]; ok {
			continue
		}
		seen[reason] = struct{}{}
		out = append(out, reason)
	}
	return out
}
