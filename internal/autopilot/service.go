package autopilot

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

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

// ValidateRunID checks a Run identifier without opening repository storage.
func ValidateRunID(runID string) error {
	return runstore.ValidateRunID(runID)
}

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

const maxRunProjectionObservations = 128

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

type sourceChoiceResolution struct {
	Receipt SourceChoiceReceipt `json:"receipt"`
	Result  ResumeResult        `json:"result"`
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
	ReviewProjection     *Review  `json:"review_projection,omitempty"`
	Voided               bool     `json:"voided,omitempty"`
	Skipped              bool     `json:"skipped,omitempty"`
}

type Run struct {
	ContractVersion           int                        `json:"contract_version"`
	ID                        string                     `json:"id"`
	Goal                      string                     `json:"goal"`
	Workspace                 string                     `json:"workspace"`
	WorkspaceIdentity         runstore.WorkspaceIdentity `json:"workspace_identity"`
	WorkspaceForeign          bool                       `json:"workspace_foreign,omitempty"`
	State                     RunState                   `json:"state"`
	PauseReason               PauseReason                `json:"pause_reason,omitempty"`
	ReviewEnabled             bool                       `json:"review_enabled"`
	ReviewPending             bool                       `json:"review_pending"`
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

	// omittedObservations tracks the synthetic projection marker while replay
	// keeps the authoritative journal history out of the in-memory Run.
	omittedObservations int

	// sourceChoiceHistory is the append-only authoritative projection rebuilt
	// from journal deltas. LastSourceChoice and LastResumeResult remain the
	// public latest-receipt projection.
	sourceChoiceHistory []sourceChoiceResolution

	// Rebuilt during journal replay so retired accepted comment identities remain
	// immutable without adding another field to the public Run projection.
	acceptedSourceComments    map[string]PinnedSourceSection
	acceptedSourceDatabaseIDs map[int64]string
}

func projectRun(run Run) Run {
	observations := run.Observations
	if run.omittedObservations > 0 {
		observations = observations[1:]
	}
	if len(observations) <= maxRunProjectionObservations {
		return run
	}

	omitted := len(observations) - maxRunProjectionObservations
	run.omittedObservations += omitted
	recent := observations[omitted:]
	run.Observations = make([]string, 0, len(recent)+1)
	run.Observations = append(run.Observations, observationHistoryMarker(run.omittedObservations))
	run.Observations = append(run.Observations, recent...)
	return run
}

func observationHistoryMarker(omitted int) string {
	return fmt.Sprintf("...[%d earlier observations in journal]", omitted)
}

func replayRunProjectionEvent(run *Run, event runstore.Event) error {
	if err := applyRunEvent(run, event); err != nil {
		return err
	}
	*run = projectRun(*run)
	return nil
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

// OpenServiceReadOnly opens recovery state without creating or repairing it.
func OpenServiceReadOnly(start string) (*Service, error) {
	store, err := runstore.OpenReadOnly(start)
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
	// Issue #434 §1.3: the user owns the process, so an error must carry a
	// usable recovery next rather than a terminal `none`. When the persisted
	// Run recorded its own worktree root, point the user at that workspace so
	// they can inspect or resume from the Run's canonical location. Only fall
	// back to `none` when no Run workspace is known at all.
	next := NoneNext(expected.ID)
	// Issue #434 §1.3: the user owns the process, so an error must carry a
	// usable recovery next rather than a terminal `none`. Only the per-Run
	// mismatch path (observed != nil) knows the Run's own worktree root to
	// point at; the open-workspace discovery failure has no Run workspace yet.
	// The identity is the Run's *persisted* identity, not a fresh re-discovery,
	// so the recovery command stays accurate even when the path now hosts a
	// different Git identity.
	if observed != nil && expected.WorktreeRoot != "" {
		next = Next{
			Operation:         NextOperationCommand,
			WorkspaceIdentity: expected.ID,
			workspaceRoot:     expected.WorktreeRoot,
			Variants: []NextVariant{{
				ID:       "inspect-run-in-its-workspace",
				BaseArgv: []string{"slipway", "status", "--root", expected.WorktreeRoot},
				Inputs:   []NextInput{},
			}},
		}
		if err := next.Validate(); err != nil {
			next = NoneNext(expected.ID)
		}
	}
	return &ProtocolError{
		Code:    "workspace_identity_mismatch",
		Message: message,
		Next:    next,
		Details: details,
	}
}

func (service *Service) RepositoryRoot() string { return service.store.RepositoryRoot() }
func (service *Service) Close() error           { return service.store.Close() }

func (service *Service) Start(goal string, options CreateOptions) (Run, error) {
	if strings.TrimSpace(goal) == "" {
		return Run{}, &ProtocolError{Code: "goal_required", Message: "goal cannot be empty", Next: NoneNext("")}
	}
	goalErr := ValidateGoal(goal)
	var limitErr *GoalLimitError
	if goalErr != nil && !errors.As(goalErr, &limitErr) {
		return Run{}, &ProtocolError{Code: "invalid_goal", Message: goalErr.Error(), Next: NoneNext("")}
	}
	identity, err := service.validateOpenWorkspace()
	if err != nil {
		return Run{}, err
	}
	workspace := identity.WorktreeRoot
	if limitErr != nil {
		return Run{}, &ProtocolError{
			Code:    "action_too_large",
			Message: limitErr.Error(),
			Next:    startRunNext(workspace, goal, options.Budget, options.ReviewEnabled, options.PinnedSource != nil),
		}
	}
	if err := ValidateBudget(options.Budget); err != nil {
		return Run{}, &ProtocolError{Code: "invalid_budget", Message: err.Error(), Next: startRunNext(workspace, goal, options.Budget, options.ReviewEnabled, false)}
	}
	pinnedSource := clonePinnedSource(options.PinnedSource)
	if pinnedSource != nil {
		if err := validatePinnedSource(*pinnedSource); err != nil {
			return Run{}, &ProtocolError{
				Code:    "invalid_source",
				Message: "invalid pinned source: " + err.Error(),
				Next:    startRunNext(workspace, goal, options.Budget, options.ReviewEnabled, true),
			}
		}
		if err := validateSourceMaterials(*pinnedSource, true); err != nil {
			return Run{}, &ProtocolError{
				Code:    "invalid_source",
				Message: "invalid pinned source materials: " + err.Error(),
				Next:    startRunNext(workspace, goal, options.Budget, options.ReviewEnabled, true),
			}
		}
	}
	observation, err := runstore.ObserveGit(workspace)
	if err != nil {
		return Run{}, &ProtocolError{Code: "git_observation_failed", Message: err.Error(), Next: startRunNext(workspace, goal, options.Budget, options.ReviewEnabled, pinnedSource != nil)}
	}
	materials := runstoreMaterials(pinnedSource)
	if pinnedSource != nil {
		pinnedSource.materials = nil
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
	if !observation.ContentObservationComplete {
		run.Uncertainties = appendUniqueString(run.Uncertainties, contentObservationUncertainty)
	}
	recordAcceptedSourceComments(&run, run.PinnedSource)
	durableRun := runBeforeMutation(run)
	if err := issueAction(&run, durableRun, ActionOrient, "Investigate repository facts, relevant code, Git state, and build/test/lint conventions before deciding what to do."); err != nil {
		if protocolErr, ok := err.(*ProtocolError); ok {
			protocolErr.Next = startRunNext(workspace, goal, options.Budget, options.ReviewEnabled, pinnedSource != nil)
		}
		return Run{}, err
	}
	event, err := newRunEvent("run_started", Run{}, run)
	if err != nil {
		return Run{}, err
	}
	if _, err := service.validateOpenWorkspace(); err != nil {
		return Run{}, err
	}
	if err := service.store.CreateWithMaterials(run.ID, event, run, materials); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (service *Service) Load(runID string) (Run, error) {
	if _, err := service.validateOpenWorkspace(); err != nil {
		return Run{}, err
	}
	if _, err := service.loadOwnedRunHeader(runID); err != nil {
		return Run{}, service.normalizeRunLoadError(err)
	}
	var run Run
	if err := service.store.Visit(runID, func(event runstore.Event) error {
		return replayRunProjectionEvent(&run, event)
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

func (service *Service) normalizeRunLoadError(err error) error {
	if err == nil {
		return nil
	}
	next, nextErr := NewCommandNext(
		NextOperationCommand,
		service.store.RepositoryRoot(),
		"list-runs",
		[]string{"slipway", "status", "--root", service.store.RepositoryRoot()},
		nil,
	)
	if nextErr != nil {
		next = NoneNext(service.store.RepositoryRoot())
	}
	var invalidID *runstore.InvalidRunIDError
	if errors.As(err, &invalidID) {
		return &ProtocolError{Code: "invalid_run_id", Message: err.Error(), Next: next}
	}
	var notFound *runstore.RunNotFoundError
	if errors.As(err, &notFound) {
		return &ProtocolError{Code: "run_not_found", Message: err.Error(), Next: next}
	}
	return err
}

func (service *Service) loadRunHeader(runID string) (Run, error) {
	event, err := service.store.FirstEvent(runID)
	if err != nil {
		return Run{}, err
	}
	var run Run
	if err := applyRunEvent(&run, event); err != nil {
		return Run{}, err
	}
	if run.ID != runID {
		return Run{}, errors.New("run journal identity mismatch")
	}
	return run, nil
}

func (service *Service) loadOwnedRunHeader(runID string) (Run, error) {
	run, err := service.loadRunHeader(runID)
	if err != nil {
		return Run{}, err
	}
	if err := service.validateRunWorkspace(run); err != nil {
		return Run{}, err
	}
	return run, nil
}

func foreignRunStub(run Run) Run {
	return Run{
		ContractVersion:   run.ContractVersion,
		ID:                run.ID,
		Goal:              run.Goal,
		Workspace:         run.Workspace,
		WorkspaceIdentity: run.WorkspaceIdentity,
		WorkspaceForeign:  true,
		State:             run.State,
		CreatedAt:         run.CreatedAt,
	}
}

// UnavailableRun preserves the identity of a recovery directory that could not
// be safely interpreted as a Run.
type UnavailableRun struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

func (service *Service) List() ([]Run, error) {
	runs, unavailable, err := service.ListRecovery()
	if err != nil {
		return nil, err
	}
	if len(unavailable) > 0 {
		return runs, fmt.Errorf(
			"%d recovery directories are unavailable (first %s: %s)",
			len(unavailable),
			unavailable[0].ID,
			unavailable[0].Detail,
		)
	}
	return runs, nil
}

// ListRecovery returns valid Runs and separately identifies unreadable local
// recovery directories without fabricating a Run state from corrupt bytes.
func (service *Service) ListRecovery() ([]Run, []UnavailableRun, error) {
	if _, err := service.validateOpenWorkspace(); err != nil {
		return nil, nil, err
	}
	ids, err := service.store.ListIDs()
	if err != nil {
		return nil, nil, err
	}
	runs := make([]Run, 0, len(ids))
	unavailable := make([]UnavailableRun, 0)
	for _, id := range ids {
		header, err := service.loadRunHeader(id)
		if err != nil {
			unavailable = append(unavailable, UnavailableRun{ID: id, Code: "run_journal_invalid", Detail: err.Error()})
			continue
		}
		if workspaceErr := service.validateRunWorkspace(header); workspaceErr != nil {
			var protocolErr *ProtocolError
			if errors.As(workspaceErr, &protocolErr) && protocolErr.Code == "workspace_identity_mismatch" {
				runs = append(runs, foreignRunStub(header))
				continue
			}
			unavailable = append(unavailable, UnavailableRun{ID: id, Code: "run_unavailable", Detail: workspaceErr.Error()})
			continue
		}
		var run Run
		loadErr := service.store.Visit(id, func(event runstore.Event) error {
			return replayRunProjectionEvent(&run, event)
		})
		if loadErr != nil {
			unavailable = append(unavailable, UnavailableRun{ID: id, Code: unavailableRunCode(loadErr), Detail: loadErr.Error()})
			continue
		}
		if run.ID != id {
			unavailable = append(unavailable, UnavailableRun{ID: id, Code: "run_journal_invalid", Detail: "run journal identity mismatch"})
			continue
		}
		if loadErr = service.validateRunWorkspace(run); loadErr != nil {
			unavailable = append(unavailable, UnavailableRun{ID: id, Code: "run_unavailable", Detail: loadErr.Error()})
			continue
		}
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].ID < runs[j].ID
		}
		return runs[i].CreatedAt.Before(runs[j].CreatedAt)
	})
	sort.Slice(unavailable, func(i, j int) bool { return unavailable[i].ID < unavailable[j].ID })
	return runs, unavailable, nil
}

func unavailableRunCode(err error) string {
	if errors.Is(err, ErrRunBusy) {
		return "run_busy"
	}
	return "run_journal_invalid"
}

// ErrRunBusy identifies a bounded timeout while acquiring a Run's
// commit-boundary lock, without exposing the storage package to CLI callers.
var ErrRunBusy = runstore.ErrRunLockTimeout

func (service *Service) Submit(runID, actionID string, outcome Outcome) (Run, error) {
	if _, err := service.validateOpenWorkspace(); err != nil {
		return Run{}, err
	}
	if _, err := service.loadOwnedRunHeader(runID); err != nil {
		return Run{}, err
	}
	payloadSHA256, err := outcomePayloadSHA256(outcome)
	if err != nil {
		return Run{}, &ProtocolError{Code: "invalid_outcome", Message: err.Error(), Next: NoneNext(service.store.RepositoryRoot())}
	}
	var run Run
	var result Run
	err = service.store.UpdateStream(runID, func(event runstore.Event) error {
		return replayRunProjectionEvent(&run, event)
	}, func() ([]runstore.Event, any, error) {
		if run.ID != runID {
			return nil, nil, errors.New("run journal identity mismatch")
		}
		if err := service.validateRunWorkspace(run); err != nil {
			return nil, nil, err
		}
		// An Action that already carries an outcome answers for itself, in every
		// run state: an exact retry replays, a different payload conflicts, and
		// an invalidated Action refuses both.
		record := findActionRecord(&run, actionID)
		if record != nil && record.Outcome != nil {
			switch {
			case record.Voided:
				return nil, nil, protocolRunError(run, "stale_action", "action_id was voided and can no longer accept an outcome")
			case record.OutcomePayloadSHA256 == payloadSHA256:
				result = run
				return nil, run, nil
			default:
				return nil, nil, protocolRunError(run, "outcome_conflict", "this action already has a different outcome payload")
			}
		}
		// For an Action with no outcome, a stopped or ended Run reports its own
		// state first: it withdrew every outstanding Action, so "that Action is
		// stale" would name a consequence rather than the fact to act on.
		if run.State == RunStopped || run.State == RunEnded {
			return nil, nil, protocolRunError(run, "run_not_active", "run is not accepting outcomes while "+string(run.State))
		}
		if record != nil && record.Voided {
			return nil, nil, protocolRunError(run, "stale_action", "action_id was voided and can no longer accept an outcome")
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
		if err := acceptOutcome(&run, before, outcome, payloadSHA256); err != nil {
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
		projection := projectRun(run)
		result = projection
		return []runstore.Event{event}, projection, nil
	})
	return result, err
}

func (service *Service) Answer(runID, actionID string, options AnswerOptions) (Run, error) {
	if _, err := service.validateOpenWorkspace(); err != nil {
		return Run{}, err
	}
	if _, err := service.loadOwnedRunHeader(runID); err != nil {
		return Run{}, err
	}
	// The answer text is an uninterpreted value (issue #434 §9.6). Preserve
	// and digest the caller's valid UTF-8 bytes verbatim. Trimming is limited
	// to the non-empty validation below.
	if err := ValidateAnswerText(options.Text); err != nil {
		code := "invalid_answer"
		var limitErr *AnswerLimitError
		if errors.As(err, &limitErr) {
			code = "answer_too_large"
		}
		return Run{}, &ProtocolError{Code: code, Message: err.Error(), Next: NoneNext(service.store.RepositoryRoot())}
	}
	payloadSHA256, err := answerPayloadSHA256(actionID, options)
	if err != nil {
		return Run{}, err
	}

	var run Run
	var result Run
	var responseErr error
	err = service.store.UpdateStream(runID, func(event runstore.Event) error {
		return replayRunProjectionEvent(&run, event)
	}, func() ([]runstore.Event, any, error) {
		if run.ID != runID {
			return nil, nil, errors.New("run journal identity mismatch")
		}
		if err := service.validateRunWorkspace(run); err != nil {
			return nil, nil, err
		}
		if receipt := findAnswerRecord(run, actionID); receipt != nil {
			if answerReceiptMatches(*receipt, payloadSHA256) {
				result = run
				if receipt.ConfirmDestructive && run.State == RunPaused && run.PauseReason == PauseBudgetExhausted && run.RemainingBudget == 0 {
					responseErr = resumeProtocolError(run, "budget_exhausted", "destructive confirmation cannot issue a fresh implement action without action budget")
				}
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
			if strings.TrimSpace(options.Text) == "" {
				return nil, nil, protocolRunError(run, "answer_required", "decision answer requires text")
			}
			if supersededActionID := decisionSupersessionForAction(run, actionID); supersededActionID != "" {
				if !markAnswerSuperseded(&run, supersededActionID, actionID) {
					return nil, nil, protocolRunError(run, "invalid_decision_supersession", "superseded answer is no longer active")
				}
			}
			voidCurrentAction(&run)
			clearDestructiveState(&run)
			receipt.Active = true
			run.Answers = append(run.Answers, receipt)
			run.PendingActions = nil
			run.State, run.PauseReason, run.CurrentAction = RunActive, "", nil
			if err := issueAction(&run, before, ActionOrient, "Re-orient after the user's decision before selecting further work."); err != nil {
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
				if strings.TrimSpace(options.Text) == "" {
					return nil, nil, protocolRunError(run, "answer_required", "destructive feedback or decline requires text")
				}
				voidCurrentAction(&run)
				clearDestructiveState(&run)
				receipt.Active = true
				run.Answers = append(run.Answers, receipt)
				run.PendingActions = nil
				run.State, run.PauseReason, run.CurrentAction = RunActive, "", nil
				if err := issueAction(&run, before, ActionOrient, "Reconsider non-destructive alternatives after destructive scope was declined or received feedback; do not perform the requested destructive operation."); err != nil {
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
			discardPendingActions(&run, "when the structured destructive confirmation issued the authorized Implement Action")
			run.PendingDestructiveRequest = cloneDestructiveRequest(&request)
			run.DestructiveGrant = cloneDestructiveAuthorization(&authorization)
			if run.RemainingBudget < 1 {
				// Confirmation cannot create a grant that survives a resume. Record the
				// receipt, void the waiting Action, and require a fresh request after
				// budget recovery.
				voidCurrentAction(&run)
				clearDestructiveState(&run)
				run.State, run.PauseReason, run.CurrentAction = RunPaused, PauseBudgetExhausted, nil
				responseErr = resumeProtocolError(run, "budget_exhausted", "destructive confirmation cannot issue a fresh implement action without action budget")
				break
			}
			run.State, run.PauseReason, run.CurrentAction = RunActive, "", nil
			if err := issueAction(&run, before, ActionImplement, "Perform only the exact destructively authorized scope. If any target or impact changes, stop and return a fresh destructive request."); err != nil {
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
		projection := projectRun(run)
		result = projection
		return []runstore.Event{event}, projection, nil
	})
	if err == nil && responseErr != nil {
		return result, responseErr
	}
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
		durableRun := runBeforeMutation(*run)
		kind := run.CurrentAction.Kind
		discardPendingActions(run, "when the user skipped the current Action")
		if record := findActionRecord(run, actionID); record != nil {
			record.Skipped = true
			if kind == ActionReview {
				record.ReviewProjection = &Review{
					Result:        ReviewNotRun,
					Findings:      []Finding{},
					Uncertainties: []string{},
				}
			}
		}
		clearDestructiveState(run)
		run.State, run.PauseReason, run.CurrentAction = RunActive, "", nil
		return transitionAfterSkip(run, durableRun, kind)
	})
}

func (service *Service) Stop(runID string) (Run, error) {
	return service.mutate(runID, "run_stopped", func(run *Run) error {
		if run.State == RunEnded {
			return protocolRunError(*run, "run_already_ended", "ended run cannot be stopped")
		}
		clearDestructiveState(run)
		// A stopped Run has no current Action: it cannot be submitted, skipped,
		// or read for material, and resume always issues a fresh Orient instead.
		// Voiding it here rather than at resume stops the Run from publishing an
		// Action nothing can execute — and, after a destructive confirmation, the
		// spent authorization embedded in it. The issued Action itself stays in
		// the journal, so what was requested and confirmed remains auditable.
		voidCurrentAction(run)
		run.CurrentAction = nil
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
	if _, err := service.loadOwnedRunHeader(runID); err != nil {
		return Run{}, err
	}
	normalized, err := normalizeResumeOptions(service.store.RepositoryRoot(), options)
	if err != nil {
		return Run{}, err
	}

	var run Run
	var result Run
	err = service.store.UpdateStreamWithMaterials(runID, func(event runstore.Event) error {
		return replayRunProjectionEvent(&run, event)
	}, func() (runstore.UpdateResult, error) {
		if run.ID != runID {
			return runstore.UpdateResult{}, errors.New("run journal identity mismatch")
		}
		if err := service.validateRunWorkspace(run); err != nil {
			return runstore.UpdateResult{}, err
		}

		if run.SourceCandidate == nil && normalized.RefreshedSource == nil && !normalized.UsePinnedSource && normalized.SourceChoice != "" && normalized.CandidateID != "" {
			if resolution := findSourceChoiceResolution(run, normalized.CandidateID); resolution != nil {
				if resolution.Receipt.Choice != normalized.SourceChoice {
					return runstore.UpdateResult{}, resumeProtocolError(run, "source_choice_conflict", "candidate_id was already resolved with a different source choice")
				}
				result = run
				receipt := resolution.Receipt
				resumeResult := resolution.Result
				result.LastSourceChoice = &receipt
				result.LastResumeResult = &resumeResult
				return runstore.UpdateResult{Projection: run}, nil
			}
		}
		if run.State == RunEnded {
			return runstore.UpdateResult{}, protocolRunError(run, "run_already_ended", "ended run cannot be resumed")
		}
		currentNext, nextErr := DeriveNext(run)
		if nextErr != nil {
			return runstore.UpdateResult{}, fmt.Errorf("derive current resume eligibility: %w", nextErr)
		}
		if currentNext.Operation != NextOperationResume {
			return runstore.UpdateResult{}, protocolRunError(
				run,
				"run_not_resumable",
				"run cannot be resumed while its current typed next operation is "+string(currentNext.Operation),
			)
		}

		materials := []runstore.Material(nil)
		if normalized.RefreshedSource != nil && normalized.RefreshedSource.Snapshot != nil {
			materials = runstoreMaterials(normalized.RefreshedSource.Snapshot)
			normalized.RefreshedSource.Snapshot.materials = nil
		}
		before := runBeforeMutation(run)
		eventType, mutated, resumeErr := resumeRun(&run, before, normalized)
		if resumeErr != nil {
			return runstore.UpdateResult{}, resumeErr
		}
		if !mutated {
			result = run
			return runstore.UpdateResult{Projection: run}, nil
		}
		if err := service.validateRunWorkspace(run); err != nil {
			return runstore.UpdateResult{}, err
		}
		run.UpdatedAt = time.Now().UTC()
		event, eventErr := newRunEvent(eventType, before, run)
		if eventErr != nil {
			return runstore.UpdateResult{}, eventErr
		}
		projection := projectRun(run)
		result = projection
		return runstore.UpdateResult{
			Events:     []runstore.Event{event},
			Projection: projection,
			Materials:  materials,
		}, nil
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

func resumeRun(run *Run, durableRun Run, options ResumeOptions) (string, bool, error) {
	if run.PinnedSource == nil {
		if options.RefreshedSource != nil || options.UsePinnedSource || options.SourceChoice != "" || options.CandidateID != "" {
			return "", false, resumeProtocolError(*run, "source_mode_not_allowed", "ad-hoc run resume does not accept source options")
		}
		invalidateOutstandingResumeState(run)
		applyResumeBudget(run, options.Budget)
		run.LastResumeResult = &ResumeResult{Operation: ResumeOperationAdHoc, BudgetApplied: true}
		if err := issueAction(run, durableRun, ActionOrient, "Re-investigate the current worktree after interruption before choosing the next action."); err != nil {
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
		return resolveSourceCandidate(run, durableRun, options)
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
		if err := issueAction(run, durableRun, ActionOrient, "Re-orient using the explicitly retained pinned source because refresh was skipped or unavailable."); err != nil {
			return "", false, err
		}
		return ResumeOperationSourceRefreshSkipped, true, nil
	}
	return refreshIssueSource(run, durableRun, *options.RefreshedSource, options.Budget)
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

func validateDecisionSupersession(run Run, outcome Outcome) error {
	if outcome.Pause == nil || outcome.Pause.SupersedesAnswerActionID == nil {
		return nil
	}
	actionID := *outcome.Pause.SupersedesAnswerActionID
	answer := findAnswerRecord(run, actionID)
	if answer == nil {
		return protocolRunError(run, "invalid_decision_supersession", "supersedes_answer_action_id does not identify a recorded answer")
	}
	if !answer.Active || answer.ConfirmDestructive {
		return protocolRunError(run, "invalid_decision_supersession", "supersedes_answer_action_id must identify an active non-authorization answer")
	}
	if run.PinnedSource == nil {
		if answer.RequirementsRevision != "" {
			return protocolRunError(run, "invalid_decision_supersession", "superseded answer does not belong to the current ad-hoc requirements context")
		}
	} else if answer.RequirementsRevision != run.PinnedSource.RequirementsRevision {
		return protocolRunError(run, "invalid_decision_supersession", "superseded answer does not belong to the current requirements revision")
	}
	return nil
}

func decisionSupersessionForAction(run Run, actionID string) string {
	record := findActionRecord(&run, actionID)
	if record == nil || record.Outcome == nil || record.Outcome.Pause == nil || record.Outcome.Pause.SupersedesAnswerActionID == nil {
		return ""
	}
	return *record.Outcome.Pause.SupersedesAnswerActionID
}

// discardPendingActions empties the host-suggested queue and records exactly
// what was dropped. Use it wherever no fresh Orient follows: the run would
// otherwise reach its Summary without ever naming a suggested decision it never
// asked, which reads as if nothing was outstanding.
func discardPendingActions(run *Run, reason string) {
	for _, pending := range run.PendingActions {
		run.Uncertainties = appendUniqueString(run.Uncertainties, fmt.Sprintf(
			"unresolved_suggestion: a queued %s suggestion was dropped %s and was never dispatched: %s",
			pending.Kind,
			reason,
			pending.Brief,
		))
	}
	run.PendingActions = nil
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
	if _, err := service.loadOwnedRunHeader(runID); err != nil {
		return Run{}, err
	}
	var run Run
	var result Run
	err := service.store.UpdateStream(runID, func(event runstore.Event) error {
		return replayRunProjectionEvent(&run, event)
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
		projection := projectRun(run)
		result = projection
		return []runstore.Event{event}, projection, nil
	})
	return result, err
}

func acceptOutcome(run *Run, durableRun Run, outcome Outcome, payloadSHA256 string) error {
	action := *run.CurrentAction
	if err := validateDecisionSupersession(*run, outcome); err != nil {
		return err
	}
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
		if !observeGitAfterAction(run, action.Kind, outcome) {
			return nil
		}
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
	if !observeGitAfterAction(run, action.Kind, outcome) {
		return nil
	}
	return transitionFrom(run, durableRun, action.Kind, outcome)
}

func transitionFrom(run *Run, durableRun Run, kind ActionKind, outcome Outcome) error {
	if kind == ActionReview {
		run.ReviewPending = false
	}
	transition := Decide(TransitionInput{
		Kind:          kind,
		Outcome:       outcome,
		CodeChanged:   run.ReviewPending,
		ReviewEnabled: run.ReviewEnabled,
		Pending:       run.PendingActions,
	})
	if transition.Next == ActionReview && run.ReviewPending {
		// A newly observed revision is a safety override, not a detour through a
		// host-selected queue. Review it, then follow the normal Summary route.
		// The override still reports every suggestion it drops, because Review
		// routes to Summary and never returns to the queue.
		discardPendingActions(run, "when a newly observed revision preempted the queue with advisory Review")
	}
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
	return issueAction(run, durableRun, transition.Next, transition.Brief)
}

const (
	observedSinceStart            = "observed_since_start: the current Git observation differs from the run-start snapshot."
	attributionUncertainty        = "attribution_uncertainty: concurrent user edits, another Run, or tools may have contributed to the observed difference."
	contentObservationUncertainty = "git_content_observation_incomplete: bounded content observation could not fingerprint every dirty regular file; any advisory Review remains skippable."
)

func observeGitAfterAction(run *Run, kind ActionKind, outcome Outcome) bool {
	if kind != ActionOrient && kind != ActionClarify && kind != ActionImplement {
		return true
	}
	current, err := runstore.ObserveGit(run.Workspace)
	if err != nil {
		clearDestructiveState(run)
		run.State = RunPaused
		run.PauseReason = PauseEnvironmentUnavailable
		run.KnownIssues = append(run.KnownIssues, "Git observation failed: "+err.Error())
		return false
	}
	changedSinceStart := recordGitObservation(run, current)
	run.ReviewPending = run.ReviewEnabled && changedSinceStart
	if kind == ActionImplement {
		recordImplementationDiscrepancy(run, outcome, changedSinceStart)
	}
	return true
}

func recordGitObservation(run *Run, current runstore.GitObservation) bool {
	run.CurrentGit = cloneGitObservation(current)
	if !current.ContentObservationComplete {
		run.Uncertainties = appendUniqueString(run.Uncertainties, contentObservationUncertainty)
	}
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

func issueAction(run *Run, durableRun Run, kind ActionKind, brief string) error {
	var authorization *DestructiveAuthorization
	if run.DestructiveGrant != nil && kind == ActionImplement && run.PendingDestructiveRequest != nil {
		if err := validateDestructiveGrant(*run.DestructiveGrant, *run.PendingDestructiveRequest, run.DestructiveGrant.OriginatingActionID); err != nil {
			clearDestructiveState(run)
			return &ProtocolError{Code: "invalid_destructive_grant", Message: err.Error(), Next: mustDeriveNext(durableRun)}
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
	brief = composeActionBrief(*run, kind, brief)
	action := Action{
		ContractVersion:          ContractVersion,
		RunID:                    run.ID,
		ActionID:                 uuid.NewString(),
		Kind:                     kind,
		Goal:                     run.Goal,
		Brief:                    brief,
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
			ManifestRevision:     pinned.ManifestRevision,
			RequirementsRevision: pinned.RequirementsRevision,
		}
		requirements := actionRequirements(run.Workspace, action.RunID, action.ActionID, pinned)
		action.Requirements = &requirements
	}
	encodedContextLimit, err := actionContextEncodedLimit(action)
	if err != nil {
		clearDestructiveState(run)
		return actionProtocolError(durableRun, err)
	}
	context, err := buildContextWithinEncodedLimit(*run, encodedContextLimit)
	if err != nil {
		clearDestructiveState(run)
		return actionProtocolError(durableRun, err)
	}
	action.Context = context
	if err := action.Validate(); err != nil {
		clearDestructiveState(run)
		return actionProtocolError(durableRun, err)
	}
	if authorization != nil {
		if err := validateDestructiveGrant(*authorization, *run.PendingDestructiveRequest, authorization.OriginatingActionID); err != nil {
			clearDestructiveState(run)
			return &ProtocolError{Code: "invalid_destructive_grant", Message: err.Error(), Next: mustDeriveNext(durableRun)}
		}
	}
	run.RemainingBudget = remaining
	run.State = RunActive
	run.PauseReason = ""
	run.Actions = append(run.Actions, ActionRecord{Action: action})
	run.CurrentAction = &action
	return nil
}

func actionProtocolError(durableRun Run, err error) *ProtocolError {
	code := "invalid_action"
	if errors.Is(err, errActionTooLarge) {
		code = "action_too_large"
	}
	return &ProtocolError{Code: code, Message: err.Error(), Next: mustDeriveNext(durableRun)}
}

func actionRequirements(workspace, runID, actionID string, source PinnedSource) ActionRequirements {
	sections := make([]ActionRequirementSection, 0, len(source.Sections))
	keys := make([]string, 0, len(source.Sections))
	for _, section := range source.Sections {
		sections = append(sections, ActionRequirementSection{
			Key:             section.Key,
			Role:            section.Role,
			Title:           section.Title,
			SectionRevision: section.SectionRevision,
			MaterialSHA256:  section.MaterialSHA256,
			Bytes:           section.Bytes,
		})
		keys = append(keys, section.Key)
	}
	return ActionRequirements{
		RequirementsRevision: source.RequirementsRevision,
		Sections:             sections,
		RequiredForAction:    append([]string(nil), keys...),
		Reader: ActionMaterialReader{
			Operation: "read_material",
			BaseArgv: []string{
				"slipway", "protocol", "material", "--root", workspace,
				"--run", runID, "--action", actionID,
			},
			Input: ActionMaterialInput{
				Name:     "section",
				Type:     "enum",
				Flag:     "--section",
				Required: true,
				Choices:  append([]string(nil), keys...),
			},
		},
	}
}

func runstoreMaterials(source *PinnedSource) []runstore.Material {
	if source == nil || len(source.materials) == 0 {
		return nil
	}
	materials := make([]runstore.Material, len(source.materials))
	for index, material := range source.materials {
		materials[index] = runstore.Material{
			Digest: material.Digest,
			Data:   append([]byte(nil), material.Data...),
		}
	}
	return materials
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
		return "Summarize confirmed human decisions, observed changes, activities, known issues, uncertainties, skipped work, and pre-existing dirty files."
	default:
		return "Perform the requested action and report observations honestly."
	}
}

func composeActionBrief(run Run, kind ActionKind, brief string) string {
	brief = strings.TrimSpace(brief)
	if brief == "" {
		brief = defaultBrief(kind)
	}
	brief = attributionAwareBrief(run, kind, brief)
	return truncateUTF8WithMarker(brief, maxActionBriefBytes)
}

func attributionAwareBrief(run Run, kind ActionKind, brief string) string {
	if (kind != ActionReview && kind != ActionSummarize) || !run.CurrentGit.ChangedFrom(run.InitialGit) {
		return brief
	}
	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(brief))
	builder.WriteString(" Attribution is uncertain: concurrent user edits, another Run, or tools may have contributed to the observed start-to-current difference.")
	fmt.Fprintf(&builder, " Pre-existing dirty path observations at Run start (count=%d; retained=%d; path_fingerprint=%s; initial_snapshot=%s):", run.InitialGit.PathCount, len(run.InitialGit.PathObservations), run.InitialGit.PathFingerprint, run.InitialGit.SnapshotHash)
	if run.InitialGit.PathCount == 0 {
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
		if run.InitialGit.DetailsTruncated {
			fmt.Fprintf(&builder, " %d additional path detail(s) omitted from the bounded projection;", run.InitialGit.PathCount-len(run.InitialGit.PathObservations))
		}
	}
	return truncateUTF8WithMarker(builder.String(), maxActionBriefBytes)
}

func runBeforeMutation(run Run) Run {
	run.Actions = append([]ActionRecord(nil), run.Actions...)
	for index := range run.Actions {
		run.Actions[index].Action.DestructiveAuthorization = cloneDestructiveAuthorization(run.Actions[index].Action.DestructiveAuthorization)
		if run.Actions[index].ReviewProjection != nil {
			review := *run.Actions[index].ReviewProjection
			review.Findings = append([]Finding{}, review.Findings...)
			review.Uncertainties = append([]string{}, review.Uncertainties...)
			run.Actions[index].ReviewProjection = &review
		}
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
	run.sourceChoiceHistory = append([]sourceChoiceResolution(nil), run.sourceChoiceHistory...)
	if acceptedSourceComments := run.acceptedSourceComments; acceptedSourceComments != nil {
		run.acceptedSourceComments = make(map[string]PinnedSourceSection, len(acceptedSourceComments))
		for nodeID, section := range acceptedSourceComments {
			run.acceptedSourceComments[nodeID] = section
		}
	}
	if acceptedSourceDatabaseIDs := run.acceptedSourceDatabaseIDs; acceptedSourceDatabaseIDs != nil {
		run.acceptedSourceDatabaseIDs = make(map[int64]string, len(acceptedSourceDatabaseIDs))
		for databaseID, nodeID := range acceptedSourceDatabaseIDs {
			run.acceptedSourceDatabaseIDs[databaseID] = nodeID
		}
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
	return framedRevision(
		"slipway-answer/v1",
		actionID,
		fmt.Sprint(options.ConfirmDestructive),
		options.ScopeSHA256,
		options.Text,
	), nil
}

func answerReceiptMatches(receipt AnswerRecord, payloadSHA256 string) bool {
	return receipt.PayloadSHA256 == payloadSHA256
}

func findAnswerRecord(run Run, actionID string) *AnswerRecord {
	for index := range run.Answers {
		if run.Answers[index].ActionID == actionID {
			return &run.Answers[index]
		}
	}
	return nil
}

func transitionAfterSkip(run *Run, durableRun Run, kind ActionKind) error {
	switch kind {
	case ActionReview:
		run.ReviewPending = false
		return issueAction(run, durableRun, ActionSummarize, "Summarize the run after the user skipped advisory Review.")
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
		changedSinceStart := recordGitObservation(run, current)
		run.ReviewPending = run.ReviewEnabled && changedSinceStart
		if run.ReviewPending {
			// Skip already emptied and reported the queue; nothing is left to drop.
			return issueAction(run, durableRun, ActionReview, "Review the complete observed start-to-current Git difference after the prior Action was skipped; report findings only.")
		}
		return issueAction(run, durableRun, ActionSummarize, "Summarize observed facts after the prior Action was skipped.")
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

func startRunNext(workspace, _ string, budget int, reviewEnabled, sourceRequired bool) Next {
	if ValidateBudget(budget) != nil {
		budget = DefaultBudget
	}
	base := []string{"slipway", "run", "--budget", fmt.Sprint(budget), "--json", "--root", workspace}
	if !reviewEnabled {
		base = append(base, "--no-review")
	}
	inputs := []NextInput{{
		Name: "goal_file", Type: NextInputPath, Flag: "--goal-file", Required: true,
	}}
	variantID := "retry-run"
	if sourceRequired {
		variantID = "start-with-source"
		inputs = append(inputs, NextInput{
			Name: "source_file", Type: NextInputPath, Flag: "--source-file", Required: true,
		})
	}
	next, err := NewCommandNext(NextOperationStart, workspace, variantID, base, inputs)
	if err != nil {
		return NoneNext(workspace)
	}
	return next
}

func refreshInstallNext(workspace string) Next {
	next, err := NewCommandNext(
		NextOperationCommand,
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
