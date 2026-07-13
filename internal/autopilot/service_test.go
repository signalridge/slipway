package autopilot

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/signalridge/slipway/internal/runstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceCompleteRequestUsesDefaultRouteAndReportsActivitiesHonestly(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)
	assert.Equal(t, ActionOrient, run.CurrentAction.Kind)

	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts gathered"})
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionImplement, run.CurrentAction.Kind)
	assert.Contains(t, run.CurrentAction.Brief, "Repair-attempt limit: 3.")

	require.NoError(t, os.WriteFile(filepath.Join(repository, "new.go"), []byte("package sample\n"), 0o600))
	implementation := implementationReport(ImplementationApplied, "new.go")
	implementation.Activities = []Activity{{Kind: "test", Command: "go test ./...", ExitCode: 1, Summary: "reported failure"}}
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "code written", Implementation: implementation})
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionReview, run.CurrentAction.Kind)

	finding := Finding{Location: "new.go:1", Summary: "edge case is not handled", Detail: "add the missing branch"}
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "one finding", Review: reviewReport(ReviewFindings, finding), KnownIssues: []string{"handle edge case"}})
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionSummarize, run.CurrentAction.Kind)

	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts reported"})
	assert.Equal(t, RunEnded, run.State)
	assert.Contains(t, run.Summary, "go test ./...")
	assert.Contains(t, run.Summary, "exit 1")
	assert.Contains(t, run.Summary, "Files reported changed by Implement:\n- new.go")
	assert.Contains(t, run.Summary, "Review findings:\n- new.go:1: edge case is not handled")
	assert.Contains(t, run.Summary, "handle edge case")
	assert.NotContains(t, run.Summary, "No test, typecheck")
}

func TestServiceOrientWithoutSuggestionRoutesSummary(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 4, false)

	run = submitCurrent(t, service, run, Outcome{
		Status:           OutcomeCompleted,
		Summary:          "no outstanding work",
		SuggestedActions: []SuggestedAction{},
	})
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionSummarize, run.CurrentAction.Kind)
}

func TestServiceOrientAndClarifyChangesRequireReview(t *testing.T) {
	for _, kind := range []ActionKind{ActionOrient, ActionClarify} {
		t.Run(string(kind), func(t *testing.T) {
			repository := newTestRepository(t)
			service := openTestService(t, repository)
			run := startTestRun(t, service, 6, true)
			if kind == ActionClarify {
				run = submitCurrent(t, service, run, Outcome{
					Status:           OutcomeCompleted,
					Summary:          "clarification needed",
					SuggestedActions: []SuggestedAction{{Kind: ActionClarify, Brief: "Clarify the remaining fact."}},
				})
				require.Equal(t, ActionClarify, run.CurrentAction.Kind)
			}
			require.NoError(t, os.WriteFile(filepath.Join(repository, string(kind)+"-change.go"), []byte("package sample\n"), 0o600))
			run = submitCurrent(t, service, run, Outcome{
				Status:           OutcomeCompleted,
				Summary:          "facts only",
				SuggestedActions: []SuggestedAction{{Kind: ActionSummarize, Brief: "Attempt to report without review."}},
			})
			require.NotNil(t, run.CurrentAction)
			assert.Equal(t, ActionReview, run.CurrentAction.Kind)
			assert.Empty(t, run.PendingActions)
		})
	}
}

func TestServiceCarriesReviewPendingAcrossDecisionAndBudgetPauses(t *testing.T) {
	t.Run("decision", func(t *testing.T) {
		repository := newTestRepository(t)
		service := openTestService(t, repository)
		run := startTestRun(t, service, 8, true)
		actionID := run.CurrentAction.ActionID
		require.NoError(t, os.WriteFile(filepath.Join(repository, "decision-change.go"), []byte("package sample\n"), 0o600))
		run = submitCurrent(t, service, run, Outcome{
			Status:  OutcomeNeedsInput,
			Summary: "one decision remains",
			Pause:   pauseReport(PauseDecisionRequired, "Choose the API?", nil),
		})
		require.Equal(t, RunPaused, run.State)
		assert.True(t, run.ReviewPending)

		run, err := service.Answer(run.ID, actionID, AnswerOptions{Text: "Use the current API"})
		require.NoError(t, err)
		assert.True(t, run.ReviewPending)
		run = submitCurrent(t, service, run, Outcome{
			Status:           OutcomeCompleted,
			Summary:          "decision incorporated",
			SuggestedActions: []SuggestedAction{{Kind: ActionSummarize, Brief: "Try to finish."}},
		})
		require.Equal(t, ActionReview, run.CurrentAction.Kind)
		assert.True(t, run.ReviewPending)
		assert.Empty(t, run.PendingActions)
		run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "reviewed", Review: reviewReport(ReviewNoFindings)})
		assert.False(t, run.ReviewPending)
	})

	t.Run("budget", func(t *testing.T) {
		repository := newTestRepository(t)
		service := openTestService(t, repository)
		run := startTestRun(t, service, 1, true)
		require.NoError(t, os.WriteFile(filepath.Join(repository, "budget-change.go"), []byte("package sample\n"), 0o600))
		run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "changed", SuggestedActions: []SuggestedAction{}})
		require.Equal(t, PauseBudgetExhausted, run.PauseReason)
		assert.True(t, run.ReviewPending)

		run, err := service.Resume(run.ID, ResumeOptions{})
		require.NoError(t, err)
		assert.True(t, run.ReviewPending)
		run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "fresh facts", SuggestedActions: []SuggestedAction{}})
		require.Equal(t, ActionReview, run.CurrentAction.Kind)
		run, err = service.Skip(run.ID, run.CurrentAction.ActionID)
		require.NoError(t, err)
		assert.False(t, run.ReviewPending)
	})
}

func TestServiceRejectsOversizeActionBeforeJournalCreation(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)

	_, err := service.Start(strings.Repeat("g", maxActionBytes), CreateOptions{Budget: 4, ReviewEnabled: false})
	assertProtocolError(t, err, "action_too_large")
	var protocolErr *ProtocolError
	require.ErrorAs(t, err, &protocolErr)
	assert.Equal(t, NextOperationStart, protocolErr.Next.Operation)
	require.Len(t, protocolErr.Next.Variants, 1)
	assert.Equal(t, "retry-run", protocolErr.Next.Variants[0].ID)
	assert.NotEqual(t, "resume-ad-hoc", protocolErr.Next.Variants[0].ID)
	assert.NotEqual(t, "refresh-source", protocolErr.Next.Variants[0].ID)

	runs, listErr := service.List()
	require.NoError(t, listErr)
	assert.Empty(t, runs)
}

func TestServiceSubmitActionFailureUsesDurableResumeNext(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	goal := strings.Repeat("g", maxActionBytes-maxActionBriefBytes/2)
	run, err := service.Start(goal, CreateOptions{Budget: 4, ReviewEnabled: false})
	require.NoError(t, err)
	require.NotNil(t, run.CurrentAction)
	actionID := run.CurrentAction.ActionID
	before, err := service.Load(run.ID)
	require.NoError(t, err)

	outcome := withEnvelope(actionID, ActionOrient, Outcome{
		Status:  OutcomeCompleted,
		Summary: "facts gathered",
		SuggestedActions: []SuggestedAction{{
			Kind:  ActionImplement,
			Brief: strings.Repeat("b", maxSuggestedActionBriefBytes),
		}},
	})
	_, err = service.Submit(run.ID, actionID, outcome)
	assertProtocolError(t, err, "action_too_large")
	var protocolErr *ProtocolError
	require.ErrorAs(t, err, &protocolErr)
	assert.Equal(t, NextOperationResume, protocolErr.Next.Operation)
	require.Len(t, protocolErr.Next.Variants, 1)
	variant := protocolErr.Next.Variants[0]
	assert.Equal(t, "resume-ad-hoc", variant.ID)
	assert.Equal(t, []string{"slipway", "_machine", "resume", run.ID, "--root", run.Workspace}, variant.BaseArgv)

	persisted, loadErr := service.Load(run.ID)
	require.NoError(t, loadErr)
	assert.Equal(t, before, persisted)
	require.NotNil(t, persisted.CurrentAction)
	assert.Equal(t, actionID, persisted.CurrentAction.ActionID)
	record := findActionRecord(&persisted, actionID)
	require.NotNil(t, record)
	assert.False(t, record.Voided)
}

func TestServiceAcceptsMaxSizeImplementSuggestionBrief(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 4, false)

	suggestionBrief := strings.Repeat("界", maxSuggestedActionBriefBytes/len("界"))
	suggestionBrief += strings.Repeat("x", maxSuggestedActionBriefBytes-len(suggestionBrief))
	require.Len(t, suggestionBrief, maxSuggestedActionBriefBytes)

	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeCompleted,
		Summary: "facts gathered",
		SuggestedActions: []SuggestedAction{{
			Kind:  ActionImplement,
			Brief: suggestionBrief,
		}},
	})
	require.NotNil(t, run.CurrentAction)
	require.Equal(t, ActionImplement, run.CurrentAction.Kind)
	assert.LessOrEqual(t, len(run.CurrentAction.Brief), maxActionBriefBytes)
	assert.True(t, utf8.ValidString(run.CurrentAction.Brief))
	assert.Contains(t, run.CurrentAction.Brief, "Repair-attempt limit:")
	assert.True(t, strings.HasSuffix(run.CurrentAction.Brief, " Repair-attempt limit: 3."))
}

func TestServiceListSkipsForeignAndCorruptRuns(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	local := startTestRun(t, service, 4, false)

	foreignRoot := filepath.Join(t.TempDir(), "foreign-worktree")
	runGit(t, repository, "worktree", "add", "-q", "-b", "foreign-list-test", foreignRoot)
	foreignService := openTestService(t, foreignRoot)
	foreign := startTestRun(t, foreignService, 4, false)
	foreignDirectory := filepath.Join(service.store.CommonDir(), "slipway", "runs", foreign.ID)
	foreignLock := filepath.Join(foreignDirectory, "run.lock")
	require.NoError(t, os.Remove(foreignLock))
	foreignJournal := filepath.Join(foreignDirectory, "journal.jsonl")
	journal, err := os.OpenFile(foreignJournal, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = journal.WriteString(`{"interrupted"`)
	require.NoError(t, err)
	require.NoError(t, journal.Close())
	foreignBefore, err := os.ReadFile(foreignJournal)
	require.NoError(t, err)

	_, err = service.Load(foreign.ID)
	assertProtocolError(t, err, "workspace_identity_mismatch")
	foreignAfterLoad, err := os.ReadFile(foreignJournal)
	require.NoError(t, err)
	assert.Equal(t, foreignBefore, foreignAfterLoad, "foreign status must not repair another worktree's journal")

	corruptDirectory := filepath.Join(service.store.CommonDir(), "slipway", "runs", "corrupt-list-test")
	require.NoError(t, os.MkdirAll(corruptDirectory, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(corruptDirectory, "journal.jsonl"), []byte("not-json\n"), 0o600))
	incompleteDirectory := filepath.Join(service.store.CommonDir(), "slipway", "runs", "incomplete-list-test")
	require.NoError(t, os.MkdirAll(incompleteDirectory, 0o700))

	runs, err := service.List()
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, local.ID, runs[0].ID)
	assert.NotEqual(t, foreign.ID, runs[0].ID)
	foreignAfterList, err := os.ReadFile(foreignJournal)
	require.NoError(t, err)
	assert.Equal(t, foreignBefore, foreignAfterList, "listing must not repair a foreign journal tail")
	for _, directory := range []string{foreignDirectory, corruptDirectory, incompleteDirectory} {
		_, statErr := os.Stat(filepath.Join(directory, "run.lock"))
		assert.ErrorIs(t, statErr, os.ErrNotExist, "read-only preflight must not create a lock before local ownership is established")
	}
}

func TestServiceLoadRecreatesMissingLockForValidatedLocalRun(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 4, false)
	runDirectory := filepath.Join(service.store.CommonDir(), "slipway", "runs", run.ID)
	lockPath := filepath.Join(runDirectory, "run.lock")
	journalPath := filepath.Join(runDirectory, "journal.jsonl")
	require.NoError(t, os.Remove(lockPath))
	journalBefore, err := os.ReadFile(journalPath)
	require.NoError(t, err)

	loaded, err := service.Load(run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.ID, loaded.ID)

	journalAfter, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.Equal(t, journalBefore, journalAfter)
	lockInfo, err := os.Lstat(lockPath)
	require.NoError(t, err)
	assert.True(t, lockInfo.Mode().IsRegular())
}

func TestServiceRejectsHostSkippedReviewStatus(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	require.NoError(t, os.WriteFile(filepath.Join(repository, "review.go"), []byte("package sample\n"), 0o600))
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "changed", Implementation: implementationReport(ImplementationApplied, "review.go")})
	require.Equal(t, ActionReview, run.CurrentAction.Kind)

	reviewID := run.CurrentAction.ActionID
	invalid := withEnvelope(reviewID, ActionReview, Outcome{Status: OutcomeStatus("skipped"), Summary: "host skipped"})
	_, err := service.Submit(run.ID, reviewID, invalid)
	assertProtocolError(t, err, "invalid_outcome")
	loaded, loadErr := service.Load(run.ID)
	require.NoError(t, loadErr)
	require.NotNil(t, loaded.CurrentAction)
	assert.Equal(t, reviewID, loaded.CurrentAction.ActionID)
}

func TestServiceGitObservationRecordsNeutralDiscrepancyAndRoutesDiffFirst(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	require.NoError(t, os.WriteFile(filepath.Join(repository, "observed.go"), []byte("package sample\n"), 0o600))
	run = submitCurrent(t, service, run, Outcome{
		Status:         OutcomeCompleted,
		Summary:        "reported no change",
		Implementation: implementationReport(ImplementationNotNeeded),
	})
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionReview, run.CurrentAction.Kind)
	joinedObservations := strings.Join(run.Observations, "\n")
	joinedUncertainties := strings.Join(run.Uncertainties, "\n")
	assert.Contains(t, joinedObservations, observedSinceStart)
	assert.Contains(t, joinedObservations, "report_discrepancy: Implement reported not_needed while a start-to-current Git difference was observed.")
	assert.Contains(t, joinedUncertainties, attributionUncertainty)
	assert.NotContains(t, joinedObservations+joinedUncertainties, "despite")
	assert.NotContains(t, joinedObservations+joinedUncertainties, "contradict")
}

func TestServiceReviewsEachNewObservedGitRevision(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 10, true)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})

	require.NoError(t, os.WriteFile(filepath.Join(repository, "first.go"), []byte("package sample\n"), 0o600))
	run = submitCurrent(t, service, run, Outcome{
		Status:         OutcomeCompleted,
		Summary:        "first revision",
		Implementation: implementationReport(ImplementationApplied, "first.go"),
	})
	require.Equal(t, ActionReview, run.CurrentAction.Kind)
	firstReviewID := run.CurrentAction.ActionID

	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeCompleted,
		Summary: "first revision reviewed",
		Review:  reviewReport(ReviewNoFindings),
	})
	require.Equal(t, ActionSummarize, run.CurrentAction.Kind)
	_, err := service.Stop(run.ID)
	require.NoError(t, err)
	run, err = service.Resume(run.ID, ResumeOptions{})
	require.NoError(t, err)
	require.Equal(t, ActionOrient, run.CurrentAction.Kind)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "fresh facts"})
	require.Equal(t, ActionImplement, run.CurrentAction.Kind)
	require.NoError(t, os.WriteFile(filepath.Join(repository, "second.go"), []byte("package sample\n"), 0o600))
	run = submitCurrent(t, service, run, Outcome{
		Status:         OutcomeCompleted,
		Summary:        "second revision",
		Implementation: implementationReport(ImplementationApplied, "second.go"),
	})
	require.Equal(t, ActionReview, run.CurrentAction.Kind)
	assert.NotEqual(t, firstReviewID, run.CurrentAction.ActionID)
}

func TestServiceResumeVoidedReviewKeepsReviewPending(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	require.NoError(t, os.WriteFile(filepath.Join(repository, "pending-review.go"), []byte("package sample\n"), 0o600))
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "changed", Implementation: implementationReport(ImplementationApplied, "pending-review.go")})
	require.Equal(t, ActionReview, run.CurrentAction.Kind)
	assert.True(t, run.ReviewPending)

	_, err := service.Stop(run.ID)
	require.NoError(t, err)
	run, err = service.Resume(run.ID, ResumeOptions{})
	require.NoError(t, err)
	assert.True(t, run.ReviewPending)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "reoriented", SuggestedActions: []SuggestedAction{}})
	require.Equal(t, ActionReview, run.CurrentAction.Kind)
	assert.True(t, run.ReviewPending)
}

func TestServicePreservesRunStartDirtyFilesAcrossGitDeltas(t *testing.T) {
	repository := newTestRepository(t)
	for _, name := range []string{"preexisting-a.txt", "preexisting-b.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(repository, name), []byte("before\n"), 0o600))
	}
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)
	initialDirty := append([]string(nil), run.InitialGit.DirtyFiles...)
	require.Equal(t, []string{"preexisting-a.txt", "preexisting-b.txt"}, initialDirty)

	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	require.Equal(t, ActionImplement, run.CurrentAction.Kind)
	require.NoError(t, os.WriteFile(filepath.Join(repository, "implemented.go"), []byte("package sample\n"), 0o600))
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "implemented", Implementation: implementationReport(ImplementationApplied, "implemented.go")})

	assert.Equal(t, initialDirty, run.InitialGit.DirtyFiles)
	assert.Contains(t, run.CurrentGit.DirtyFiles, "implemented.go")
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionReview, run.CurrentAction.Kind)
	assert.NotContains(t, run.CurrentAction.Context, "preexisting-a.txt", "Git headers must not enter bounded context")
	assert.NotContains(t, run.CurrentAction.Context, "preexisting-b.txt", "Git headers must not enter bounded context")
	assert.Contains(t, run.CurrentAction.Brief, "preexisting-a.txt")
	assert.Contains(t, run.CurrentAction.Brief, "preexisting-b.txt")
	assert.NotContains(t, run.CurrentAction.Brief, "implemented.go")
	assert.Contains(t, run.CurrentAction.Brief, "Attribution is uncertain")

	loaded, err := service.Load(run.ID)
	require.NoError(t, err)
	assert.Equal(t, initialDirty, loaded.InitialGit.DirtyFiles)
}

func TestServiceClarifiesDependentDecisionsOneAtATime(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 10, true)
	run = submitCurrent(t, service, run, Outcome{
		Status:           OutcomeCompleted,
		Summary:          "first human choice remains",
		SuggestedActions: []SuggestedAction{{Kind: ActionClarify, Brief: "Choose the public API. Recommend option A because it matches current conventions; alternatives: B."}},
	})
	require.NotNil(t, run.CurrentAction)
	firstID := run.CurrentAction.ActionID
	assert.Equal(t, ActionClarify, run.CurrentAction.Kind)

	run = submitCurrent(t, service, run, Outcome{Status: OutcomeNeedsInput, Summary: "Which API?", Pause: pauseReport(PauseDecisionRequired, "Which public API should be used?", nil)})
	assert.Equal(t, RunPaused, run.State)
	run, err := service.Answer(run.ID, firstID, AnswerOptions{Text: "Use option A"})
	require.NoError(t, err)
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionOrient, run.CurrentAction.Kind)
	assert.Len(t, run.Answers, 1)

	run = submitCurrent(t, service, run, Outcome{
		Status:           OutcomeCompleted,
		Summary:          "second dependent choice remains",
		SuggestedActions: []SuggestedAction{{Kind: ActionClarify, Brief: "Choose persistence. Recommend local files; alternative: remote storage."}},
	})
	require.Equal(t, ActionClarify, run.CurrentAction.Kind)
	secondID := run.CurrentAction.ActionID
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeNeedsInput, Summary: "Where persist?", Pause: pauseReport(PauseDecisionRequired, "Where should data persist?", nil)})
	run, err = service.Answer(run.ID, secondID, AnswerOptions{Text: "Use local files"})
	require.NoError(t, err)
	require.Equal(t, ActionOrient, run.CurrentAction.Kind)

	run = submitCurrent(t, service, run, Outcome{
		Status:           OutcomeCompleted,
		Summary:          "all decisions are confirmed",
		SuggestedActions: []SuggestedAction{{Kind: ActionImplement, Brief: "Implement the confirmed choices."}},
	})
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionImplement, run.CurrentAction.Kind)
}

func TestServiceSkipStopResumeAndStaleActionRejection(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)
	oldAction := run.CurrentAction.ActionID

	stopped, err := service.Stop(run.ID)
	require.NoError(t, err)
	assert.Equal(t, RunStopped, stopped.State)
	next, nextErr := DeriveNext(stopped)
	require.NoError(t, nextErr)
	assert.Equal(t, NextOperationResume, next.Operation)
	assert.Equal(t, "resume-ad-hoc", next.Variants[0].ID)

	_, err = service.Submit(run.ID, oldAction, withEnvelope(oldAction, ActionOrient, Outcome{Status: OutcomeCompleted, Summary: "late"}))
	assertProtocolError(t, err, "run_not_active")

	resumed, err := service.Resume(run.ID, ResumeOptions{})
	require.NoError(t, err)
	require.NotNil(t, resumed.CurrentAction)
	assert.Equal(t, ActionOrient, resumed.CurrentAction.Kind)
	assert.NotEqual(t, oldAction, resumed.CurrentAction.ActionID)
	assert.True(t, resumed.Actions[0].Voided)

	_, err = service.Submit(run.ID, oldAction, withEnvelope(oldAction, ActionOrient, Outcome{Status: OutcomeCompleted, Summary: "stale"}))
	assertProtocolError(t, err, "stale_action")

	skipped, err := service.Skip(run.ID, resumed.CurrentAction.ActionID)
	require.NoError(t, err)
	require.NotNil(t, skipped.CurrentAction)
	assert.Equal(t, ActionSummarize, skipped.CurrentAction.Kind)
}

func TestServiceResumeVoidsSuggestedActionAndStartsFreshOrient(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 10, false)
	run = submitCurrent(t, service, run, Outcome{
		Status:           OutcomeCompleted,
		Summary:          "old decision",
		SuggestedActions: []SuggestedAction{{Kind: ActionClarify, Brief: "old decision"}},
	})
	require.NotNil(t, run.CurrentAction)
	oldActionID := run.CurrentAction.ActionID
	assert.Equal(t, ActionClarify, run.CurrentAction.Kind)

	resumed, err := service.Resume(run.ID, ResumeOptions{})
	require.NoError(t, err)
	require.NotNil(t, resumed.CurrentAction)
	assert.Equal(t, ActionOrient, resumed.CurrentAction.Kind)
	assert.NotEqual(t, oldActionID, resumed.CurrentAction.ActionID)
	oldRecord := findActionRecord(&resumed, oldActionID)
	require.NotNil(t, oldRecord)
	assert.True(t, oldRecord.Voided)
	assert.Empty(t, resumed.PendingActions)

	resumed = submitCurrent(t, service, resumed, Outcome{Status: OutcomeCompleted, Summary: "fresh facts; no work remains", SuggestedActions: []SuggestedAction{}})
	require.NotNil(t, resumed.CurrentAction)
	assert.Equal(t, ActionSummarize, resumed.CurrentAction.Kind)
}

func TestServiceSkipReviewRecordsSkippedWorkAndEnds(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	require.NoError(t, os.WriteFile(filepath.Join(repository, "change.go"), []byte("package sample\n"), 0o600))
	run = submitCurrent(t, service, run, Outcome{
		Status:         OutcomeCompleted,
		Summary:        "change written",
		Implementation: implementationReport(ImplementationApplied, "change.go"),
	})
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionReview, run.CurrentAction.Kind)

	reviewID := run.CurrentAction.ActionID
	run, err := service.Skip(run.ID, reviewID)
	require.NoError(t, err)
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionSummarize, run.CurrentAction.Kind)
	reviewRecord := findActionRecord(&run, reviewID)
	require.NotNil(t, reviewRecord)
	assert.True(t, reviewRecord.Skipped)
	assert.Nil(t, reviewRecord.Outcome)
	require.NotNil(t, reviewRecord.ReviewProjection)
	assert.Equal(t, ReviewNotRun, reviewRecord.ReviewProjection.Result)
	assert.Empty(t, reviewRecord.ReviewProjection.Findings)
	assert.Empty(t, reviewRecord.ReviewProjection.Uncertainties)

	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "reported"})
	assert.Equal(t, RunEnded, run.State)
	assert.Contains(t, run.Summary, "Review was skipped by the user.")
	assert.Contains(t, run.Summary, "Skipped Actions:\n- review")
}

func TestServiceFinalSummaryReobservesGitAfterReview(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	path := filepath.Join(repository, "temporary.go")
	require.NoError(t, os.WriteFile(path, []byte("package sample\n"), 0o600))
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "temporary change", Implementation: implementationReport(ImplementationApplied, "temporary.go")})
	require.Equal(t, ActionReview, run.CurrentAction.Kind)
	require.NoError(t, os.Remove(path))
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "reviewed", Review: reviewReport(ReviewNoFindings)})
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "reported"})

	assert.True(t, run.FinalGitObserved)
	assert.False(t, run.CurrentGit.ChangedFrom(run.InitialGit))
	assert.Contains(t, run.Summary, "no difference from the run-start snapshot")
}

func TestServiceSkipPausedDecisionPreservesQuestionAndRecordsSkip(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	waitingID := run.CurrentAction.ActionID
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "Choose a storage format",
		Pause:   pauseReport(PauseDecisionRequired, "host question", nil),
	})
	assert.Equal(t, RunPaused, run.State)

	run, err := service.Skip(run.ID, waitingID)
	require.NoError(t, err)
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionSummarize, run.CurrentAction.Kind)
	waitingRecord := findActionRecord(&run, waitingID)
	require.NotNil(t, waitingRecord)
	assert.True(t, waitingRecord.Skipped)
	require.NotNil(t, waitingRecord.Outcome)
	assert.Equal(t, OutcomeNeedsInput, waitingRecord.Outcome.Status)
}

func TestServiceSkipPausedEnvironmentActionUsesStructuredControl(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	waitingID := run.CurrentAction.ActionID
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "The local environment is unavailable",
		Pause:   pauseReport(PauseEnvironmentUnavailable, "Retry after the environment recovers.", nil),
	})
	next, err := DeriveNext(run)
	require.NoError(t, err)
	require.Equal(t, "skip-action", next.Variants[len(next.Variants)-1].ID)

	run, err = service.Skip(run.ID, waitingID)
	require.NoError(t, err)
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionSummarize, run.CurrentAction.Kind)
}

func TestServiceSkipPausedDestructiveActionClearsRequest(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeCompleted,
		Summary: "Implementation is ready",
		SuggestedActions: []SuggestedAction{{
			Kind:  ActionImplement,
			Brief: "Apply the change.",
		}},
	})
	waitingID := run.CurrentAction.ActionID
	request := destructiveRequestForTest(t)
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "The exact destructive scope needs confirmation",
		Pause:   pauseReport(PauseDestructiveConfirm, "Confirm the exact scope.", &request),
	})
	next, err := DeriveNext(run)
	require.NoError(t, err)
	require.Equal(t, "skip-action", next.Variants[len(next.Variants)-1].ID)

	run, err = service.Skip(run.ID, waitingID)
	require.NoError(t, err)
	assert.Nil(t, run.PendingDestructiveRequest)
	assert.Nil(t, run.DestructiveGrant)
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionSummarize, run.CurrentAction.Kind)
}

func TestServiceDuplicateSubmitIsIdempotentAndConflictingDuplicateIsRejected(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)
	actionID := run.CurrentAction.ActionID
	outcome := withEnvelope(actionID, ActionOrient, Outcome{Status: OutcomeCompleted, Summary: "facts"})

	first, err := service.Submit(run.ID, actionID, outcome)
	require.NoError(t, err)
	second, err := service.Submit(run.ID, actionID, outcome)
	require.NoError(t, err)
	assert.Equal(t, first.CurrentAction.ActionID, second.CurrentAction.ActionID)
	assert.Len(t, second.Actions, 2)

	conflict := outcome
	conflict.Summary = "different"
	_, err = service.Submit(run.ID, actionID, conflict)
	assertProtocolError(t, err, "outcome_conflict")
}

func TestServiceDuplicateSubmitReplayAcrossTerminalStates(t *testing.T) {
	t.Run("voided remains stale", func(t *testing.T) {
		repository := newTestRepository(t)
		service := openTestService(t, repository)
		run := startTestRun(t, service, 8, false)
		actionID := run.CurrentAction.ActionID
		waiting := withEnvelope(actionID, ActionOrient, Outcome{Status: OutcomeNeedsInput, Summary: "choose", Pause: pauseReport(PauseDecisionRequired, "host question", nil)})
		run, err := service.Submit(run.ID, actionID, waiting)
		require.NoError(t, err)
		_, err = service.Resume(run.ID, ResumeOptions{})
		require.NoError(t, err)
		_, err = service.Submit(run.ID, actionID, waiting)
		assertProtocolError(t, err, "stale_action")
	})

	t.Run("stopped exact replay", func(t *testing.T) {
		repository := newTestRepository(t)
		service := openTestService(t, repository)
		run := startTestRun(t, service, 8, false)
		actionID := run.CurrentAction.ActionID
		completed := withEnvelope(actionID, ActionOrient, Outcome{Status: OutcomeCompleted, Summary: "facts"})
		run, err := service.Submit(run.ID, actionID, completed)
		require.NoError(t, err)
		_, err = service.Stop(run.ID)
		require.NoError(t, err)
		replayed, err := service.Submit(run.ID, actionID, completed)
		require.NoError(t, err)
		assert.Equal(t, RunStopped, replayed.State)
		conflict := completed
		conflict.Summary = "different"
		_, err = service.Submit(run.ID, actionID, conflict)
		assertProtocolError(t, err, "outcome_conflict")
	})

	t.Run("ended exact replay", func(t *testing.T) {
		repository := newTestRepository(t)
		service := openTestService(t, repository)
		run := startTestRun(t, service, 8, false)
		run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
		run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "no change", Implementation: implementationReport(ImplementationNotNeeded)})
		summarizeID := run.CurrentAction.ActionID
		summary := withEnvelope(summarizeID, ActionSummarize, Outcome{Status: OutcomeCompleted, Summary: "reported"})
		run, err := service.Submit(run.ID, summarizeID, summary)
		require.NoError(t, err)
		assert.Equal(t, RunEnded, run.State)
		replayed, err := service.Submit(run.ID, summarizeID, summary)
		require.NoError(t, err)
		assert.Equal(t, RunEnded, replayed.State)
		conflict := summary
		conflict.Summary = "different"
		_, err = service.Submit(run.ID, summarizeID, conflict)
		assertProtocolError(t, err, "outcome_conflict")
	})
}

func TestServiceBudgetPauseAndResumeReplenishment(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 1, true)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	assert.Equal(t, RunPaused, run.State)
	assert.Equal(t, PauseBudgetExhausted, run.PauseReason)
	assert.Nil(t, run.CurrentAction)
	next, nextErr := DeriveNext(run)
	require.NoError(t, nextErr)
	assert.Equal(t, NextOperationResume, next.Operation)

	resumed, err := service.Resume(run.ID, ResumeOptions{})
	require.NoError(t, err)
	assert.Equal(t, RunActive, resumed.State)
	require.NotNil(t, resumed.CurrentAction)
	assert.Equal(t, ActionOrient, resumed.CurrentAction.Kind)
	assert.Equal(t, 2, resumed.RemainingBudget)
	resumed = submitCurrent(t, service, resumed, Outcome{Status: OutcomeCompleted, Summary: "fresh facts"})
	require.NotNil(t, resumed.CurrentAction)
	assert.Equal(t, ActionImplement, resumed.CurrentAction.Kind)
	assert.Equal(t, 1, resumed.RemainingBudget)
}

func TestServiceEnvironmentPauseProvidesResumeCommand(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "required tool is unavailable",
		Pause:   pauseReport(PauseEnvironmentUnavailable, "host question", nil),
	})
	assert.Equal(t, RunPaused, run.State)
	assert.Equal(t, PauseEnvironmentUnavailable, run.PauseReason)
	next, nextErr := DeriveNext(run)
	require.NoError(t, nextErr)
	assert.Equal(t, NextOperationResume, next.Operation)
	assert.Equal(t, "resume-ad-hoc", next.Variants[0].ID)
	waitingID := run.CurrentAction.ActionID
	_, answerErr := service.Answer(run.ID, waitingID, AnswerOptions{Text: "retry"})
	assertProtocolError(t, answerErr, "answer_not_allowed")
	unchanged, loadErr := service.Load(run.ID)
	require.NoError(t, loadErr)
	assert.Equal(t, waitingID, unchanged.CurrentAction.ActionID)
	assert.Empty(t, unchanged.Answers)
}

func TestServiceNaturalLanguageDestructiveAnswerReorientsWithoutGrant(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	implementID := run.CurrentAction.ActionID
	request := destructiveRequestForTest(t)
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "destructive scope requires confirmation",
		Pause:   pauseReport(PauseDestructiveConfirm, "Confirm permanent deletion?", &request),
	})
	assert.Equal(t, RunPaused, run.State)
	require.NotNil(t, run.PendingDestructiveRequest)
	assert.Equal(t, request, *run.PendingDestructiveRequest)
	assert.Nil(t, run.DestructiveGrant)

	reoriented, err := service.Answer(run.ID, implementID, AnswerOptions{Text: "yes, delete it"})
	require.NoError(t, err)
	require.NotNil(t, reoriented.CurrentAction)
	assert.Equal(t, ActionOrient, reoriented.CurrentAction.Kind)
	assert.Nil(t, reoriented.CurrentAction.DestructiveAuthorization)
	assert.Nil(t, reoriented.PendingDestructiveRequest)
	assert.Nil(t, reoriented.DestructiveGrant)
	assert.Contains(t, reoriented.CurrentAction.Brief, "non-destructive")
	assert.Contains(t, reoriented.CurrentAction.Context, "yes, delete it")
	record := findActionRecord(&reoriented, implementID)
	require.NotNil(t, record)
	assert.True(t, record.Voided)
}

func TestServiceEndsWithoutActivitiesAndDoesNotInventThem(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	implementation := implementationReport(ImplementationNotNeeded)
	implementation.Uncertainties = []string{"fixture-test executable was unavailable; the technical process never started"}
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "nothing to change", Implementation: implementation})
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionSummarize, run.CurrentAction.Kind)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "reported"})
	assert.Equal(t, RunEnded, run.State)
	assert.Contains(t, run.Summary, "No test, typecheck, build, or lint activity was reported.")
	assert.Empty(t, run.Activities)
	assert.Contains(t, run.Uncertainties, "fixture-test executable was unavailable; the technical process never started")
	assert.Contains(t, run.Summary, "fixture-test executable was unavailable; the technical process never started")
}

func TestServiceConcurrentDuplicateSubmitRecordsOneTransition(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)
	actionID := run.CurrentAction.ActionID
	outcome := withEnvelope(actionID, ActionOrient, Outcome{Status: OutcomeCompleted, Summary: "facts"})

	const workers = 8
	results := make(chan Run, workers)
	errorsSeen := make(chan error, workers)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := service.Submit(run.ID, actionID, outcome)
			results <- result
			errorsSeen <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errorsSeen)
	for err := range errorsSeen {
		require.NoError(t, err)
	}
	for result := range results {
		assert.Len(t, result.Actions, 2)
	}
	loaded, err := service.Load(run.ID)
	require.NoError(t, err)
	assert.Len(t, loaded.Actions, 2)
}

func TestRunJournalStoresLinearDeltasInsteadOfRepeatedProjections(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 24, false)
	bigObservation := strings.Repeat("x", 64<<10)
	run = submitCurrent(t, service, run, Outcome{
		Status:           OutcomeCompleted,
		Summary:          "first decision",
		Observations:     []string{bigObservation},
		SuggestedActions: []SuggestedAction{{Kind: ActionClarify, Brief: "Clarify one bounded decision."}},
	})
	for index := range 12 {
		require.Equal(t, ActionClarify, run.CurrentAction.Kind)
		next := SuggestedAction{Kind: ActionClarify, Brief: "Clarify one bounded decision."}
		if index == 11 {
			next = SuggestedAction{Kind: ActionImplement, Brief: "Implement confirmed decisions."}
		}
		run = submitCurrent(t, service, run, Outcome{
			Status:           OutcomeCompleted,
			Summary:          "decision recorded",
			Observations:     []string{bigObservation},
			SuggestedActions: []SuggestedAction{next},
		})
	}
	require.Equal(t, ActionImplement, run.CurrentAction.Kind)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "nothing to change", Implementation: implementationReport(ImplementationNotNeeded)})
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "reported"})
	require.Equal(t, RunEnded, run.State)

	runDirectory := filepath.Join(service.store.CommonDir(), "slipway", "runs", run.ID)
	journalInfo, err := os.Stat(filepath.Join(runDirectory, "journal.jsonl"))
	require.NoError(t, err)
	projectionInfo, err := os.Stat(filepath.Join(runDirectory, "run.json"))
	require.NoError(t, err)
	assert.Less(t, journalInfo.Size(), projectionInfo.Size()*3, "journal must grow linearly rather than repeat the accumulated projection")
	for _, name := range []string{"journal.jsonl", "run.json"} {
		content, readErr := os.ReadFile(filepath.Join(runDirectory, name))
		require.NoError(t, readErr)
		assert.NotContains(t, string(content), "next_command", name)
		assert.NotContains(t, string(content), "base_argv", name)
		assert.NotContains(t, string(content), "rendered command", name)
	}
}

func TestRunJournalRejectsUnknownAndMislabeledEventTypes(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 4, false)
	_, err := service.Submit(run.ID, run.CurrentAction.ActionID, withEnvelope(run.CurrentAction.ActionID, run.CurrentAction.Kind, Outcome{
		Status: OutcomeCompleted, Summary: "orientation complete",
	}))
	require.NoError(t, err)

	journalPath := filepath.Join(service.store.CommonDir(), "slipway", "runs", run.ID, "journal.jsonl")
	content, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	lines := bytes.Split(bytes.TrimSpace(content), []byte("\n"))
	require.GreaterOrEqual(t, len(lines), 2)
	var first, second runstore.Event
	require.NoError(t, json.Unmarshal(lines[0], &first))
	require.NoError(t, json.Unmarshal(lines[1], &second))

	var replay Run
	unknown := first
	unknown.Type = "invented_event"
	err = applyRunEvent(&replay, unknown)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported run journal event type")

	var nonCanonicalData map[string]any
	require.NoError(t, json.Unmarshal(first.Data, &nonCanonicalData))
	nonCanonicalData["answers_append"] = []any{}
	nonCanonical := first
	nonCanonical.Data, err = json.Marshal(nonCanonicalData)
	require.NoError(t, err)
	replay = Run{}
	err = applyRunEvent(&replay, nonCanonical)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-canonical delta")
	replay = Run{}

	require.NoError(t, applyRunEvent(&replay, first))
	second.Type = "action_skipped"
	err = applyRunEvent(&replay, second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "action_skipped must mark one existing action skipped")
}

func TestRunResumedJournalEventRequiresAdHocResumeReceipt(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 4, false)
	before := runBeforeMutation(run)
	after := runBeforeMutation(run)
	after.UpdatedAt = after.UpdatedAt.Add(time.Second)
	event, err := newRunEvent("run_resumed", before, after)
	require.NoError(t, err)

	replayed := runBeforeMutation(before)
	err = applyRunEvent(&replayed, event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run_resumed must record the ad-hoc resume operation")
}

func TestBuildContextRetainsActiveDecisionsAndOutcomeProjectionsWithinBoundedUTF8(t *testing.T) {
	t.Parallel()
	criticalAnswer := "Use the user-selected durable storage option."
	run := Run{
		Goal:    "must not enter context",
		Answers: []AnswerRecord{{ActionID: "decision-1", Text: criticalAnswer, Active: true}},
		Actions: []ActionRecord{
			{Action: Action{ActionID: "orient-1", Kind: ActionOrient}, Outcome: &Outcome{Summary: strings.Repeat("界", maxActionContextBytes*100), KnownIssues: []string{}}},
			{Action: Action{ActionID: "clarify-1", Kind: ActionClarify}, Outcome: &Outcome{Summary: "Latest repository fact.", KnownIssues: []string{"One known issue."}}},
		},
	}

	context, err := buildContext(run)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(context), maxActionContextBytes)
	assert.True(t, utf8.ValidString(context))
	assert.NotContains(t, context, run.Goal)
	assert.Contains(t, context, criticalAnswer)
	assert.Contains(t, context, "Latest repository fact.")
	assert.Contains(t, context, "One known issue.")
}

func TestServiceOutcomeIdempotencyUsesExactOriginalPayloadBytes(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	actionID := run.CurrentAction.ActionID
	outcome := withEnvelope(actionID, ActionOrient, Outcome{
		Status:           OutcomeCompleted,
		Summary:          "exact payload",
		SuggestedActions: []SuggestedAction{{Kind: ActionImplement, Brief: "Implement exact bytes."}},
	})
	raw, err := json.Marshal(outcome)
	require.NoError(t, err)
	firstPayload := append(append([]byte(nil), raw...), '\n')
	secondPayload, err := json.MarshalIndent(outcome, "", "  ")
	require.NoError(t, err)

	decoded, err := DecodeOutcome(strings.NewReader(string(firstPayload)))
	require.NoError(t, err)
	first, err := service.Submit(run.ID, actionID, decoded)
	require.NoError(t, err)
	require.NotNil(t, first.CurrentAction)

	identical, err := DecodeOutcome(strings.NewReader(string(firstPayload)))
	require.NoError(t, err)
	retried, err := service.Submit(run.ID, actionID, identical)
	require.NoError(t, err)
	assert.Equal(t, first.CurrentAction.ActionID, retried.CurrentAction.ActionID)
	assert.Len(t, retried.Actions, 2)
	require.Equal(t, decoded.RawSHA256, retried.Actions[0].OutcomePayloadSHA256)

	semanticOnly, err := DecodeOutcome(strings.NewReader(string(secondPayload)))
	require.NoError(t, err)
	_, err = service.Submit(run.ID, actionID, semanticOnly)
	assertProtocolError(t, err, "outcome_conflict")
	loaded, err := service.Load(run.ID)
	require.NoError(t, err)
	assert.Equal(t, first.CurrentAction.ActionID, loaded.CurrentAction.ActionID)
	assert.Len(t, loaded.Actions, 2)
}

func TestServiceStructuredDestructiveGrantIsExactIdempotentAndScopeBound(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 12, false)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	originatingActionID := run.CurrentAction.ActionID
	request := destructiveRequestForTest(t)
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "confirmation required",
		Pause:   pauseReport(PauseDestructiveConfirm, "Confirm the exact deletion?", &request),
	})
	require.Equal(t, RunPaused, run.State)
	require.Equal(t, request, *run.PendingDestructiveRequest)

	beforeMismatch, err := json.Marshal(run)
	require.NoError(t, err)
	invalidAnswers := []struct {
		name    string
		action  string
		options AnswerOptions
		code    string
	}{
		{name: "missing digest", action: originatingActionID, options: AnswerOptions{ConfirmDestructive: true}, code: "destructive_scope_required"},
		{name: "scope without flag", action: originatingActionID, options: AnswerOptions{Text: "confirm", ScopeSHA256: request.ScopeSHA256}, code: "destructive_confirmation_flag_required"},
		{name: "missing feedback", action: originatingActionID, options: AnswerOptions{}, code: "answer_required"},
		{name: "malformed digest", action: originatingActionID, options: AnswerOptions{ConfirmDestructive: true, ScopeSHA256: "sha256:ABC"}, code: "destructive_scope_required"},
		{name: "scope mismatch", action: originatingActionID, options: AnswerOptions{ConfirmDestructive: true, ScopeSHA256: "sha256:" + strings.Repeat("0", 64)}, code: "destructive_scope_mismatch"},
		{name: "stale action", action: "stale-action", options: AnswerOptions{ConfirmDestructive: true, ScopeSHA256: request.ScopeSHA256}, code: "stale_action"},
	}
	for _, test := range invalidAnswers {
		t.Run(test.name, func(t *testing.T) {
			_, answerErr := service.Answer(run.ID, test.action, test.options)
			assertProtocolError(t, answerErr, test.code)
			unchanged, loadErr := service.Load(run.ID)
			require.NoError(t, loadErr)
			afterMismatch, marshalErr := json.Marshal(unchanged)
			require.NoError(t, marshalErr)
			assert.JSONEq(t, string(beforeMismatch), string(afterMismatch))
		})
	}

	options := AnswerOptions{Text: "confirmed in the trusted host UI", ConfirmDestructive: true, ScopeSHA256: request.ScopeSHA256}
	authorized, err := service.Answer(run.ID, originatingActionID, options)
	require.NoError(t, err)
	require.Equal(t, RunActive, authorized.State)
	require.NotNil(t, authorized.CurrentAction)
	assert.Equal(t, ActionImplement, authorized.CurrentAction.Kind)
	assert.NotEqual(t, originatingActionID, authorized.CurrentAction.ActionID)
	require.NotNil(t, authorized.PendingDestructiveRequest)
	require.NotNil(t, authorized.DestructiveGrant)
	require.NotNil(t, authorized.CurrentAction.DestructiveAuthorization)
	grant := *authorized.DestructiveGrant
	assert.Equal(t, request.RequestID, grant.RequestID)
	assert.Equal(t, originatingActionID, grant.OriginatingActionID)
	assert.Equal(t, DestructiveScopeVersion, grant.ScopeVersion)
	assert.Equal(t, request.ScopeSHA256, grant.ScopeSHA256)
	assert.Equal(t, request.Targets, grant.Targets)
	assert.Equal(t, request.Impact, grant.Impact)
	assert.Equal(t, grant, *authorized.CurrentAction.DestructiveAuthorization)
	require.NoError(t, validateDestructiveAuthorization(grant))
	require.Len(t, authorized.Answers, 1)
	assert.False(t, authorized.Answers[0].Active)
	assert.NotContains(t, authorized.CurrentAction.Context, options.Text)

	replayed, err := service.Load(run.ID)
	require.NoError(t, err)
	assert.Equal(t, authorized.DestructiveGrant, replayed.DestructiveGrant)
	assert.Equal(t, authorized.PendingDestructiveRequest, replayed.PendingDestructiveRequest)
	firstNext, err := DeriveNext(authorized)
	require.NoError(t, err)
	replayedNext, err := DeriveNext(replayed)
	require.NoError(t, err)
	assert.Equal(t, firstNext, replayedNext)

	retried, err := service.Answer(run.ID, originatingActionID, options)
	require.NoError(t, err)
	assert.Equal(t, authorized.CurrentAction.ActionID, retried.CurrentAction.ActionID)
	assert.Equal(t, authorized.UpdatedAt, retried.UpdatedAt)
	assert.Len(t, retried.Answers, 1)
	_, err = service.Answer(run.ID, originatingActionID, AnswerOptions{
		Text:               "different attestation",
		ConfirmDestructive: true,
		ScopeSHA256:        request.ScopeSHA256,
	})
	assertProtocolError(t, err, "answer_conflict")

	expandedTargets := []DestructiveTarget{
		{Kind: DestructiveTargetPath, Value: "/absolute/target"},
		{Kind: DestructiveTargetPath, Value: "/absolute/target/two"},
	}
	expandedDigest, err := ComputeDestructiveScopeSHA256("22222222-2222-4222-8222-222222222222", expandedTargets, "delete two targets permanently")
	require.NoError(t, err)
	expanded := DestructiveRequest{
		RequestID: "22222222-2222-4222-8222-222222222222", Targets: expandedTargets,
		Impact: "delete two targets permanently", ScopeSHA256: expandedDigest,
	}
	paused := submitCurrent(t, service, authorized, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "scope expanded",
		Pause:   pauseReport(PauseDestructiveConfirm, "Confirm expanded scope?", &expanded),
	})
	assert.Equal(t, RunPaused, paused.State)
	assert.Nil(t, paused.DestructiveGrant)
	require.NotNil(t, paused.PendingDestructiveRequest)
	assert.Equal(t, expanded, *paused.PendingDestructiveRequest)
	_, err = service.Answer(paused.ID, paused.CurrentAction.ActionID, AnswerOptions{ConfirmDestructive: true, ScopeSHA256: request.ScopeSHA256})
	assertProtocolError(t, err, "destructive_scope_mismatch")
}

func TestServiceAnswerIdempotencyUsesCanonicalStructuredPayload(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	waitingID := run.CurrentAction.ActionID
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "decision required",
		Pause:   pauseReport(PauseDecisionRequired, "Choose one?", nil),
	})

	_, err := service.Answer(run.ID, waitingID, AnswerOptions{
		Text: "choose alpha", ConfirmDestructive: true, ScopeSHA256: "sha256:" + strings.Repeat("a", 64),
	})
	assertProtocolError(t, err, "destructive_confirmation_not_expected")
	unchanged, err := service.Load(run.ID)
	require.NoError(t, err)
	assert.Equal(t, waitingID, unchanged.CurrentAction.ActionID)
	assert.Empty(t, unchanged.Answers)

	options := AnswerOptions{Text: "choose alpha"}
	answered, err := service.Answer(run.ID, waitingID, options)
	require.NoError(t, err)
	require.NotNil(t, answered.CurrentAction)
	assert.Equal(t, ActionOrient, answered.CurrentAction.Kind)
	assert.Len(t, answered.Answers, 1)
	assert.True(t, validSHA256(answered.Answers[0].PayloadSHA256))
	actionCount := len(answered.Actions)
	updatedAt := answered.UpdatedAt

	retried, err := service.Answer(run.ID, waitingID, options)
	require.NoError(t, err)
	assert.Equal(t, actionCount, len(retried.Actions))
	assert.Equal(t, updatedAt, retried.UpdatedAt)
	assert.Equal(t, answered.CurrentAction.ActionID, retried.CurrentAction.ActionID)
	assert.Len(t, retried.Answers, 1)

	_, err = service.Answer(run.ID, waitingID, AnswerOptions{Text: "choose beta"})
	assertProtocolError(t, err, "answer_conflict")
}

func TestServiceDestructiveGrantClearsOnEveryInvalidatingOperation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *Service, Run) Run
	}{
		{
			name: "completed outcome",
			mutate: func(t *testing.T, service *Service, run Run) Run {
				return submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "done", Implementation: implementationReport(ImplementationNotNeeded)})
			},
		},
		{
			name: "partial outcome",
			mutate: func(t *testing.T, service *Service, run Run) Run {
				return submitCurrent(t, service, run, Outcome{Status: OutcomePartial, Summary: "partial", Implementation: implementationReport(ImplementationPartial)})
			},
		},
		{
			name: "error outcome",
			mutate: func(t *testing.T, service *Service, run Run) Run {
				return submitCurrent(t, service, run, Outcome{Status: OutcomeError, Summary: "unable", Implementation: implementationReport(ImplementationUnable)})
			},
		},
		{
			name: "decision pause",
			mutate: func(t *testing.T, service *Service, run Run) Run {
				return submitCurrent(t, service, run, Outcome{Status: OutcomeNeedsInput, Summary: "choose", Pause: pauseReport(PauseDecisionRequired, "Choose?", nil)})
			},
		},
		{
			name: "skip",
			mutate: func(t *testing.T, service *Service, run Run) Run {
				updated, err := service.Skip(run.ID, run.CurrentAction.ActionID)
				require.NoError(t, err)
				return updated
			},
		},
		{
			name: "stop",
			mutate: func(t *testing.T, service *Service, run Run) Run {
				updated, err := service.Stop(run.ID)
				require.NoError(t, err)
				return updated
			},
		},
		{
			name: "resume",
			mutate: func(t *testing.T, service *Service, run Run) Run {
				updated, err := service.Resume(run.ID, ResumeOptions{})
				require.NoError(t, err)
				return updated
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			repository := newTestRepository(t)
			service := openTestService(t, repository)
			authorized := authorizeDestructiveRunForTest(t, service, false)
			require.NotNil(t, authorized.DestructiveGrant)
			updated := test.mutate(t, service, authorized)
			assert.Nil(t, updated.DestructiveGrant)
			assert.Nil(t, updated.PendingDestructiveRequest)
			replayed, err := service.Load(updated.ID)
			require.NoError(t, err)
			assert.Nil(t, replayed.DestructiveGrant)
			assert.Nil(t, replayed.PendingDestructiveRequest)
		})
	}

	t.Run("issue source resume", func(t *testing.T) {
		repository := newTestRepository(t)
		service := openTestService(t, repository)
		authorized := authorizeDestructiveRunForTest(t, service, true)
		resumed, err := service.Resume(authorized.ID, ResumeOptions{UsePinnedSource: true})
		require.NoError(t, err)
		assert.Nil(t, resumed.DestructiveGrant)
		assert.Nil(t, resumed.PendingDestructiveRequest)
	})
}

func TestServiceSkipRoutesObservedDiffBeforeActionKind(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, *Service, Run) Run
	}{
		{name: "orient", setup: func(_ *testing.T, _ *Service, run Run) Run { return run }},
		{
			name: "clarify",
			setup: func(t *testing.T, service *Service, run Run) Run {
				return submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "clarify", SuggestedActions: []SuggestedAction{{Kind: ActionClarify, Brief: "Ask one decision."}}})
			},
		},
		{
			name: "implement",
			setup: func(t *testing.T, service *Service, run Run) Run {
				return submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "implement"})
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			repository := newTestRepository(t)
			service := openTestService(t, repository)
			run := test.setup(t, service, startTestRun(t, service, 8, true))
			require.NoError(t, os.WriteFile(filepath.Join(repository, test.name+"-diff.txt"), []byte("changed\n"), 0o600))
			updated, err := service.Skip(run.ID, run.CurrentAction.ActionID)
			require.NoError(t, err)
			require.NotNil(t, updated.CurrentAction)
			assert.Equal(t, ActionReview, updated.CurrentAction.Kind)
		})
	}
}

func TestServiceSummarySkipEndsWithMinimalFactualSummary(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 5, false)
	run, err := service.Skip(run.ID, run.CurrentAction.ActionID)
	require.NoError(t, err)
	require.Equal(t, ActionSummarize, run.CurrentAction.Kind)
	run, err = service.Skip(run.ID, run.CurrentAction.ActionID)
	require.NoError(t, err)
	assert.Equal(t, RunEnded, run.State)
	assert.Nil(t, run.CurrentAction)
	assert.Contains(t, run.Summary, "Summary Action was skipped.")
	assert.Contains(t, run.Summary, "No host-authored final report was submitted.")
	next, err := DeriveNext(run)
	require.NoError(t, err)
	assert.Equal(t, NextOperationNone, next.Operation)
	assert.Empty(t, next.Variants)
}

func authorizeDestructiveRunForTest(t *testing.T, service *Service, issueBound bool) Run {
	t.Helper()
	var run Run
	if issueBound {
		run = startIssueTestRun(t, service, 12)
	} else {
		run = startTestRun(t, service, 12, false)
	}
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	originatingActionID := run.CurrentAction.ActionID
	request := destructiveRequestForTest(t)
	run = submitCurrent(t, service, run, Outcome{
		Status: OutcomeNeedsInput, Summary: "confirm",
		Pause: pauseReport(PauseDestructiveConfirm, "Confirm?", &request),
	})
	authorized, err := service.Answer(run.ID, originatingActionID, AnswerOptions{
		ConfirmDestructive: true,
		ScopeSHA256:        request.ScopeSHA256,
	})
	require.NoError(t, err)
	require.NotNil(t, authorized.DestructiveGrant)
	return authorized
}

func newTestRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.name", "Slipway Test")
	runGit(t, root, "config", "user.email", "test@example.com")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o600))
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "initial")
	return root
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s", output)
}

func openTestService(t *testing.T, repository string) *Service {
	t.Helper()
	service, err := OpenService(repository)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, service.Close()) })
	return service
}

func startTestRun(t *testing.T, service *Service, budget int, review bool) Run {
	t.Helper()
	run, err := service.Start("make the requested change", CreateOptions{Budget: budget, ReviewEnabled: review})
	require.NoError(t, err)
	require.NotNil(t, run.CurrentAction)
	return run
}

func submitCurrent(t *testing.T, service *Service, run Run, outcome Outcome) Run {
	t.Helper()
	require.NotNil(t, run.CurrentAction)
	if outcome.SuggestedActions == nil && outcome.Status != OutcomeNeedsInput &&
		(run.CurrentAction.Kind == ActionOrient || run.CurrentAction.Kind == ActionClarify) {
		outcome.SuggestedActions = []SuggestedAction{{Kind: ActionImplement, Brief: "Implement the requested change."}}
	}
	outcome = withEnvelope(run.CurrentAction.ActionID, run.CurrentAction.Kind, outcome)
	updated, err := service.Submit(run.ID, run.CurrentAction.ActionID, outcome)
	require.NoError(t, err)
	return updated
}

func withEnvelope(actionID string, actionKind ActionKind, outcome Outcome) Outcome {
	outcome.ContractVersion = ContractVersion
	outcome.ActionID = actionID
	outcome.ActionKind = actionKind
	if outcome.Observations == nil {
		outcome.Observations = []string{}
	}
	if outcome.KnownIssues == nil {
		outcome.KnownIssues = []string{}
	}
	if outcome.SuggestedActions == nil {
		outcome.SuggestedActions = []SuggestedAction{}
	}
	if outcome.Implementation != nil {
		if outcome.Implementation.FilesChanged == nil {
			outcome.Implementation.FilesChanged = []string{}
		}
		if outcome.Implementation.Activities == nil {
			outcome.Implementation.Activities = []Activity{}
		}
		if outcome.Implementation.Uncertainties == nil {
			outcome.Implementation.Uncertainties = []string{}
		}
	}
	if outcome.Review != nil {
		if outcome.Review.Findings == nil {
			outcome.Review.Findings = []Finding{}
		}
		if outcome.Review.Uncertainties == nil {
			outcome.Review.Uncertainties = []string{}
		}
	}
	return outcome
}

func implementationReport(result ImplementationResult, files ...string) *Implementation {
	return &Implementation{
		Result:        result,
		FilesChanged:  append([]string{}, files...),
		Activities:    []Activity{},
		Uncertainties: []string{},
		Attempts:      1,
	}
}

func reviewReport(result ReviewResult, findings ...Finding) *Review {
	return &Review{
		Result:        result,
		Findings:      append([]Finding{}, findings...),
		Uncertainties: []string{},
	}
}

func pauseReport(reason PauseReason, question string, request *DestructiveRequest) *Pause {
	return &Pause{Reason: reason, Question: question, DestructiveRequest: request}
}

func assertProtocolError(t *testing.T, err error, code string) {
	t.Helper()
	var protocolErr *ProtocolError
	require.True(t, errors.As(err, &protocolErr), "%v", err)
	assert.Equal(t, code, protocolErr.Code)
	require.NoError(t, protocolErr.Next.Validate())
}

func TestServiceReadsOnlyCurrentPinnedActionMaterial(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 5)
	currentActionID := run.CurrentAction.ActionID

	material, err := service.ReadActionMaterial(run.ID, currentActionID, "requirements")
	require.NoError(t, err)
	assert.Equal(t, ContractVersion, material.ContractVersion)
	assert.Equal(t, "action_material", material.MessageType)
	assert.Equal(t, run.PinnedSource.RequirementsRevision, material.RequirementsRevision)
	assert.Equal(t, "requirements", material.Section.Key)
	assert.Contains(t, material.Section.Markdown, "Keep order")

	_, err = service.ReadActionMaterial(run.ID, currentActionID, "missing")
	assertProtocolError(t, err, "material_section_not_found")

	completed := withEnvelope(currentActionID, ActionOrient, Outcome{
		Status:           OutcomeCompleted,
		Summary:          "oriented",
		SuggestedActions: []SuggestedAction{{Kind: ActionSummarize, Brief: "summarize"}},
	})
	next, err := service.Submit(run.ID, currentActionID, completed)
	require.NoError(t, err)
	require.NotNil(t, next.CurrentAction)
	_, err = service.ReadActionMaterial(run.ID, currentActionID, "requirements")
	assertProtocolError(t, err, "material_action_stale")

	stoppedActionID := next.CurrentAction.ActionID
	_, err = service.Stop(run.ID)
	require.NoError(t, err)
	_, err = service.ReadActionMaterial(run.ID, stoppedActionID, "requirements")
	assertProtocolError(t, err, "material_action_stale")
}

func TestServiceIssueBoundStartPersistsSafeDefensiveSnapshot(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	envelope := validSourceEnvelope()
	source := mustParseSource(t, envelope)
	expected := clonePinnedSourceValue(source)
	expected.materials = nil

	run, err := service.Start("implement the accepted Change", CreateOptions{
		Budget:        8,
		ReviewEnabled: true,
		PinnedSource:  &source,
	})
	require.NoError(t, err)
	require.NotNil(t, run.PinnedSource)
	assert.Equal(t, expected, *run.PinnedSource)
	require.NotNil(t, run.CurrentAction)
	require.NotNil(t, run.CurrentAction.Source)
	require.NotNil(t, run.CurrentAction.Requirements)
	assert.Equal(t, expected.CanonicalURL, run.CurrentAction.Source.CanonicalURL)
	assert.Equal(t, expected.IssueID, run.CurrentAction.Source.IssueID)
	assert.Equal(t, expected.SourceRevision, run.CurrentAction.Source.SourceRevision)
	assert.Equal(t, expected.ManifestRevision, run.CurrentAction.Source.ManifestRevision)
	assert.Equal(t, expected.RequirementsRevision, run.CurrentAction.Source.RequirementsRevision)
	expectedRequirements := actionRequirements(
		run.Workspace,
		run.CurrentAction.RunID,
		run.CurrentAction.ActionID,
		expected,
	)
	assert.Equal(t, expectedRequirements, *run.CurrentAction.Requirements)

	source.URLAliases = append(source.URLAliases, "https://github.com/signalridge/slipway/issues/41")
	source.Parent.IssueID = "I_kwDOMutatedParent"
	source.Sections[0].Title = "mutated caller data"
	run.CurrentAction.Source.CanonicalURL = "https://github.com/attacker/changed/issues/1"
	run.CurrentAction.Requirements.Sections[0].Title = "mutated returned action"

	loaded, err := service.Load(run.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded.PinnedSource)
	assert.Equal(t, expected, *loaded.PinnedSource)
	require.NotNil(t, loaded.CurrentAction)
	assert.Equal(t, expected.CanonicalURL, loaded.CurrentAction.Source.CanonicalURL)
	assert.Equal(t, expectedRequirements, *loaded.CurrentAction.Requirements)

	adHoc, err := service.Start("ad-hoc escape hatch", CreateOptions{Budget: 3, ReviewEnabled: false})
	require.NoError(t, err)
	require.NotNil(t, adHoc.CurrentAction)
	encodedAction, err := json.Marshal(adHoc.CurrentAction)
	require.NoError(t, err)
	var actionObject map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encodedAction, &actionObject))
	assert.NotContains(t, actionObject, "source")
	assert.NotContains(t, actionObject, "requirements")

	runDirectory := filepath.Join(service.store.CommonDir(), "slipway", "runs", run.ID)
	for _, name := range []string{"journal.jsonl", "run.json"} {
		content, readErr := os.ReadFile(filepath.Join(runDirectory, name))
		require.NoError(t, readErr)
		serialized := string(content)
		assert.NotContains(t, serialized, changeSourceMarker)
		assert.NotContains(t, serialized, "Implementation checklist")
		assert.NotContains(t, serialized, envelope.UpdatedAt)
		assert.NotContains(t, serialized, envelope.FetchedAt)
		assert.NotContains(t, serialized, envelope.Labels[0])
	}

	invalid := expected
	invalid.RequirementsRevision = "sha256:" + strings.Repeat("0", 64)
	beforeRuns, err := service.List()
	require.NoError(t, err)
	_, err = service.Start("must not create a journal", CreateOptions{Budget: 3, PinnedSource: &invalid})
	assertProtocolError(t, err, "invalid_source")
	afterRuns, err := service.List()
	require.NoError(t, err)
	assert.Len(t, afterRuns, len(beforeRuns))
}

func TestServiceResumeModesSeparateAdHocAndIssueBoundRuns(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)

	adHoc := startTestRun(t, service, 5, false)
	candidate := sourceCandidateForTest(t, validSourceEnvelope())
	_, err := service.Resume(adHoc.ID, ResumeOptions{RefreshedSource: &candidate})
	assertProtocolError(t, err, "source_mode_not_allowed")
	unchangedAdHoc, err := service.Load(adHoc.ID)
	require.NoError(t, err)
	assert.Equal(t, adHoc.CurrentAction.ActionID, unchangedAdHoc.CurrentAction.ActionID)

	adHocResumed, err := service.Resume(adHoc.ID, ResumeOptions{})
	require.NoError(t, err)
	require.NotNil(t, adHocResumed.CurrentAction)
	assert.Equal(t, ActionOrient, adHocResumed.CurrentAction.Kind)
	assert.Nil(t, adHocResumed.CurrentAction.Source)
	assert.Nil(t, adHocResumed.CurrentAction.Requirements)

	issueRun := startIssueTestRun(t, service, 8)
	issueActionID := issueRun.CurrentAction.ActionID
	_, err = service.Resume(issueRun.ID, ResumeOptions{})
	assertProtocolError(t, err, "source_mode_required")
	unchangedIssue, err := service.Load(issueRun.ID)
	require.NoError(t, err)
	assert.Equal(t, issueActionID, unchangedIssue.CurrentAction.ActionID)

	replacement := 5
	resumed, err := service.Resume(issueRun.ID, ResumeOptions{UsePinnedSource: true, Budget: &replacement})
	require.NoError(t, err)
	require.NotNil(t, resumed.CurrentAction)
	assert.Equal(t, ActionOrient, resumed.CurrentAction.Kind)
	assert.Equal(t, 4, resumed.RemainingBudget)
	assert.Equal(t, ResumeOperationSourceRefreshSkipped, resumed.Observations[len(resumed.Observations)-1])
	require.NotNil(t, resumed.LastResumeResult)
	assert.True(t, resumed.LastResumeResult.BudgetApplied)
	oldRecord := findActionRecord(&resumed, issueActionID)
	require.NotNil(t, oldRecord)
	assert.True(t, oldRecord.Voided)
	assert.Empty(t, resumed.PendingActions)
	assert.Empty(t, resumed.DecisionSuggestions)
	assertIssueActionMatchesPinned(t, resumed)
}

func TestServiceNonMaterialSourceRefreshesIssueFreshOrient(t *testing.T) {
	tests := []struct {
		name            string
		mutate          func(*RawSourceEnvelope)
		observation     string
		projectionDrift bool
		nonMaterial     bool
	}{
		{name: "identical", mutate: func(*RawSourceEnvelope) {}, observation: "source_refreshed_unchanged"},
		{
			name: "projection drift",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.IssueNumber = 77
				envelope.CanonicalURL = "https://github.com/signalridge/slipway/issues/77"
			},
			observation:     "source_projection_drift",
			projectionDrift: true,
		},
		{
			name: "non material body refresh",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Title = "[Change] Preserve source requirements after editorial refresh"
			},
			observation: "source_refreshed_non_material",
			nonMaterial: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newTestRepository(t)
			service := openTestService(t, repository)
			run := startIssueTestRun(t, service, 9)
			original := clonePinnedSourceValue(*run.PinnedSource)
			oldActionID := run.CurrentAction.ActionID

			envelope := validSourceEnvelope()
			test.mutate(&envelope)
			for index := range envelope.Comments {
				envelope.Comments[index].URL = envelope.CanonicalURL + "#issuecomment-" + jsonNumber(envelope.Comments[index].DatabaseID)
			}
			candidate := sourceCandidateForTest(t, envelope)
			replacement := 6
			resumed, err := service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidate, Budget: &replacement})
			require.NoError(t, err)
			require.NotNil(t, resumed.CurrentAction)
			assert.Equal(t, ActionOrient, resumed.CurrentAction.Kind)
			assert.NotEqual(t, oldActionID, resumed.CurrentAction.ActionID)
			assert.Equal(t, 5, resumed.RemainingBudget)
			assert.Nil(t, resumed.SourceCandidate)
			assert.Equal(t, test.observation, resumed.Observations[len(resumed.Observations)-1])
			require.NotNil(t, resumed.LastResumeResult)
			assert.True(t, resumed.LastResumeResult.BudgetApplied)
			assertIssueActionMatchesPinned(t, resumed)
			oldRecord := findActionRecord(&resumed, oldActionID)
			require.NotNil(t, oldRecord)
			assert.True(t, oldRecord.Voided)

			if test.projectionDrift {
				assert.Equal(t, envelope.CanonicalURL, resumed.PinnedSource.CanonicalURL)
				assert.Equal(t, []string{original.CanonicalURL}, resumed.PinnedSource.URLAliases)
				second, secondErr := service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidate})
				require.NoError(t, secondErr)
				assert.Equal(t, []string{original.CanonicalURL}, second.PinnedSource.URLAliases)
			}
			if test.nonMaterial {
				assert.NotEqual(t, original.SourceRevision, resumed.PinnedSource.SourceRevision)
				assert.Equal(t, original.RequirementsRevision, resumed.PinnedSource.RequirementsRevision)
				assert.Equal(t, envelope.Title, resumed.PinnedSource.Title)
			} else {
				assert.Equal(t, original.SourceRevision, resumed.PinnedSource.SourceRevision)
				assert.Equal(t, original.RequirementsRevision, resumed.PinnedSource.RequirementsRevision)
			}

			candidate.Snapshot.Sections[0].Title = "caller mutation after resume"
			loaded, loadErr := service.Load(run.ID)
			require.NoError(t, loadErr)
			assert.NotEqual(t, "caller mutation after resume", loaded.PinnedSource.Sections[0].Title)
		})
	}
}

func TestServiceRejectsInPlaceSectionAmendment(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 6)

	envelope := validSourceEnvelope()
	for index := range envelope.Comments {
		if testSourceCommentKey(envelope.Comments[index].Body) == "requirements" {
			envelope.Comments[index].Body = sectionMarkerPrefix + "requirements -->\n# Requirements\n\n- Edited in place.\n"
		}
	}
	rebuildSourceManifestBody(t, &envelope)
	setEnvelopeParentRequirementsRevision(t, &envelope, run.PinnedSource.RequirementsRevision)
	candidate := sourceCandidateForTest(t, envelope)
	_, err := service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidate})
	assertProtocolError(t, err, "source_history_in_place_edit")
}

func TestServiceRejectsAcceptedCommentDatabaseIdentityRebind(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 6)

	envelope := validSourceEnvelope()
	for index := range envelope.Comments {
		if testSourceCommentKey(envelope.Comments[index].Body) == "requirements" {
			envelope.Comments[index].DatabaseID += 100_000
			envelope.Comments[index].URL = envelope.CanonicalURL + "#issuecomment-" + jsonNumber(envelope.Comments[index].DatabaseID)
		}
	}
	rebuildSourceManifestBody(t, &envelope)
	setEnvelopeParentRequirementsRevision(t, &envelope, run.PinnedSource.RequirementsRevision)
	candidate := sourceCandidateForTest(t, envelope)
	require.NotNil(t, candidate.Snapshot)
	assert.NotEqual(t, run.PinnedSource.ManifestRevision, candidate.Snapshot.ManifestRevision)
	assert.Equal(t, run.PinnedSource.RequirementsRevision, candidate.Snapshot.RequirementsRevision)

	_, err := service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidate})
	assertProtocolError(t, err, "source_history_in_place_edit")
}

func TestServiceManifestOnlyReplacementRequiresExplicitAdoption(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 7)
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "one requirements-derived decision is needed",
		Pause:   pauseReport(PauseDecisionRequired, "Choose the retained behavior?", nil),
	})
	waitingActionID := run.CurrentAction.ActionID
	const retainedAnswer = "retain the exact accepted behavior"
	var err error
	run, err = service.Answer(run.ID, waitingActionID, AnswerOptions{Text: retainedAnswer})
	require.NoError(t, err)
	require.Len(t, run.Answers, 1)
	assert.True(t, run.Answers[0].Active)
	originalRequirements := run.PinnedSource.RequirementsRevision

	envelope := validSourceEnvelope()
	envelope.Comments[0].NodeID += "_replacement"
	envelope.Comments[0].DatabaseID += 100_000
	envelope.Comments[0].URL = envelope.CanonicalURL + "#issuecomment-" + jsonNumber(envelope.Comments[0].DatabaseID)
	rebuildSourceManifestBody(t, &envelope)
	setEnvelopeParentRequirementsRevision(t, &envelope, originalRequirements)
	candidate := sourceCandidateForTest(t, envelope)
	require.Equal(t, originalRequirements, candidate.RequirementsRevision)

	paused, err := service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidate})
	require.NoError(t, err)
	require.NotNil(t, paused.SourceCandidate)
	assert.Equal(t, RunPaused, paused.State)
	assert.NotEqual(t, run.PinnedSource.ManifestRevision, paused.SourceCandidate.Snapshot.ManifestRevision)

	adopted, err := service.Resume(run.ID, ResumeOptions{
		SourceChoice: SourceChoiceAdopt,
		CandidateID:  paused.SourceCandidate.CandidateID,
	})
	require.NoError(t, err)
	assert.Equal(t, originalRequirements, adopted.PinnedSource.RequirementsRevision)
	assert.Equal(t, ResumeOperationSourceAmended, adopted.LastResumeResult.Operation)
	require.Len(t, adopted.Answers, 1)
	assert.True(t, adopted.Answers[0].Active)
	assert.Empty(t, adopted.Answers[0].SupersededBy)
	require.NotNil(t, adopted.CurrentAction)
	assert.Contains(t, adopted.CurrentAction.Context, retainedAnswer)
}

func TestServiceRejectsAmendmentWithStaleManifestParent(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 6)
	actionID := run.CurrentAction.ActionID

	envelope := validSourceEnvelope()
	setEnvelopeSection(t, &envelope, "requirements", "\n# Requirements\n\n- Divergent amendment.\n")
	candidate := sourceCandidateForTest(t, envelope)
	_, err := service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidate})
	assertProtocolError(t, err, "source_history_fork")

	loaded, err := service.Load(run.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded.CurrentAction)
	assert.Equal(t, actionID, loaded.CurrentAction.ActionID)
	assert.Nil(t, loaded.SourceCandidate)
}

func TestServiceMaterialCandidateDefersBudgetAndAdoptDeactivatesAnswers(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 15)
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "one requirements-derived decision is needed",
		Pause:   pauseReport(PauseDecisionRequired, "Choose the storage boundary?", nil),
	})
	waitingActionID := run.CurrentAction.ActionID
	const historicalAnswer = "use the uniquely named glacier storage boundary"
	run, err := service.Answer(run.ID, waitingActionID, AnswerOptions{Text: historicalAnswer})
	require.NoError(t, err)
	require.Len(t, run.Answers, 1)
	oldRequirementsRevision := run.PinnedSource.RequirementsRevision
	assert.Equal(t, oldRequirementsRevision, run.Answers[0].RequirementsRevision)
	oldRemaining := run.RemainingBudget
	outstandingActionID := run.CurrentAction.ActionID

	envelope := validSourceEnvelope()
	setEnvelopeSection(t, &envelope, "requirements", "\n# Requirements\n\n- Keep amended order.\n")
	setEnvelopeParentRequirementsRevision(t, &envelope, oldRequirementsRevision)
	candidateInput := sourceCandidateForTest(t, envelope)
	replacementNotYetApplied := 100
	paused, err := service.Resume(run.ID, ResumeOptions{
		RefreshedSource: &candidateInput,
		Budget:          &replacementNotYetApplied,
	})
	require.NoError(t, err)
	assert.Equal(t, RunPaused, paused.State)
	assert.Equal(t, PauseDecisionRequired, paused.PauseReason)
	assert.Equal(t, oldRemaining, paused.RemainingBudget)
	assert.Nil(t, paused.CurrentAction)
	require.NotNil(t, paused.SourceCandidate)
	assert.True(t, paused.SourceCandidate.Valid)
	require.NotNil(t, paused.SourceCandidate.Snapshot)
	assert.NotEqual(t, oldRequirementsRevision, paused.SourceCandidate.RequirementsRevision)
	require.NotNil(t, paused.LastResumeResult)
	assert.False(t, paused.LastResumeResult.BudgetApplied)
	assert.Equal(t, paused.SourceCandidate.CandidateID, paused.LastResumeResult.CandidateID)
	outstandingRecord := findActionRecord(&paused, outstandingActionID)
	require.NotNil(t, outstandingRecord)
	assert.True(t, outstandingRecord.Voided)
	assert.Empty(t, paused.PendingActions)
	assert.Empty(t, paused.DecisionSuggestions)
	candidateID := paused.SourceCandidate.CandidateID

	candidateInput.Snapshot.Sections[0].Title = "mutated after candidate creation"
	replayedCandidate, err := service.Load(run.ID)
	require.NoError(t, err)
	assert.NotEqual(t, "mutated after candidate creation", replayedCandidate.SourceCandidate.Snapshot.Sections[0].Title)

	choiceBudget := 5
	adopted, err := service.Resume(run.ID, ResumeOptions{
		Budget:       &choiceBudget,
		SourceChoice: SourceChoiceAdopt,
		CandidateID:  candidateID,
	})
	require.NoError(t, err)
	assert.Equal(t, RunActive, adopted.State)
	assert.Equal(t, 4, adopted.RemainingBudget)
	assert.Nil(t, adopted.SourceCandidate)
	assert.Equal(t, candidateInput.RequirementsRevision, adopted.PinnedSource.RequirementsRevision)
	require.Len(t, adopted.Answers, 1)
	assert.Equal(t, oldRequirementsRevision, adopted.Answers[0].RequirementsRevision)
	assert.NotEqual(t, adopted.PinnedSource.RequirementsRevision, adopted.Answers[0].RequirementsRevision)
	assert.False(t, adopted.Answers[0].Active)
	assert.Equal(t, "requirements:"+adopted.PinnedSource.RequirementsRevision, adopted.Answers[0].SupersededBy)
	require.NotNil(t, adopted.CurrentAction)
	assert.NotContains(t, adopted.CurrentAction.Context, historicalAnswer)
	assert.Equal(t, ResumeOperationSourceAmended, adopted.Observations[len(adopted.Observations)-1])
	require.NotNil(t, adopted.LastSourceChoice)
	assert.Equal(t, candidateID, adopted.LastSourceChoice.CandidateID)
	assert.Equal(t, SourceChoiceAdopt, adopted.LastSourceChoice.Choice)
	assertIssueActionMatchesPinned(t, adopted)

	actionCount := len(adopted.Actions)
	actionID := adopted.CurrentAction.ActionID
	updatedAt := adopted.UpdatedAt
	retryBudget := 999
	retried, err := service.Resume(run.ID, ResumeOptions{
		Budget:       &retryBudget,
		SourceChoice: SourceChoiceAdopt,
		CandidateID:  candidateID,
	})
	require.NoError(t, err)
	assert.Equal(t, actionID, retried.CurrentAction.ActionID)
	assert.Equal(t, actionCount, len(retried.Actions))
	assert.Equal(t, 4, retried.RemainingBudget)
	assert.True(t, retried.UpdatedAt.Equal(updatedAt))

	_, err = service.Resume(run.ID, ResumeOptions{SourceChoice: SourceChoicePinned, CandidateID: candidateID})
	assertProtocolError(t, err, "source_choice_conflict")
	_, err = service.Resume(run.ID, ResumeOptions{SourceChoice: SourceChoiceAdopt, CandidateID: "stale-candidate-id"})
	assertProtocolError(t, err, "stale_source_candidate")

	replayed, err := service.Load(run.ID)
	require.NoError(t, err)
	assert.Nil(t, replayed.SourceCandidate)
	require.NotNil(t, replayed.LastSourceChoice)
	assert.Equal(t, candidateID, replayed.LastSourceChoice.CandidateID)
	assert.NotEqual(t, replayed.PinnedSource.RequirementsRevision, replayed.Answers[0].RequirementsRevision)

	eventsPath := filepath.Join(service.store.CommonDir(), "slipway", "runs", run.ID, "journal.jsonl")
	events, err := os.ReadFile(eventsPath)
	require.NoError(t, err)
	eventLines := strings.Split(strings.TrimSpace(string(events)), "\n")
	require.NotEmpty(t, eventLines)
	assert.NotContains(t, eventLines[len(eventLines)-1], historicalAnswer, "source adoption must not reserialize cumulative answer history")
	assert.NotContains(t, string(events), changeSourceMarker)
	assert.NotContains(t, string(events), "Implementation checklist")
}

func TestServiceSourceChoiceExactReplayAfterRunEnded(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 10)

	envelope := validSourceEnvelope()
	setEnvelopeSection(t, &envelope, "requirements", "\n# Requirements\n\n- Preserve source-choice receipts after completion.\n")
	setEnvelopeParentRequirementsRevision(t, &envelope, run.PinnedSource.RequirementsRevision)
	candidate := sourceCandidateForTest(t, envelope)
	paused, err := service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidate})
	require.NoError(t, err)
	require.NotNil(t, paused.SourceCandidate)
	candidateID := paused.SourceCandidate.CandidateID

	adopted, err := service.Resume(run.ID, ResumeOptions{
		SourceChoice: SourceChoiceAdopt,
		CandidateID:  candidateID,
	})
	require.NoError(t, err)
	require.Equal(t, RunActive, adopted.State)
	require.NotNil(t, adopted.LastSourceChoice)
	assert.Equal(t, candidateID, adopted.LastSourceChoice.CandidateID)
	assert.Equal(t, SourceChoiceAdopt, adopted.LastSourceChoice.Choice)

	run = submitCurrent(t, service, adopted, Outcome{Status: OutcomeCompleted, Summary: "source accepted"})
	require.Equal(t, ActionImplement, run.CurrentAction.Kind)
	run = submitCurrent(t, service, run, Outcome{
		Status:         OutcomeCompleted,
		Summary:        "nothing to change",
		Implementation: implementationReport(ImplementationNotNeeded),
	})
	require.Equal(t, ActionSummarize, run.CurrentAction.Kind)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "reported"})
	require.Equal(t, RunEnded, run.State)

	journalPath := filepath.Join(service.store.CommonDir(), "slipway", "runs", run.ID, "journal.jsonl")
	journalBefore, err := os.ReadFile(journalPath)
	require.NoError(t, err)

	replayed, err := service.Resume(run.ID, ResumeOptions{
		SourceChoice: SourceChoiceAdopt,
		CandidateID:  candidateID,
	})
	require.NoError(t, err)
	assert.Equal(t, run, replayed)

	_, err = service.Resume(run.ID, ResumeOptions{SourceChoice: SourceChoicePinned, CandidateID: candidateID})
	assertProtocolError(t, err, "source_choice_conflict")
	_, err = service.Resume(run.ID, ResumeOptions{SourceChoice: SourceChoiceAdopt, CandidateID: "stale-candidate-id"})
	assertProtocolError(t, err, "run_already_ended")

	journalAfter, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.Equal(t, journalBefore, journalAfter, "exact source-choice replay must not append a journal event")
}

func TestServiceCandidatePinnedRetainsSourceAndActiveAnswers(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 12)
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "decision needed",
		Pause:   pauseReport(PauseDecisionRequired, "Choose one?", nil),
	})
	run, err := service.Answer(run.ID, run.CurrentAction.ActionID, AnswerOptions{Text: "retain this active answer"})
	require.NoError(t, err)
	oldSource := clonePinnedSourceValue(*run.PinnedSource)

	envelope := validSourceEnvelope()
	setEnvelopeSection(t, &envelope, "requirements", "\n# Requirements\n\n- Replace the ordering rule.\n")
	setEnvelopeParentRequirementsRevision(t, &envelope, oldSource.RequirementsRevision)
	candidate := sourceCandidateForTest(t, envelope)
	paused, err := service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidate})
	require.NoError(t, err)
	candidateID := paused.SourceCandidate.CandidateID

	kept, err := service.Resume(run.ID, ResumeOptions{SourceChoice: SourceChoicePinned, CandidateID: candidateID})
	require.NoError(t, err)
	assert.Equal(t, oldSource.RequirementsRevision, kept.PinnedSource.RequirementsRevision)
	require.Len(t, kept.Answers, 1)
	assert.Equal(t, kept.PinnedSource.RequirementsRevision, kept.Answers[0].RequirementsRevision)
	require.NotNil(t, kept.CurrentAction)
	assert.Contains(t, kept.CurrentAction.Context, "retain this active answer")
	assert.Equal(t, ResumeOperationSourcePinned, kept.Observations[len(kept.Observations)-1])
}

func TestServiceInvalidCandidateAllowsOnlyPinnedChoice(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RawSourceEnvelope)
		code   string
	}{
		{
			name: "objective marker",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Body = strings.Replace(envelope.Body, changeSourceMarker, "<!-- slipway-level: objective/v1 -->", 1)
			},
			code: SourceClassificationObjectiveMarker,
		},
		{
			name: "missing section",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Comments = envelope.Comments[:len(envelope.Comments)-1]
			},
			code: SourceClassificationSectionMissing,
		},
		{
			name: "invalid marker position",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Body = "preface\r\n" + envelope.Body
			},
			code: SourceClassificationChangeMarkerRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newTestRepository(t)
			service := openTestService(t, repository)
			run := startIssueTestRun(t, service, 8)
			remaining := run.RemainingBudget
			envelope := validSourceEnvelope()
			test.mutate(&envelope)
			candidateInput := sourceCandidateForTest(t, envelope)
			replacement := 20

			paused, err := service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidateInput, Budget: &replacement})
			require.NoError(t, err)
			require.NotNil(t, paused.SourceCandidate)
			assert.False(t, paused.SourceCandidate.Valid)
			assert.Equal(t, test.code, paused.SourceCandidate.ClassificationCode)
			assert.Empty(t, paused.SourceCandidate.RequirementsRevision)
			assert.Nil(t, paused.SourceCandidate.Snapshot)
			assert.Equal(t, remaining, paused.RemainingBudget)
			assert.False(t, paused.LastResumeResult.BudgetApplied)
			candidateID := paused.SourceCandidate.CandidateID

			_, err = service.Resume(run.ID, ResumeOptions{})
			assertProtocolError(t, err, "source_choice_required")
			_, err = service.Resume(run.ID, ResumeOptions{UsePinnedSource: true})
			assertProtocolError(t, err, "source_candidate_pending")
			_, err = service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidateInput})
			assertProtocolError(t, err, "source_candidate_pending")
			_, err = service.Resume(run.ID, ResumeOptions{SourceChoice: SourceChoicePinned, CandidateID: "wrong-candidate"})
			assertProtocolError(t, err, "stale_source_candidate")
			_, err = service.Resume(run.ID, ResumeOptions{SourceChoice: SourceChoiceAdopt, CandidateID: candidateID})
			assertProtocolError(t, err, "invalid_source_candidate_choice")

			choiceBudget := 1
			resumed, err := service.Resume(run.ID, ResumeOptions{
				Budget:       &choiceBudget,
				SourceChoice: SourceChoicePinned,
				CandidateID:  candidateID,
			})
			require.NoError(t, err)
			require.NotNil(t, resumed.CurrentAction)
			assert.Equal(t, 0, resumed.RemainingBudget)
			assert.Nil(t, resumed.SourceCandidate)
			assert.Equal(t, ResumeOperationSourcePinned, resumed.Observations[len(resumed.Observations)-1])
		})
	}
}

func TestServiceRejectsTransferBeyondURLAliasLimit(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	source := mustParseSource(t, validSourceEnvelope())
	source.URLAliases = make([]string, maxSourceURLAliases)
	for index := range source.URLAliases {
		source.URLAliases[index] = "https://github.com/example/repository/issues/" + jsonNumber(int64(1000+index))
	}
	run, err := service.Start("bounded transfer history", CreateOptions{
		Budget:        6,
		ReviewEnabled: false,
		PinnedSource:  &source,
	})
	require.NoError(t, err)
	actionID := run.CurrentAction.ActionID

	envelope := validSourceEnvelope()
	envelope.RepositoryID = "R_kgDOTransferredAtLimit"
	envelope.IssueNumber = 99
	envelope.CanonicalURL = "https://github.com/example/transferred/issues/99"
	for index := range envelope.Comments {
		envelope.Comments[index].URL = envelope.CanonicalURL + "#issuecomment-" + jsonNumber(envelope.Comments[index].DatabaseID)
	}
	candidate := sourceCandidateForTest(t, envelope)
	_, err = service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidate})
	assertProtocolError(t, err, "source_alias_limit")
	var protocolErr *ProtocolError
	require.ErrorAs(t, err, &protocolErr)
	require.Len(t, protocolErr.Next.Variants, 1)
	assert.Equal(t, "start-with-source", protocolErr.Next.Variants[0].ID)

	unchanged, err := service.Load(run.ID)
	require.NoError(t, err)
	assert.Equal(t, actionID, unchanged.CurrentAction.ActionID)
	assert.Equal(t, source.URLAliases, unchanged.PinnedSource.URLAliases)
}

func TestServiceRefreshRejectsCrossIssueWithoutMutationAndTracksTransfer(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 10)
	before, err := json.Marshal(run)
	require.NoError(t, err)

	crossIssueEnvelope := validSourceEnvelope()
	crossIssueEnvelope.IssueID = "I_kwDOAnotherIssue"
	crossIssue := sourceCandidateForTest(t, crossIssueEnvelope)
	_, err = service.Resume(run.ID, ResumeOptions{RefreshedSource: &crossIssue})
	assertProtocolError(t, err, "source_issue_mismatch")
	unchanged, err := service.Load(run.ID)
	require.NoError(t, err)
	after, err := json.Marshal(unchanged)
	require.NoError(t, err)
	assert.JSONEq(t, string(before), string(after))

	crossIssueEnvelope.Body = strings.Replace(crossIssueEnvelope.Body, changeSourceMarker, "<!-- slipway-level: objective/v1 -->", 1)
	crossIssueInvalidBody := sourceCandidateForTest(t, crossIssueEnvelope)
	require.False(t, crossIssueInvalidBody.Valid)
	_, err = service.Resume(run.ID, ResumeOptions{RefreshedSource: &crossIssueInvalidBody})
	assertProtocolError(t, err, "source_issue_mismatch")
	stillUnchanged, err := service.Load(run.ID)
	require.NoError(t, err)
	stillAfter, err := json.Marshal(stillUnchanged)
	require.NoError(t, err)
	assert.JSONEq(t, string(before), string(stillAfter))

	transferRun := startIssueTestRun(t, service, 10)
	oldURL := transferRun.PinnedSource.CanonicalURL
	transferEnvelope := validSourceEnvelope()
	transferEnvelope.RepositoryID = "R_kgDOTransferred"
	transferEnvelope.IssueNumber = 99
	transferEnvelope.CanonicalURL = "https://github.com/signalridge/slipway-next/issues/99"
	for index := range transferEnvelope.Comments {
		transferEnvelope.Comments[index].URL = transferEnvelope.CanonicalURL + "#issuecomment-" + jsonNumber(transferEnvelope.Comments[index].DatabaseID)
	}
	setEnvelopeSection(t, &transferEnvelope, "requirements", "\n# Requirements\n\n- Use transferred requirements.\n")
	setEnvelopeParentRequirementsRevision(t, &transferEnvelope, transferRun.PinnedSource.RequirementsRevision)
	transferred := sourceCandidateForTest(t, transferEnvelope)
	paused, err := service.Resume(transferRun.ID, ResumeOptions{RefreshedSource: &transferred})
	require.NoError(t, err)
	require.NotNil(t, paused.SourceCandidate)
	assert.True(t, paused.SourceCandidate.Valid, "repository transfer must not skip body amendment classification")
	assert.Equal(t, transferRun.PinnedSource.RepositoryID, paused.PinnedSource.RepositoryID)
	assert.Equal(t, oldURL, paused.PinnedSource.CanonicalURL)
	assert.Equal(t, transferRun.PinnedSource.URLAliases, paused.PinnedSource.URLAliases)
	assert.Equal(t, transferEnvelope.RepositoryID, paused.SourceCandidate.RepositoryID)
	assert.Equal(t, transferEnvelope.CanonicalURL, paused.SourceCandidate.CanonicalURL)
	assert.Equal(t, []string{oldURL}, paused.SourceCandidate.URLAliases)
	require.NotNil(t, paused.SourceCandidate.Snapshot)
	assert.Equal(t, []string{oldURL}, paused.SourceCandidate.Snapshot.URLAliases)
	assert.NotEqual(t, transferRun.PinnedSource.RequirementsRevision, paused.SourceCandidate.RequirementsRevision)
}

func TestServicePinnedTransferChoiceKeepsRequirementsAndUpdatesProjection(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 10)
	original := clonePinnedSourceValue(*run.PinnedSource)

	transferEnvelope := validSourceEnvelope()
	transferEnvelope.RepositoryID = "R_kgDOTransferredPinned"
	transferEnvelope.IssueNumber = 99
	transferEnvelope.CanonicalURL = "https://github.com/signalridge/slipway-next/issues/99"
	for index := range transferEnvelope.Comments {
		transferEnvelope.Comments[index].URL = transferEnvelope.CanonicalURL + "#issuecomment-" + jsonNumber(transferEnvelope.Comments[index].DatabaseID)
	}
	setEnvelopeSection(t, &transferEnvelope, "requirements", "\n# Requirements\n\n- Candidate requirements that remain unadopted.\n")
	setEnvelopeParentRequirementsRevision(t, &transferEnvelope, original.RequirementsRevision)
	candidate := sourceCandidateForTest(t, transferEnvelope)
	paused, err := service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidate})
	require.NoError(t, err)
	require.NotNil(t, paused.SourceCandidate)

	resolved, err := service.Resume(run.ID, ResumeOptions{
		SourceChoice: SourceChoicePinned,
		CandidateID:  paused.SourceCandidate.CandidateID,
	})
	require.NoError(t, err)
	require.NotNil(t, resolved.PinnedSource)
	assert.Equal(t, original.ManifestRevision, resolved.PinnedSource.ManifestRevision)
	assert.Equal(t, original.RequirementsRevision, resolved.PinnedSource.RequirementsRevision)
	assert.Equal(t, original.Sections, resolved.PinnedSource.Sections)
	assert.Equal(t, transferEnvelope.RepositoryID, resolved.PinnedSource.RepositoryID)
	assert.Equal(t, transferEnvelope.IssueNumber, resolved.PinnedSource.IssueNumber)
	assert.Equal(t, transferEnvelope.CanonicalURL, resolved.PinnedSource.CanonicalURL)
	assert.Equal(t, []string{original.CanonicalURL}, resolved.PinnedSource.URLAliases)
	assert.NotEqual(t, original.SourceRevision, resolved.PinnedSource.SourceRevision)

	loaded, err := service.Load(run.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded.PinnedSource)
	assert.Equal(t, resolved.PinnedSource, loaded.PinnedSource)
}

func TestServiceResumeUsesImportedSourceAfterEphemeralFileRemoval(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 7)
	envelope := validSourceEnvelope()
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "refresh-source.json")
	require.NoError(t, os.WriteFile(path, raw, 0o400))
	candidate, err := ImportSourceCandidateFile(path)
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))

	resumed, err := service.Resume(run.ID, ResumeOptions{RefreshedSource: &candidate})
	require.NoError(t, err)
	require.NotNil(t, resumed.CurrentAction)
	assert.Equal(t, ActionOrient, resumed.CurrentAction.Kind)
	runDirectory := filepath.Join(service.store.CommonDir(), "slipway", "runs", run.ID)
	for _, name := range []string{"journal.jsonl", "run.json"} {
		content, readErr := os.ReadFile(filepath.Join(runDirectory, name))
		require.NoError(t, readErr)
		assert.NotContains(t, string(content), path)
		assert.NotContains(t, string(content), filepath.Base(path))
	}
}

func startIssueTestRun(t *testing.T, service *Service, budget int) Run {
	t.Helper()
	source := mustParseSource(t, validSourceEnvelope())
	run, err := service.Start("implement the accepted Change", CreateOptions{
		Budget:        budget,
		ReviewEnabled: false,
		PinnedSource:  &source,
	})
	require.NoError(t, err)
	return run
}

func sourceCandidateForTest(t *testing.T, envelope RawSourceEnvelope) SourceCandidateInput {
	t.Helper()
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	candidate, err := ParseSourceCandidate(raw)
	require.NoError(t, err)
	return candidate
}

func assertIssueActionMatchesPinned(t *testing.T, run Run) {
	t.Helper()
	require.NotNil(t, run.PinnedSource)
	require.NotNil(t, run.CurrentAction)
	require.NotNil(t, run.CurrentAction.Source)
	require.NotNil(t, run.CurrentAction.Requirements)
	assert.Equal(t, run.PinnedSource.CanonicalURL, run.CurrentAction.Source.CanonicalURL)
	assert.Equal(t, run.PinnedSource.IssueID, run.CurrentAction.Source.IssueID)
	assert.Equal(t, run.PinnedSource.SourceRevision, run.CurrentAction.Source.SourceRevision)
	assert.Equal(t, run.PinnedSource.ManifestRevision, run.CurrentAction.Source.ManifestRevision)
	assert.Equal(t, run.PinnedSource.RequirementsRevision, run.CurrentAction.Source.RequirementsRevision)
	expected := actionRequirements(
		run.Workspace,
		run.CurrentAction.RunID,
		run.CurrentAction.ActionID,
		*run.PinnedSource,
	)
	assert.Equal(t, expected, *run.CurrentAction.Requirements)
}
