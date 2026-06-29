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
	GateShip  GateID = "G_ship"
)

type GateEvaluation struct {
	GateID      GateID             `json:"gate_id"`
	Status      model.GateStatus   `json:"status"`
	ReasonCodes []model.ReasonCode `json:"reason_codes,omitempty"`
}

// DiscoveryEvidenceState captures the non-passing discovery evidence taxonomy
// without relying on adjacent bool parameters at EvaluateGScope call sites.
type DiscoveryEvidenceState struct {
	Present bool
	Stale   bool
}

func EvaluateGScope(
	change model.Change,
	researchContent string,
	discoveryEvidenceOK bool,
	worktreeValidationReasons []model.ReasonCode,
	discoveryEvidence DiscoveryEvidenceState,
	researchArtifactReasons ...model.ReasonCode,
) GateEvaluation {
	reasonCodes := []model.ReasonCode{}
	if discoveryEvidence.Stale {
		discoveryEvidence.Present = true
	}
	if change.NeedsDiscovery {
		if !discoveryEvidenceOK {
			// Distinguish three present-state cases so the generic discovery reason
			// never contradicts the specific one in the same response (mirrors the
			// EvaluateGShip ship-evidence taxonomy):
			//   - stale: a research-orchestration record EXISTS, was passing, and its
			//     certified discovery inputs changed after the verdict. Report _stale
			//     via the merged required_skill_stale blocker, never _missing — _missing
			//     would hide the present-but-stale state and contradict required_actions.
			//   - present but failed on its OWN merits (fail verdict / recorded
			//     blockers): the specific required_skill_* blocker already explains it,
			//     so emit no generic reason and let the specific blocker stand; a _missing
			//     here would misdirect recovery toward a first-time discovery run.
			//   - genuinely absent: reserve the _missing code.
			switch {
			case discoveryEvidence.Stale:
				// required_skill_stale carries it; emit no generic reason.
			case discoveryEvidence.Present:
				// Present but not passing for its own reason; specific blocker carries it.
			default:
				reasonCodes = append(reasonCodes, model.NewReasonCode("missing_discovery_evidence", ""))
			}
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

func EvaluateGPlan(bundleReady bool, blockers []model.ReasonCode) GateEvaluation {
	reasonCodes := []model.ReasonCode{}
	if !bundleReady {
		reasonCodes = append(reasonCodes, model.NewReasonCode("artifact_not_ready", ""))
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

func EvaluateGShip(
	change model.Change,
	artifactReady bool,
	verificationReady bool,
	manifestR0Valid bool,
	unresolvedBlockers []model.ReasonCode,
	highRiskChecks map[string]bool,
	shipRecordPresent bool,
	shipRecordStale bool,
) GateEvaluation {
	reasonCodes := []model.ReasonCode{}
	if change.CurrentState != model.StateS3Review {
		reasonCodes = append(
			reasonCodes,
			model.NewReasonCode("review_required", "ship_gate_s3_exit:"+string(change.CurrentState)),
		)
	}
	if !artifactReady {
		reasonCodes = append(reasonCodes, model.NewReasonCode("artifact_not_ready", ""))
	}
	if !verificationReady {
		// Distinguish three present-state cases so the generic reason never
		// contradicts the specific one in the same response:
		//   - stale: the record EXISTS, was passing, and upstream execution/review
		//     evidence went stale. Report _stale ("refresh upstream, do not restamp"),
		//     never _missing — _missing would hide the present-but-stale state and
		//     contradict skills_ready.ship-verification.
		//   - present but failed on its OWN merits (fail verdict / recorded blockers):
		//     the specific required_skill_not_passed / required_skill_blockers_present
		//     blocker already explains it. Adding _stale here would misdirect recovery
		//     toward an upstream refresh when the ship record itself failed, so emit no
		//     generic reason and let the specific blocker stand.
		//   - genuinely absent: reserve the _missing code.
		switch {
		case shipRecordStale:
			reasonCodes = append(reasonCodes, model.NewReasonCode("ship_verification_evidence_stale", ""))
		case shipRecordPresent:
			// Present but not passing for its own reason; specific blocker carries it.
		default:
			reasonCodes = append(reasonCodes, model.NewReasonCode("ship_verification_evidence_missing", ""))
		}
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
