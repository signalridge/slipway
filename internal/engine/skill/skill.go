package skill

import (
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

type Definition struct {
	Name              string              `json:"name"`
	State             model.WorkflowState `json:"state"`
	PlanSubStep       model.PlanSubStep   `json:"plan_substep,omitempty"`
	Mitigation        string              `json:"mitigation"`
	RunSummaryBound   bool                `json:"run_summary_bound"`
	DiscoveryOnly     bool                `json:"discovery_only,omitempty"`
	AllowedOperations []string            `json:"allowed_operations,omitempty"`
	RequiredOutputs   []string            `json:"required_outputs,omitempty"`
	HardGate          string              `json:"hard_gate,omitempty"`
}

const (
	reviewSkillSpecCompliance = "spec-compliance-review"
	reviewSkillCodeQuality    = "code-quality-review"
	reviewSkillIndependent    = "independent-review"
	reviewSkillSecurity       = "security-review"
)

type ReviewSkillSelection struct {
	SecurityReviewSelected bool
}

func SelectedReviewSkills(selection ReviewSkillSelection) []string {
	selected := []string{
		reviewSkillSpecCompliance,
		reviewSkillCodeQuality,
		reviewSkillIndependent,
	}
	if selection.SecurityReviewSelected {
		selected = append(selected, reviewSkillSecurity)
	}
	return selected
}

func SelectedReviewSkillsForWorkflowProfile(selection ReviewSkillSelection, profile model.WorkflowProfile) []string {
	return FilterRequiredSkillsForWorkflowProfileWithReviewSelection(SelectedReviewSkills(selection), profile, selection)
}

func ReviewSkillSelected(skillName string, selection ReviewSkillSelection) bool {
	return slices.Contains(SelectedReviewSkills(selection), strings.TrimSpace(skillName))
}

func IsReviewSkill(skillName string) bool {
	switch strings.TrimSpace(skillName) {
	case reviewSkillSpecCompliance, reviewSkillCodeQuality, reviewSkillIndependent, reviewSkillSecurity:
		return true
	default:
		return false
	}
}

var defaultGovernanceRegistry = map[string]Definition{
	"intake-clarification": {
		Name:              "intake-clarification",
		State:             model.StateS0Intake,
		Mitigation:        "scope ambiguity and intent drift before planning",
		AllowedOperations: []string{"read_codebase", "read_artifacts", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record"},
	},
	"research-orchestration": {
		Name:              "research-orchestration",
		State:             model.StateS1Plan,
		PlanSubStep:       model.PlanSubStepResearch,
		Mitigation:        "insufficient technical research before plan bundling",
		DiscoveryOnly:     true,
		AllowedOperations: []string{"read_codebase", "read_artifacts", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record"},
	},
	"plan-audit": {
		Name:              "plan-audit",
		State:             model.StateS1Plan,
		PlanSubStep:       model.PlanSubStepAudit,
		Mitigation:        "stale or incomplete plan bundle",
		AllowedOperations: []string{"read_artifacts", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record"},
		HardGate:          "G_plan",
	},
	"wave-orchestration": {
		Name:              "wave-orchestration",
		State:             model.StateS2Implement,
		Mitigation:        "uncontrolled parallel execution drift",
		RunSummaryBound:   true,
		AllowedOperations: []string{"read_codebase", "read_artifacts", "write_code", "run_tests", "write_evidence", "git_commit"},
		RequiredOutputs:   []string{"evidence_record", "task_results", "changed_files"},
	},
	"spec-compliance-review": {
		Name:              "spec-compliance-review",
		State:             model.StateS3Review,
		Mitigation:        "implementation divergence from spec",
		RunSummaryBound:   true,
		AllowedOperations: []string{"read_codebase", "read_artifacts", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record", "review_findings"},
	},
	"code-quality-review": {
		Name:              "code-quality-review",
		State:             model.StateS3Review,
		Mitigation:        "cross-artifact inconsistency and code quality gaps",
		RunSummaryBound:   true,
		AllowedOperations: []string{"read_codebase", "read_artifacts", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record", "review_findings"},
	},
	"independent-review": {
		Name:              "independent-review",
		State:             model.StateS3Review,
		Mitigation:        "same-context review bias before ship verification",
		RunSummaryBound:   true,
		AllowedOperations: []string{"read_codebase", "read_artifacts", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record", "review_findings"},
	},
	"security-review": {
		Name:              "security-review",
		State:             model.StateS3Review,
		Mitigation:        "security-sensitive implementation gaps",
		RunSummaryBound:   true,
		AllowedOperations: []string{"read_codebase", "read_artifacts", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record", "review_findings"},
	},
	"ship-verification": {
		Name:              "ship-verification",
		State:             model.StateS3Review,
		Mitigation:        "false completion claims and stale final evidence before the governed ship decision",
		RunSummaryBound:   true,
		AllowedOperations: []string{"read_codebase", "read_artifacts", "run_tests", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record"},
		HardGate:          "G_ship",
	},
}

// LookupDefinitionInRegistry returns the governance skill Definition from a loaded registry.
func LookupDefinitionInRegistry(registry []Definition, name string) (Definition, bool) {
	byName := governanceDefinitionByName(registry)
	def, ok := byName[name]
	return def, ok
}

func RequiredSkillsForStateWithRegistryWithReviewSelection(
	registry []Definition,
	needsDiscovery bool,
	state model.WorkflowState,
	reviewSelection ReviewSkillSelection,
	planSubSteps ...model.PlanSubStep,
) []string {
	required := []string{}

	// Build set of active plan sub-steps for S1_PLAN matching.
	activeSubSteps := make(map[model.PlanSubStep]bool, len(planSubSteps))
	for _, ps := range planSubSteps {
		activeSubSteps[ps] = true
	}

	for _, def := range registry {
		if def.State != state {
			continue
		}
		// For S1_PLAN skills, match by PlanSubStep when sub-steps are provided.
		if def.State == model.StateS1Plan && def.PlanSubStep != model.PlanSubStepNone && len(activeSubSteps) > 0 {
			if !activeSubSteps[def.PlanSubStep] {
				continue
			}
		}
		if def.DiscoveryOnly && !needsDiscovery {
			continue
		}
		if def.State == model.StateS3Review && IsReviewSkill(def.Name) && !ReviewSkillSelected(def.Name, reviewSelection) {
			continue
		}
		required = append(required, def.Name)
	}
	if len(required) == 0 {
		return nil
	}
	slices.Sort(required)
	return required
}

func FilterRequiredSkillsForWorkflowProfileWithReviewSelection(
	required []string,
	profile model.WorkflowProfile,
	reviewSelection ReviewSkillSelection,
) []string {
	if profile.RequiresCodeQualityReview() {
		return filterRequiredSkillsForReviewSelection(required, reviewSelection)
	}
	filtered := make([]string, 0, len(required))
	for _, skillName := range required {
		if skillName == "code-quality-review" {
			continue
		}
		if IsReviewSkill(skillName) && !ReviewSkillSelected(skillName, reviewSelection) {
			continue
		}
		filtered = append(filtered, skillName)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func filterRequiredSkillsForReviewSelection(required []string, reviewSelection ReviewSkillSelection) []string {
	filtered := make([]string, 0, len(required))
	for _, skillName := range required {
		if IsReviewSkill(skillName) && !ReviewSkillSelected(skillName, reviewSelection) {
			continue
		}
		filtered = append(filtered, skillName)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func governanceDefinitionByName(registry []Definition) map[string]Definition {
	out := map[string]Definition{}
	for _, def := range registry {
		if strings.TrimSpace(def.Name) == "" {
			continue
		}
		out[def.Name] = def
	}
	return out
}

func defaultGovernanceRegistryMap() map[string]Definition {
	out := map[string]Definition{}
	for key, def := range defaultGovernanceRegistry {
		copied := def
		out[key] = copied
	}
	return out
}
