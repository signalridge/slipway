package autopilot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceRecordsAppliedWithoutObservedDifferenceAsNeutralDiscrepancy(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	run = submitCurrent(t, service, run, Outcome{
		Status: OutcomeCompleted, Summary: "reported application",
		Implementation: implementationReport(ImplementationApplied, "reported.go"),
	})

	joined := strings.Join(run.Observations, "\n")
	assert.Contains(t, joined, "report_discrepancy: Implement reported applied while no start-to-current Git difference was observed.")
	assert.NotContains(t, joined, observedSinceStart)
	assert.NotContains(t, joined, "contradict")
	assert.NotContains(t, joined, "despite")
	assert.NotContains(t, joined, "claim mismatch")
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionSummarize, run.CurrentAction.Kind)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "summary"})
	assert.Contains(t, run.Summary, "report_discrepancy: Implement reported applied while no start-to-current Git difference was observed.")
}

func TestServiceSkipWithObservedDifferenceRecordsNeutralAttributionBeforeReview(t *testing.T) {
	repository := newTestRepository(t)
	require.NoError(t, os.WriteFile(filepath.Join(repository, "preexisting.txt"), []byte("before\n"), 0o600))
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, true)
	require.NoError(t, os.WriteFile(filepath.Join(repository, "concurrent.txt"), []byte("current\n"), 0o600))

	var err error
	run, err = service.Skip(run.ID, run.CurrentAction.ActionID)
	require.NoError(t, err)
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, ActionReview, run.CurrentAction.Kind)
	assert.Contains(t, strings.Join(run.Observations, "\n"), observedSinceStart)
	assert.Contains(t, strings.Join(run.Uncertainties, "\n"), attributionUncertainty)
	assert.Contains(t, run.CurrentAction.Brief, "concurrent user edits, another Run, or tools")
	assert.Contains(t, run.CurrentAction.Brief, "preexisting.txt")
	assert.NotContains(t, run.CurrentAction.Brief, "concurrent.txt")
}

func TestFinalSummaryPreservesAttributionUncertaintyAndPreexistingObservations(t *testing.T) {
	repository := newTestRepository(t)
	require.NoError(t, os.WriteFile(filepath.Join(repository, "preexisting.txt"), []byte("before\n"), 0o600))
	service := openTestService(t, repository)
	run := startTestRun(t, service, 10, true)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "facts"})
	require.NoError(t, os.WriteFile(filepath.Join(repository, "implemented.go"), []byte("package sample\n"), 0o600))
	run = submitCurrent(t, service, run, Outcome{
		Status: OutcomeCompleted, Summary: "implementation report",
		Implementation: implementationReport(ImplementationApplied, "implemented.go"),
	})
	require.Equal(t, ActionReview, run.CurrentAction.Kind)
	run = submitCurrent(t, service, run, Outcome{
		Status: OutcomeCompleted, Summary: "review complete",
		Review: reviewReport(ReviewNoFindings),
	})
	require.Equal(t, ActionSummarize, run.CurrentAction.Kind)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "summary complete"})

	assert.Equal(t, RunEnded, run.State)
	assert.Contains(t, run.Summary, observedSinceStart)
	assert.Contains(t, run.Summary, attributionUncertainty)
	assert.Contains(t, run.Summary, "Pre-existing dirty path observations at Run start")
	assert.Contains(t, run.Summary, "preexisting.txt")
	assert.Contains(t, run.Summary, "content_sha256=sha256:")
	assert.NotContains(t, run.Summary, "contradictory")
	assert.NotContains(t, run.Summary, "despite")
	assert.NotContains(t, run.Summary, "claim mismatch")
}
