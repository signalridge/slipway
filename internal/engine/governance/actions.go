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
	SatisfiedBy []SatisfiedBy      `json:"satisfied_by,omitempty"`
}

// SatisfiedBy names the evidence source that satisfied a governance action.
type SatisfiedBy struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	EvidenceRef string `json:"evidence_ref,omitempty"`
	Reason      string `json:"reason,omitempty"`
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
			action.Description = researchActionDescription(input.CurrentState)
			action.Satisfied = input.IntentExists && input.ScopeConfirmed && input.ResearchStructureOK

		case model.ControlDomainReview:
			action.Description = "run domain-aware compliance review and attach review evidence"
			action.Satisfied = input.DomainReviewDone
			if action.Satisfied {
				action.SatisfiedBy = append(action.SatisfiedBy, input.DomainReviewSatisfiedBy...)
			}

		case model.ControlIndependentReview:
			// Independent review is a review-scope gate (S3/S4), so it runs on
			// execution evidence — not before execution. Wording that says
			// "before further execution" contradicts the gate and the review
			// command (which requires an execution summary), misdirecting agents
			// at the S2 handoff (issue #36, comment 1).
			action.Description = "run independent review after wave execution produces execution evidence"
			action.Satisfied = input.IndependentReviewDone
			if action.Satisfied {
				action.SatisfiedBy = append(action.SatisfiedBy, input.IndependentReviewSatisfiedBy...)
			}

		case model.ControlSecurityReview:
			action.Description = "run security review after wave execution produces execution evidence"
			action.Satisfied = input.SecurityReviewDone
			if action.Satisfied {
				action.SatisfiedBy = append(action.SatisfiedBy, input.SecurityReviewSatisfiedBy...)
			}

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
	ActiveControls               []model.ControlActivation
	CurrentState                 model.WorkflowState
	HasBlockingOpenQuestions     bool
	IntentExists                 bool
	ScopeConfirmed               bool
	ResearchStructureOK          bool // research.md has all required sections (always true for non-discovery)
	DomainReviewDone             bool
	DomainReviewSatisfiedBy      []SatisfiedBy
	IndependentReviewDone        bool
	IndependentReviewSatisfiedBy []SatisfiedBy
	SecurityReviewDone           bool
	SecurityReviewSatisfiedBy    []SatisfiedBy
	WorktreePreflightDone        bool
	RollbackSectionExists        bool
}

func researchActionDescription(state model.WorkflowState) string {
	if state == model.StateS0Intake {
		return "resolve S0 intake research questions in intent.md and confirm scope; S1 research.md is required after intake"
	}
	return "complete research.md, resolve unknowns, and confirm scope via intake before continuing"
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
