// Run state invariants and the destructive-authorization helpers they guard.
// These run on both the mutation path and journal replay, so a rule added here
// also applies to every Run an earlier version already wrote.

package autopilot

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

func cloneDestructiveRequest(request *DestructiveRequest) *DestructiveRequest {
	if request == nil {
		return nil
	}
	clone := *request
	clone.Targets = append([]DestructiveTarget(nil), request.Targets...)
	return &clone
}

func cloneDestructiveAuthorization(authorization *DestructiveAuthorization) *DestructiveAuthorization {
	if authorization == nil {
		return nil
	}
	clone := *authorization
	clone.Targets = append([]DestructiveTarget(nil), authorization.Targets...)
	return &clone
}

func clearDestructiveState(run *Run) {
	run.PendingDestructiveRequest = nil
	run.DestructiveGrant = nil
}

func voidCurrentAction(run *Run) {
	if run.CurrentAction == nil {
		return
	}
	if record := findActionRecord(run, run.CurrentAction.ActionID); record != nil {
		record.Voided = true
	}
}

func currentDestructiveRequest(run Run, actionID string) (DestructiveRequest, error) {
	if run.PendingDestructiveRequest == nil {
		return DestructiveRequest{}, protocolRunError(run, "destructive_request_missing", "current destructive request is missing")
	}
	request, err := NormalizeDestructiveRequest(*run.PendingDestructiveRequest)
	if err != nil {
		return DestructiveRequest{}, protocolRunError(run, "invalid_destructive_request", err.Error())
	}
	record := findActionRecord(&run, actionID)
	if record == nil || record.Outcome == nil || record.Outcome.Pause == nil || record.Outcome.Pause.DestructiveRequest == nil {
		return DestructiveRequest{}, protocolRunError(run, "destructive_request_missing", "waiting action does not contain the current destructive request")
	}
	outcomeRequest, err := NormalizeDestructiveRequest(*record.Outcome.Pause.DestructiveRequest)
	if err != nil {
		return DestructiveRequest{}, protocolRunError(run, "invalid_destructive_request", err.Error())
	}
	if !reflect.DeepEqual(request, outcomeRequest) {
		return DestructiveRequest{}, protocolRunError(run, "destructive_scope_changed", "persisted destructive scope differs from the waiting action")
	}
	if run.DestructiveGrant != nil {
		return DestructiveRequest{}, protocolRunError(run, "destructive_grant_conflict", "a destructive pause cannot retain an older grant")
	}
	return request, nil
}

func validateDestructiveGrant(authorization DestructiveAuthorization, request DestructiveRequest, originatingActionID string) error {
	if err := validateDestructiveAuthorization(authorization); err != nil {
		return err
	}
	normalized, err := NormalizeDestructiveRequest(request)
	if err != nil {
		return err
	}
	if authorization.OriginatingActionID != originatingActionID {
		return errors.New("destructive grant originating_action_id does not match the waiting action")
	}
	if authorization.RequestID != normalized.RequestID ||
		authorization.ScopeVersion != DestructiveScopeVersion ||
		authorization.ScopeSHA256 != normalized.ScopeSHA256 ||
		authorization.Impact != normalized.Impact ||
		!reflect.DeepEqual(authorization.Targets, normalized.Targets) {
		return errors.New("destructive grant does not match the current request field-for-field")
	}
	return nil
}

func validateRunResumeReceipt(run Run) error {
	result := run.LastResumeResult
	if result == nil {
		return nil
	}

	switch result.Operation {
	case ResumeOperationAdHoc:
		if !result.BudgetApplied || result.CandidateID != "" || run.PinnedSource != nil {
			return errors.New("ad-hoc resume receipt is inconsistent")
		}
	case ResumeOperationSourceRefreshed, ResumeOperationSourceRefreshSkipped:
		if !result.BudgetApplied || result.CandidateID != "" || run.PinnedSource == nil {
			return errors.New("source refresh receipt is inconsistent")
		}
	case ResumeOperationSourceCandidate:
		if result.BudgetApplied || result.CandidateID == "" || run.SourceCandidate == nil ||
			run.SourceCandidate.CandidateID != result.CandidateID {
			return errors.New("source candidate receipt is inconsistent")
		}
	case ResumeOperationSourceAmended:
		if !result.BudgetApplied || result.CandidateID == "" || run.SourceCandidate != nil ||
			run.LastSourceChoice == nil || run.LastSourceChoice.Choice != SourceChoiceAdopt ||
			run.LastSourceChoice.CandidateID != result.CandidateID {
			return errors.New("source amendment receipt is inconsistent")
		}
	case ResumeOperationSourcePinned:
		if !result.BudgetApplied || run.PinnedSource == nil {
			return errors.New("pinned source receipt is inconsistent")
		}
		if result.CandidateID != "" &&
			(run.SourceCandidate != nil || run.LastSourceChoice == nil ||
				run.LastSourceChoice.Choice != SourceChoicePinned ||
				run.LastSourceChoice.CandidateID != result.CandidateID) {
			return errors.New("pinned candidate receipt is inconsistent")
		}
	default:
		return fmt.Errorf("resume receipt has unknown operation %q", result.Operation)
	}
	return nil
}

func validateRunReceipts(run Run) error {
	if err := validateRunResumeReceipt(run); err != nil {
		return err
	}
	choiceCandidates := make(map[string]struct{}, len(run.sourceChoiceHistory))
	for index, resolution := range run.sourceChoiceHistory {
		if err := validateSourceChoiceResolution(resolution); err != nil {
			return fmt.Errorf("source choice resolution %d: %w", index, err)
		}
		if _, exists := choiceCandidates[resolution.Receipt.CandidateID]; exists {
			return fmt.Errorf("source choice resolution candidate_id %q is duplicated", resolution.Receipt.CandidateID)
		}
		choiceCandidates[resolution.Receipt.CandidateID] = struct{}{}
	}
	if len(run.sourceChoiceHistory) > 0 {
		latest := run.sourceChoiceHistory[len(run.sourceChoiceHistory)-1].Receipt
		if run.LastSourceChoice == nil || *run.LastSourceChoice != latest {
			return errors.New("last source choice does not match append-only history")
		}
	} else if run.LastSourceChoice != nil {
		return errors.New("last source choice has no append-only history")
	}
	answerActions := make(map[string]struct{}, len(run.Answers))
	for index, answer := range run.Answers {
		if strings.TrimSpace(answer.ActionID) == "" || !validSHA256(answer.PayloadSHA256) || answer.At.IsZero() {
			return fmt.Errorf("answer receipt %d is malformed", index)
		}
		if _, exists := answerActions[answer.ActionID]; exists {
			return fmt.Errorf("answer receipt action_id %q is duplicated", answer.ActionID)
		}
		answerActions[answer.ActionID] = struct{}{}
		if answer.ConfirmDestructive {
			if !validSHA256(answer.ScopeSHA256) {
				return fmt.Errorf("answer receipt %d has malformed destructive scope", index)
			}
		} else if answer.ScopeSHA256 != "" {
			return fmt.Errorf("answer receipt %d has scope without confirmation", index)
		}
		options := AnswerOptions{
			Text: answer.Text, ConfirmDestructive: answer.ConfirmDestructive, ScopeSHA256: answer.ScopeSHA256,
		}
		digest, err := answerPayloadSHA256(answer.ActionID, options)
		if err != nil {
			return fmt.Errorf("answer receipt %d payload digest does not match", index)
		}
		if digest != answer.PayloadSHA256 {
			return fmt.Errorf("answer receipt %d payload digest does not match", index)
		}
	}
	for index, record := range run.Actions {
		if record.Outcome == nil {
			if record.OutcomePayloadSHA256 != "" {
				return fmt.Errorf("action record %d has an outcome digest without an outcome", index)
			}
			continue
		}
		if !validSHA256(record.OutcomePayloadSHA256) {
			return fmt.Errorf("action record %d has malformed outcome payload digest", index)
		}
	}
	return nil
}

func validateCurrentActionState(run Run) error {
	if run.CurrentAction == nil {
		return nil
	}
	// A stopped or ended Run publishing a current Action is deliberately not an
	// invariant here. Stop withdraws the Action, so this version cannot produce
	// that state, but these validators also run during replay: rejecting it
	// would strand every Run an earlier version stopped, reporting it as an
	// invalid journal that can never be resumed. Stranding a user's Run is a
	// worse outcome than replaying one stale Action that the next resume voids.
	if err := run.CurrentAction.Validate(); err != nil {
		return fmt.Errorf("current action is invalid: %w", err)
	}
	record := findActionRecord(&run, run.CurrentAction.ActionID)
	if record == nil || !reflect.DeepEqual(record.Action, *run.CurrentAction) {
		return errors.New("current action does not match its run history record")
	}
	if record.Voided || record.Skipped {
		return errors.New("current action cannot reference a voided or skipped record")
	}
	if record.Outcome != nil && record.Outcome.Status != OutcomeNeedsInput {
		return errors.New("current action cannot reference a completed action record")
	}
	if run.PinnedSource == nil {
		if run.CurrentAction.Source != nil || run.CurrentAction.Requirements != nil {
			return errors.New("ad-hoc current action cannot carry issue source")
		}
		return nil
	}

	expectedSource := ActionSource{
		Kind:                 ActionSourceChangeIssue,
		CanonicalURL:         run.PinnedSource.CanonicalURL,
		IssueID:              run.PinnedSource.IssueID,
		SourceRevision:       run.PinnedSource.SourceRevision,
		ManifestRevision:     run.PinnedSource.ManifestRevision,
		RequirementsRevision: run.PinnedSource.RequirementsRevision,
	}
	if run.CurrentAction.Source == nil || !reflect.DeepEqual(*run.CurrentAction.Source, expectedSource) {
		return errors.New("current action source does not match pinned source")
	}
	expectedRequirements := actionRequirements(
		run.Workspace,
		run.CurrentAction.RunID,
		run.CurrentAction.ActionID,
		*run.PinnedSource,
	)
	if run.CurrentAction.Requirements == nil ||
		!reflect.DeepEqual(*run.CurrentAction.Requirements, expectedRequirements) {
		return errors.New("current action requirements do not match pinned source")
	}
	return nil
}

func validateRunDestructiveState(run Run) error {
	if err := validateRunReceipts(run); err != nil {
		return err
	}
	if err := validateCurrentActionState(run); err != nil {
		return err
	}
	if run.PendingDestructiveRequest == nil && run.DestructiveGrant != nil {
		return errors.New("destructive grant requires a pending destructive request")
	}
	if run.PendingDestructiveRequest != nil {
		if _, err := NormalizeDestructiveRequest(*run.PendingDestructiveRequest); err != nil {
			return err
		}
	}
	if run.State == RunStopped || run.State == RunEnded ||
		run.PauseReason == PauseDecisionRequired || run.PauseReason == PauseEnvironmentUnavailable {
		if run.PendingDestructiveRequest != nil || run.DestructiveGrant != nil {
			return errors.New("current run state cannot retain destructive request or grant")
		}
	}
	if run.State == RunPaused && run.PauseReason == PauseDestructiveConfirm {
		if run.CurrentAction == nil || run.PendingDestructiveRequest == nil || run.DestructiveGrant != nil {
			return errors.New("destructive pause requires one current action and request without a grant")
		}
		if _, err := currentDestructiveRequest(run, run.CurrentAction.ActionID); err != nil {
			return err
		}
	}
	if run.DestructiveGrant != nil {
		if run.State != RunActive || run.CurrentAction == nil || run.CurrentAction.Kind != ActionImplement || run.CurrentAction.DestructiveAuthorization == nil {
			return errors.New("destructive grant requires the current authorized implement action")
		}
		if err := validateDestructiveGrant(*run.DestructiveGrant, *run.PendingDestructiveRequest, run.DestructiveGrant.OriginatingActionID); err != nil {
			return err
		}
		if !reflect.DeepEqual(*run.CurrentAction.DestructiveAuthorization, *run.DestructiveGrant) {
			return errors.New("current implement authorization differs from the one-shot grant")
		}
	} else if run.State == RunActive && run.CurrentAction != nil && run.CurrentAction.DestructiveAuthorization != nil {
		return errors.New("active action authorization requires a current one-shot grant")
	}
	if run.PendingDestructiveRequest != nil && run.PauseReason != PauseDestructiveConfirm && run.DestructiveGrant == nil {
		return errors.New("pending destructive request is not attached to a pause or grant")
	}
	return nil
}
