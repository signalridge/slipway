package skill

import (
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

type Definition struct {
	Name                string              `json:"name"`
	State               model.WorkflowState `json:"state"`
	PlanSubStep         model.PlanSubStep   `json:"plan_substep,omitempty"`
	Mitigation          string              `json:"mitigation"`
	RunSummaryBound     bool                `json:"run_summary_bound"`
	DiscoveryOnly       bool                `json:"discovery_only,omitempty"`
	CloseoutConditional bool                `json:"closeout_conditional,omitempty"`
	AgentHint           string              `json:"agent_hint,omitempty"`
	AllowedOperations   []string            `json:"allowed_operations,omitempty"`
	RequiredOutputs     []string            `json:"required_outputs,omitempty"`
	HardGate            string              `json:"hard_gate,omitempty"`
}

var defaultGovernanceRegistry = map[string]Definition{
	"intake-clarification": {
		Name:              "intake-clarification",
		State:             model.StateS0Intake,
		Mitigation:        "scope ambiguity and intent drift before planning",
		AgentHint:         "slipway-planner",
		AllowedOperations: []string{"read_codebase", "read_artifacts", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record"},
	},
	"research-orchestration": {
		Name:              "research-orchestration",
		State:             model.StateS1Plan,
		PlanSubStep:       model.PlanSubStepResearch,
		Mitigation:        "insufficient technical research before plan bundling",
		DiscoveryOnly:     true,
		AgentHint:         "slipway-researcher",
		AllowedOperations: []string{"read_codebase", "read_artifacts", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record"},
	},
	"plan-audit": {
		Name:              "plan-audit",
		State:             model.StateS1Plan,
		PlanSubStep:       model.PlanSubStepAudit,
		Mitigation:        "stale or incomplete plan bundle",
		AgentHint:         "slipway-auditor",
		AllowedOperations: []string{"read_artifacts", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record"},
		HardGate:          "G_plan",
	},
	"wave-orchestration": {
		Name:              "wave-orchestration",
		State:             model.StateS2Execute,
		Mitigation:        "uncontrolled parallel execution drift",
		RunSummaryBound:   true,
		AgentHint:         "slipway-orchestrator",
		AllowedOperations: []string{"read_codebase", "read_artifacts", "write_code", "run_tests", "write_evidence", "git_commit"},
		RequiredOutputs:   []string{"evidence_record", "task_results", "changed_files"},
	},
	"spec-compliance-review": {
		Name:              "spec-compliance-review",
		State:             model.StateS3Review,
		Mitigation:        "implementation divergence from spec",
		RunSummaryBound:   true,
		AgentHint:         "slipway-reviewer",
		AllowedOperations: []string{"read_codebase", "read_artifacts", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record", "review_findings"},
	},
	"code-quality-review": {
		Name:              "code-quality-review",
		State:             model.StateS3Review,
		Mitigation:        "cross-artifact inconsistency and code quality gaps",
		RunSummaryBound:   true,
		AgentHint:         "slipway-reviewer",
		AllowedOperations: []string{"read_codebase", "read_artifacts", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record", "review_findings"},
	},
	"goal-verification": {
		Name:              "goal-verification",
		State:             model.StateS4Verify,
		Mitigation:        "false completion claims",
		RunSummaryBound:   true,
		AgentHint:         "slipway-verifier",
		AllowedOperations: []string{"read_codebase", "read_artifacts", "run_tests", "write_evidence"},
		RequiredOutputs:   []string{"evidence_record"},
		HardGate:          "G_ship",
	},
	"final-closeout": {
		Name:                "final-closeout",
		State:               model.StateS4Verify,
		Mitigation:          "stale final evidence before governed ship decision",
		RunSummaryBound:     true,
		CloseoutConditional: true,
		AgentHint:           "slipway-closer",
		AllowedOperations:   []string{"read_codebase", "read_artifacts", "run_tests", "write_evidence"},
		RequiredOutputs:     []string{"evidence_record"},
	},
}

func GovernanceRegistry() []Definition {
	return definitionsToSortedSlice(defaultGovernanceRegistry)
}

func AgentHintForSkillInRegistry(registry []Definition, name string) string {
	if def, ok := LookupDefinitionInRegistry(registry, name); ok {
		return def.AgentHint
	}
	return ""
}

// LookupDefinition returns the governance skill Definition for the given name.
// Returns the zero value and false if the skill is not in the registry.
func LookupDefinition(name string) (Definition, bool) {
	def, ok := defaultGovernanceRegistry[name]
	return def, ok
}

// LookupDefinitionInRegistry returns the governance skill Definition from a loaded registry.
func LookupDefinitionInRegistry(registry []Definition, name string) (Definition, bool) {
	byName := governanceDefinitionByName(registry)
	def, ok := byName[name]
	return def, ok
}

func RequiredSkillsForStateWithRegistry(
	registry []Definition,
	needsDiscovery bool,
	state model.WorkflowState,
	closeoutRequired bool,
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
		if def.CloseoutConditional && !closeoutRequired {
			continue
		}
		if def.DiscoveryOnly && !needsDiscovery {
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
