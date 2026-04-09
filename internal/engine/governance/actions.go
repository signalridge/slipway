package governance

import (
	"github.com/signalridge/slipway/internal/model"
)

// RequiredAction represents a governance obligation derived from an active control.
type RequiredAction struct {
	ControlID   model.ControlID    `json:"control_id"`
	Mode        model.ControlMode  `json:"mode"`
	Scope       model.ControlScope `json:"scope"`
	Description string             `json:"description"`
	Satisfied   bool               `json:"satisfied"`
}

// ResolveRequiredActions derives the queue of required governance actions
// from the current state, active controls, and runtime evidence.
//
// This function consumes governance outputs (snapshot, controls) but does not
// import from progression, workflow, or wave packages — keeping the dependency
// direction one-way.
func ResolveRequiredActions(input RequiredActionsInput) []RequiredAction {
	var actions []RequiredAction

	for _, ctrl := range input.ActiveControls {
		if !ctrl.Active {
			continue
		}

		action := RequiredAction{
			ControlID: ctrl.ControlID,
			Mode:      ctrl.Mode,
			Scope:     ctrl.Scope,
		}

		switch ctrl.ControlID {
		case model.ControlClarification:
			action.Description = "resolve or defer blocking open questions in intent before downstream artifacts"
			action.Satisfied = !input.HasBlockingOpenQuestions

		case model.ControlResearch:
			action.Description = "complete research.md, resolve unknowns, and confirm scope via intake before continuing"
			action.Satisfied = input.IntentExists && input.ScopeConfirmed && input.ResearchStructureOK

		case model.ControlDomainReview:
			action.Description = "run domain-aware compliance review and attach review evidence"
			action.Satisfied = input.DomainReviewDone

		case model.ControlIndependentReview:
			action.Description = "run independent review before further execution"
			action.Satisfied = input.IndependentReviewDone

		case model.ControlWorktreeIsolation:
			action.Description = "ensure worktree preflight before code execution continues"
			action.Satisfied = input.WorktreePreflightDone

		case model.ControlRollbackRequired:
			action.Description = "write rollback section in decision.md and assurance.md"
			action.Satisfied = input.RollbackSectionExists
		}

		actions = append(actions, action)
	}

	return actions
}

// RequiredActionsInput captures the runtime evidence needed to resolve action satisfaction.
type RequiredActionsInput struct {
	ActiveControls           []model.ControlActivation
	HasBlockingOpenQuestions bool
	IntentExists             bool
	ScopeConfirmed           bool
	ResearchStructureOK      bool // research.md has all required sections (always true for non-discovery)
	DomainReviewDone         bool
	IndependentReviewDone    bool
	WorktreePreflightDone    bool
	RollbackSectionExists    bool
}

// UnsatisfiedBlockingActions returns actions that are blocking and not yet satisfied.
func UnsatisfiedBlockingActions(actions []RequiredAction) []RequiredAction {
	var result []RequiredAction
	for _, a := range actions {
		if a.Mode == model.ControlModeBlocking && !a.Satisfied {
			result = append(result, a)
		}
	}
	return result
}
