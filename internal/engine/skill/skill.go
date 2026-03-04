package skill

import (
	"fmt"
	"slices"

	"github.com/signalridge/speclane/internal/model"
)

type Definition struct {
	Name                string              `json:"name"`
	State               model.WorkflowState `json:"state"`
	Mitigation          string              `json:"mitigation"`
	RunSummaryBound     bool                `json:"run_summary_bound"`
	RequiredLevels      []model.Level       `json:"required_levels,omitempty"`
	AutoModeRequired    bool                `json:"auto_mode_required,omitempty"`
	CloseoutConditional bool                `json:"closeout_conditional,omitempty"`
	ReviewerIndependent bool                `json:"reviewer_independent,omitempty"`
}

var defaultGovernanceRegistry = map[string]Definition{
	"intake-analysis": {
		Name:             "intake-analysis",
		State:            model.StateS1Analyze,
		Mitigation:       "unclear intent and hidden guardrail risk",
		RunSummaryBound:  false,
		RequiredLevels:   []model.Level{model.LevelL2, model.LevelL3},
		AutoModeRequired: true,
	},
	"scope-confirmation": {
		Name:            "scope-confirmation",
		State:           model.StateS3ScopeConfirmation,
		Mitigation:      "L3 discovery/scope drift",
		RunSummaryBound: false,
		RequiredLevels:  []model.Level{model.LevelL3},
	},
	"plan-audit": {
		Name:            "plan-audit",
		State:           model.StateS5PlanAudit,
		Mitigation:      "stale or incomplete plan bundle",
		RunSummaryBound: false,
		RequiredLevels:  []model.Level{model.LevelL2, model.LevelL3},
	},
	"wave-orchestration": {
		Name:            "wave-orchestration",
		State:           model.StateS6RunWaves,
		Mitigation:      "uncontrolled parallel execution drift",
		RunSummaryBound: true,
		RequiredLevels:  []model.Level{model.LevelL2, model.LevelL3},
	},
	"artifact-review": {
		Name:                "artifact-review",
		State:               model.StateS7Review,
		Mitigation:          "cross-artifact inconsistency",
		RunSummaryBound:     true,
		RequiredLevels:      []model.Level{model.LevelL2, model.LevelL3},
		ReviewerIndependent: true,
	},
	"goal-verification": {
		Name:            "goal-verification",
		State:           model.StateS8Verify,
		Mitigation:      "false completion claims",
		RunSummaryBound: true,
		RequiredLevels:  []model.Level{model.LevelL2, model.LevelL3},
	},
	"final-closeout": {
		Name:                "final-closeout",
		State:               model.StateS8Verify,
		Mitigation:          "stale final evidence before governed ship decision",
		RunSummaryBound:     true,
		RequiredLevels:      []model.Level{model.LevelL2, model.LevelL3},
		CloseoutConditional: true,
		ReviewerIndependent: true,
	},
}

func GovernanceRegistry() []Definition {
	out := make([]Definition, 0, len(defaultGovernanceRegistry))
	for _, def := range defaultGovernanceRegistry {
		out = append(out, def)
	}
	slices.SortFunc(out, func(a, b Definition) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return out
}

func IsGovernanceSkill(name string) bool {
	_, ok := defaultGovernanceRegistry[name]
	return ok
}

func IsGovernanceSkillFromRegistry(registry []Definition, name string) bool {
	if stringsTrim(name) == "" {
		return false
	}
	_, ok := governanceDefinitionByName(registry)[name]
	return ok
}

// RequiredSkillsForState returns level/state-governed mandatory skills for the current state.
func RequiredSkillsForState(
	level model.Level,
	state model.WorkflowState,
	autoMode bool,
	closeoutRequired bool,
) []string {
	return RequiredSkillsForStateWithRegistry(
		GovernanceRegistry(),
		level,
		state,
		autoMode,
		closeoutRequired,
	)
}

func RequiredSkillsForStateWithRegistry(
	registry []Definition,
	level model.Level,
	state model.WorkflowState,
	autoMode bool,
	closeoutRequired bool,
) []string {
	required := []string{}
	for _, def := range registry {
		if def.State != state {
			continue
		}
		if def.CloseoutConditional && !closeoutRequired {
			continue
		}
		if def.AutoModeRequired && autoMode && state == model.StateS1Analyze {
			required = append(required, def.Name)
			continue
		}
		if requiredForLevel(def, level) {
			required = append(required, def.Name)
		}
	}
	if len(required) == 0 {
		return nil
	}
	slices.Sort(required)
	return required
}

type EvidenceReadinessInput struct {
	Level                         model.Level
	Record                        model.EvidenceRecord
	LatestFrozenRunSummaryVersion int
	ImplementerBaselineSessionID  string
}

func ValidateGovernanceEvidenceReadiness(input EvidenceReadinessInput) error {
	return ValidateGovernanceEvidenceReadinessWithRegistry(GovernanceRegistry(), input)
}

func ValidateGovernanceEvidenceReadinessWithRegistry(registry []Definition, input EvidenceReadinessInput) error {
	definitions := governanceDefinitionByName(registry)
	def, ok := definitions[input.Record.SkillName]
	if !ok {
		return fmt.Errorf("unknown governance skill %q", input.Record.SkillName)
	}
	if input.Record.State != def.State {
		return fmt.Errorf(
			"skill %q must emit evidence at state %q (got %q)",
			input.Record.SkillName,
			def.State,
			input.Record.State,
		)
	}
	if err := input.Record.Validate(); err != nil {
		return err
	}
	if input.Record.MitigationTarget != "" && input.Record.MitigationTarget != def.Mitigation {
		return fmt.Errorf(
			"mitigation_target mismatch for skill_name=%q: expected %q got %q",
			input.Record.SkillName,
			def.Mitigation,
			input.Record.MitigationTarget,
		)
	}

	if def.RunSummaryBound {
		if input.LatestFrozenRunSummaryVersion < 1 {
			return fmt.Errorf("missing latest frozen run summary version")
		}
		if input.Record.RunSummaryVersion != input.LatestFrozenRunSummaryVersion {
			return fmt.Errorf(
				"run_summary_version mismatch: evidence=%d latest=%d",
				input.Record.RunSummaryVersion,
				input.LatestFrozenRunSummaryVersion,
			)
		}
	}

	if def.ReviewerIndependent && isGovernedLevel(input.Level) {
		if !model.IsUUIDv7(input.ImplementerBaselineSessionID) {
			return fmt.Errorf("missing implementer baseline session for governed reviewer independence")
		}
		if input.Record.SessionID == input.ImplementerBaselineSessionID {
			return fmt.Errorf("reviewer session_id must differ from implementer baseline")
		}
	}

	return nil
}

func NewSessionID() (string, error) {
	return model.NewRequestID()
}

func IsValidSessionID(sessionID string) bool {
	return model.IsUUIDv7(sessionID)
}

func CanonicalInputHash(payload map[string]any) (string, error) {
	return model.ComputeInputHash(payload)
}

func requiredForLevel(def Definition, level model.Level) bool {
	if level != model.LevelL1 && level != model.LevelL2 && level != model.LevelL3 {
		return false
	}
	for _, allowed := range def.RequiredLevels {
		if allowed == level {
			return true
		}
	}
	return false
}

func isGovernedLevel(level model.Level) bool {
	return level == model.LevelL2 || level == model.LevelL3
}

func governanceDefinitionByName(registry []Definition) map[string]Definition {
	out := map[string]Definition{}
	for _, def := range registry {
		if stringsTrim(def.Name) == "" {
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
		copied.RequiredLevels = append([]model.Level(nil), def.RequiredLevels...)
		out[key] = copied
	}
	return out
}

func stringsTrim(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
