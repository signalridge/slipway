package progression

import (
	"fmt"
	"os"
	"strings"

	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
	"gopkg.in/yaml.v3"
)

func EvaluateRequiredSkillsForChange(
	root string,
	change model.Change,
	workflowState model.WorkflowState,
	latestRunSummaryVersion int,
	closeoutRequired bool,
	planSubSteps ...model.PlanSubStep,
) (map[string]model.VerificationRecord, []string, error) {
	return evaluateRequiredSkills(
		root,
		change.Slug,
		change.NeedsDiscovery,
		change.GuardrailDomain,
		workflowState,
		latestRunSummaryVersion,
		closeoutRequired,
		func() (map[string]model.VerificationRecord, error) {
			return state.ListVerificationsForChange(root, change)
		},
		planSubSteps...,
	)
}

func activePlanningSubStepsForState(change model.Change, workflowState model.WorkflowState) []model.PlanSubStep {
	if workflowState != model.StateS1Plan || change.PlanSubStep == model.PlanSubStepNone {
		return nil
	}
	return []model.PlanSubStep{change.PlanSubStep}
}

func evaluateRequiredSkills(
	root string,
	slug string,
	needsDiscovery bool,
	guardrailDomain string,
	workflowState model.WorkflowState,
	latestRunSummaryVersion int,
	closeoutRequired bool,
	loadVerifications func() (map[string]model.VerificationRecord, error),
	planSubSteps ...model.PlanSubStep,
) (map[string]model.VerificationRecord, []string, error) {
	registry, err := skill.LoadGovernanceRegistry(root)
	if err != nil {
		return nil, nil, err
	}
	required := skill.RequiredSkillsForStateWithRegistry(
		registry,
		needsDiscovery,
		workflowState,
		closeoutRequired,
		guardrailDomain,
		planSubSteps...,
	)
	// Read authoritative verification files before the empty-required-skills
	// early return so malformed evidence still fails closed with explicit
	// integrity reporting.
	verifications, err := loadVerifications()
	if err != nil {
		return nil, nil, err
	}
	if len(required) == 0 {
		return map[string]model.VerificationRecord{}, nil, nil
	}

	passing := map[string]model.VerificationRecord{}
	var blockers []string

	for _, skillName := range required {
		rec, ok := verifications[skillName]
		if !ok {
			blockers = append(blockers, "required_skill_missing:"+skillName)
			continue
		}

		// Validate readiness against skill definition.
		def, defFound := skill.LookupDefinitionInRegistry(registry, skillName)
		if !defFound {
			blockers = append(blockers, "required_skill_not_ready:"+skillName+":unknown_skill")
			continue
		}

		if def.RunSummaryBound && latestRunSummaryVersion < 1 {
			blockers = append(blockers, "required_skill_not_ready:"+skillName+":run_summary_missing")
			continue
		}

		if def.RunSummaryBound && latestRunSummaryVersion > 0 && rec.RunVersion != latestRunSummaryVersion {
			blockers = append(blockers, fmt.Sprintf(
				"required_skill_not_ready:%s:run_version_mismatch(got=%d,want=%d)",
				skillName,
				rec.RunVersion,
				latestRunSummaryVersion,
			))
			continue
		}

		if !rec.IsPassing() {
			switch {
			case rec.Verdict == model.VerificationVerdictFail:
				blockers = append(blockers, "required_skill_not_passed:"+skillName)
			case len(rec.Blockers) > 0:
				blockers = append(blockers, "required_skill_blockers_present:"+skillName)
			default:
				blockers = append(blockers, "required_skill_not_ready:"+skillName)
			}
			continue
		}

		passing[skillName] = rec
	}

	return passing, stringutil.UniqueSorted(blockers), nil
}

// ExtractHighRiskChecks extracts high-risk check results from passing skills.
func ExtractHighRiskChecks(
	passingSkills map[string]model.VerificationRecord,
) map[string]bool {
	checks := map[string]bool{}
	for _, record := range passingSkills {
		for _, ref := range record.References {
			checkID, pass, ok := ParseHighRiskCheckReference(ref)
			if !ok {
				continue
			}
			checks[checkID] = pass
		}
	}

	return checks
}

// ParseHighRiskCheckReference parses a high-risk check reference string.
//
// Accepted formats:
//   - "high_risk_check:<id>=<verdict>"  (e.g. "high_risk_check:auth=pass")
//   - "check:<id>=<verdict>"            (e.g. "check:auth=fail")
//   - "<id>=<verdict>"                  (e.g. "auth=pass")
//   - "<id>:<verdict>"                  (e.g. "auth:pass")
//   - "<id>"                            (bare check ID, implies pass)
//
// Verdict tokens: pass/passed/true/ok -> true; fail/failed/false -> false.
func ParseHighRiskCheckReference(reference string) (checkID string, pass bool, ok bool) {
	ref := strings.TrimSpace(reference)
	if ref == "" {
		return "", false, false
	}

	normalized := strings.ToLower(ref)
	normalized = strings.TrimPrefix(normalized, "high_risk_check:")
	normalized = strings.TrimPrefix(normalized, "check:")

	var verdictToken string
	if idx := strings.LastIndex(normalized, "="); idx > 0 {
		checkID = strings.TrimSpace(normalized[:idx])
		verdictToken = strings.TrimSpace(normalized[idx+1:])
	} else if idx := strings.LastIndex(normalized, ":"); idx > 0 {
		checkID = strings.TrimSpace(normalized[:idx])
		verdictToken = strings.TrimSpace(normalized[idx+1:])
	} else {
		checkID = strings.TrimSpace(normalized)
		if gate.IsRegisteredCheckID(checkID) {
			return checkID, true, true
		}
		return "", false, false
	}

	if !gate.IsRegisteredCheckID(checkID) {
		return "", false, false
	}
	switch verdictToken {
	case "pass", "passed", "true", "ok":
		return checkID, true, true
	case "fail", "failed", "false":
		return checkID, false, true
	default:
		return "", false, false
	}
}

// ValidateChangeYamlR0 validates the change.yaml in the governed bundle
// against the expected slug.
func ValidateChangeYamlR0(changeYamlPath string, slug string) (bool, []string) {
	raw, err := os.ReadFile(changeYamlPath)
	if err != nil {
		return false, []string{"manifest_missing"}
	}
	var change model.Change
	if err := yaml.Unmarshal(raw, &change); err != nil {
		return false, []string{"manifest_parse_invalid"}
	}

	reasons := []string{}
	if strings.TrimSpace(change.Slug) != strings.TrimSpace(slug) {
		reasons = append(reasons, "manifest_slug_mismatch")
	}
	if change.BaseRef == "" {
		reasons = append(reasons, "manifest_base_ref_missing")
	}
	return len(reasons) == 0, stringutil.UniqueSorted(reasons)
}
