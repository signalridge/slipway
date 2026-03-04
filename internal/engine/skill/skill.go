package skill

import (
	"fmt"
	"slices"

	"github.com/signalridge/speclane/internal/model"
)

type Definition struct {
	Name            string              `json:"name"`
	State           model.WorkflowState `json:"state"`
	Mitigation      string              `json:"mitigation"`
	RunSummaryBound bool                `json:"run_summary_bound"`
}

var governanceRegistry = map[string]Definition{
	"intake-analysis": {
		Name:            "intake-analysis",
		State:           model.StateS1Analyze,
		Mitigation:      "unclear intent and hidden guardrail risk",
		RunSummaryBound: false,
	},
	"scope-confirmation": {
		Name:            "scope-confirmation",
		State:           model.StateS3ScopeConfirmation,
		Mitigation:      "L3 discovery/scope drift",
		RunSummaryBound: false,
	},
	"plan-audit": {
		Name:            "plan-audit",
		State:           model.StateS5PlanAudit,
		Mitigation:      "stale or incomplete plan bundle",
		RunSummaryBound: false,
	},
	"wave-orchestration": {
		Name:            "wave-orchestration",
		State:           model.StateS6RunWaves,
		Mitigation:      "uncontrolled parallel execution drift",
		RunSummaryBound: true,
	},
	"artifact-review": {
		Name:            "artifact-review",
		State:           model.StateS7Review,
		Mitigation:      "cross-artifact inconsistency",
		RunSummaryBound: true,
	},
	"goal-verification": {
		Name:            "goal-verification",
		State:           model.StateS8Verify,
		Mitigation:      "false completion claims",
		RunSummaryBound: true,
	},
	"final-closeout": {
		Name:            "final-closeout",
		State:           model.StateS8Verify,
		Mitigation:      "stale final evidence before governed ship decision",
		RunSummaryBound: true,
	},
}

func GovernanceRegistry() []Definition {
	out := make([]Definition, 0, len(governanceRegistry))
	for _, def := range governanceRegistry {
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
	_, ok := governanceRegistry[name]
	return ok
}

// RequiredSkillsForState returns level/state-governed mandatory skills for the current state.
func RequiredSkillsForState(
	level model.Level,
	state model.WorkflowState,
	autoMode bool,
	closeoutRequired bool,
) []string {
	governed := level == model.LevelL2 || level == model.LevelL3

	switch state {
	case model.StateS1Analyze:
		if autoMode || governed {
			return []string{"intake-analysis"}
		}
		return nil
	case model.StateS3ScopeConfirmation:
		if level == model.LevelL3 {
			return []string{"scope-confirmation"}
		}
	case model.StateS5PlanAudit:
		if governed {
			return []string{"plan-audit"}
		}
	case model.StateS6RunWaves:
		if governed {
			return []string{"wave-orchestration"}
		}
	case model.StateS7Review:
		if governed {
			return []string{"artifact-review"}
		}
	case model.StateS8Verify:
		if governed {
			skills := []string{"goal-verification"}
			if closeoutRequired {
				skills = append(skills, "final-closeout")
			}
			return skills
		}
	}
	return nil
}

type EvidenceReadinessInput struct {
	Level                         model.Level
	Record                        model.EvidenceRecord
	LatestFrozenRunSummaryVersion int
	ImplementerBaselineSessionID  string
}

func ValidateGovernanceEvidenceReadiness(input EvidenceReadinessInput) error {
	def, ok := governanceRegistry[input.Record.SkillName]
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

	if isGovernedReviewerSkill(input.Level, input.Record.SkillName) {
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

func isGovernedReviewerSkill(level model.Level, skillName string) bool {
	if level != model.LevelL2 && level != model.LevelL3 {
		return false
	}
	return skillName == "artifact-review" || skillName == "final-closeout"
}
