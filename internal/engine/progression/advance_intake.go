package progression

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
)

// advanceIntake handles S0_INTAKE substep progression.
// S0_INTAKE is a lightweight phase with no bundle, worktree, or wave sync.
func advanceIntake(root string, change *model.Change, fromState model.WorkflowState) (AdvanceSummary, error) {
	switch change.IntakeSubStep {
	case model.IntakeSubStepClarify:
		return advanceIntakeClarify(root, change, fromState)
	case model.IntakeSubStepResearch:
		return advanceIntakeResearch(root, change, fromState)
	case model.IntakeSubStepConfirm:
		return advanceIntakeConfirm(root, change, fromState)
	default:
		return AdvanceSummary{}, fmt.Errorf("unknown intake substep: %q", change.IntakeSubStep)
	}
}

// advanceIntakeClarify checks intent.md for sufficient clarification evidence
// and advances to research (if open questions) or confirm (if ready).
func advanceIntakeClarify(root string, change *model.Change, fromState model.WorkflowState) (AdvanceSummary, error) {
	intentContent, err := readIntentContent(root, *change)
	if err != nil {
		return AdvanceSummary{}, err
	}

	// Check skill evidence for intake-clarification
	nextSkillName, evidenceState := ResolveNextSkill(*change)
	var passingSkills map[string]model.VerificationRecord
	if nextSkillName != "" {
		executionSummaryCtx, err := state.LoadRelevantExecutionSummaryContext(root, *change)
		if err != nil {
			return AdvanceSummary{}, err
		}
		var skillBlockers []string
		passingSkills, skillBlockers, err = EvaluateRequiredSkillsForChange(
			root,
			*change,
			model.WorkflowState(evidenceState),
			executionSummaryCtx.LatestRunVersion,
			false,
		)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if len(skillBlockers) > 0 {
			return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(skillBlockers)), nil
		}
	}

	// Check required intent.md sections — complexity-proportional.
	// trivial: In Scope + Acceptance Signals only
	// simple+: In Scope + Out of Scope + Acceptance Signals
	hasScope := sectionNonEmpty(intentContent, "## In Scope")
	hasAcceptance := sectionNonEmpty(intentContent, "## Acceptance Signals")

	var missing []string
	if !hasScope {
		missing = append(missing, "In Scope")
	}
	if !hasAcceptance {
		missing = append(missing, "Acceptance Signals")
	}
	// Out of Scope only required for simple and above
	if change.ComplexityLevel != "trivial" {
		if !sectionNonEmpty(intentContent, "## Out of Scope") {
			missing = append(missing, "Out of Scope")
		}
	}
	if len(missing) > 0 {
		return blockedAdvanceSummary(fromState, []model.ReasonCode{
			model.NewReasonCode("intake_clarification_incomplete", "intent.md requires non-empty sections: "+strings.Join(missing, ", ")),
		}), nil
	}

	// If open questions have critical unknowns, route to research
	openQuestions := hasOpenQuestions(intentContent)
	fromSub := string(change.IntakeSubStep)
	if openQuestions {
		change.AdvanceIntakeSubStep(model.IntakeSubStepResearch)
		if err := state.SaveChange(root, *change); err != nil {
			return AdvanceSummary{}, err
		}
		return AdvanceSummary{
			Action:        "advanced",
			FromState:     fromState,
			ToState:       fromState,
			FromSubStep:   fromSub,
			ToSubStep:     string(model.IntakeSubStepResearch),
			Reason:        "open_questions_detected",
			Signals:       map[string]bool{"open_questions_detected": true},
			SideEffects:   skillEvidenceSideEffects(passingSkills),
			SkillEvidence: skillEvidenceTraceFromPassing(root, *change, passingSkills),
			Message:       "Advanced to S0_INTAKE/research for open questions.",
		}, nil
	}

	change.AdvanceIntakeSubStep(model.IntakeSubStepConfirm)
	if err := state.SaveChange(root, *change); err != nil {
		return AdvanceSummary{}, err
	}
	return AdvanceSummary{
		Action:        "advanced",
		FromState:     fromState,
		ToState:       fromState,
		FromSubStep:   fromSub,
		ToSubStep:     string(model.IntakeSubStepConfirm),
		Reason:        "clarification_complete",
		SideEffects:   skillEvidenceSideEffects(passingSkills),
		SkillEvidence: skillEvidenceTraceFromPassing(root, *change, passingSkills),
		Message:       "Advanced to S0_INTAKE/confirm.",
	}, nil
}

// advanceIntakeResearch checks if research resolved open questions.
func advanceIntakeResearch(root string, change *model.Change, fromState model.WorkflowState) (AdvanceSummary, error) {
	intentContent, err := readIntentContent(root, *change)
	if err != nil {
		return AdvanceSummary{}, err
	}

	// Check skill evidence
	nextSkillName, evidenceState := ResolveNextSkill(*change)
	var passingSkills map[string]model.VerificationRecord
	if nextSkillName != "" {
		executionSummaryCtx, err := state.LoadRelevantExecutionSummaryContext(root, *change)
		if err != nil {
			return AdvanceSummary{}, err
		}
		var skillBlockers []string
		passingSkills, skillBlockers, err = EvaluateRequiredSkillsForChange(
			root,
			*change,
			model.WorkflowState(evidenceState),
			executionSummaryCtx.LatestRunVersion,
			false,
		)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if len(skillBlockers) > 0 {
			return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(skillBlockers)), nil
		}
	}

	// If still has open questions, go back to clarify
	fromSub := string(change.IntakeSubStep)
	if hasOpenQuestions(intentContent) {
		change.AdvanceIntakeSubStep(model.IntakeSubStepClarify)
		if err := state.SaveChange(root, *change); err != nil {
			return AdvanceSummary{}, err
		}
		return AdvanceSummary{
			Action:        "advanced",
			FromState:     fromState,
			ToState:       fromState,
			FromSubStep:   fromSub,
			ToSubStep:     string(model.IntakeSubStepClarify),
			Reason:        "open_questions_remaining",
			Signals:       map[string]bool{"open_questions_detected": true},
			SideEffects:   skillEvidenceSideEffects(passingSkills),
			SkillEvidence: skillEvidenceTraceFromPassing(root, *change, passingSkills),
			Message:       "Returned to S0_INTAKE/clarify with remaining questions.",
		}, nil
	}

	change.AdvanceIntakeSubStep(model.IntakeSubStepConfirm)
	if err := state.SaveChange(root, *change); err != nil {
		return AdvanceSummary{}, err
	}
	return AdvanceSummary{
		Action:        "advanced",
		FromState:     fromState,
		ToState:       fromState,
		FromSubStep:   fromSub,
		ToSubStep:     string(model.IntakeSubStepConfirm),
		Reason:        "research_resolved",
		SideEffects:   skillEvidenceSideEffects(passingSkills),
		SkillEvidence: skillEvidenceTraceFromPassing(root, *change, passingSkills),
		Message:       "Advanced to S0_INTAKE/confirm.",
	}, nil
}

// advanceIntakeConfirm checks for Approved Summary and advances to S1_PLAN.
func advanceIntakeConfirm(root string, change *model.Change, fromState model.WorkflowState) (AdvanceSummary, error) {
	intentContent, err := readIntentContent(root, *change)
	if err != nil {
		return AdvanceSummary{}, err
	}

	if !sectionNonEmpty(intentContent, "## Approved Summary") {
		return blockedAdvanceSummary(fromState, []model.ReasonCode{
			model.NewReasonCode("intake_confirmation_incomplete", "intent.md requires non-empty 'Approved Summary'"),
		}), nil
	}

	// Transition to S1_PLAN
	fromSub := string(change.IntakeSubStep)
	cleared := change.EnterPlanning(change.NeedsDiscovery)
	if change.ClearAutoPassHistory() {
		cleared = append(cleared, "last_auto_passed_states")
	}
	sideEffects := []SideEffect{}
	if change.NeedsDiscovery && change.PlanSubStep == model.PlanSubStepResearch {
		if err := artifact.EnsureResearchArtifactForChange(root, *change); err != nil {
			return AdvanceSummary{}, err
		}
		sideEffects = append(sideEffects, SideEffect{
			Kind:   "scaffolded_research",
			Detail: "research.md created or verified for S1_PLAN/research",
		})
	}
	if err := state.SaveChange(root, *change); err != nil {
		return AdvanceSummary{}, err
	}
	return AdvanceSummary{
		Action:        "advanced",
		FromState:     fromState,
		ToState:       model.StateS1Plan,
		FromSubStep:   fromSub,
		ToSubStep:     string(change.PlanSubStep),
		Reason:        "intake_confirmed",
		SideEffects:   sideEffects,
		ClearedFields: cleared,
		Message:       fmt.Sprintf("Advanced to S1_PLAN/%s.", change.PlanSubStep),
	}, nil
}

// readIntentContent reads the intent.md file for a change.
// Returns an error if the file cannot be read — intent.md is always
// scaffolded by `slipway new`, so a missing file indicates a broken state.
func readIntentContent(root string, change model.Change) (string, error) {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return "", err
	}
	intentPath := filepath.Join(paths.GovernedBundleDir, "intent.md")
	data, err := os.ReadFile(intentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("intent.md not found at %s — run `slipway new` to create the change scaffold", intentPath)
		}
		return "", err
	}
	return string(data), nil
}

// sectionNonEmpty checks if a markdown section contains non-placeholder content.
func sectionNonEmpty(content, heading string) bool {
	return stringutil.LastMarkdownSectionContent(content, heading) != ""
}

// hasOpenQuestions checks if the Open Questions section has unresolved items.
func hasOpenQuestions(content string) bool {
	// Non-empty open questions section means there are unresolved items.
	return stringutil.LastMarkdownSectionContent(content, "## Open Questions") != ""
}
