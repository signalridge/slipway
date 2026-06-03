package progression

import (
	"errors"
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

	if blockers := intakeClarificationBlockers(*change, intentContent); len(blockers) > 0 {
		return blockedAdvanceSummary(fromState, blockers), nil
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

	// The research substep surfaces no host skill (machine-only), but the
	// intake-clarification evidence gate must still hold before advancing into
	// planning. Evaluate it independently of ResolveNextSkill so the mutating
	// run path stays fail-closed even though no skill handoff is shown here.
	evidenceBlockers, passingSkills, err := intakeClarificationEvidenceBlockers(root, *change)
	if err != nil {
		return AdvanceSummary{}, err
	}
	if len(evidenceBlockers) > 0 {
		return blockedAdvanceSummary(fromState, evidenceBlockers), nil
	}

	fromSub := string(change.IntakeSubStep)
	if hasOpenQuestions(intentContent) {
		if change.NeedsDiscovery {
			sideEffects, cleared, err := enterPlanningFromIntake(root, change, true)
			if err != nil {
				return AdvanceSummary{}, err
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
				Reason:        "open_questions_require_discovery",
				Signals:       map[string]bool{"open_questions_detected": true},
				SideEffects:   sideEffects,
				ClearedFields: cleared,
				Message:       fmt.Sprintf("Advanced to S1_PLAN/%s for structured discovery research.", change.PlanSubStep),
			}, nil
		}

		// Non-discovery intake research stays in S0 clarification until the open
		// questions are resolved.
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

	if blockers := intakeConfirmationBlockers(intentContent); len(blockers) > 0 {
		return blockedAdvanceSummary(fromState, blockers), nil
	}

	// Transition to S1_PLAN
	fromSub := string(change.IntakeSubStep)
	sideEffects, cleared, err := enterPlanningFromIntake(root, change, change.NeedsDiscovery)
	if err != nil {
		return AdvanceSummary{}, err
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

func enterPlanningFromIntake(root string, change *model.Change, clearResearchEvidence bool) ([]SideEffect, []string, error) {
	sideEffects := []SideEffect{}
	if clearResearchEvidence {
		cleared, err := clearSkillVerification(root, *change, SkillResearchOrchestration)
		if err != nil {
			return nil, nil, err
		}
		sideEffects = append(sideEffects, cleared...)
	}

	cleared := change.EnterPlanning(change.NeedsDiscovery)
	if change.ClearAutoPassHistory() {
		cleared = append(cleared, "last_auto_passed_states")
	}
	if change.NeedsDiscovery && change.PlanSubStep == model.PlanSubStepResearch {
		if err := artifact.EnsureResearchArtifactForChange(root, *change); err != nil {
			return nil, nil, err
		}
		sideEffects = append(sideEffects, SideEffect{
			Kind:   "scaffolded_research",
			Detail: "research.md created or verified for S1_PLAN/research",
		})
	}
	return sideEffects, cleared, nil
}

func clearSkillVerification(root string, change model.Change, skillName string) ([]SideEffect, error) {
	skillName = strings.TrimSpace(skillName)
	if skillName == "" {
		return nil, nil
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(paths.GovernedBundleDir, "verification", skillName+".yaml")
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("clear stale %s verification: %w", skillName, err)
	}
	return []SideEffect{{
		Kind:   "cleared_verification",
		Detail: state.DisplayPath(root, path),
	}}, nil
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

// IntakeAdvanceBlockers returns S0 machine-step blockers that AdvanceGoverned
// would enforce after skill evidence has passed.
func IntakeAdvanceBlockers(root string, change model.Change) []model.ReasonCode {
	if change.CurrentState != model.StateS0Intake {
		return nil
	}
	intentContent, err := readIntentContent(root, change)
	if err != nil {
		return []model.ReasonCode{model.NewReasonCode("artifact_not_ready", err.Error())}
	}
	switch change.IntakeSubStep {
	case model.IntakeSubStepClarify:
		return intakeClarificationBlockers(change, intentContent)
	case model.IntakeSubStepResearch:
		// Machine-only advance: no artifact gate beyond the clarify baseline, but
		// the intake-clarification evidence must still pass so the next view's
		// advanceability matches the fail-closed run path in advanceIntakeResearch.
		evidenceBlockers, _, err := intakeClarificationEvidenceBlockers(root, change)
		if err != nil {
			return []model.ReasonCode{model.NewReasonCode("artifact_not_ready", err.Error())}
		}
		return evidenceBlockers
	case model.IntakeSubStepConfirm:
		return intakeConfirmationBlockers(intentContent)
	default:
		return []model.ReasonCode{model.NewReasonCode("intake_substep_invalid", string(change.IntakeSubStep))}
	}
}

// intakeClarificationEvidenceBlockers re-evaluates the S0 intake-clarification
// skill evidence. The machine-only research advance surfaces no host skill via
// ResolveNextSkill, so this check runs independently to keep the intake gate
// fail-closed on both the next-view advanceability and the mutating run path.
func intakeClarificationEvidenceBlockers(root string, change model.Change) ([]model.ReasonCode, map[string]model.VerificationRecord, error) {
	summaryCtx, err := state.LoadRelevantExecutionSummaryContext(root, change)
	if err != nil {
		return nil, nil, err
	}
	passing, skillBlockers, err := EvaluateRequiredSkillsForChange(
		root,
		change,
		model.StateS0Intake,
		summaryCtx.LatestRunVersion,
		false,
	)
	if err != nil {
		return nil, nil, err
	}
	return model.ReasonCodesFromSpecs(skillBlockers), passing, nil
}

func intakeClarificationBlockers(change model.Change, intentContent string) []model.ReasonCode {
	// Required sections are complexity-proportional:
	// trivial: In Scope + Acceptance Signals
	// simple+: In Scope + Out of Scope + Acceptance Signals
	var missing []string
	if !sectionNonEmpty(intentContent, "## In Scope") {
		missing = append(missing, "In Scope")
	}
	if !sectionNonEmpty(intentContent, "## Acceptance Signals") {
		missing = append(missing, "Acceptance Signals")
	}
	if change.ComplexityLevel != "trivial" && !sectionNonEmpty(intentContent, "## Out of Scope") {
		missing = append(missing, "Out of Scope")
	}
	if len(missing) == 0 {
		return nil
	}
	return []model.ReasonCode{
		model.NewReasonCode("intake_clarification_incomplete", "intent.md requires non-empty sections: "+strings.Join(missing, ", ")),
	}
}

func intakeConfirmationBlockers(intentContent string) []model.ReasonCode {
	if sectionNonEmpty(intentContent, "## Approved Summary") {
		return nil
	}
	return []model.ReasonCode{
		model.NewReasonCode("intake_confirmation_incomplete", "intent.md requires non-empty 'Approved Summary'"),
	}
}

// sectionNonEmpty checks if a markdown section contains non-placeholder content.
func sectionNonEmpty(content, heading string) bool {
	return stringutil.LastMarkdownSectionContent(content, heading) != ""
}

// hasOpenQuestions checks if the Open Questions section has unresolved items.
func hasOpenQuestions(content string) bool {
	return stringutil.HasBlockingOpenQuestions(content)
}
