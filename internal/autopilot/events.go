package autopilot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/signalridge/slipway/internal/runstore"
)

const runEventVersion = 2

type runInitialization struct {
	Goal              string                     `json:"goal"`
	Workspace         string                     `json:"workspace"`
	WorkspaceIdentity runstore.WorkspaceIdentity `json:"workspace_identity"`
	ReviewEnabled     bool                       `json:"review_enabled"`
	InitialBudget     int                        `json:"initial_budget"`
	InitialGit        runstore.GitObservation    `json:"initial_git"`
	PinnedSource      *PinnedSource              `json:"pinned_source,omitempty"`
	CreatedAt         time.Time                  `json:"created_at"`
}

type actionUpdate struct {
	Index  int          `json:"index"`
	Record ActionRecord `json:"record"`
}

type answerUpdate struct {
	Index        int    `json:"index"`
	Active       bool   `json:"active"`
	SupersededBy string `json:"superseded_by,omitempty"`
}

type gitObservationDelta struct {
	Observation runstore.GitObservation `json:"observation"`
}

// runDelta stores only the mutation made by one command. The append-only
// journal therefore grows with submitted data rather than repeatedly embedding
// the complete Run projection.
type runDelta struct {
	EventVersion    int                `json:"event_version"`
	ContractVersion int                `json:"contract_version"`
	RunID           string             `json:"run_id"`
	Initialize      *runInitialization `json:"initialize,omitempty"`

	State                         *RunState                 `json:"state,omitempty"`
	PauseReason                   *PauseReason              `json:"pause_reason,omitempty"`
	ReviewPending                 *bool                     `json:"review_pending,omitempty"`
	RemainingBudget               *int                      `json:"remaining_budget,omitempty"`
	CurrentActionSet              bool                      `json:"current_action_set,omitempty"`
	CurrentAction                 *Action                   `json:"current_action,omitempty"`
	CurrentGit                    *gitObservationDelta      `json:"current_git,omitempty"`
	FinalGitObserved              *bool                     `json:"final_git_observed,omitempty"`
	PinnedSourceSet               bool                      `json:"pinned_source_set,omitempty"`
	PinnedSource                  *PinnedSource             `json:"pinned_source,omitempty"`
	SourceCandidateSet            bool                      `json:"source_candidate_set,omitempty"`
	SourceCandidate               *SourceCandidate          `json:"source_candidate,omitempty"`
	LastSourceChoiceSet           bool                      `json:"last_source_choice_set,omitempty"`
	LastSourceChoice              *SourceChoiceReceipt      `json:"last_source_choice,omitempty"`
	SourceChoiceResolutionsAppend []sourceChoiceResolution  `json:"source_choice_resolutions_append,omitempty"`
	LastResumeResultSet           bool                      `json:"last_resume_result_set,omitempty"`
	LastResumeResult              *ResumeResult             `json:"last_resume_result,omitempty"`
	Summary                       *string                   `json:"summary,omitempty"`
	PendingDestructiveRequestSet  bool                      `json:"pending_destructive_request_set,omitempty"`
	PendingDestructiveRequest     *DestructiveRequest       `json:"pending_destructive_request,omitempty"`
	DestructiveGrantSet           bool                      `json:"destructive_grant_set,omitempty"`
	DestructiveGrant              *DestructiveAuthorization `json:"destructive_grant,omitempty"`

	ActionUpdates       []actionUpdate    `json:"action_updates,omitempty"`
	AnswerUpdates       []answerUpdate    `json:"answer_updates,omitempty"`
	AnswersAppend       []AnswerRecord    `json:"answers_append,omitempty"`
	PendingDrop         int               `json:"pending_drop,omitempty"`
	PendingAppend       []SuggestedAction `json:"pending_append,omitempty"`
	ObservationsAppend  []string          `json:"observations_append,omitempty"`
	KnownIssuesAppend   []string          `json:"known_issues_append,omitempty"`
	UncertaintiesAppend []string          `json:"uncertainties_append,omitempty"`
	ActivitiesAppend    []Activity        `json:"activities_append,omitempty"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

func validRunEventType(eventType string) bool {
	switch eventType {
	case "run_started", "outcome_submitted", "answer_recorded", "action_skipped", "run_stopped", "run_resumed",
		ResumeOperationSourceRefreshed, ResumeOperationSourceCandidate,
		ResumeOperationSourceRefreshSkipped, ResumeOperationSourceAmended, ResumeOperationSourcePinned:
		return true
	default:
		return false
	}
}

func validateRunJournalValues(state RunState, pauseReason PauseReason, remainingBudget int) error {
	switch state {
	case RunActive, RunPaused, RunStopped, RunEnded:
	default:
		return fmt.Errorf("unknown run state %q", state)
	}

	// The empty value is the canonical absence of a pause reason.
	switch pauseReason {
	case "", PauseDecisionRequired, PauseDestructiveConfirm, PauseEnvironmentUnavailable, PauseBudgetExhausted:
	default:
		return fmt.Errorf("unknown pause reason %q", pauseReason)
	}

	if remainingBudget < 0 {
		return errors.New("remaining budget cannot be negative")
	}
	return nil
}

func validateRunEventMutation(eventType string, before, after Run, delta runDelta) error {
	switch eventType {
	case "run_started":
		if before.ID != "" || delta.Initialize == nil {
			return errors.New("run_started must initialize one run")
		}
	case "outcome_submitted":
		for _, update := range delta.ActionUpdates {
			if update.Index < len(before.Actions) && before.Actions[update.Index].Outcome == nil && update.Record.Outcome != nil {
				return nil
			}
		}
		return errors.New("outcome_submitted must record a new action outcome")
	case "answer_recorded":
		if len(delta.AnswersAppend) != 1 {
			return errors.New("answer_recorded must append exactly one answer")
		}
	case "action_skipped":
		for _, update := range delta.ActionUpdates {
			if update.Index < len(before.Actions) && !before.Actions[update.Index].Skipped && update.Record.Skipped {
				return nil
			}
		}
		return errors.New("action_skipped must mark one existing action skipped")
	case "run_stopped":
		if after.State != RunStopped {
			return errors.New("run_stopped must enter stopped state")
		}
	case "run_resumed":
		if after.LastResumeResult == nil || after.LastResumeResult.Operation != ResumeOperationAdHoc {
			return errors.New("run_resumed must record the ad-hoc resume operation")
		}
	}
	return nil
}

func newRunEvent(eventType string, before, after Run) (runstore.Event, error) {
	delta, err := diffRun(eventType, before, after)
	if err != nil {
		return runstore.Event{}, err
	}
	return runstore.NewEvent(eventType, delta)
}

func diffRun(eventType string, before, after Run) (runDelta, error) {
	if after.ContractVersion != ContractVersion || after.ID == "" {
		return runDelta{}, errors.New("cannot journal a run with invalid identity")
	}
	if err := after.WorkspaceIdentity.Validate(); err != nil {
		return runDelta{}, fmt.Errorf("cannot journal invalid workspace identity: %w", err)
	}
	if after.Workspace != after.WorkspaceIdentity.WorktreeRoot {
		return runDelta{}, errors.New("cannot journal a run whose workspace root differs from its identity")
	}
	if err := after.InitialGit.Validate(); err != nil {
		return runDelta{}, fmt.Errorf("cannot journal invalid initial git observation: %w", err)
	}
	if err := after.CurrentGit.Validate(); err != nil {
		return runDelta{}, fmt.Errorf("cannot journal invalid current git observation: %w", err)
	}
	if err := validateActionHistoryTransition(before, after); err != nil {
		return runDelta{}, fmt.Errorf("cannot journal invalid action history: %w", err)
	}
	if err := validateRunDestructiveState(after); err != nil {
		return runDelta{}, fmt.Errorf("cannot journal invalid destructive run state: %w", err)
	}
	if after.ReviewPending && !after.ReviewEnabled {
		return runDelta{}, errors.New("cannot journal pending review when review is disabled")
	}
	if err := validateRunSourceTransition(eventType, before, after); err != nil {
		return runDelta{}, fmt.Errorf("cannot journal invalid source transition: %w", err)
	}
	if _, err := DeriveNext(after); err != nil {
		return runDelta{}, fmt.Errorf("cannot journal run with invalid next operation: %w", err)
	}
	delta := runDelta{
		EventVersion:    runEventVersion,
		ContractVersion: ContractVersion,
		RunID:           after.ID,
		UpdatedAt:       after.UpdatedAt,
	}
	if delta.UpdatedAt.IsZero() {
		return runDelta{}, errors.New("cannot journal a run without updated_at")
	}
	if before.ID == "" {
		delta.Initialize = &runInitialization{
			Goal:              after.Goal,
			Workspace:         after.Workspace,
			WorkspaceIdentity: after.WorkspaceIdentity,
			ReviewEnabled:     after.ReviewEnabled,
			InitialBudget:     after.InitialBudget,
			InitialGit:        cloneGitObservation(after.InitialGit),
			PinnedSource:      clonePinnedSource(after.PinnedSource),
			CreatedAt:         after.CreatedAt,
		}
		before = Run{
			ContractVersion:   after.ContractVersion,
			ID:                after.ID,
			Goal:              after.Goal,
			Workspace:         after.Workspace,
			WorkspaceIdentity: after.WorkspaceIdentity,
			ReviewEnabled:     after.ReviewEnabled,
			InitialBudget:     after.InitialBudget,
			InitialGit:        cloneGitObservation(after.InitialGit),
			CurrentGit:        cloneGitObservation(after.InitialGit),
			PinnedSource:      clonePinnedSource(after.PinnedSource),
			CreatedAt:         after.CreatedAt,
		}
	} else if before.ContractVersion != after.ContractVersion || before.ID != after.ID ||
		before.Goal != after.Goal || before.Workspace != after.Workspace ||
		!before.WorkspaceIdentity.Equal(after.WorkspaceIdentity) ||
		before.ReviewEnabled != after.ReviewEnabled || before.InitialBudget != after.InitialBudget ||
		!reflect.DeepEqual(before.InitialGit, after.InitialGit) || !before.CreatedAt.Equal(after.CreatedAt) {
		return runDelta{}, errors.New("immutable run identity changed")
	}

	if before.State != after.State {
		delta.State = pointer(after.State)
	}
	if before.PauseReason != after.PauseReason {
		delta.PauseReason = pointer(after.PauseReason)
	}
	if before.ReviewPending != after.ReviewPending {
		delta.ReviewPending = pointer(after.ReviewPending)
	}
	if before.RemainingBudget != after.RemainingBudget {
		delta.RemainingBudget = pointer(after.RemainingBudget)
	}
	if !reflect.DeepEqual(before.CurrentAction, after.CurrentAction) {
		delta.CurrentActionSet = true
		delta.CurrentAction = after.CurrentAction
	}
	if gitDelta := diffGitObservation(before.CurrentGit, after.CurrentGit); gitDelta != nil {
		delta.CurrentGit = gitDelta
	}
	if before.FinalGitObserved != after.FinalGitObserved {
		delta.FinalGitObserved = pointer(after.FinalGitObserved)
	}
	if !reflect.DeepEqual(before.PinnedSource, after.PinnedSource) {
		delta.PinnedSourceSet = true
		delta.PinnedSource = clonePinnedSource(after.PinnedSource)
	}
	if !reflect.DeepEqual(before.SourceCandidate, after.SourceCandidate) {
		delta.SourceCandidateSet = true
		if after.SourceCandidate != nil {
			candidate := cloneSourceCandidate(*after.SourceCandidate)
			delta.SourceCandidate = &candidate
		}
	}
	if !reflect.DeepEqual(before.LastSourceChoice, after.LastSourceChoice) {
		delta.LastSourceChoiceSet = true
		if after.LastSourceChoice != nil {
			receipt := *after.LastSourceChoice
			delta.LastSourceChoice = &receipt
		}
	}
	if len(after.sourceChoiceHistory) < len(before.sourceChoiceHistory) {
		return runDelta{}, errors.New("source choice resolution history is not append-only")
	}
	for index := range before.sourceChoiceHistory {
		if before.sourceChoiceHistory[index] != after.sourceChoiceHistory[index] {
			return runDelta{}, errors.New("source choice resolution history is not append-only")
		}
	}
	delta.SourceChoiceResolutionsAppend = append(
		[]sourceChoiceResolution(nil),
		after.sourceChoiceHistory[len(before.sourceChoiceHistory):]...,
	)
	if !reflect.DeepEqual(before.LastResumeResult, after.LastResumeResult) {
		delta.LastResumeResultSet = true
		if after.LastResumeResult != nil {
			result := *after.LastResumeResult
			delta.LastResumeResult = &result
		}
	}
	if before.Summary != after.Summary {
		delta.Summary = pointer(after.Summary)
	}
	if !reflect.DeepEqual(before.PendingDestructiveRequest, after.PendingDestructiveRequest) {
		delta.PendingDestructiveRequestSet = true
		delta.PendingDestructiveRequest = cloneDestructiveRequest(after.PendingDestructiveRequest)
	}
	if !reflect.DeepEqual(before.DestructiveGrant, after.DestructiveGrant) {
		delta.DestructiveGrantSet = true
		delta.DestructiveGrant = cloneDestructiveAuthorization(after.DestructiveGrant)
	}

	if len(after.Actions) < len(before.Actions) {
		return runDelta{}, errors.New("run action history shrank")
	}
	for index := range after.Actions {
		if index >= len(before.Actions) || !reflect.DeepEqual(before.Actions[index], after.Actions[index]) {
			delta.ActionUpdates = append(delta.ActionUpdates, actionUpdate{Index: index, Record: after.Actions[index]})
		}
	}
	var err error
	if delta.PendingDrop, delta.PendingAppend, err = diffPending(before.PendingActions, after.PendingActions); err != nil {
		return runDelta{}, err
	}
	if delta.AnswerUpdates, delta.AnswersAppend, err = diffAnswers(before.Answers, after.Answers); err != nil {
		return runDelta{}, err
	}
	if delta.ObservationsAppend, err = appendedSuffix(before.Observations, after.Observations, "observation"); err != nil {
		return runDelta{}, err
	}
	if delta.KnownIssuesAppend, err = appendedSuffix(before.KnownIssues, after.KnownIssues, "known issue"); err != nil {
		return runDelta{}, err
	}
	if delta.UncertaintiesAppend, err = appendedSuffix(before.Uncertainties, after.Uncertainties, "uncertainty"); err != nil {
		return runDelta{}, err
	}
	if delta.ActivitiesAppend, err = appendedSuffix(before.Activities, after.Activities, "activity"); err != nil {
		return runDelta{}, err
	}
	return delta, nil
}

func applyRunEvent(run *Run, event runstore.Event) error {
	if !validRunEventType(event.Type) {
		return fmt.Errorf("unsupported run journal event type %q", event.Type)
	}
	before := runBeforeMutation(*run)
	var delta runDelta
	decoder := json.NewDecoder(bytes.NewReader(event.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&delta); err != nil {
		return fmt.Errorf("decode %s event: %w", event.Type, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("decode %s event: multiple JSON values", event.Type)
		}
		return fmt.Errorf("decode %s event: %w", event.Type, err)
	}
	if delta.EventVersion != runEventVersion || delta.ContractVersion != ContractVersion || delta.RunID == "" || delta.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid %s event envelope", event.Type)
	}
	if run.ID == "" {
		if delta.Initialize == nil || event.Type != "run_started" {
			return errors.New("run journal does not begin with run_started")
		}
		initial := delta.Initialize
		if initial.Goal == "" || initial.Workspace == "" || initial.CreatedAt.IsZero() {
			return errors.New("run journal has invalid initialization")
		}
		if err := ValidateBudget(initial.InitialBudget); err != nil {
			return fmt.Errorf("run journal has invalid initialization: %w", err)
		}
		if err := initial.WorkspaceIdentity.Validate(); err != nil {
			return fmt.Errorf("run journal has invalid workspace identity: %w", err)
		}
		if initial.Workspace != initial.WorkspaceIdentity.WorktreeRoot {
			return errors.New("run journal workspace root differs from its identity")
		}
		if err := initial.InitialGit.Validate(); err != nil {
			return fmt.Errorf("run journal has invalid initial git observation: %w", err)
		}
		if initial.PinnedSource != nil {
			if err := validatePinnedSource(*initial.PinnedSource); err != nil {
				return fmt.Errorf("run journal has invalid pinned source: %w", err)
			}
		}
		*run = Run{
			ContractVersion:   ContractVersion,
			ID:                delta.RunID,
			Goal:              initial.Goal,
			Workspace:         initial.Workspace,
			WorkspaceIdentity: initial.WorkspaceIdentity,
			ReviewEnabled:     initial.ReviewEnabled,
			InitialBudget:     initial.InitialBudget,
			InitialGit:        cloneGitObservation(initial.InitialGit),
			CurrentGit:        cloneGitObservation(initial.InitialGit),
			PinnedSource:      clonePinnedSource(initial.PinnedSource),
			CreatedAt:         initial.CreatedAt,
		}
	} else if delta.Initialize != nil || run.ID != delta.RunID || run.ContractVersion != delta.ContractVersion {
		return errors.New("run journal identity or contract version mismatch")
	}

	state := run.State
	if delta.State != nil {
		state = *delta.State
	}
	pauseReason := run.PauseReason
	if delta.PauseReason != nil {
		pauseReason = *delta.PauseReason
	}
	remainingBudget := run.RemainingBudget
	if delta.RemainingBudget != nil {
		remainingBudget = *delta.RemainingBudget
	}
	// Validate the resulting projection for both initialization and later deltas.
	if err := validateRunJournalValues(state, pauseReason, remainingBudget); err != nil {
		return fmt.Errorf("invalid %s event state: %w", event.Type, err)
	}
	run.State = state
	run.PauseReason = pauseReason
	run.RemainingBudget = remainingBudget
	if delta.ReviewPending != nil {
		run.ReviewPending = *delta.ReviewPending
	}
	if delta.CurrentActionSet {
		run.CurrentAction = delta.CurrentAction
	} else if delta.CurrentAction != nil {
		return errors.New("current_action requires current_action_set")
	}
	if delta.CurrentGit != nil {
		if err := applyGitObservationDelta(&run.CurrentGit, *delta.CurrentGit); err != nil {
			return err
		}
	}
	if delta.FinalGitObserved != nil {
		run.FinalGitObserved = *delta.FinalGitObserved
	}
	if delta.PinnedSourceSet {
		if delta.PinnedSource == nil {
			return errors.New("pinned_source_set cannot clear pinned source")
		}
		if err := validatePinnedSource(*delta.PinnedSource); err != nil {
			return fmt.Errorf("invalid pinned source delta: %w", err)
		}
		run.PinnedSource = clonePinnedSource(delta.PinnedSource)
	} else if delta.PinnedSource != nil {
		return errors.New("pinned_source requires pinned_source_set")
	}
	if delta.SourceCandidateSet {
		if delta.SourceCandidate == nil {
			run.SourceCandidate = nil
		} else {
			if delta.SourceCandidate.CandidateID == "" || delta.SourceCandidate.CreatedAt.IsZero() {
				return errors.New("invalid source candidate delta")
			}
			if err := validateSourceCandidateInput(delta.SourceCandidate.SourceCandidateInput); err != nil {
				return fmt.Errorf("invalid source candidate delta: %w", err)
			}
			candidate := cloneSourceCandidate(*delta.SourceCandidate)
			run.SourceCandidate = &candidate
		}
	} else if delta.SourceCandidate != nil {
		return errors.New("source_candidate requires source_candidate_set")
	}
	if delta.LastSourceChoiceSet {
		if delta.LastSourceChoice == nil {
			run.LastSourceChoice = nil
		} else {
			if delta.LastSourceChoice.CandidateID == "" ||
				(delta.LastSourceChoice.Choice != SourceChoicePinned && delta.LastSourceChoice.Choice != SourceChoiceAdopt) ||
				delta.LastSourceChoice.ResultingActionID == "" || delta.LastSourceChoice.At.IsZero() {
				return errors.New("invalid last source choice delta")
			}
			receipt := *delta.LastSourceChoice
			run.LastSourceChoice = &receipt
		}
	} else if delta.LastSourceChoice != nil {
		return errors.New("last_source_choice requires last_source_choice_set")
	}
	if delta.LastResumeResultSet {
		if delta.LastResumeResult == nil || delta.LastResumeResult.Operation == "" {
			return errors.New("invalid last resume result delta")
		}
		result := *delta.LastResumeResult
		run.LastResumeResult = &result
	} else if delta.LastResumeResult != nil {
		return errors.New("last_resume_result requires last_resume_result_set")
	}
	for _, resolution := range delta.SourceChoiceResolutionsAppend {
		if err := validateSourceChoiceResolution(resolution); err != nil {
			return err
		}
		if findSourceChoiceResolution(*run, resolution.Receipt.CandidateID) != nil {
			return errors.New("source choice resolution candidate_id is duplicated")
		}
		run.sourceChoiceHistory = append(run.sourceChoiceHistory, resolution)
	}
	if len(delta.SourceChoiceResolutionsAppend) == 0 && delta.LastSourceChoiceSet &&
		delta.LastSourceChoice != nil && delta.LastResumeResultSet && delta.LastResumeResult != nil {
		legacy := sourceChoiceResolution{Receipt: *delta.LastSourceChoice, Result: *delta.LastResumeResult}
		if err := validateSourceChoiceResolution(legacy); err == nil {
			if findSourceChoiceResolution(*run, legacy.Receipt.CandidateID) != nil {
				return errors.New("source choice resolution candidate_id is duplicated")
			}
			run.sourceChoiceHistory = append(run.sourceChoiceHistory, legacy)
		}
	}
	if delta.Summary != nil {
		run.Summary = *delta.Summary
	}
	if delta.PendingDestructiveRequestSet {
		if delta.PendingDestructiveRequest != nil {
			request, err := NormalizeDestructiveRequest(*delta.PendingDestructiveRequest)
			if err != nil {
				return fmt.Errorf("invalid pending destructive request delta: %w", err)
			}
			run.PendingDestructiveRequest = &request
		} else {
			run.PendingDestructiveRequest = nil
		}
	} else if delta.PendingDestructiveRequest != nil {
		return errors.New("pending_destructive_request requires pending_destructive_request_set")
	}
	if delta.DestructiveGrantSet {
		if delta.DestructiveGrant != nil {
			if err := validateDestructiveAuthorization(*delta.DestructiveGrant); err != nil {
				return fmt.Errorf("invalid destructive grant delta: %w", err)
			}
			run.DestructiveGrant = cloneDestructiveAuthorization(delta.DestructiveGrant)
		} else {
			run.DestructiveGrant = nil
		}
	} else if delta.DestructiveGrant != nil {
		return errors.New("destructive_grant requires destructive_grant_set")
	}
	for _, update := range delta.ActionUpdates {
		switch {
		case update.Index < 0 || update.Index > len(run.Actions):
			return fmt.Errorf("invalid action update index %d", update.Index)
		case update.Index == len(run.Actions):
			run.Actions = append(run.Actions, update.Record)
		default:
			run.Actions[update.Index] = update.Record
		}
	}
	if delta.PendingDrop < 0 || delta.PendingDrop > len(run.PendingActions) {
		return errors.New("invalid pending action drop")
	}
	run.PendingActions = append(append([]SuggestedAction(nil), run.PendingActions[delta.PendingDrop:]...), delta.PendingAppend...)
	for _, update := range delta.AnswerUpdates {
		if update.Index < 0 || update.Index >= len(run.Answers) {
			return fmt.Errorf("invalid answer update index %d", update.Index)
		}
		run.Answers[update.Index].Active = update.Active
		run.Answers[update.Index].SupersededBy = update.SupersededBy
	}
	run.Answers = append(run.Answers, delta.AnswersAppend...)
	run.Observations = append(run.Observations, delta.ObservationsAppend...)
	run.KnownIssues = append(run.KnownIssues, delta.KnownIssuesAppend...)
	run.Uncertainties = append(run.Uncertainties, delta.UncertaintiesAppend...)
	run.Activities = append(run.Activities, delta.ActivitiesAppend...)
	run.UpdatedAt = delta.UpdatedAt
	if err := run.WorkspaceIdentity.Validate(); err != nil {
		return fmt.Errorf("invalid workspace identity after %s: %w", event.Type, err)
	}
	if err := run.InitialGit.Validate(); err != nil {
		return fmt.Errorf("invalid initial git observation after %s: %w", event.Type, err)
	}
	if err := run.CurrentGit.Validate(); err != nil {
		return fmt.Errorf("invalid current git observation after %s: %w", event.Type, err)
	}
	if err := validateActionHistoryTransition(before, *run); err != nil {
		return fmt.Errorf("invalid action history after %s: %w", event.Type, err)
	}
	if err := validateRunDestructiveState(*run); err != nil {
		return fmt.Errorf("invalid destructive run state after %s: %w", event.Type, err)
	}
	if run.ReviewPending && !run.ReviewEnabled {
		return fmt.Errorf("invalid pending review after %s", event.Type)
	}
	if err := validateRunSourceTransition(event.Type, before, *run); err != nil {
		return fmt.Errorf("invalid source transition after %s: %w", event.Type, err)
	}
	if err := validateRunEventMutation(event.Type, before, *run, delta); err != nil {
		return fmt.Errorf("invalid %s event mutation: %w", event.Type, err)
	}
	canonicalDelta, err := diffRun(event.Type, before, *run)
	if err != nil {
		return fmt.Errorf("reconstruct canonical %s event: %w", event.Type, err)
	}
	// Event version 1 journals written before append-only source-choice history
	// carried only the two Last* projection fields. Replay reconstructs their
	// single resolution while retaining that exact legacy delta as canonical.
	if len(delta.SourceChoiceResolutionsAppend) == 0 && delta.LastSourceChoiceSet && delta.LastSourceChoice != nil {
		canonicalDelta.SourceChoiceResolutionsAppend = nil
	}
	if !reflect.DeepEqual(delta, canonicalDelta) {
		return fmt.Errorf("non-canonical delta for %s event", event.Type)
	}
	recordAcceptedSourceComments(run, run.PinnedSource)
	if _, err := DeriveNext(*run); err != nil {
		return fmt.Errorf("derive next after %s: %w", event.Type, err)
	}
	return nil
}

func validateActionHistoryTransition(before, after Run) error {
	if len(after.Actions) < len(before.Actions) {
		return errors.New("run action history shrank")
	}
	if len(after.Actions) > len(before.Actions)+1 {
		return errors.New("one journal event cannot append multiple actions")
	}

	currentActionID := ""
	if before.CurrentAction != nil {
		currentActionID = before.CurrentAction.ActionID
	}
	seenActionIDs := make(map[string]struct{}, len(after.Actions))
	for index, record := range after.Actions {
		if err := record.Action.Validate(); err != nil {
			return fmt.Errorf("action record %d is invalid: %w", index, err)
		}
		if record.Action.RunID != after.ID {
			return fmt.Errorf("action record %d belongs to another run", index)
		}
		if _, exists := seenActionIDs[record.Action.ActionID]; exists {
			return fmt.Errorf("action_id %q is duplicated", record.Action.ActionID)
		}
		seenActionIDs[record.Action.ActionID] = struct{}{}

		if record.Outcome == nil {
			if record.OutcomePayloadSHA256 != "" {
				return fmt.Errorf("action record %d has an outcome digest without an outcome", index)
			}
		} else {
			if err := record.Outcome.Validate(record.Action.Kind, record.Action.ActionID); err != nil {
				return fmt.Errorf("action record %d has an invalid outcome: %w", index, err)
			}
			// The digest commits to the exact host bytes, whose formatting is
			// intentionally not journaled. Replay can validate its shape and
			// immutability, but cannot recompute it from the decoded Outcome.
			if !validSHA256(record.OutcomePayloadSHA256) {
				return fmt.Errorf("action record %d has a malformed outcome payload digest", index)
			}
		}
		if record.ReviewProjection != nil {
			if record.Action.Kind != ActionReview || !record.Skipped || record.Outcome != nil ||
				record.ReviewProjection.Result != ReviewNotRun || len(record.ReviewProjection.Findings) != 0 ||
				len(record.ReviewProjection.Uncertainties) != 0 {
				return fmt.Errorf("action record %d has an invalid skipped-review projection", index)
			}
			if err := validateReview(*record.ReviewProjection); err != nil {
				return fmt.Errorf("action record %d has an invalid review projection: %w", index, err)
			}
		}

		if index >= len(before.Actions) {
			if record.Outcome != nil || record.OutcomePayloadSHA256 != "" || record.ReviewProjection != nil ||
				record.Voided || record.Skipped {
				return fmt.Errorf("new action record %d must begin pending and non-void", index)
			}
			continue
		}

		prior := before.Actions[index]
		if !reflect.DeepEqual(prior.Action, record.Action) {
			return fmt.Errorf("action record %d rewrote its issued action", index)
		}
		if prior.Voided && !record.Voided {
			return fmt.Errorf("action record %d was unvoided", index)
		}
		if !prior.Voided && record.Voided && prior.Action.ActionID != currentActionID {
			return fmt.Errorf("action record %d voided a non-current action", index)
		}
		if prior.Skipped && !record.Skipped {
			return fmt.Errorf("action record %d was unskipped", index)
		}
		if !prior.Skipped && record.Skipped && prior.Action.ActionID != currentActionID {
			return fmt.Errorf("action record %d skipped a non-current action", index)
		}
		if prior.Outcome != nil &&
			(!reflect.DeepEqual(prior.Outcome, record.Outcome) ||
				prior.OutcomePayloadSHA256 != record.OutcomePayloadSHA256) {
			return fmt.Errorf("action record %d rewrote its accepted outcome", index)
		}
		if prior.Outcome == nil && record.Outcome != nil && prior.Action.ActionID != currentActionID {
			return fmt.Errorf("action record %d completed a non-current action", index)
		}
		if prior.ReviewProjection != nil && !reflect.DeepEqual(prior.ReviewProjection, record.ReviewProjection) {
			return fmt.Errorf("action record %d rewrote its review projection", index)
		}
		if prior.ReviewProjection == nil && record.ReviewProjection != nil && prior.Action.ActionID != currentActionID {
			return fmt.Errorf("action record %d projected review status for a non-current action", index)
		}
	}
	return nil
}

func diffGitObservation(before, after runstore.GitObservation) *gitObservationDelta {
	if reflect.DeepEqual(before, after) {
		return nil
	}
	return &gitObservationDelta{Observation: cloneGitObservation(after)}
}

func applyGitObservationDelta(observation *runstore.GitObservation, delta gitObservationDelta) error {
	if err := delta.Observation.Validate(); err != nil {
		return fmt.Errorf("invalid current git observation delta: %w", err)
	}
	*observation = cloneGitObservation(delta.Observation)
	return nil
}

func cloneGitObservation(observation runstore.GitObservation) runstore.GitObservation {
	observation.DirtyFiles = append(make([]string, 0, len(observation.DirtyFiles)), observation.DirtyFiles...)
	observation.PathObservations = append(make([]runstore.PathObservation, 0, len(observation.PathObservations)), observation.PathObservations...)
	for index := range observation.PathObservations {
		if observation.PathObservations[index].Size != nil {
			size := *observation.PathObservations[index].Size
			observation.PathObservations[index].Size = &size
		}
	}
	return observation
}

func diffPending(before, after []SuggestedAction) (int, []SuggestedAction, error) {
	for drop := 0; drop <= len(before); drop++ {
		retained := before[drop:]
		if len(retained) > len(after) {
			continue
		}
		matches := true
		for index := range retained {
			if !reflect.DeepEqual(retained[index], after[index]) {
				matches = false
				break
			}
		}
		if matches {
			return drop, append([]SuggestedAction(nil), after[len(retained):]...), nil
		}
	}
	return 0, nil, errors.New("pending action queue was reordered instead of consumed/appended")
}

func diffAnswers(before, after []AnswerRecord) ([]answerUpdate, []AnswerRecord, error) {
	if len(after) < len(before) {
		return nil, nil, errors.New("run answer history shrank")
	}
	var updates []answerUpdate
	for index := range before {
		beforeRecord := before[index]
		afterRecord := after[index]
		beforeRecord.Active, afterRecord.Active = false, false
		beforeRecord.SupersededBy, afterRecord.SupersededBy = "", ""
		if !reflect.DeepEqual(beforeRecord, afterRecord) {
			return nil, nil, fmt.Errorf("run answer history was rewritten at index %d", index)
		}
		if before[index].Active != after[index].Active || before[index].SupersededBy != after[index].SupersededBy {
			updates = append(updates, answerUpdate{
				Index:        index,
				Active:       after[index].Active,
				SupersededBy: after[index].SupersededBy,
			})
		}
	}
	return updates, append([]AnswerRecord(nil), after[len(before):]...), nil
}

func appendedSuffix[T any](before, after []T, name string) ([]T, error) {
	if len(after) < len(before) {
		return nil, fmt.Errorf("run %s history was rewritten", name)
	}
	for index := range before {
		if !reflect.DeepEqual(before[index], after[index]) {
			return nil, fmt.Errorf("run %s history was rewritten", name)
		}
	}
	return append([]T(nil), after[len(before):]...), nil
}

func pointer[T any](value T) *T { return &value }
