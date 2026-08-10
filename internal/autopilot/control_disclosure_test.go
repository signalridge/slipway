package autopilot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStopRevokesThePublishedDestructiveAuthorization pins the boundary that a
// Run which can no longer execute must not keep advertising destructive
// authority. The issued Action stays in the journal — it is immutable — but the
// Run stops publishing it as current.
func TestStopRevokesThePublishedDestructiveAuthorization(t *testing.T) {
	t.Parallel()
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	authorized := authorizeDestructiveRunForTest(t, service, false)
	require.NotNil(t, authorized.CurrentAction)
	require.NotNil(t, authorized.CurrentAction.DestructiveAuthorization)
	originatingActionID := authorized.CurrentAction.DestructiveAuthorization.OriginatingActionID

	authorizedActionID := authorized.CurrentAction.ActionID

	stopped, err := service.Stop(authorized.ID)
	require.NoError(t, err)
	assert.Equal(t, RunStopped, stopped.State)
	assert.Nil(t, stopped.DestructiveGrant)
	assert.Nil(t, stopped.PendingDestructiveRequest)
	assert.Nil(t, stopped.CurrentAction,
		"a stopped run must not publish an Action carrying a spent destructive authorization")

	replayed, err := service.Load(authorized.ID)
	require.NoError(t, err)
	assert.Nil(t, replayed.CurrentAction, "replay must not republish the stopped run's Action")
	voided := findActionRecord(&replayed, authorizedActionID)
	require.NotNil(t, voided)
	assert.True(t, voided.Voided)

	// Withdrawing the Action must not erase the record of what was asked and
	// confirmed: both remain on the originating Action and its answer receipt.
	originating := findActionRecord(&replayed, originatingActionID)
	require.NotNil(t, originating)
	require.NotNil(t, originating.Outcome)
	require.NotNil(t, originating.Outcome.Pause)
	require.NotNil(t, originating.Outcome.Pause.DestructiveRequest)
	assert.Equal(t, "permanent deletion", originating.Outcome.Pause.DestructiveRequest.Impact)
	confirmed := false
	for _, answer := range replayed.Answers {
		if answer.ActionID == originatingActionID && answer.ConfirmDestructive {
			confirmed = true
		}
	}
	assert.True(t, confirmed, "the confirmation receipt must survive the revocation")
}

func TestStopIsIdempotentAfterRevokingTheAuthorization(t *testing.T) {
	t.Parallel()
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	authorized := authorizeDestructiveRunForTest(t, service, false)

	_, err := service.Stop(authorized.ID)
	require.NoError(t, err)
	repeated, err := service.Stop(authorized.ID)
	require.NoError(t, err)
	assert.Equal(t, RunStopped, repeated.State)
	assert.Nil(t, repeated.CurrentAction)

	// The Run must still be resumable into a fresh Orient after the withdrawal.
	resumed, err := service.Resume(authorized.ID, ResumeOptions{})
	require.NoError(t, err)
	require.NotNil(t, resumed.CurrentAction)
	assert.Equal(t, ActionOrient, resumed.CurrentAction.Kind)
	assert.Nil(t, resumed.CurrentAction.DestructiveAuthorization)
	assert.Nil(t, resumed.DestructiveGrant)
}

// TestReviewPreemptionReportsTheSuggestionsItDrops covers the case that used to
// end a run without ever asking a decision the host had already identified: an
// observed revision preempts the queue with advisory Review, Review routes to
// Summary, and the queued Clarify is never dispatched.
func TestReviewPreemptionReportsTheSuggestionsItDrops(t *testing.T) {
	t.Parallel()
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)

	require.NoError(t, os.WriteFile(filepath.Join(repository, "concurrent.go"), []byte("package sample\n"), 0o600))
	run = submitCurrent(t, service, run, Outcome{
		Status:           OutcomeCompleted,
		Summary:          "facts gathered",
		SuggestedActions: []SuggestedAction{{Kind: ActionClarify, Brief: "Choose the retention window."}},
	})
	require.NotNil(t, run.CurrentAction)
	require.Equal(t, ActionReview, run.CurrentAction.Kind)
	assert.Empty(t, run.PendingActions)
	assert.Contains(t, run.Uncertainties,
		"unresolved_suggestion: a queued clarify suggestion was dropped when a newly observed revision preempted the queue with advisory Review and was never dispatched: Choose the retention window.")

	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeCompleted,
		Summary: "no findings",
		Review:  reviewReport(ReviewNoFindings),
	})
	require.NotNil(t, run.CurrentAction)
	require.Equal(t, ActionSummarize, run.CurrentAction.Kind)

	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts reported"})
	require.Equal(t, RunEnded, run.State)
	assert.Contains(t, run.Summary, "unresolved_suggestion:")
	assert.Contains(t, run.Summary, "Choose the retention window.")
}

// TestDiscardPendingActionsNamesEveryDroppedSuggestion covers the helper the
// remaining discard sites share. Skip, the destructive confirmation, and the
// Review override all reach it; only the Review override can currently observe
// a non-empty queue, so the others are covered here rather than through a run
// state the Outcome contract cannot produce today.
func TestDiscardPendingActionsNamesEveryDroppedSuggestion(t *testing.T) {
	t.Parallel()
	run := Run{PendingActions: []SuggestedAction{
		{Kind: ActionClarify, Brief: "Choose the retention window."},
		{Kind: ActionImplement, Brief: "Apply the chosen retention window."},
	}}
	discardPendingActions(&run, "for a stated reason")
	assert.Empty(t, run.PendingActions)
	require.Len(t, run.Uncertainties, 2)
	assert.Equal(t,
		"unresolved_suggestion: a queued clarify suggestion was dropped for a stated reason and was never dispatched: Choose the retention window.",
		run.Uncertainties[0])
	assert.Contains(t, run.Uncertainties[1], "Apply the chosen retention window.")

	empty := Run{}
	discardPendingActions(&empty, "for a stated reason")
	assert.Empty(t, empty.Uncertainties)
}

// TestAnsweredDecisionDropsItsQueueWithoutInventingAnUncertainty guards the
// other half of the rule: a fresh Orient immediately follows an answered
// decision and re-derives the queue, so nothing was silently lost there.
func TestAnsweredDecisionDropsItsQueueWithoutInventingAnUncertainty(t *testing.T) {
	t.Parallel()
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	run = submitCurrent(t, service, run, Outcome{
		Status:           OutcomeCompleted,
		Summary:          "one human decision remains",
		SuggestedActions: []SuggestedAction{{Kind: ActionClarify, Brief: "Choose the release channel."}},
	})
	require.NotNil(t, run.CurrentAction)
	require.Equal(t, ActionClarify, run.CurrentAction.Kind)
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "waiting",
		Pause:   pauseReport(PauseDecisionRequired, "Which channel?", nil),
	})
	answered, err := service.Answer(run.ID, run.CurrentAction.ActionID, AnswerOptions{Text: "stable"})
	require.NoError(t, err)
	require.NotNil(t, answered.CurrentAction)
	assert.Equal(t, ActionOrient, answered.CurrentAction.Kind)
	assert.NotContains(t, strings.Join(answered.Uncertainties, "\n"), "unresolved_suggestion:")
}

// TestStoppedRunReportsItsStateBeforeActionStaleness pins the precedence the
// withdrawal introduced: an Action that already carries an outcome still
// answers for itself in any state, while an Action the stop withdrew reports
// the run state the caller has to act on.
func TestStoppedRunReportsItsStateBeforeActionStaleness(t *testing.T) {
	t.Parallel()
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	orientID := run.CurrentAction.ActionID
	orient := withEnvelope(orientID, ActionOrient, Outcome{
		Status:           OutcomeCompleted,
		Summary:          "facts",
		SuggestedActions: []SuggestedAction{{Kind: ActionImplement, Brief: "Apply the change."}},
	})
	run, err := service.Submit(run.ID, orientID, orient)
	require.NoError(t, err)
	require.NotNil(t, run.CurrentAction)
	withdrawnID := run.CurrentAction.ActionID

	_, err = service.Stop(run.ID)
	require.NoError(t, err)

	// The withdrawn Action never carried an outcome: report the run state.
	_, err = service.Submit(run.ID, withdrawnID, withEnvelope(withdrawnID, ActionImplement, Outcome{
		Status:         OutcomeCompleted,
		Summary:        "late",
		Implementation: implementationReport(ImplementationNotNeeded),
	}))
	assertProtocolError(t, err, "run_not_active")

	// The completed Action still replays idempotently and still conflicts.
	replayed, err := service.Submit(run.ID, orientID, orient)
	require.NoError(t, err)
	assert.Equal(t, RunStopped, replayed.State)
	conflicting := orient
	conflicting.Summary = "different"
	_, err = service.Submit(run.ID, orientID, conflicting)
	assertProtocolError(t, err, "outcome_conflict")
}
