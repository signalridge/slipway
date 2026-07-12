package autopilot

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildContextRendersOnlyActiveDecisionsAndOutcomeProjectionsInClassOrder(t *testing.T) {
	t.Parallel()
	currentRevision := "sha256:" + strings.Repeat("b", 64)
	run := Run{
		Goal: "raw goal must stay outside context",
		PinnedSource: &PinnedSource{
			RequirementsRevision: currentRevision,
			AcceptedRequirements: AcceptedRequirements{RequirementsMarkdown: "raw requirements must stay outside context"},
		},
		Answers: []AnswerRecord{
			{ActionID: "old-source", Text: "old source decision", Active: true, RequirementsRevision: "sha256:" + strings.Repeat("a", 64)},
			{ActionID: "decision-1", Text: "first line\r\nsecond line", Active: true, RequirementsRevision: currentRevision},
			{ActionID: "overturned", Text: "overturned decision", Active: false, SupersededBy: "decision-2", RequirementsRevision: currentRevision},
			{ActionID: "decision-2", Text: "latest active decision", Active: true, RequirementsRevision: currentRevision},
			{ActionID: "structural-confirmation", Text: "confirmed in host UI", ConfirmDestructive: true, Active: false, RequirementsRevision: currentRevision},
		},
		Actions: []ActionRecord{
			{Action: Action{ActionID: "action-1", Kind: ActionOrient}, Outcome: &Outcome{Summary: "first outcome", KnownIssues: []string{}}},
			{Action: Action{ActionID: "action-2", Kind: ActionImplement}, Outcome: &Outcome{Summary: "second outcome", KnownIssues: []string{"earlier issue"}}},
			{Action: Action{ActionID: "action-3", Kind: ActionReview}, Outcome: &Outcome{Summary: "most recent outcome", KnownIssues: []string{"recent issue one", "recent issue two"}}},
		},
	}

	context, err := buildContext(run)
	require.NoError(t, err)
	expected := "Decisions:\n" +
		"- action decision-1 decision:\n  first line\n  second line\n" +
		"- action decision-2 decision:\n  latest active decision\n" +
		"Recent outcome:\n" +
		"- review action action-3: most recent outcome\n" +
		"  Known issues:\n  - recent issue one\n  - recent issue two\n" +
		"Earlier outcomes:\n" +
		"- orient action action-1: first outcome\n" +
		"- implement action action-2: second outcome\n" +
		"  Known issues:\n  - earlier issue\n"
	assert.Equal(t, expected, context)
	assert.True(t, utf8.ValidString(context))
	assert.NotContains(t, context, "\r")
	assert.NotContains(t, context, run.Goal)
	assert.NotContains(t, context, run.PinnedSource.AcceptedRequirements.RequirementsMarkdown)
	assert.NotContains(t, context, "old source decision")
	assert.NotContains(t, context, "overturned decision")
	assert.NotContains(t, context, "structural-confirmation")
	assert.NotContains(t, context, "confirmed in host UI")
	assert.Contains(t, context, "first line\n  second line")
	assert.Contains(t, context, "latest active decision")

	decisionsIndex := strings.Index(context, "Decisions:")
	recentIndex := strings.Index(context, "Recent outcome:")
	earlierIndex := strings.Index(context, "Earlier outcomes:")
	assert.Less(t, decisionsIndex, recentIndex)
	assert.Less(t, recentIndex, earlierIndex)
	assert.Less(t, strings.Index(context, "decision-1"), strings.Index(context, "decision-2"), "selected decisions render chronologically")
	assert.Contains(t, context[recentIndex:earlierIndex], "most recent outcome")
	assert.Contains(t, context[recentIndex:earlierIndex], "recent issue one")
	assert.Less(t, strings.Index(context, "first outcome"), strings.Index(context, "second outcome"), "earlier outcomes render chronologically")
	assert.Contains(t, context, "earlier issue")
}

func TestBuildContextPrioritizesLatestDecisionAndUsesExactUTF8TruncationMarker(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("x", maxActionContextBytes*2)
	run := Run{
		Answers: []AnswerRecord{
			{ActionID: "older", Text: "lower-priority decision", Active: true},
			{ActionID: "latest", Text: huge, Active: true},
		},
		Actions: []ActionRecord{
			{Action: Action{ActionID: "earlier", Kind: ActionOrient}, Outcome: &Outcome{Summary: "lower-priority earlier outcome", KnownIssues: []string{}}},
			{Action: Action{ActionID: "recent", Kind: ActionImplement}, Outcome: &Outcome{Summary: "lower-priority recent outcome", KnownIssues: []string{"lower-priority issue"}}},
		},
	}

	context, err := buildContext(run)
	require.NoError(t, err)
	assert.Len(t, []byte(context), maxActionContextBytes)
	assert.True(t, utf8.ValidString(context))
	assert.Contains(t, context, "action latest decision")
	assert.NotContains(t, context, "lower-priority decision")
	assert.NotContains(t, context, "lower-priority recent outcome")
	assert.NotContains(t, context, "lower-priority earlier outcome")
	assert.Contains(t, context, "[omitted decisions: 1]")
	assert.Contains(t, context, "[omitted recent outcomes: 1]")
	assert.Contains(t, context, "[omitted earlier outcomes: 1]")

	original := fmt.Sprintf("- action latest decision:\n  %s\n", huge)
	digest := sha256.Sum256([]byte(original))
	expectedMarker := fmt.Sprintf("...[truncated original_bytes=%d sha256=%x]", len(original), digest)
	assert.Contains(t, context, expectedMarker)
	assert.NotContains(t, context, "\ufffd")
}

func TestBuildContextTruncatesLongMultilingualTextAtCodePointBoundary(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("界🙂\r\n", maxActionContextBytes)
	run := Run{Answers: []AnswerRecord{{ActionID: "multilingual", Text: huge, Active: true}}}
	context, err := buildContext(run)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(context), maxActionContextBytes)
	assert.GreaterOrEqual(t, len(context), maxActionContextBytes-3)
	assert.True(t, utf8.ValidString(context))
	assert.NotContains(t, context, "\r")

	normalizedText := strings.ReplaceAll(huge, "\r\n", "\n")
	indentedText := "  " + strings.ReplaceAll(normalizedText, "\n", "\n  ")
	original := fmt.Sprintf("- action multilingual decision:\n%s\n", indentedText)
	digest := sha256.Sum256([]byte(original))
	marker := fmt.Sprintf("...[truncated original_bytes=%d sha256=%x]", len(original), digest)
	assert.Contains(t, context, marker)
}

func TestBuildContextRejectsInvalidUTF8Candidate(t *testing.T) {
	t.Parallel()
	run := Run{Answers: []AnswerRecord{{ActionID: "decision", Text: string([]byte{0xff}), Active: true}}}
	_, err := buildContext(run)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "valid utf-8")
}

func TestMarkAnswerSupersededTargetsOneExplicitPriorDecision(t *testing.T) {
	t.Parallel()
	run := Run{Answers: []AnswerRecord{
		{ActionID: "prior", Text: "old decision", Active: true},
		{ActionID: "independent", Text: "independent decision", Active: true},
	}}
	assert.True(t, markAnswerSuperseded(&run, "prior", "replacement"))
	assert.False(t, run.Answers[0].Active)
	assert.Equal(t, "replacement", run.Answers[0].SupersededBy)
	assert.True(t, run.Answers[1].Active)
	assert.False(t, markAnswerSuperseded(&run, "prior", "another"), "an inactive decision cannot be superseded twice")
}

func TestMarkActiveAnswersSupersededIsDeterministicAndContextExcludesThem(t *testing.T) {
	t.Parallel()
	revision := "sha256:" + strings.Repeat("c", 64)
	run := Run{
		PinnedSource: &PinnedSource{RequirementsRevision: revision},
		Answers: []AnswerRecord{
			{ActionID: "one", Text: "first superseded", Active: true, RequirementsRevision: revision},
			{ActionID: "two", Text: "second superseded", Active: true, RequirementsRevision: revision},
			{ActionID: "other", Text: "other revision", Active: true, RequirementsRevision: "sha256:" + strings.Repeat("d", 64)},
		},
	}
	markActiveAnswersSuperseded(&run, revision, "replacement-action")
	assert.False(t, run.Answers[0].Active)
	assert.False(t, run.Answers[1].Active)
	assert.Equal(t, "replacement-action", run.Answers[0].SupersededBy)
	assert.True(t, run.Answers[2].Active)

	context, err := buildContext(run)
	require.NoError(t, err)
	assert.NotContains(t, context, "first superseded")
	assert.NotContains(t, context, "second superseded")
	assert.NotContains(t, context, "other revision")
}

func TestContextIsByteIdenticalAfterJournalReplay(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startTestRun(t, service, 8, false)
	run = submitCurrent(t, service, run, Outcome{Status: OutcomeCompleted, Summary: "repository facts", KnownIssues: []string{"known fact"}})
	require.NotNil(t, run.CurrentAction)

	loaded, err := service.Load(run.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded.CurrentAction)
	assert.Equal(t, run.CurrentAction.Context, loaded.CurrentAction.Context)
	fromProjection, err := buildContext(run)
	require.NoError(t, err)
	fromReplay, err := buildContext(loaded)
	require.NoError(t, err)
	assert.Equal(t, fromProjection, fromReplay)
}

func TestUntruncatedRequirementsStillReturnActionTooLarge(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	envelope := validSourceEnvelope()
	envelope.Body = strings.Replace(envelope.Body, "- Keep order.", "- "+strings.Repeat("r", 60<<10), 1)
	source := mustParseSource(t, envelope)
	run, err := service.Start(strings.Repeat("g", 100<<10), CreateOptions{Budget: 4, ReviewEnabled: false, PinnedSource: &source})
	require.NoError(t, err)

	outcome := withEnvelope(run.CurrentAction.ActionID, run.CurrentAction.Kind, Outcome{Status: OutcomeCompleted, Summary: strings.Repeat("s", 110<<10)})
	_, err = service.Submit(run.ID, run.CurrentAction.ActionID, outcome)
	assertProtocolError(t, err, "action_too_large")

	unchanged, loadErr := service.Load(run.ID)
	require.NoError(t, loadErr)
	require.NotNil(t, unchanged.CurrentAction)
	assert.Equal(t, run.CurrentAction.ActionID, unchanged.CurrentAction.ActionID)
	assert.Nil(t, findActionRecord(&unchanged, run.CurrentAction.ActionID).Outcome)
}
