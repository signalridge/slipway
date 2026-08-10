// Issue-source refresh, amendment candidates, and the transition rules that
// keep a Run's pinned source, its lineage, and its accepted chapter history
// consistent across resume.

package autopilot

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
)

func refreshIssueSource(run *Run, durableRun Run, refreshed SourceCandidateInput, budget *int) (string, bool, error) {
	current := clonePinnedSourceValue(*run.PinnedSource)
	if refreshed.Provider != current.Provider || refreshed.Host != current.Host {
		return "", false, resumeProtocolError(*run, "source_provider_mismatch", "refreshed source provider and host must match the pinned source")
	}
	if refreshed.IssueID != current.IssueID {
		err := resumeProtocolError(*run, "source_issue_mismatch", "refreshed source belongs to a different issue; start a new run")
		err.Next = startRunNext(run.Workspace, run.Goal, run.InitialBudget, run.ReviewEnabled, true)
		err.Details["pinned_issue_id"] = current.IssueID
		err.Details["refreshed_issue_id"] = refreshed.IssueID
		return "", false, err
	}

	projectionChanged := sourceProjectionChanged(current, refreshed)
	_, refreshed = mergeRefreshedProjection(current, refreshed)
	if len(refreshed.URLAliases) > maxSourceURLAliases {
		err := resumeProtocolError(
			*run,
			"source_alias_limit",
			"source transfer history exceeds the URL alias limit; start a new run from the refreshed source",
		)
		err.Next = startRunNext(run.Workspace, run.Goal, run.InitialBudget, run.ReviewEnabled, true)
		return "", false, err
	}
	if err := validateSourceCandidateInput(refreshed); err != nil {
		return "", false, resumeProtocolError(
			*run,
			"invalid_source_candidate",
			"merged refreshed source is invalid: "+err.Error(),
		)
	}

	if !refreshed.Valid {
		candidate := newSourceCandidate(refreshed)
		run.SourceCandidate = &candidate
		invalidateOutstandingResumeState(run)
		run.State = RunPaused
		run.PauseReason = PauseDecisionRequired
		run.Observations = append(run.Observations, "source_candidate_invalid")
		run.LastResumeResult = &ResumeResult{
			Operation:     ResumeOperationSourceCandidate,
			BudgetApplied: false,
			CandidateID:   candidate.CandidateID,
		}
		return ResumeOperationSourceCandidate, true, nil
	}

	if err := validateAcceptedSourceCommentHistory(*run, *refreshed.Snapshot); err != nil {
		return "", false, resumeProtocolError(
			*run,
			"source_history_in_place_edit",
			err.Error(),
		)
	}
	if refreshed.Snapshot.ManifestRevision != current.ManifestRevision {
		if refreshed.Snapshot.ParentRequirementsRevision != current.RequirementsRevision {
			err := resumeProtocolError(
				*run,
				"source_history_fork",
				"amended source parent_requirements_revision does not match the pinned requirements revision",
			)
			err.Next = startRunNext(run.Workspace, run.Goal, run.InitialBudget, run.ReviewEnabled, true)
			return "", false, err
		}
		if err := validateReplacementOnlyAmendment(current, *refreshed.Snapshot); err != nil {
			return "", false, resumeProtocolError(
				*run,
				"source_history_in_place_edit",
				err.Error(),
			)
		}
		candidate := newSourceCandidate(refreshed)
		run.SourceCandidate = &candidate
		invalidateOutstandingResumeState(run)
		run.State = RunPaused
		run.PauseReason = PauseDecisionRequired
		run.Observations = append(run.Observations, "source_amendment_candidate")
		run.LastResumeResult = &ResumeResult{
			Operation:     ResumeOperationSourceCandidate,
			BudgetApplied: false,
			CandidateID:   candidate.CandidateID,
		}
		return ResumeOperationSourceCandidate, true, nil
	}
	if refreshed.RequirementsRevision != current.RequirementsRevision {
		return "", false, resumeProtocolError(
			*run,
			"source_integrity_mismatch",
			"requirements revision changed without a new manifest revision",
		)
	}

	run.PinnedSource = clonePinnedSource(refreshed.Snapshot)
	invalidateOutstandingResumeState(run)
	applyResumeBudget(run, budget)
	switch {
	case refreshed.SourceRevision != current.SourceRevision:
		run.Observations = append(run.Observations, "source_refreshed_non_material")
	case projectionChanged:
		run.Observations = append(run.Observations, "source_projection_drift")
	default:
		run.Observations = append(run.Observations, "source_refreshed_unchanged")
	}
	run.LastResumeResult = &ResumeResult{Operation: ResumeOperationSourceRefreshed, BudgetApplied: true}
	if err := issueAction(run, durableRun, ActionOrient, "Re-orient against the refreshed source snapshot before selecting further work."); err != nil {
		return "", false, err
	}
	return ResumeOperationSourceRefreshed, true, nil
}

func resolveSourceCandidate(run *Run, durableRun Run, options ResumeOptions) (string, bool, error) {
	candidate := cloneSourceCandidate(*run.SourceCandidate)
	if options.SourceChoice == SourceChoiceAdopt && !candidate.Valid {
		return "", false, resumeProtocolError(*run, "invalid_source_candidate_choice", "invalid source candidate cannot be adopted")
	}
	if candidate.IssueID != run.PinnedSource.IssueID {
		return "", false, resumeProtocolError(*run, "source_issue_mismatch", "source candidate no longer matches the pinned issue")
	}

	operation := ResumeOperationSourcePinned
	if options.SourceChoice == SourceChoiceAdopt {
		oldRequirementsRevision := run.PinnedSource.RequirementsRevision
		run.PinnedSource = clonePinnedSource(candidate.Snapshot)
		if run.PinnedSource.RequirementsRevision != oldRequirementsRevision {
			markActiveAnswersSuperseded(run, oldRequirementsRevision, "requirements:"+run.PinnedSource.RequirementsRevision)
		}
		operation = ResumeOperationSourceAmended
	} else {
		projected, _ := mergeRefreshedProjection(*run.PinnedSource, candidate.SourceCandidateInput)
		run.PinnedSource = &projected
	}
	recordAcceptedSourceComments(run, run.PinnedSource)
	invalidateOutstandingResumeState(run)
	run.SourceCandidate = nil
	applyResumeBudget(run, options.Budget)
	run.Observations = append(run.Observations, operation)
	resumeResult := ResumeResult{Operation: operation, BudgetApplied: true, CandidateID: candidate.CandidateID}
	run.LastResumeResult = &resumeResult
	if err := issueAction(run, durableRun, ActionOrient, "Re-orient after the explicit source amendment decision before selecting further work."); err != nil {
		return "", false, err
	}
	actionID := ""
	if run.CurrentAction != nil {
		actionID = run.CurrentAction.ActionID
	}
	receipt := SourceChoiceReceipt{
		CandidateID:       candidate.CandidateID,
		Choice:            options.SourceChoice,
		ResultingActionID: actionID,
		At:                time.Now().UTC(),
	}
	run.LastSourceChoice = &receipt
	run.sourceChoiceHistory = append(run.sourceChoiceHistory, sourceChoiceResolution{
		Receipt: receipt,
		Result:  resumeResult,
	})
	return operation, true, nil
}

func findSourceChoiceResolution(run Run, candidateID string) *sourceChoiceResolution {
	for index := range run.sourceChoiceHistory {
		if run.sourceChoiceHistory[index].Receipt.CandidateID == candidateID {
			return &run.sourceChoiceHistory[index]
		}
	}
	return nil
}

func validateSourceChoiceResolution(resolution sourceChoiceResolution) error {
	receipt := resolution.Receipt
	result := resolution.Result
	if receipt.CandidateID == "" ||
		(receipt.Choice != SourceChoicePinned && receipt.Choice != SourceChoiceAdopt) ||
		receipt.ResultingActionID == "" || receipt.At.IsZero() {
		return errors.New("invalid source choice resolution receipt")
	}
	if !result.BudgetApplied || result.CandidateID != receipt.CandidateID {
		return errors.New("invalid source choice resolution result")
	}
	switch receipt.Choice {
	case SourceChoicePinned:
		if result.Operation != ResumeOperationSourcePinned {
			return errors.New("pinned source choice resolution has the wrong operation")
		}
	case SourceChoiceAdopt:
		if result.Operation != ResumeOperationSourceAmended {
			return errors.New("adopted source choice resolution has the wrong operation")
		}
	}
	return nil
}

func newSourceCandidate(input SourceCandidateInput) SourceCandidate {
	return SourceCandidate{
		CandidateID:          uuid.NewString(),
		SourceCandidateInput: cloneSourceCandidateInput(input),
		CreatedAt:            time.Now().UTC(),
	}
}

func cloneSourceCandidate(candidate SourceCandidate) SourceCandidate {
	candidate.SourceCandidateInput = cloneSourceCandidateInput(candidate.SourceCandidateInput)
	return candidate
}

func validateRunSourceTransition(eventType string, before, after Run) error {
	if before.ID == "" {
		if eventType != "run_started" {
			return errors.New("new run requires the run_started event")
		}
		if after.SourceCandidate != nil || after.LastSourceChoice != nil || len(after.sourceChoiceHistory) != 0 {
			return errors.New("new run cannot begin with source candidate state")
		}
		return nil
	}
	if before.PinnedSource == nil {
		if after.PinnedSource != nil || after.SourceCandidate != nil || after.LastSourceChoice != nil || len(after.sourceChoiceHistory) != 0 {
			return errors.New("ad-hoc run cannot acquire issue source state")
		}
		return nil
	}
	if after.PinnedSource == nil {
		return errors.New("issue-bound run cannot clear its pinned source")
	}
	if err := validateSameSourceIssue(*before.PinnedSource, *after.PinnedSource); err != nil {
		return err
	}

	switch {
	case before.SourceCandidate == nil && after.SourceCandidate != nil:
		if eventType != ResumeOperationSourceCandidate {
			return errors.New("source candidate creation requires the source_candidate_created event")
		}
		if !reflect.DeepEqual(before.PinnedSource, after.PinnedSource) {
			return errors.New("source candidate creation cannot mutate pinned source")
		}
		if err := validateCandidateLineage(before, *after.SourceCandidate); err != nil {
			return err
		}
		if after.LastResumeResult == nil ||
			after.LastResumeResult.Operation != ResumeOperationSourceCandidate ||
			after.LastResumeResult.BudgetApplied ||
			after.LastResumeResult.CandidateID != after.SourceCandidate.CandidateID {
			return errors.New("source candidate creation requires a matching resume receipt")
		}
		if !reflect.DeepEqual(before.LastSourceChoice, after.LastSourceChoice) ||
			!reflect.DeepEqual(before.sourceChoiceHistory, after.sourceChoiceHistory) {
			return errors.New("source candidate creation cannot rewrite prior choice receipts")
		}
		if after.State != RunPaused || after.PauseReason != PauseDecisionRequired ||
			after.CurrentAction != nil || len(after.Actions) != len(before.Actions) {
			return errors.New("source candidate creation must pause without issuing an action")
		}
		if len(after.PendingActions) != 0 || after.PendingDestructiveRequest != nil ||
			after.DestructiveGrant != nil {
			return errors.New("source candidate creation must clear outstanding execution state")
		}
		if before.CurrentAction != nil {
			record := findActionRecord(&after, before.CurrentAction.ActionID)
			if record == nil || !record.Voided {
				return errors.New("source candidate creation must void the outstanding action")
			}
		}
		return nil
	case before.SourceCandidate != nil && after.SourceCandidate != nil:
		if !reflect.DeepEqual(before.SourceCandidate, after.SourceCandidate) ||
			!reflect.DeepEqual(before.PinnedSource, after.PinnedSource) ||
			!reflect.DeepEqual(before.LastSourceChoice, after.LastSourceChoice) ||
			!reflect.DeepEqual(before.LastResumeResult, after.LastResumeResult) ||
			!reflect.DeepEqual(before.sourceChoiceHistory, after.sourceChoiceHistory) {
			return errors.New("pending source candidate is immutable until resolved")
		}
		if after.CurrentAction != nil || len(after.Actions) != len(before.Actions) {
			return errors.New("pending source candidate cannot issue an action before resolution")
		}
		return nil
	case before.SourceCandidate != nil && after.SourceCandidate == nil:
		return validateCandidateResolution(eventType, before, after)
	}

	if !reflect.DeepEqual(before.LastSourceChoice, after.LastSourceChoice) ||
		!reflect.DeepEqual(before.sourceChoiceHistory, after.sourceChoiceHistory) {
		return errors.New("source choice receipt requires a current candidate")
	}
	pinnedChanged := !reflect.DeepEqual(before.PinnedSource, after.PinnedSource)
	switch eventType {
	case ResumeOperationSourceRefreshed:
		if before.PinnedSource.ManifestRevision != after.PinnedSource.ManifestRevision ||
			before.PinnedSource.RequirementsRevision != after.PinnedSource.RequirementsRevision {
			return errors.New("pinned manifest can change only by adopting its current candidate")
		}
		if err := validateAcceptedSourceCommentHistory(before, *after.PinnedSource); err != nil {
			return err
		}
		if err := validateSourceAliasTransition(*before.PinnedSource, *after.PinnedSource); err != nil {
			return err
		}
		if after.LastResumeResult == nil ||
			after.LastResumeResult.Operation != ResumeOperationSourceRefreshed ||
			!after.LastResumeResult.BudgetApplied ||
			after.LastResumeResult.CandidateID != "" {
			return errors.New("source refresh requires a matching resume receipt")
		}
		return validateFreshSourceOrient(before, after)
	case ResumeOperationSourceRefreshSkipped:
		if pinnedChanged {
			return errors.New("skipping source refresh cannot mutate pinned source")
		}
		if after.LastResumeResult == nil ||
			after.LastResumeResult.Operation != ResumeOperationSourceRefreshSkipped ||
			!after.LastResumeResult.BudgetApplied ||
			after.LastResumeResult.CandidateID != "" {
			return errors.New("skipped source refresh requires a matching resume receipt")
		}
		return validateFreshSourceOrient(before, after)
	default:
		if pinnedChanged {
			if before.PinnedSource.ManifestRevision != after.PinnedSource.ManifestRevision ||
				before.PinnedSource.RequirementsRevision != after.PinnedSource.RequirementsRevision {
				return errors.New("pinned manifest can change only by adopting its current candidate")
			}
			return errors.New("pinned source projection can change only in a source_refreshed event")
		}
		if !reflect.DeepEqual(before.LastResumeResult, after.LastResumeResult) {
			return errors.New("source resume receipt requires its matching source event")
		}
	}
	return nil
}

func validateSameSourceIssue(current, next PinnedSource) error {
	if current.Provider != next.Provider || current.Host != next.Host || current.IssueID != next.IssueID {
		return errors.New("source transition changed provider, host, or issue identity")
	}
	return nil
}

func validateCandidateLineage(run Run, candidate SourceCandidate) error {
	current := *run.PinnedSource
	if candidate.Provider != current.Provider || candidate.Host != current.Host || candidate.IssueID != current.IssueID {
		return errors.New("source candidate does not belong to the pinned issue")
	}
	if !candidate.Valid {
		return nil
	}
	if candidate.Snapshot == nil {
		return errors.New("valid source candidate has no snapshot")
	}
	if candidate.Snapshot.ManifestRevision == current.ManifestRevision {
		return errors.New("valid source candidate must publish a new manifest head")
	}
	if candidate.Snapshot.ParentRequirementsRevision != current.RequirementsRevision {
		return errors.New("source candidate parent does not match pinned requirements")
	}
	if err := validateSourceAliasTransition(current, *candidate.Snapshot); err != nil {
		return err
	}
	if err := validateReplacementOnlyAmendment(current, *candidate.Snapshot); err != nil {
		return err
	}
	if err := validateAcceptedSourceCommentHistory(run, *candidate.Snapshot); err != nil {
		return err
	}
	return nil
}

func validateCandidateResolution(eventType string, before, after Run) error {
	candidate := *before.SourceCandidate
	if len(after.sourceChoiceHistory) != len(before.sourceChoiceHistory)+1 {
		return errors.New("source candidate resolution must append exactly one choice receipt")
	}
	for index := range before.sourceChoiceHistory {
		if before.sourceChoiceHistory[index] != after.sourceChoiceHistory[index] {
			return errors.New("source candidate resolution cannot rewrite prior choice receipts")
		}
	}
	resolution := after.sourceChoiceHistory[len(after.sourceChoiceHistory)-1]
	receipt := &resolution.Receipt
	if !reflect.DeepEqual(after.LastSourceChoice, receipt) {
		return errors.New("source candidate resolution requires a fresh choice receipt")
	}
	if receipt.CandidateID != candidate.CandidateID {
		return errors.New("source choice receipt does not match the resolved candidate")
	}
	if after.LastResumeResult == nil || !reflect.DeepEqual(*after.LastResumeResult, resolution.Result) ||
		!after.LastResumeResult.BudgetApplied ||
		after.LastResumeResult.CandidateID != candidate.CandidateID {
		return errors.New("source candidate resolution requires a matching resume receipt")
	}
	if after.CurrentAction == nil || receipt.ResultingActionID != after.CurrentAction.ActionID {
		return errors.New("source choice receipt does not match the resulting action")
	}
	if err := validateFreshSourceOrient(before, after); err != nil {
		return err
	}
	switch receipt.Choice {
	case SourceChoicePinned:
		if eventType != ResumeOperationSourcePinned {
			return errors.New("pinned source choice requires the source_pinned event")
		}
		if after.LastResumeResult.Operation != ResumeOperationSourcePinned {
			return errors.New("pinned source choice has the wrong resume operation")
		}
		expected, _ := mergeRefreshedProjection(*before.PinnedSource, candidate.SourceCandidateInput)
		if !reflect.DeepEqual(&expected, after.PinnedSource) {
			return errors.New("pinned source choice must retain accepted content while applying the candidate projection")
		}
	case SourceChoiceAdopt:
		if eventType != ResumeOperationSourceAmended {
			return errors.New("adopted source choice requires the source_amended event")
		}
		if after.LastResumeResult.Operation != ResumeOperationSourceAmended {
			return errors.New("adopted source choice has the wrong resume operation")
		}
		if !candidate.Valid || candidate.Snapshot == nil {
			return errors.New("invalid source candidate cannot be adopted")
		}
		if !reflect.DeepEqual(candidate.Snapshot, after.PinnedSource) {
			return errors.New("adopted pinned source does not equal the chosen candidate")
		}
		if err := validateCandidateLineage(before, candidate); err != nil {
			return err
		}
	default:
		return errors.New("source choice receipt has an invalid choice")
	}
	return nil
}

func validateFreshSourceOrient(before, after Run) error {
	if after.State != RunActive || after.PauseReason != "" || after.CurrentAction == nil ||
		after.CurrentAction.Kind != ActionOrient {
		return errors.New("source resume must issue a fresh active Orient action")
	}
	if len(after.Actions) != len(before.Actions)+1 {
		return errors.New("source resume must append exactly one fresh Orient action")
	}
	record := after.Actions[len(before.Actions)]
	if !reflect.DeepEqual(record.Action, *after.CurrentAction) || record.Outcome != nil ||
		record.OutcomePayloadSHA256 != "" || record.Voided || record.Skipped {
		return errors.New("source resume resulting action is not a fresh pending action")
	}
	if before.CurrentAction != nil {
		prior := findActionRecord(&after, before.CurrentAction.ActionID)
		if prior == nil || !prior.Voided {
			return errors.New("source resume must void the outstanding action")
		}
	}
	if len(after.PendingActions) != 0 || after.PendingDestructiveRequest != nil ||
		after.DestructiveGrant != nil {
		return errors.New("source resume must clear outstanding execution state")
	}
	return nil
}

func validateAcceptedSourceCommentHistory(run Run, source PinnedSource) error {
	if run.acceptedSourceComments == nil || run.acceptedSourceDatabaseIDs == nil {
		recordAcceptedSourceComments(&run, run.PinnedSource)
	}
	for _, section := range source.Sections {
		nodeID := section.Provenance.CommentNodeID
		databaseID := section.Provenance.CommentDatabaseID
		if prior, ok := run.acceptedSourceComments[nodeID]; ok {
			if prior.Provenance.CommentDatabaseID != databaseID {
				return fmt.Errorf(
					"accepted comment node %q was rebound from database id %d to %d",
					nodeID,
					prior.Provenance.CommentDatabaseID,
					databaseID,
				)
			}
			if !sameAcceptedSection(prior, section) {
				return fmt.Errorf(
					"accepted comment node %q was changed in place; publish a replacement comment",
					nodeID,
				)
			}
		}
		if priorNodeID, ok := run.acceptedSourceDatabaseIDs[databaseID]; ok && priorNodeID != nodeID {
			return fmt.Errorf(
				"comment database id %d was rebound from node %q to %q",
				databaseID,
				priorNodeID,
				nodeID,
			)
		}
	}
	return nil
}

func recordAcceptedSourceComments(run *Run, source *PinnedSource) {
	if source == nil {
		return
	}
	if run.acceptedSourceComments == nil {
		run.acceptedSourceComments = make(map[string]PinnedSourceSection)
	}
	if run.acceptedSourceDatabaseIDs == nil {
		run.acceptedSourceDatabaseIDs = make(map[int64]string)
	}
	for _, section := range source.Sections {
		nodeID := section.Provenance.CommentNodeID
		databaseID := section.Provenance.CommentDatabaseID
		if _, exists := run.acceptedSourceComments[nodeID]; !exists {
			run.acceptedSourceComments[nodeID] = section
		}
		if _, exists := run.acceptedSourceDatabaseIDs[databaseID]; !exists {
			run.acceptedSourceDatabaseIDs[databaseID] = nodeID
		}
	}
}

func sourceProjectionChanged(current PinnedSource, refreshed SourceCandidateInput) bool {
	return current.RepositoryID != refreshed.RepositoryID ||
		current.IssueNumber != refreshed.IssueNumber ||
		current.CanonicalURL != refreshed.CanonicalURL ||
		!sourceParentsEqual(current.Parent, refreshed.Parent)
}

func mergeRefreshedProjection(current PinnedSource, refreshed SourceCandidateInput) (PinnedSource, SourceCandidateInput) {
	aliases := append(make([]string, 0, len(current.URLAliases)+1), current.URLAliases...)
	if current.CanonicalURL != refreshed.CanonicalURL {
		aliases = appendUniqueString(aliases, current.CanonicalURL)
	}
	filteredAliases := aliases[:0]
	for _, alias := range aliases {
		if alias != refreshed.CanonicalURL {
			filteredAliases = append(filteredAliases, alias)
		}
	}
	aliases = append([]string(nil), filteredAliases...)

	projected := clonePinnedSourceValue(current)
	projected.RepositoryID = refreshed.RepositoryID
	projected.IssueNumber = refreshed.IssueNumber
	projected.CanonicalURL = refreshed.CanonicalURL
	projected.URLAliases = append(make([]string, 0, len(aliases)), aliases...)
	projected.Parent = cloneSourceParent(refreshed.Parent)
	projected.SourceRevision = sourceRevisionFromIdentity(
		projected.Host,
		projected.RepositoryID,
		projected.IssueID,
		projected.Title,
		projected.ManifestRevision,
	)

	refreshed = cloneSourceCandidateInput(refreshed)
	refreshed.URLAliases = append(make([]string, 0, len(aliases)), aliases...)
	if refreshed.Snapshot != nil {
		refreshed.Snapshot.RepositoryID = refreshed.RepositoryID
		refreshed.Snapshot.IssueNumber = refreshed.IssueNumber
		refreshed.Snapshot.CanonicalURL = refreshed.CanonicalURL
		refreshed.Snapshot.URLAliases = append(make([]string, 0, len(aliases)), aliases...)
		refreshed.Snapshot.Parent = cloneSourceParent(refreshed.Parent)
	}
	return projected, refreshed
}

func validateSourceAliasTransition(current, next PinnedSource) error {
	expected := append(make([]string, 0, len(current.URLAliases)+1), current.URLAliases...)
	if current.CanonicalURL != next.CanonicalURL {
		expected = appendUniqueString(expected, current.CanonicalURL)
	}
	filtered := expected[:0]
	for _, alias := range expected {
		if alias != next.CanonicalURL {
			filtered = append(filtered, alias)
		}
	}
	if !stringSlicesEqual(filtered, next.URLAliases) {
		return errors.New("source refresh rewrote URL alias history")
	}
	return nil
}
