package autopilot

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/signalridge/slipway/internal/runstore"
)

type ProtocolError struct {
	Code    string
	Message string
	Next    Next
	Details map[string]any
}

func (err *ProtocolError) Error() string { return err.Message }

type SourceChoice string

const (
	SourceChoicePinned SourceChoice = "pinned"
	SourceChoiceAdopt  SourceChoice = "adopt"
)

const (
	ResumeOperationAdHoc                = "ad_hoc_resumed"
	ResumeOperationSourceRefreshed      = "source_refreshed"
	ResumeOperationSourceCandidate      = "source_candidate_created"
	ResumeOperationSourceRefreshSkipped = "source_refresh_skipped"
	ResumeOperationSourceAmended        = "source_amended"
	ResumeOperationSourcePinned         = "source_pinned"
)

type CreateOptions struct {
	Budget        int
	ReviewEnabled bool
	PinnedSource  *PinnedSource
}

type ResumeOptions struct {
	Budget          *int
	RefreshedSource *SourceCandidateInput
	UsePinnedSource bool
	SourceChoice    SourceChoice
	CandidateID     string
}

type AnswerOptions struct {
	Text               string
	ConfirmDestructive bool
	ScopeSHA256        string
}

// SourceCandidate is a run-local amendment decision. The embedded input is
// already path-free and contains a normalized snapshot only when Valid is true.
type SourceCandidate struct {
	CandidateID string `json:"candidate_id"`
	SourceCandidateInput
	CreatedAt time.Time `json:"created_at"`
}

type SourceChoiceReceipt struct {
	CandidateID       string       `json:"candidate_id"`
	Choice            SourceChoice `json:"choice"`
	ResultingActionID string       `json:"resulting_action_id"`
	At                time.Time    `json:"at"`
}

type ResumeResult struct {
	Operation     string `json:"operation"`
	BudgetApplied bool   `json:"budget_applied"`
	CandidateID   string `json:"candidate_id,omitempty"`
}

type AnswerRecord struct {
	ActionID             string    `json:"action_id"`
	Text                 string    `json:"text"`
	ConfirmDestructive   bool      `json:"confirm_destructive"`
	ScopeSHA256          string    `json:"scope_sha256,omitempty"`
	PayloadSHA256        string    `json:"payload_sha256"`
	SourceRevision       string    `json:"source_revision,omitempty"`
	RequirementsRevision string    `json:"requirements_revision,omitempty"`
	Active               bool      `json:"active"`
	SupersededBy         string    `json:"superseded_by,omitempty"`
	At                   time.Time `json:"at"`
}

type ActionRecord struct {
	Action               Action   `json:"action"`
	Outcome              *Outcome `json:"outcome,omitempty"`
	OutcomePayloadSHA256 string   `json:"outcome_payload_sha256,omitempty"`
	Voided               bool     `json:"voided,omitempty"`
	Skipped              bool     `json:"skipped,omitempty"`
}

type Run struct {
	ContractVersion           int                        `json:"contract_version"`
	ID                        string                     `json:"id"`
	Goal                      string                     `json:"goal"`
	Workspace                 string                     `json:"workspace"`
	WorkspaceIdentity         runstore.WorkspaceIdentity `json:"workspace_identity"`
	State                     RunState                   `json:"state"`
	PauseReason               PauseReason                `json:"pause_reason,omitempty"`
	ReviewEnabled             bool                       `json:"review_enabled"`
	InitialBudget             int                        `json:"initial_budget"`
	RemainingBudget           int                        `json:"remaining_budget"`
	InitialGit                runstore.GitObservation    `json:"initial_git"`
	CurrentGit                runstore.GitObservation    `json:"current_git"`
	FinalGitObserved          bool                       `json:"final_git_observed"`
	PinnedSource              *PinnedSource              `json:"pinned_source,omitempty"`
	SourceCandidate           *SourceCandidate           `json:"source_candidate,omitempty"`
	LastSourceChoice          *SourceChoiceReceipt       `json:"last_source_choice,omitempty"`
	LastResumeResult          *ResumeResult              `json:"last_resume_result,omitempty"`
	CurrentAction             *Action                    `json:"current_action,omitempty"`
	Actions                   []ActionRecord             `json:"actions"`
	PendingActions            []SuggestedAction          `json:"pending_actions,omitempty"`
	DecisionSuggestions       []SuggestedAction          `json:"decision_suggestions,omitempty"`
	Answers                   []AnswerRecord             `json:"answers,omitempty"`
	Observations              []string                   `json:"observations,omitempty"`
	KnownIssues               []string                   `json:"known_issues,omitempty"`
	Uncertainties             []string                   `json:"uncertainties,omitempty"`
	Activities                []Activity                 `json:"activities,omitempty"`
	Summary                   string                     `json:"summary,omitempty"`
	PendingDestructiveRequest *DestructiveRequest        `json:"pending_destructive_request,omitempty"`
	DestructiveGrant          *DestructiveAuthorization  `json:"destructive_grant,omitempty"`
	CreatedAt                 time.Time                  `json:"created_at"`
	UpdatedAt                 time.Time                  `json:"updated_at"`
}

type Service struct {
	store        *runstore.Store
	openIdentity runstore.WorkspaceIdentity
}

func OpenService(start string) (*Service, error) {
	store, err := runstore.Open(start)
	if err != nil {
		return nil, err
	}
	identity, err := runstore.DiscoverWorkspaceIdentity(store.RepositoryRoot())
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("discover service workspace identity: %w", err)
	}
	return &Service{store: store, openIdentity: identity}, nil
}

func (service *Service) validateOpenWorkspace() (runstore.WorkspaceIdentity, error) {
	observed, err := runstore.DiscoverWorkspaceIdentity(service.store.RepositoryRoot())
	if err != nil {
		return runstore.WorkspaceIdentity{}, workspaceIdentityMismatchError(service.openIdentity, nil, err)
	}
	if !service.openIdentity.Equal(observed) {
		return runstore.WorkspaceIdentity{}, workspaceIdentityMismatchError(service.openIdentity, &observed, nil)
	}
	return observed, nil
}

func (service *Service) validateRunWorkspace(run Run) error {
	observed, err := service.validateOpenWorkspace()
	if err != nil {
		return err
	}
	if validationErr := run.WorkspaceIdentity.Validate(); validationErr != nil {
		return workspaceIdentityMismatchError(run.WorkspaceIdentity, &observed, validationErr)
	}
	if run.Workspace != run.WorkspaceIdentity.WorktreeRoot || !run.WorkspaceIdentity.Equal(observed) {
		return workspaceIdentityMismatchError(run.WorkspaceIdentity, &observed, nil)
	}
	return nil
}

func workspaceIdentityMismatchError(expected runstore.WorkspaceIdentity, observed *runstore.WorkspaceIdentity, cause error) *ProtocolError {
	details := map[string]any{"expected_workspace_identity": expected.ID}
	message := "workspace identity mismatch"
	if observed != nil {
		details["observed_workspace_identity"] = observed.ID
		message += ": the current Git worktree or metadata directories differ from the persisted Run identity"
	}
	if cause != nil {
		details["discovery_error"] = cause.Error()
		message += ": " + cause.Error()
	}
	return &ProtocolError{
		Code:    "workspace_identity_mismatch",
		Message: message,
		Next:    NoneNext(expected.ID),
		Details: details,
	}
}

func (service *Service) RepositoryRoot() string { return service.store.RepositoryRoot() }
func (service *Service) Close() error           { return service.store.Close() }

func (service *Service) Start(goal string, options CreateOptions) (Run, error) {
	identity, err := service.validateOpenWorkspace()
	if err != nil {
		return Run{}, err
	}
	goal = strings.TrimSpace(goal)
	workspace := identity.WorktreeRoot
	if goal == "" {
		return Run{}, &ProtocolError{Code: "goal_required", Message: "goal cannot be empty", Next: NoneNext(identity.ID)}
	}
	if err := ValidateBudget(options.Budget); err != nil {
		return Run{}, &ProtocolError{Code: "invalid_budget", Message: err.Error(), Next: startRunNext(workspace, goal, false)}
	}
	pinnedSource := clonePinnedSource(options.PinnedSource)
	if pinnedSource != nil {
		if err := validatePinnedSource(*pinnedSource); err != nil {
			return Run{}, &ProtocolError{
				Code:    "invalid_source",
				Message: "invalid pinned source: " + err.Error(),
				Next:    startRunNext(workspace, goal, true),
			}
		}
	}
	observation, err := runstore.ObserveGit(workspace)
	if err != nil {
		return Run{}, &ProtocolError{Code: "git_observation_failed", Message: err.Error(), Next: startRunNext(workspace, goal, pinnedSource != nil)}
	}
	now := time.Now().UTC()
	run := Run{
		ContractVersion:   ContractVersion,
		ID:                uuid.NewString(),
		Goal:              goal,
		Workspace:         workspace,
		WorkspaceIdentity: identity,
		State:             RunActive,
		ReviewEnabled:     options.ReviewEnabled,
		InitialBudget:     options.Budget,
		RemainingBudget:   options.Budget,
		InitialGit:        cloneGitObservation(observation),
		CurrentGit:        cloneGitObservation(observation),
		PinnedSource:      pinnedSource,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := issueAction(&run, ActionOrient, "Investigate repository facts, relevant code, Git state, and build/test/lint conventions before deciding what to do."); err != nil {
		return Run{}, err
	}
	event, err := newRunEvent("run_started", Run{}, run)
	if err != nil {
		return Run{}, err
	}
	if _, err := service.validateOpenWorkspace(); err != nil {
		return Run{}, err
	}
	if err := service.store.Create(run.ID, event, run); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (service *Service) Load(runID string) (Run, error) {
	if _, err := service.validateOpenWorkspace(); err != nil {
		return Run{}, err
	}
	var run Run
	if err := service.store.Visit(runID, func(event runstore.Event) error {
		return applyRunEvent(&run, event)
	}); err != nil {
		return Run{}, err
	}
	if run.ID != runID {
		return Run{}, errors.New("run journal identity mismatch")
	}
	if err := service.validateRunWorkspace(run); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (service *Service) List() ([]Run, error) {
	ids, err := service.store.ListIDs()
	if err != nil {
		return nil, err
	}
	runs := make([]Run, 0, len(ids))
	for _, id := range ids {
		run, loadErr := service.Load(id)
		if loadErr != nil {
			return nil, loadErr
		}
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].ID < runs[j].ID
		}
		return runs[i].CreatedAt.Before(runs[j].CreatedAt)
	})
	return runs, nil
}

func (service *Service) Submit(runID, actionID string, outcome Outcome) (Run, error) {
	if _, err := service.validateOpenWorkspace(); err != nil {
		return Run{}, err
	}
	payloadSHA256, err := outcomePayloadSHA256(outcome)
	if err != nil {
		return Run{}, &ProtocolError{Code: "invalid_outcome", Message: err.Error(), Next: NoneNext(service.store.RepositoryRoot())}
	}
	var run Run
	var result Run
	err = service.store.UpdateStream(runID, func(event runstore.Event) error {
		return applyRunEvent(&run, event)
	}, func() ([]runstore.Event, any, error) {
		if run.ID != runID {
			return nil, nil, errors.New("run journal identity mismatch")
		}
		if err := service.validateRunWorkspace(run); err != nil {
			return nil, nil, err
		}
		record := findActionRecord(&run, actionID)
		if record != nil && record.Voided {
			return nil, nil, protocolRunError(run, "stale_action", "action_id was voided by resume")
		}
		if run.State == RunStopped || run.State == RunEnded {
			return nil, nil, protocolRunError(run, "run_not_active", "run is not accepting outcomes while "+string(run.State))
		}
		if record != nil && record.Outcome != nil {
			if record.OutcomePayloadSHA256 == payloadSHA256 {
				result = run
				return nil, run, nil
			}
			return nil, nil, protocolRunError(run, "outcome_conflict", "this action already has a different outcome payload")
		}
		if run.State != RunActive {
			return nil, nil, protocolRunError(run, "run_not_active", "run is not accepting outcomes while "+string(run.State))
		}
		if run.CurrentAction == nil || run.CurrentAction.ActionID != actionID {
			return nil, nil, protocolRunError(run, "stale_action", "action_id is not the current action")
		}
		if err := outcome.Validate(run.CurrentAction.Kind, actionID); err != nil {
			var versionErr *VersionError
			if errors.As(err, &versionErr) {
				return nil, nil, &ProtocolError{Code: "contract_version_mismatch", Message: err.Error(), Next: refreshInstallNext(run.Workspace)}
			}
			return nil, nil, &ProtocolError{Code: "invalid_outcome", Message: err.Error(), Next: mustDeriveNext(run)}
		}
		before := runBeforeMutation(run)
		if err := acceptOutcome(&run, outcome, payloadSHA256); err != nil {
			return nil, nil, err
		}
		if err := service.validateRunWorkspace(run); err != nil {
			return nil, nil, err
		}
		run.UpdatedAt = time.Now().UTC()
		event, err := newRunEvent("outcome_submitted", before, run)
		if err != nil {
			return nil, nil, err
		}
		result = run
		return []runstore.Event{event}, run, nil
	})
	return result, err
}

func (service *Service) Answer(runID, actionID string, options AnswerOptions) (Run, error) {
	if _, err := service.validateOpenWorkspace(); err != nil {
		return Run{}, err
	}
	options.Text = strings.TrimSpace(options.Text)
	if !utf8.ValidString(options.Text) {
		return Run{}, &ProtocolError{Code: "invalid_answer", Message: "answer text must be valid utf-8", Next: NoneNext(service.store.RepositoryRoot())}
	}
	payloadSHA256, err := answerPayloadSHA256(actionID, options)
	if err != nil {
		return Run{}, err
	}

	var run Run
	var result Run
	err = service.store.UpdateStream(runID, func(event runstore.Event) error {
		return applyRunEvent(&run, event)
	}, func() ([]runstore.Event, any, error) {
		if run.ID != runID {
			return nil, nil, errors.New("run journal identity mismatch")
		}
		if err := service.validateRunWorkspace(run); err != nil {
			return nil, nil, err
		}
		if receipt := findAnswerRecord(run, actionID); receipt != nil {
			if receipt.PayloadSHA256 == payloadSHA256 {
				result = run
				return nil, run, nil
			}
			return nil, nil, protocolRunError(run, "answer_conflict", "this action already has a different answer")
		}
		if run.State != RunPaused {
			return nil, nil, protocolRunError(run, "answer_not_expected", "run is not waiting for an answer")
		}
		if run.PauseReason == PauseEnvironmentUnavailable {
			return nil, nil, resumeProtocolError(run, "answer_not_allowed", "environment_unavailable must be resumed after the environment recovers")
		}
		if run.PauseReason != PauseDecisionRequired && run.PauseReason != PauseDestructiveConfirm {
			return nil, nil, protocolRunError(run, "answer_not_expected", "run is not waiting for an answer")
		}
		if run.CurrentAction == nil || run.CurrentAction.ActionID != actionID {
			return nil, nil, protocolRunError(run, "stale_action", "action_id is not the waiting action")
		}

		before := runBeforeMutation(run)
		now := time.Now().UTC()
		receipt := AnswerRecord{
			ActionID:           actionID,
			Text:               options.Text,
			ConfirmDestructive: options.ConfirmDestructive,
			ScopeSHA256:        options.ScopeSHA256,
			PayloadSHA256:      payloadSHA256,
			At:                 now,
		}
		if run.PinnedSource != nil {
			receipt.SourceRevision = run.PinnedSource.SourceRevision
			receipt.RequirementsRevision = run.PinnedSource.RequirementsRevision
		}

		switch run.PauseReason {
		case PauseDecisionRequired:
			if options.ConfirmDestructive || options.ScopeSHA256 != "" {
				return nil, nil, protocolRunError(run, "destructive_confirmation_not_expected", "decision answer forbids destructive confirmation fields")
			}
			if options.Text == "" {
				return nil, nil, protocolRunError(run, "answer_required", "decision answer requires text")
			}
			voidCurrentAction(&run)
			clearDestructiveState(&run)
			receipt.Active = true
			run.Answers = append(run.Answers, receipt)
			run.DecisionSuggestions = nil
			run.PendingActions = nil
			run.State, run.PauseReason, run.CurrentAction = RunActive, "", nil
			if err := issueAction(&run, ActionOrient, "Re-orient after the user's decision before selecting further work."); err != nil {
				return nil, nil, err
			}
		case PauseDestructiveConfirm:
			request, validationErr := currentDestructiveRequest(run, actionID)
			if validationErr != nil {
				return nil, nil, validationErr
			}
			if !options.ConfirmDestructive {
				if options.ScopeSHA256 != "" {
					return nil, nil, protocolRunError(run, "destructive_confirmation_flag_required", "scope_sha256 requires confirm_destructive")
				}
				if options.Text == "" {
					return nil, nil, protocolRunError(run, "answer_required", "destructive feedback or decline requires text")
				}
				voidCurrentAction(&run)
				clearDestructiveState(&run)
				receipt.Active = true
				run.Answers = append(run.Answers, receipt)
				run.DecisionSuggestions = nil
				run.PendingActions = nil
				run.State, run.PauseReason, run.CurrentAction = RunActive, "", nil
				if err := issueAction(&run, ActionOrient, "Reconsider non-destructive alternatives after destructive scope was declined or received feedback; do not perform the requested destructive operation."); err != nil {
					return nil, nil, err
				}
				break
			}
			if !validSHA256(options.ScopeSHA256) {
				return nil, nil, protocolRunError(run, "destructive_scope_required", "structured destructive confirmation requires a lowercase sha256 scope digest")
			}
			if options.ScopeSHA256 != request.ScopeSHA256 {
				return nil, nil, protocolRunError(run, "destructive_scope_mismatch", "scope_sha256 does not match the current destructive request")
			}
			if run.RemainingBudget < 1 {
				return nil, nil, resumeProtocolError(run, "budget_exhausted", "destructive confirmation cannot issue a fresh implement action without action budget")
			}
			authorization := DestructiveAuthorization{
				RequestID:           request.RequestID,
				OriginatingActionID: actionID,
				ScopeVersion:        DestructiveScopeVersion,
				ScopeSHA256:         request.ScopeSHA256,
				Targets:             append([]DestructiveTarget(nil), request.Targets...),
				Impact:              request.Impact,
				ConfirmedAt:         now.Format(time.RFC3339Nano),
			}
			if err := validateDestructiveGrant(authorization, request, actionID); err != nil {
				return nil, nil, protocolRunError(run, "invalid_destructive_grant", err.Error())
			}
			// Structured destructive confirmation is an authorization attestation,
			// not a product decision. Product feedback is recorded by the separate
			// decline-or-feedback branch above.
			receipt.Active = false
			run.Answers = append(run.Answers, receipt)
			run.DecisionSuggestions = nil
			run.PendingActions = nil
			run.State, run.PauseReason, run.CurrentAction = RunActive, "", nil
			run.PendingDestructiveRequest = cloneDestructiveRequest(&request)
			run.DestructiveGrant = cloneDestructiveAuthorization(&authorization)
			if err := issueAction(&run, ActionImplement, "Perform only the exact destructively authorized scope. If any target or impact changes, stop and return a fresh destructive request."); err != nil {
				return nil, nil, err
			}
		}

		if err := service.validateRunWorkspace(run); err != nil {
			return nil, nil, err
		}
		run.UpdatedAt = now
		event, eventErr := newRunEvent("answer_recorded", before, run)
		if eventErr != nil {
			return nil, nil, eventErr
		}
		result = run
		return []runstore.Event{event}, run, nil
	})
	return result, err
}

func (service *Service) Skip(runID, actionID string) (Run, error) {
	return service.mutate(runID, "action_skipped", func(run *Run) error {
		if run.State != RunActive && run.State != RunPaused {
			return protocolRunError(*run, "run_not_skippable", "run has no skippable action")
		}
		if run.CurrentAction == nil || run.CurrentAction.ActionID != actionID {
			return protocolRunError(*run, "stale_action", "action_id is not the current action")
		}
		kind := run.CurrentAction.Kind
		run.DecisionSuggestions = nil
		run.PendingActions = nil
		if record := findActionRecord(run, actionID); record != nil {
			record.Skipped = true
		}
		clearDestructiveState(run)
		run.State, run.PauseReason, run.CurrentAction = RunActive, "", nil
		return transitionAfterSkip(run, kind)
	})
}

func (service *Service) Stop(runID string) (Run, error) {
	return service.mutate(runID, "run_stopped", func(run *Run) error {
		if run.State == RunEnded {
			return protocolRunError(*run, "run_already_ended", "ended run cannot be stopped")
		}
		clearDestructiveState(run)
		if run.State == RunStopped {
			return nil
		}
		run.State = RunStopped
		run.PauseReason = ""
		return nil
	})
}

func (service *Service) Resume(runID string, options ResumeOptions) (Run, error) {
	if _, err := service.validateOpenWorkspace(); err != nil {
		return Run{}, err
	}
	normalized, err := normalizeResumeOptions(service.store.RepositoryRoot(), options)
	if err != nil {
		return Run{}, err
	}

	var run Run
	var result Run
	err = service.store.UpdateStream(runID, func(event runstore.Event) error {
		return applyRunEvent(&run, event)
	}, func() ([]runstore.Event, any, error) {
		if run.ID != runID {
			return nil, nil, errors.New("run journal identity mismatch")
		}
		if err := service.validateRunWorkspace(run); err != nil {
			return nil, nil, err
		}

		if run.State == RunEnded {
			return nil, nil, protocolRunError(run, "run_already_ended", "ended run cannot be resumed")
		}
		if run.SourceCandidate == nil && normalized.RefreshedSource == nil && !normalized.UsePinnedSource && normalized.SourceChoice != "" && normalized.CandidateID != "" {
			if run.LastSourceChoice != nil && run.LastSourceChoice.CandidateID == normalized.CandidateID {
				if run.LastSourceChoice.Choice != normalized.SourceChoice {
					return nil, nil, resumeProtocolError(run, "source_choice_conflict", "candidate_id was already resolved with a different source choice")
				}
				result = run
				return nil, run, nil
			}
		}

		before := runBeforeMutation(run)
		eventType, mutated, resumeErr := resumeRun(&run, normalized)
		if resumeErr != nil {
			return nil, nil, resumeErr
		}
		if !mutated {
			result = run
			return nil, run, nil
		}
		if err := service.validateRunWorkspace(run); err != nil {
			return nil, nil, err
		}
		run.UpdatedAt = time.Now().UTC()
		event, eventErr := newRunEvent(eventType, before, run)
		if eventErr != nil {
			return nil, nil, eventErr
		}
		result = run
		return []runstore.Event{event}, run, nil
	})
	return result, err
}

func normalizeResumeOptions(workspace string, options ResumeOptions) (ResumeOptions, error) {
	normalized := ResumeOptions{
		UsePinnedSource: options.UsePinnedSource,
		SourceChoice:    options.SourceChoice,
		CandidateID:     options.CandidateID,
	}
	if options.Budget != nil {
		budget := *options.Budget
		if err := ValidateBudget(budget); err != nil {
			return ResumeOptions{}, &ProtocolError{Code: "invalid_budget", Message: err.Error(), Next: NoneNext(workspace)}
		}
		normalized.Budget = &budget
	}
	if options.RefreshedSource != nil {
		candidate := cloneSourceCandidateInput(*options.RefreshedSource)
		if err := validateSourceCandidateInput(candidate); err != nil {
			return ResumeOptions{}, &ProtocolError{
				Code:    "invalid_source_candidate",
				Message: "invalid refreshed source: " + err.Error(),
				Next:    NoneNext(workspace),
			}
		}
		normalized.RefreshedSource = &candidate
	}
	if normalized.SourceChoice != "" && normalized.SourceChoice != SourceChoicePinned && normalized.SourceChoice != SourceChoiceAdopt {
		return ResumeOptions{}, &ProtocolError{
			Code:    "invalid_source_choice",
			Message: "source choice must be pinned or adopt",
			Next:    NoneNext(workspace),
		}
	}
	return normalized, nil
}

func resumeRun(run *Run, options ResumeOptions) (string, bool, error) {
	if run.PinnedSource == nil {
		if options.RefreshedSource != nil || options.UsePinnedSource || options.SourceChoice != "" || options.CandidateID != "" {
			return "", false, resumeProtocolError(*run, "source_mode_not_allowed", "ad-hoc run resume does not accept source options")
		}
		invalidateOutstandingResumeState(run)
		applyResumeBudget(run, options.Budget)
		run.LastResumeResult = &ResumeResult{Operation: ResumeOperationAdHoc, BudgetApplied: true}
		if err := issueAction(run, ActionOrient, "Re-investigate the current worktree after interruption before choosing the next action."); err != nil {
			return "", false, err
		}
		return "run_resumed", true, nil
	}

	if run.SourceCandidate != nil {
		if options.RefreshedSource != nil || options.UsePinnedSource {
			return "", false, resumeProtocolError(*run, "source_candidate_pending", "current source candidate must be resolved before another refresh mode")
		}
		if options.SourceChoice == "" || options.CandidateID == "" {
			return "", false, resumeProtocolError(*run, "source_choice_required", "current source candidate requires both source choice and candidate_id")
		}
		if options.CandidateID != run.SourceCandidate.CandidateID {
			return "", false, protocolRunError(*run, "stale_source_candidate", "candidate_id is not the current source candidate")
		}
		return resolveSourceCandidate(run, options)
	}

	if options.SourceChoice != "" || options.CandidateID != "" {
		if options.SourceChoice == "" || options.CandidateID == "" {
			return "", false, resumeProtocolError(*run, "source_choice_requires_candidate", "source choice and candidate_id must be provided together")
		}
		return "", false, protocolRunError(*run, "stale_source_candidate", "candidate_id is no longer current")
	}
	if options.RefreshedSource != nil && options.UsePinnedSource {
		return "", false, resumeProtocolError(*run, "source_mode_conflict", "issue-bound run resume accepts exactly one source mode")
	}
	if options.RefreshedSource == nil && !options.UsePinnedSource {
		return "", false, resumeProtocolError(*run, "source_mode_required", "issue-bound run resume requires a refreshed source file or explicit pinned source")
	}
	if options.UsePinnedSource {
		invalidateOutstandingResumeState(run)
		applyResumeBudget(run, options.Budget)
		run.Observations = append(run.Observations, ResumeOperationSourceRefreshSkipped)
		run.LastResumeResult = &ResumeResult{Operation: ResumeOperationSourceRefreshSkipped, BudgetApplied: true}
		if err := issueAction(run, ActionOrient, "Re-orient using the explicitly retained pinned source because refresh was skipped or unavailable."); err != nil {
			return "", false, err
		}
		return ResumeOperationSourceRefreshSkipped, true, nil
	}
	return refreshIssueSource(run, *options.RefreshedSource, options.Budget)
}

func refreshIssueSource(run *Run, refreshed SourceCandidateInput, budget *int) (string, bool, error) {
	current := clonePinnedSourceValue(*run.PinnedSource)
	if refreshed.Provider != current.Provider || refreshed.Host != current.Host {
		return "", false, resumeProtocolError(*run, "source_provider_mismatch", "refreshed source provider and host must match the pinned source")
	}
	if refreshed.IssueID != current.IssueID {
		err := resumeProtocolError(*run, "source_issue_mismatch", "refreshed source belongs to a different issue; start a new run")
		err.Next = startRunNext(run.Workspace, run.Goal, true)
		err.Details["pinned_issue_id"] = current.IssueID
		err.Details["refreshed_issue_id"] = refreshed.IssueID
		return "", false, err
	}

	projectionChanged := sourceProjectionChanged(current, refreshed)
	projectedCurrent, refreshed := mergeRefreshedProjection(current, refreshed)
	run.PinnedSource = clonePinnedSource(&projectedCurrent)

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

	if refreshed.RequirementsRevision != current.RequirementsRevision {
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
	if err := issueAction(run, ActionOrient, "Re-orient against the refreshed source snapshot before selecting further work."); err != nil {
		return "", false, err
	}
	return ResumeOperationSourceRefreshed, true, nil
}

func resolveSourceCandidate(run *Run, options ResumeOptions) (string, bool, error) {
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
		markActiveAnswersSuperseded(run, oldRequirementsRevision, "requirements:"+run.PinnedSource.RequirementsRevision)
		operation = ResumeOperationSourceAmended
	}
	invalidateOutstandingResumeState(run)
	run.SourceCandidate = nil
	applyResumeBudget(run, options.Budget)
	run.Observations = append(run.Observations, operation)
	run.LastResumeResult = &ResumeResult{Operation: operation, BudgetApplied: true, CandidateID: candidate.CandidateID}
	if err := issueAction(run, ActionOrient, "Re-orient after the explicit source amendment decision before selecting further work."); err != nil {
		return "", false, err
	}
	actionID := ""
	if run.CurrentAction != nil {
		actionID = run.CurrentAction.ActionID
	}
	run.LastSourceChoice = &SourceChoiceReceipt{
		CandidateID:       candidate.CandidateID,
		Choice:            options.SourceChoice,
		ResultingActionID: actionID,
		At:                time.Now().UTC(),
	}
	return operation, true, nil
}

func markActiveAnswersSuperseded(run *Run, requirementsRevision, supersededBy string) {
	for index := range run.Answers {
		answer := &run.Answers[index]
		if answer.Active && answer.RequirementsRevision == requirementsRevision {
			answer.Active = false
			answer.SupersededBy = supersededBy
		}
	}
}

func markAnswerSuperseded(run *Run, actionID, replacingActionID string) bool {
	for index := range run.Answers {
		answer := &run.Answers[index]
		if answer.ActionID == actionID && answer.Active {
			answer.Active = false
			answer.SupersededBy = replacingActionID
			return true
		}
	}
	return false
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

func sourceProjectionChanged(current PinnedSource, refreshed SourceCandidateInput) bool {
	return current.RepositoryID != refreshed.RepositoryID ||
		current.IssueNumber != refreshed.IssueNumber ||
		current.CanonicalURL != refreshed.CanonicalURL ||
		!sourceParentsEqual(current.Parent, refreshed.Parent)
}

func mergeRefreshedProjection(current PinnedSource, refreshed SourceCandidateInput) (PinnedSource, SourceCandidateInput) {
	aliases := append([]string(nil), current.URLAliases...)
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
	projected.URLAliases = append([]string(nil), aliases...)
	projected.Parent = cloneSourceParent(refreshed.Parent)

	refreshed = cloneSourceCandidateInput(refreshed)
	refreshed.URLAliases = append([]string(nil), aliases...)
	if refreshed.Snapshot != nil {
		refreshed.Snapshot.RepositoryID = refreshed.RepositoryID
		refreshed.Snapshot.IssueNumber = refreshed.IssueNumber
		refreshed.Snapshot.CanonicalURL = refreshed.CanonicalURL
		refreshed.Snapshot.URLAliases = append([]string(nil), aliases...)
		refreshed.Snapshot.Parent = cloneSourceParent(refreshed.Parent)
	}
	return projected, refreshed
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func invalidateOutstandingResumeState(run *Run) {
	voidCurrentAction(run)
	run.CurrentAction = nil
	run.PendingActions = nil
	run.DecisionSuggestions = nil
	clearDestructiveState(run)
	run.State = RunActive
	run.PauseReason = ""
}

func applyResumeBudget(run *Run, replacement *int) {
	if replacement != nil {
		run.RemainingBudget = *replacement
		return
	}
	if run.RemainingBudget > 0 {
		return
	}
	run.RemainingBudget = run.InitialBudget
	if run.RemainingBudget < minimumResumeBudget {
		run.RemainingBudget = minimumResumeBudget
	}
}

func (service *Service) mutate(runID, eventType string, callback func(*Run) error) (Run, error) {
	if _, err := service.validateOpenWorkspace(); err != nil {
		return Run{}, err
	}
	var run Run
	var result Run
	err := service.store.UpdateStream(runID, func(event runstore.Event) error {
		return applyRunEvent(&run, event)
	}, func() ([]runstore.Event, any, error) {
		if run.ID != runID {
			return nil, nil, errors.New("run journal identity mismatch")
		}
		if err := service.validateRunWorkspace(run); err != nil {
			return nil, nil, err
		}
		before := runBeforeMutation(run)
		if err := callback(&run); err != nil {
			return nil, nil, err
		}
		if err := service.validateRunWorkspace(run); err != nil {
			return nil, nil, err
		}
		run.UpdatedAt = time.Now().UTC()
		event, err := newRunEvent(eventType, before, run)
		if err != nil {
			return nil, nil, err
		}
		result = run
		return []runstore.Event{event}, run, nil
	})
	return result, err
}

func acceptOutcome(run *Run, outcome Outcome, payloadSHA256 string) error {
	action := *run.CurrentAction
	record := findActionRecord(run, action.ActionID)
	if record == nil {
		return errors.New("current action is missing from run history")
	}
	record.Outcome = &outcome
	record.OutcomePayloadSHA256 = payloadSHA256
	run.Observations = append(run.Observations, outcome.Observations...)
	run.KnownIssues = append(run.KnownIssues, outcome.KnownIssues...)
	if outcome.Implementation != nil {
		run.Uncertainties = append(run.Uncertainties, outcome.Implementation.Uncertainties...)
		run.Activities = append(run.Activities, outcome.Implementation.Activities...)
	}
	if outcome.Review != nil {
		run.Uncertainties = append(run.Uncertainties, outcome.Review.Uncertainties...)
	}
	clearDestructiveState(run)
	if outcome.Status == OutcomeNeedsInput {
		run.State = RunPaused
		run.PauseReason = outcome.Pause.Reason
		if outcome.Pause.Reason == PauseDestructiveConfirm {
			request, err := NormalizeDestructiveRequest(*outcome.Pause.DestructiveRequest)
			if err != nil {
				return err
			}
			run.PendingDestructiveRequest = cloneDestructiveRequest(&request)
		}
		return nil
	}
	run.PendingActions = append(run.PendingActions, outcome.SuggestedActions...)
	run.CurrentAction = nil
	return transitionFrom(run, action.Kind, outcome)
}

func transitionFrom(run *Run, kind ActionKind, outcome Outcome) error {
	changed := false
	if kind == ActionImplement {
		current, err := runstore.ObserveGit(run.Workspace)
		if err != nil {
			clearDestructiveState(run)
			run.State = RunPaused
			run.PauseReason = PauseEnvironmentUnavailable
			run.KnownIssues = append(run.KnownIssues, "Git observation failed: "+err.Error())
			return nil
		}
		changed = recordGitObservation(run, current)
		recordImplementationDiscrepancy(run, outcome, changed)
	}
	transition := Decide(TransitionInput{
		Kind:          kind,
		Outcome:       outcome,
		CodeChanged:   changed,
		ReviewEnabled: run.ReviewEnabled && !reviewAlreadyReported(*run),
		Pending:       run.PendingActions,
	})
	if transition.PauseReason != "" {
		clearDestructiveState(run)
		run.State = RunPaused
		run.PauseReason = transition.PauseReason
		return nil
	}
	if transition.End {
		clearDestructiveState(run)
		current, err := runstore.ObserveGit(run.Workspace)
		if err != nil {
			run.FinalGitObserved = false
			run.Uncertainties = appendUniqueString(run.Uncertainties, "Final Git observation was unavailable: "+err.Error())
		} else {
			recordGitObservation(run, current)
			run.FinalGitObserved = true
		}
		run.State = RunEnded
		run.CurrentAction = nil
		run.Summary = finalSummary(*run)
		return nil
	}
	if len(run.PendingActions) > 0 && transition.Next == run.PendingActions[0].Kind && transition.Brief == run.PendingActions[0].Brief {
		run.PendingActions = run.PendingActions[1:]
	}
	return issueAction(run, transition.Next, transition.Brief)
}

const (
	observedSinceStart     = "observed_since_start: the current Git observation differs from the run-start snapshot."
	attributionUncertainty = "attribution_uncertainty: concurrent user edits, another Run, or tools may have contributed to the observed difference."
)

func recordGitObservation(run *Run, current runstore.GitObservation) bool {
	run.CurrentGit = cloneGitObservation(current)
	changed := current.ChangedFrom(run.InitialGit)
	if changed {
		run.Observations = appendUniqueString(run.Observations, observedSinceStart)
		run.Uncertainties = appendUniqueString(run.Uncertainties, attributionUncertainty)
	}
	return changed
}

func recordImplementationDiscrepancy(run *Run, outcome Outcome, changed bool) {
	if outcome.Implementation == nil {
		return
	}
	result := outcome.Implementation.Result
	switch {
	case !changed && (result == ImplementationApplied || result == ImplementationPartial):
		run.Observations = appendUniqueString(run.Observations,
			"report_discrepancy: Implement reported "+string(result)+" while no start-to-current Git difference was observed.")
	case changed && (result == ImplementationNotNeeded || result == ImplementationUnable):
		run.Observations = appendUniqueString(run.Observations,
			"report_discrepancy: Implement reported "+string(result)+" while a start-to-current Git difference was observed.")
	}
}

func issueAction(run *Run, kind ActionKind, brief string) error {
	var authorization *DestructiveAuthorization
	if run.DestructiveGrant != nil && kind == ActionImplement && run.PendingDestructiveRequest != nil {
		if err := validateDestructiveGrant(*run.DestructiveGrant, *run.PendingDestructiveRequest, run.DestructiveGrant.OriginatingActionID); err != nil {
			clearDestructiveState(run)
			return &ProtocolError{Code: "invalid_destructive_grant", Message: err.Error(), Next: mustDeriveResumeNext(*run)}
		}
		authorization = cloneDestructiveAuthorization(run.DestructiveGrant)
	} else {
		clearDestructiveState(run)
	}

	remaining, err := ConsumeBudget(run.RemainingBudget)
	if err != nil {
		clearDestructiveState(run)
		run.State = RunPaused
		run.PauseReason = PauseBudgetExhausted
		run.CurrentAction = nil
		return nil
	}
	if strings.TrimSpace(brief) == "" {
		brief = defaultBrief(kind)
	}
	if kind == ActionImplement && !strings.Contains(brief, "Repair-attempt limit:") {
		brief = strings.TrimSpace(brief) + " Repair-attempt limit: 3."
	}
	brief = attributionAwareBrief(*run, kind, brief)
	context, err := buildContext(*run)
	if err != nil {
		clearDestructiveState(run)
		return &ProtocolError{Code: "invalid_action", Message: err.Error(), Next: mustDeriveResumeNext(*run)}
	}
	action := Action{
		ContractVersion:          ContractVersion,
		RunID:                    run.ID,
		ActionID:                 uuid.NewString(),
		Kind:                     kind,
		Goal:                     run.Goal,
		Brief:                    brief,
		Context:                  context,
		DestructiveAuthorization: authorization,
		RemainingBudget:          remaining,
	}
	if run.PinnedSource != nil {
		pinned := clonePinnedSourceValue(*run.PinnedSource)
		action.Source = &ActionSource{
			Kind:                 ActionSourceChangeIssue,
			CanonicalURL:         pinned.CanonicalURL,
			IssueID:              pinned.IssueID,
			SourceRevision:       pinned.SourceRevision,
			RequirementsRevision: pinned.RequirementsRevision,
		}
		requirements := pinned.AcceptedRequirements
		action.Requirements = &requirements
	}
	if err := action.Validate(); err != nil {
		code := "invalid_action"
		if strings.Contains(err.Error(), "exceeds") {
			code = "action_too_large"
		}
		clearDestructiveState(run)
		return &ProtocolError{Code: code, Message: err.Error(), Next: mustDeriveResumeNext(*run)}
	}
	if authorization != nil {
		if err := validateDestructiveGrant(*authorization, *run.PendingDestructiveRequest, authorization.OriginatingActionID); err != nil {
			clearDestructiveState(run)
			return &ProtocolError{Code: "invalid_destructive_grant", Message: err.Error(), Next: mustDeriveResumeNext(*run)}
		}
	}
	run.RemainingBudget = remaining
	run.State = RunActive
	run.PauseReason = ""
	run.Actions = append(run.Actions, ActionRecord{Action: action})
	run.CurrentAction = &action
	return nil
}

func defaultBrief(kind ActionKind) string {
	switch kind {
	case ActionOrient:
		return "Investigate repository facts and identify only unresolved human decisions."
	case ActionClarify:
		return "Ask exactly one unresolved decision with a recommendation, rationale, and alternatives."
	case ActionImplement:
		return "Implement the authorized goal, run relevant technical activities, and report exact results."
	case ActionReview:
		return "Inspect intent and quality against the run-start Git baseline; report findings and uncertainties only, without modifying code."
	case ActionSummarize:
		return "Summarize observed changes, activities, known issues, uncertainties, skipped work, and pre-existing dirty files."
	default:
		return "Perform the requested action and report observations honestly."
	}
}

func attributionAwareBrief(run Run, kind ActionKind, brief string) string {
	if (kind != ActionReview && kind != ActionSummarize) || !run.CurrentGit.ChangedFrom(run.InitialGit) {
		return brief
	}
	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(brief))
	builder.WriteString(" Attribution is uncertain: concurrent user edits, another Run, or tools may have contributed to the observed start-to-current difference.")
	fmt.Fprintf(&builder, " Pre-existing dirty path observations at Run start (count=%d; initial_snapshot=%s; full records remain in Run status):", len(run.InitialGit.PathObservations), run.InitialGit.SnapshotHash)
	if len(run.InitialGit.PathObservations) == 0 {
		builder.WriteString(" none.")
	} else {
		for _, item := range run.InitialGit.PathObservations {
			fmt.Fprintf(&builder, " %s [%s %s; %s", item.Path, item.Category, item.State, item.Observation)
			if item.Size != nil {
				fmt.Fprintf(&builder, "; size=%d", *item.Size)
			}
			if item.ContentSHA256 != "" {
				fmt.Fprintf(&builder, "; content_sha256=%s", item.ContentSHA256)
			}
			builder.WriteString("];")
		}
	}
	return truncateUTF8WithMarker(builder.String(), maxActionBriefBytes)
}

func truncateUTF8WithMarker(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	marker := contextTruncationMarker(value)
	if len(marker) >= limit {
		return marker[:limit]
	}
	prefix := limit - len(marker)
	for prefix > 0 && !utf8.ValidString(value[:prefix]) {
		prefix--
	}
	return value[:prefix] + marker
}

type contextCandidate struct {
	class    *contextClass
	content  string
	selected string
}

type contextClass struct {
	heading    string
	omittedKey string
	candidates []*contextCandidate
}

func buildContext(run Run) (string, error) {
	decisions := &contextClass{heading: "Decisions:", omittedKey: "decisions", candidates: make([]*contextCandidate, 0)}
	recent := &contextClass{heading: "Recent outcome:", omittedKey: "recent outcomes", candidates: make([]*contextCandidate, 0, 1)}
	earlier := &contextClass{heading: "Earlier outcomes:", omittedKey: "earlier outcomes", candidates: make([]*contextCandidate, 0)}
	classes := []*contextClass{decisions, recent, earlier}

	for _, answer := range run.Answers {
		if !answer.Active || answer.Text == "" || answer.ConfirmDestructive {
			continue
		}
		if run.PinnedSource == nil {
			if answer.RequirementsRevision != "" {
				continue
			}
		} else if answer.RequirementsRevision != run.PinnedSource.RequirementsRevision {
			continue
		}
		item := fmt.Sprintf("- action %s decision:\n%s\n", answer.ActionID, indentContextText(answer.Text, "  "))
		normalized, err := normalizeContextItem(item)
		if err != nil {
			return "", fmt.Errorf("normalize decision context: %w", err)
		}
		candidate := &contextCandidate{class: decisions, content: normalized}
		decisions.candidates = append(decisions.candidates, candidate)
	}

	outcomes := make([]*contextCandidate, 0, len(run.Actions))
	for _, record := range run.Actions {
		if record.Outcome == nil {
			continue
		}
		item := renderOutcomeContextItem(record)
		normalized, err := normalizeContextItem(item)
		if err != nil {
			return "", fmt.Errorf("normalize outcome context: %w", err)
		}
		outcomes = append(outcomes, &contextCandidate{content: normalized})
	}
	if len(outcomes) > 0 {
		latest := outcomes[len(outcomes)-1]
		latest.class = recent
		recent.candidates = append(recent.candidates, latest)
		for _, candidate := range outcomes[:len(outcomes)-1] {
			candidate.class = earlier
			earlier.candidates = append(earlier.candidates, candidate)
		}
	}

	priority := make([]*contextCandidate, 0, len(decisions.candidates)+len(outcomes))
	for index := len(decisions.candidates) - 1; index >= 0; index-- {
		priority = append(priority, decisions.candidates[index])
	}
	if len(recent.candidates) == 1 {
		priority = append(priority, recent.candidates[0])
	}
	for index := len(earlier.candidates) - 1; index >= 0; index-- {
		priority = append(priority, earlier.candidates[index])
	}

	for _, candidate := range priority {
		candidate.selected = candidate.content
		if rendered := renderContext(classes); len(rendered) <= maxActionContextBytes {
			continue
		}
		candidate.selected = ""
		marker := contextTruncationMarker(candidate.content)
		candidate.selected = marker + "\n"
		rendered := renderContext(classes)
		if len(rendered) > maxActionContextBytes {
			candidate.selected = ""
			break
		}
		availablePrefix := maxActionContextBytes - len(rendered)
		if availablePrefix > len(candidate.content) {
			availablePrefix = len(candidate.content)
		}
		for availablePrefix > 0 && !utf8.ValidString(candidate.content[:availablePrefix]) {
			availablePrefix--
		}
		candidate.selected = candidate.content[:availablePrefix] + marker + "\n"
		break
	}

	context := renderContext(classes)
	if len(context) > maxActionContextBytes {
		return "", fmt.Errorf("context exceeds %d bytes", maxActionContextBytes)
	}
	if !utf8.ValidString(context) {
		return "", errors.New("context must be valid utf-8")
	}
	return context, nil
}

func renderOutcomeContextItem(record ActionRecord) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "- %s action %s: %s\n", record.Action.Kind, record.Action.ActionID, record.Outcome.Summary)
	if len(record.Outcome.KnownIssues) > 0 {
		builder.WriteString("  Known issues:\n")
		for _, issue := range record.Outcome.KnownIssues {
			builder.WriteString("  - ")
			builder.WriteString(indentContextContinuation(issue, "    "))
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

func normalizeContextItem(value string) (string, error) {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	if !utf8.ValidString(value) {
		return "", errors.New("context candidate must be valid utf-8")
	}
	return value, nil
}

func indentContextText(value, indentation string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return indentation + strings.ReplaceAll(value, "\n", "\n"+indentation)
}

func indentContextContinuation(value, indentation string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.ReplaceAll(value, "\n", "\n"+indentation)
}

func contextTruncationMarker(normalized string) string {
	digest := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("...[truncated original_bytes=%d sha256=%x]", len(normalized), digest)
}

func renderContext(classes []*contextClass) string {
	var builder strings.Builder
	for _, class := range classes {
		builder.WriteString(class.heading)
		builder.WriteByte('\n')
		selected := 0
		for _, candidate := range class.candidates {
			if candidate.selected == "" {
				continue
			}
			builder.WriteString(candidate.selected)
			selected++
		}
		if len(class.candidates) == 0 {
			builder.WriteString("(none)\n")
		} else if omitted := len(class.candidates) - selected; omitted > 0 {
			fmt.Fprintf(&builder, "[omitted %s: %d]\n", class.omittedKey, omitted)
		}
	}
	return builder.String()
}

func finalSummary(run Run) string {
	var builder strings.Builder
	builder.WriteString("The automatic action queue has ended.\n")
	builder.WriteString("Observed action reports:\n")

	changedFiles := map[string]struct{}{}
	var observations, reviewFindings, skipped, voided []string
	var reviewOutcome *Outcome
	var reviewSkippedByUser bool
	for index := range run.Actions {
		record := &run.Actions[index]
		if record.Skipped {
			skipped = append(skipped, string(record.Action.Kind))
			if record.Action.Kind == ActionReview {
				reviewSkippedByUser = true
			}
		}
		if record.Voided {
			voided = append(voided, string(record.Action.Kind))
		}
		if record.Outcome == nil {
			continue
		}
		annotation := ""
		if record.Skipped {
			annotation = " (skipped)"
		} else if record.Voided {
			annotation = " (voided on resume)"
		}
		fmt.Fprintf(&builder, "- %s%s: %s\n", record.Action.Kind, annotation, record.Outcome.Summary)
		if record.Outcome.Implementation != nil {
			for _, path := range record.Outcome.Implementation.FilesChanged {
				if path = strings.TrimSpace(path); path != "" {
					changedFiles[path] = struct{}{}
				}
			}
		}
		if record.Outcome.Review != nil {
			reviewOutcome = record.Outcome
			if record.Outcome.Review.Result == ReviewFindings {
				for _, finding := range record.Outcome.Review.Findings {
					reviewFindings = append(reviewFindings, fmt.Sprintf("%s: %s — %s", finding.Location, finding.Summary, finding.Detail))
				}
			}
		} else {
			observations = append(observations, record.Outcome.Observations...)
		}
	}
	for _, observation := range run.Observations {
		if strings.HasPrefix(observation, "observed_since_start:") || strings.HasPrefix(observation, "report_discrepancy:") {
			observations = appendUniqueString(observations, observation)
		}
	}

	if !run.FinalGitObserved {
		builder.WriteString("CLI Git observation: final worktree state was unavailable; no present-tense change claim is made.\n")
	} else if run.CurrentGit.ChangedFrom(run.InitialGit) {
		builder.WriteString(observedSinceStart + "\n")
		builder.WriteString(attributionUncertainty + "\n")
	} else {
		builder.WriteString("CLI Git observation: no difference from the run-start snapshot was observed.\n")
	}
	files := make([]string, 0, len(changedFiles))
	for path := range changedFiles {
		files = append(files, path)
	}
	sort.Strings(files)
	if len(files) > 0 {
		builder.WriteString("Files reported changed by Implement:\n- " + strings.Join(files, "\n- ") + "\n")
	} else {
		builder.WriteString("No files were reported changed by Implement.\n")
	}
	if len(observations) > 0 {
		builder.WriteString("Observations:\n- " + strings.Join(observations, "\n- ") + "\n")
	}
	if len(reviewFindings) > 0 {
		builder.WriteString("Review findings:\n- " + strings.Join(reviewFindings, "\n- ") + "\n")
	} else if reviewSkippedByUser {
		builder.WriteString("Review was skipped by the user.\n")
	} else if reviewOutcome != nil {
		fmt.Fprintf(&builder, "Review report: %s: %s\n", reviewOutcome.Review.Result, reviewOutcome.Summary)
	} else if !run.ReviewEnabled {
		builder.WriteString("Review was disabled for this run.\n")
	} else {
		builder.WriteString("Review was not run because no changed-code review Action was dispatched.\n")
	}
	if len(run.Activities) == 0 {
		builder.WriteString("No test, typecheck, build, or lint activity was reported.\n")
	} else {
		builder.WriteString("Reported technical activities:\n")
		for _, activity := range run.Activities {
			fmt.Fprintf(&builder, "- %s: %s (exit %d): %s\n", activity.Kind, activity.Command, activity.ExitCode, activity.Summary)
		}
	}
	if len(skipped) > 0 {
		builder.WriteString("Skipped Actions:\n- " + strings.Join(skipped, "\n- ") + "\n")
	}
	if len(voided) > 0 {
		builder.WriteString("Actions voided on resume:\n- " + strings.Join(voided, "\n- ") + "\n")
	}
	if len(run.KnownIssues) > 0 {
		builder.WriteString("Known issues:\n- " + strings.Join(run.KnownIssues, "\n- ") + "\n")
	}
	if len(run.Uncertainties) > 0 {
		builder.WriteString("Uncertainties:\n- " + strings.Join(run.Uncertainties, "\n- ") + "\n")
	}
	if len(run.InitialGit.PathObservations) > 0 {
		builder.WriteString("Pre-existing dirty path observations at Run start:\n")
		for _, item := range run.InitialGit.PathObservations {
			fmt.Fprintf(&builder, "- %s [%s %s; %s", item.Path, item.Category, item.State, item.Observation)
			if item.Size != nil {
				fmt.Fprintf(&builder, "; size=%d", *item.Size)
			}
			if item.ContentSHA256 != "" {
				fmt.Fprintf(&builder, "; content_sha256=%s", item.ContentSHA256)
			}
			builder.WriteString("]\n")
		}
	}
	return builder.String()
}

func runBeforeMutation(run Run) Run {
	run.Actions = append([]ActionRecord(nil), run.Actions...)
	for index := range run.Actions {
		run.Actions[index].Action.DestructiveAuthorization = cloneDestructiveAuthorization(run.Actions[index].Action.DestructiveAuthorization)
		if run.Actions[index].Outcome != nil {
			outcome := *run.Actions[index].Outcome
			if outcome.Pause != nil && outcome.Pause.DestructiveRequest != nil {
				pause := *outcome.Pause
				pause.DestructiveRequest = cloneDestructiveRequest(outcome.Pause.DestructiveRequest)
				outcome.Pause = &pause
			}
			run.Actions[index].Outcome = &outcome
		}
	}
	run.Answers = append([]AnswerRecord(nil), run.Answers...)
	run.PinnedSource = clonePinnedSource(run.PinnedSource)
	run.PendingDestructiveRequest = cloneDestructiveRequest(run.PendingDestructiveRequest)
	run.DestructiveGrant = cloneDestructiveAuthorization(run.DestructiveGrant)
	if run.SourceCandidate != nil {
		candidate := cloneSourceCandidate(*run.SourceCandidate)
		run.SourceCandidate = &candidate
	}
	if run.LastSourceChoice != nil {
		receipt := *run.LastSourceChoice
		run.LastSourceChoice = &receipt
	}
	if run.LastResumeResult != nil {
		result := *run.LastResumeResult
		run.LastResumeResult = &result
	}
	return run
}

func findActionRecord(run *Run, actionID string) *ActionRecord {
	for index := range run.Actions {
		if run.Actions[index].Action.ActionID == actionID {
			return &run.Actions[index]
		}
	}
	return nil
}

func reviewAlreadyReported(run Run) bool {
	for _, record := range run.Actions {
		if record.Action.Kind != ActionReview || record.Voided {
			continue
		}
		if record.Outcome != nil || record.Skipped {
			return true
		}
	}
	return false
}

func outcomePayloadSHA256(outcome Outcome) (string, error) {
	// DecodeOutcome always supplies the exact host bytes. The canonical fallback
	// is only for trusted in-process callers and programmatic tests.
	if outcome.RawSHA256 != "" {
		if !validSHA256(outcome.RawSHA256) {
			return "", errors.New("outcome payload digest is malformed")
		}
		return outcome.RawSHA256, nil
	}
	encoded, err := json.Marshal(outcome)
	if err != nil {
		return "", fmt.Errorf("encode outcome for payload digest: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return fmt.Sprintf("sha256:%x", digest), nil
}

func answerPayloadSHA256(actionID string, options AnswerOptions) (string, error) {
	payload := struct {
		ActionID           string `json:"action_id"`
		ConfirmDestructive bool   `json:"confirm_destructive"`
		ScopeSHA256        string `json:"scope_sha256"`
		Text               string `json:"text"`
	}{
		ActionID:           actionID,
		ConfirmDestructive: options.ConfirmDestructive,
		ScopeSHA256:        options.ScopeSHA256,
		Text:               options.Text,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode answer payload: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return fmt.Sprintf("sha256:%x", digest), nil
}

func findAnswerRecord(run Run, actionID string) *AnswerRecord {
	for index := range run.Answers {
		if run.Answers[index].ActionID == actionID {
			return &run.Answers[index]
		}
	}
	return nil
}

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

func validateRunReceipts(run Run) error {
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
		digest, err := answerPayloadSHA256(answer.ActionID, AnswerOptions{
			Text: answer.Text, ConfirmDestructive: answer.ConfirmDestructive, ScopeSHA256: answer.ScopeSHA256,
		})
		if err != nil || digest != answer.PayloadSHA256 {
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

func validateRunDestructiveState(run Run) error {
	if err := validateRunReceipts(run); err != nil {
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
	if run.State == RunStopped || run.State == RunEnded || run.PauseReason == PauseDecisionRequired || run.PauseReason == PauseEnvironmentUnavailable || run.PauseReason == PauseBudgetExhausted {
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

func transitionAfterSkip(run *Run, kind ActionKind) error {
	switch kind {
	case ActionReview:
		return issueAction(run, ActionSummarize, "Summarize the run after the user skipped advisory Review.")
	case ActionSummarize:
		return endAfterSummarySkip(run)
	case ActionOrient, ActionClarify, ActionImplement:
		current, err := runstore.ObserveGit(run.Workspace)
		if err != nil {
			run.State = RunPaused
			run.PauseReason = PauseEnvironmentUnavailable
			run.KnownIssues = append(run.KnownIssues, "Git observation failed after action skip: "+err.Error())
			return nil
		}
		changed := recordGitObservation(run, current)
		if changed && run.ReviewEnabled && !reviewAlreadyReported(*run) {
			return issueAction(run, ActionReview, "Review the complete observed start-to-current Git difference after the prior Action was skipped; report findings only.")
		}
		return issueAction(run, ActionSummarize, "Summarize observed facts after the prior Action was skipped.")
	default:
		return endAfterSummarySkip(run)
	}
}

func endAfterSummarySkip(run *Run) error {
	current, err := runstore.ObserveGit(run.Workspace)
	observation := "CLI Git observation: final worktree state was unavailable."
	if err != nil {
		run.FinalGitObserved = false
		run.Uncertainties = appendUniqueString(run.Uncertainties, "Final Git observation was unavailable: "+err.Error())
	} else {
		run.FinalGitObserved = true
		if recordGitObservation(run, current) {
			observation = observedSinceStart + " " + attributionUncertainty
		} else {
			observation = "CLI Git observation: no difference from the run-start snapshot was observed."
		}
	}
	run.State = RunEnded
	run.PauseReason = ""
	run.CurrentAction = nil
	clearDestructiveState(run)
	run.Summary = "Summary Action was skipped.\n" + observation + "\nNo host-authored final report was submitted.\n"
	return nil
}

func protocolRunError(run Run, code, message string) *ProtocolError {
	return &ProtocolError{
		Code:    code,
		Message: message,
		Next:    mustDeriveNext(run),
		Details: map[string]any{"run_id": run.ID, "state": run.State},
	}
}

func resumeProtocolError(run Run, code, message string) *ProtocolError {
	err := protocolRunError(run, code, message)
	err.Next = mustDeriveResumeNext(run)
	return err
}

func mustDeriveNext(run Run) Next {
	next, err := DeriveNext(run)
	if err == nil {
		return next
	}
	return NoneNext(run.WorkspaceIdentity.ID)
}

func mustDeriveResumeNext(run Run) Next {
	next, err := DeriveResumeNext(run)
	if err == nil {
		return next
	}
	return NoneNext(run.WorkspaceIdentity.ID)
}

func startRunNext(workspace, goal string, sourceRequired bool) Next {
	base := []string{"slipway", "run", goal, "--budget", fmt.Sprint(DefaultBudget), "--json", "--root", workspace}
	inputs := []NextInput{}
	variantID := "retry-run"
	if sourceRequired {
		variantID = "start-with-source"
		inputs = []NextInput{{Name: "source_file", Type: NextInputPath, Flag: "--source-file", Required: true}}
	}
	next, err := NewCommandNext(NextOperationResume, workspace, variantID, base, inputs)
	if err != nil {
		return NoneNext(workspace)
	}
	return next
}

func refreshInstallNext(workspace string) Next {
	next, err := NewCommandNext(
		NextOperationResume,
		workspace,
		"refresh-adapters",
		[]string{"slipway", "install", "--refresh", "--root", workspace},
		[]NextInput{},
	)
	if err != nil {
		return NoneNext(workspace)
	}
	return next
}
