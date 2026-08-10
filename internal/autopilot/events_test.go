package autopilot

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/runstore"
	"github.com/stretchr/testify/require"
)

func TestApplyRunEventValidatesStatePauseReasonAndBudget(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 4, false)
	run = submitCurrent(t, service, run, Outcome{
		Status:  OutcomeNeedsInput,
		Summary: "a decision is required",
		Pause: &Pause{
			Reason:   PauseDecisionRequired,
			Question: "Which option should be used?",
		},
	})
	require.Equal(t, RunPaused, run.State)

	journalPath := filepath.Join(service.store.CommonDir(), "slipway", "runs", run.ID, "journal.jsonl")
	content, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	lines := bytes.Split(bytes.TrimSpace(content), []byte("\n"))
	require.Len(t, lines, 2)
	events := make([]runstore.Event, len(lines))
	for index := range lines {
		require.NoError(t, json.Unmarshal(lines[index], &events[index]))
	}

	tests := []struct {
		name       string
		eventIndex int
		mutate     func(*runDelta)
		wantError  string
	}{
		{
			name:       "initialization rejects unknown state",
			eventIndex: 0,
			mutate: func(delta *runDelta) {
				delta.State = pointer(RunState("tampered"))
			},
			wantError: "unknown run state",
		},
		{
			name:       "initialization rejects budget above maximum",
			eventIndex: 0,
			mutate: func(delta *runDelta) {
				delta.Initialize.InitialBudget = 1001
			},
			wantError: "budget cannot exceed 1000",
		},
		{
			name:       "delta rejects unknown pause reason",
			eventIndex: 1,
			mutate: func(delta *runDelta) {
				delta.PauseReason = pointer(PauseReason("tampered"))
			},
			wantError: "unknown pause reason",
		},
		{
			name:       "delta rejects negative remaining budget",
			eventIndex: 1,
			mutate: func(delta *runDelta) {
				delta.RemainingBudget = pointer(-1)
			},
			wantError: "remaining budget cannot be negative",
		},
		{name: "valid initialization values are accepted", eventIndex: 0},
		{name: "valid delta values are accepted", eventIndex: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var replay Run
			for index := 0; index < test.eventIndex; index++ {
				require.NoError(t, applyRunEvent(&replay, events[index]))
			}

			event := events[test.eventIndex]
			var delta runDelta
			require.NoError(t, json.Unmarshal(event.Data, &delta))
			if test.mutate != nil {
				test.mutate(&delta)
			}
			event.Data, err = json.Marshal(delta)
			require.NoError(t, err)

			err = applyRunEvent(&replay, event)
			if test.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantError)
		})
	}
}

func TestApplyRunEventMinimizesLegacyInvalidCandidateTitle(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 6)
	stopped := stopRunForResume(t, service, run)

	envelope := validSourceEnvelope()
	envelope.Body = "<!-- slipway-level: objective/v1 -->\n"
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	candidateInput, err := ParseSourceCandidate(raw)
	require.NoError(t, err)
	require.False(t, candidateInput.Valid)
	require.Empty(t, candidateInput.Title)

	paused, err := service.Resume(
		run.ID,
		ResumeOptions{RefreshedSource: &candidateInput},
	)
	require.NoError(t, err)
	require.NotNil(t, paused.SourceCandidate)

	event, err := newRunEvent(ResumeOperationSourceCandidate, stopped, paused)
	require.NoError(t, err)
	var legacyDelta runDelta
	require.NoError(t, json.Unmarshal(event.Data, &legacyDelta))
	require.NotNil(t, legacyDelta.SourceCandidate)
	legacyDelta.SourceCandidate.Title = envelope.Title
	event.Data, err = json.Marshal(legacyDelta)
	require.NoError(t, err)

	replayed := runBeforeMutation(stopped)
	require.NoError(t, applyRunEvent(&replayed, event))
	require.Equal(t, paused, replayed)
	require.NotNil(t, replayed.SourceCandidate)
	require.Empty(t, replayed.SourceCandidate.Title)
}
