package autopilot

// TransitionInput contains only facts needed by the default recommendation.
// The caller remains free to enqueue an explicit SuggestedAction first.
type TransitionInput struct {
	Kind          ActionKind
	Outcome       Outcome
	CodeChanged   bool
	ReviewEnabled bool
	Pending       []SuggestedAction
}

type Transition struct {
	Next        ActionKind
	Brief       string
	End         bool
	PauseReason PauseReason
}

// Decide applies the editable default transition table. Test/build/lint exit
// codes and review findings are deliberately absent from the input: they never
// control progression.
func Decide(input TransitionInput) Transition {
	if input.Outcome.Status == OutcomeNeedsInput {
		if input.Outcome.Pause == nil {
			return Transition{}
		}
		return Transition{PauseReason: input.Outcome.Pause.Reason}
	}
	if len(input.Pending) > 0 {
		return Transition{Next: input.Pending[0].Kind, Brief: input.Pending[0].Brief}
	}

	switch input.Kind {
	case ActionOrient, ActionClarify:
		return Transition{Next: ActionSummarize}
	case ActionImplement:
		if input.CodeChanged && input.ReviewEnabled {
			return Transition{Next: ActionReview}
		}
		return Transition{Next: ActionSummarize}
	case ActionReview:
		return Transition{Next: ActionSummarize}
	case ActionSummarize:
		return Transition{End: true}
	default:
		return Transition{End: true}
	}
}
