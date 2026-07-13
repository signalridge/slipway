package autopilot

import (
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/runstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunEventReplayRejectsInvalidSourceTransitions(t *testing.T) {
	t.Run("manifest head installed without candidate choice", func(t *testing.T) {
		repository := newTestRepository(t)
		service := openTestService(t, repository)
		run := startIssueTestRun(t, service, 6)

		envelope := validSourceEnvelope()
		setEnvelopeSection(t, &envelope, "requirements", "\n# Requirements\n\n- Publish a replacement.\n")
		setEnvelopeParentRequirementsRevision(t, &envelope, run.PinnedSource.RequirementsRevision)
		candidate := sourceCandidateForTest(t, envelope)
		require.NotNil(t, candidate.Snapshot)

		record := run.Actions[len(run.Actions)-1]
		record.Voided = true
		delta := runDelta{
			EventVersion:     runEventVersion,
			ContractVersion:  ContractVersion,
			RunID:            run.ID,
			CurrentActionSet: true,
			ActionUpdates: []actionUpdate{{
				Index:  len(run.Actions) - 1,
				Record: record,
			}},
			PinnedSourceSet: true,
			PinnedSource:    candidate.Snapshot,
			UpdatedAt:       run.UpdatedAt.Add(time.Second),
		}
		event, err := runstore.NewEvent(ResumeOperationSourceAmended, delta)
		require.NoError(t, err)

		replayed := runBeforeMutation(run)
		err = applyRunEvent(&replayed, event)
		require.Error(t, err)
		assert.ErrorContains(t, err, "pinned manifest can change only by adopting its current candidate")
	})

	t.Run("candidate has stale parent", func(t *testing.T) {
		repository := newTestRepository(t)
		service := openTestService(t, repository)
		run := startIssueTestRun(t, service, 6)

		envelope := validSourceEnvelope()
		setEnvelopeSection(t, &envelope, "requirements", "\n# Requirements\n\n- Diverge from the pinned head.\n")
		setEnvelopeParentRequirementsRevision(
			t,
			&envelope,
			"sha256:"+strings.Repeat("0", 64),
		)
		input := sourceCandidateForTest(t, envelope)
		candidate := newSourceCandidate(input)
		result := ResumeResult{
			Operation:     ResumeOperationSourceCandidate,
			BudgetApplied: false,
			CandidateID:   candidate.CandidateID,
		}
		delta := runDelta{
			EventVersion:        runEventVersion,
			ContractVersion:     ContractVersion,
			RunID:               run.ID,
			SourceCandidateSet:  true,
			SourceCandidate:     &candidate,
			LastResumeResultSet: true,
			LastResumeResult:    &result,
			UpdatedAt:           run.UpdatedAt.Add(time.Second),
		}
		event, err := runstore.NewEvent(ResumeOperationSourceCandidate, delta)
		require.NoError(t, err)

		replayed := runBeforeMutation(run)
		err = applyRunEvent(&replayed, event)
		require.Error(t, err)
		assert.ErrorContains(t, err, "source candidate parent does not match pinned requirements")
	})

	t.Run("candidate edits accepted comment in place", func(t *testing.T) {
		repository := newTestRepository(t)
		service := openTestService(t, repository)
		run := startIssueTestRun(t, service, 6)

		envelope := validSourceEnvelope()
		for index := range envelope.Comments {
			if testSourceCommentKey(envelope.Comments[index].Body) == "requirements" {
				envelope.Comments[index].Body += "\nEdited in place.\n"
			}
		}
		rebuildSourceManifestBody(t, &envelope)
		setEnvelopeParentRequirementsRevision(t, &envelope, run.PinnedSource.RequirementsRevision)
		input := sourceCandidateForTest(t, envelope)
		candidate := newSourceCandidate(input)
		result := ResumeResult{
			Operation:     ResumeOperationSourceCandidate,
			BudgetApplied: false,
			CandidateID:   candidate.CandidateID,
		}
		delta := runDelta{
			EventVersion:        runEventVersion,
			ContractVersion:     ContractVersion,
			RunID:               run.ID,
			SourceCandidateSet:  true,
			SourceCandidate:     &candidate,
			LastResumeResultSet: true,
			LastResumeResult:    &result,
			UpdatedAt:           run.UpdatedAt.Add(time.Second),
		}
		event, err := runstore.NewEvent(ResumeOperationSourceCandidate, delta)
		require.NoError(t, err)

		replayed := runBeforeMutation(run)
		err = applyRunEvent(&replayed, event)
		require.Error(t, err)
		assert.ErrorContains(t, err, "was changed in place")
	})
}

func TestRunEventReplayRejectsMalformedResumeReceipts(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	base := startIssueTestRun(t, service, 6)

	tests := []struct {
		name      string
		eventType string
		result    ResumeResult
		want      string
	}{
		{
			name:      "unknown operation",
			eventType: ResumeOperationSourceRefreshed,
			result:    ResumeResult{Operation: "invented", BudgetApplied: true},
			want:      "unknown operation",
		},
		{
			name:      "candidate receipt without candidate",
			eventType: ResumeOperationSourceCandidate,
			result: ResumeResult{
				Operation:     ResumeOperationSourceCandidate,
				BudgetApplied: false,
				CandidateID:   "candidate-missing",
			},
			want: "source candidate receipt is inconsistent",
		},
		{
			name:      "refresh receipt with candidate id",
			eventType: ResumeOperationSourceRefreshed,
			result: ResumeResult{
				Operation:     ResumeOperationSourceRefreshed,
				BudgetApplied: true,
				CandidateID:   "candidate-unexpected",
			},
			want: "source refresh receipt is inconsistent",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := runBeforeMutation(base)
			delta := runDelta{
				EventVersion:        runEventVersion,
				ContractVersion:     ContractVersion,
				RunID:               run.ID,
				LastResumeResultSet: true,
				LastResumeResult:    &test.result,
				UpdatedAt:           run.UpdatedAt.Add(time.Second),
			}
			event, err := runstore.NewEvent(test.eventType, delta)
			require.NoError(t, err)

			err = applyRunEvent(&run, event)
			require.Error(t, err)
			assert.ErrorContains(t, err, test.want)
		})
	}
}

func TestRunEventReplayRejectsCurrentActionCatalogDrift(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 6)

	forgedAction := *run.CurrentAction
	forgedRequirements := *forgedAction.Requirements
	forgedRequirements.Sections = append(
		[]ActionRequirementSection(nil),
		forgedRequirements.Sections...,
	)
	forgedRequirements.Sections[0].MaterialSHA256 = "sha256:" + strings.Repeat("0", 64)
	forgedAction.Requirements = &forgedRequirements
	record := run.Actions[len(run.Actions)-1]
	record.Action = forgedAction
	delta := runDelta{
		EventVersion:     runEventVersion,
		ContractVersion:  ContractVersion,
		RunID:            run.ID,
		CurrentActionSet: true,
		CurrentAction:    &forgedAction,
		ActionUpdates: []actionUpdate{{
			Index:  len(run.Actions) - 1,
			Record: record,
		}},
		UpdatedAt: run.UpdatedAt.Add(time.Second),
	}
	event, err := runstore.NewEvent("action_skipped", delta)
	require.NoError(t, err)

	replayed := runBeforeMutation(run)
	err = applyRunEvent(&replayed, event)
	require.Error(t, err)
	assert.ErrorContains(t, err, "action record 0 rewrote its issued action")
}

func TestServiceRejectsEditingRetiredAcceptedCommentIdentity(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 8)

	firstAmendment := validSourceEnvelope()
	setEnvelopeSection(t, &firstAmendment, "requirements", "\n# Requirements\n\n- Use the first replacement identity.\n")
	setEnvelopeParentRequirementsRevision(t, &firstAmendment, run.PinnedSource.RequirementsRevision)
	firstCandidate := sourceCandidateForTest(t, firstAmendment)
	paused, err := service.Resume(run.ID, ResumeOptions{RefreshedSource: &firstCandidate})
	require.NoError(t, err)
	require.NotNil(t, paused.SourceCandidate)

	adopted, err := service.Resume(run.ID, ResumeOptions{
		SourceChoice: SourceChoiceAdopt,
		CandidateID:  paused.SourceCandidate.CandidateID,
	})
	require.NoError(t, err)
	require.NotNil(t, adopted.PinnedSource)

	reusedRetiredIdentity := validSourceEnvelope()
	for index := range reusedRetiredIdentity.Comments {
		if testSourceCommentKey(reusedRetiredIdentity.Comments[index].Body) == "requirements" {
			reusedRetiredIdentity.Comments[index].Body += "\nEdited after the identity was retired.\n"
		}
	}
	rebuildSourceManifestBody(t, &reusedRetiredIdentity)
	setEnvelopeParentRequirementsRevision(
		t,
		&reusedRetiredIdentity,
		adopted.PinnedSource.RequirementsRevision,
	)
	secondCandidate := sourceCandidateForTest(t, reusedRetiredIdentity)
	_, err = service.Resume(run.ID, ResumeOptions{RefreshedSource: &secondCandidate})
	assertProtocolError(t, err, "source_history_in_place_edit")

	unchanged, err := service.Load(run.ID)
	require.NoError(t, err)
	assert.Equal(t, adopted.PinnedSource.ManifestRevision, unchanged.PinnedSource.ManifestRevision)
	assert.Nil(t, unchanged.SourceCandidate)
}

func TestRunEventReplayRejectsProjectionChangeOutsideRefresh(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 6)

	envelope := validSourceEnvelope()
	envelope.IssueNumber = 77
	envelope.CanonicalURL = "https://github.com/signalridge/slipway/issues/77"
	for index := range envelope.Comments {
		envelope.Comments[index].URL = envelope.CanonicalURL + "#issuecomment-" + jsonNumber(envelope.Comments[index].DatabaseID)
	}
	refreshed := sourceCandidateForTest(t, envelope)
	require.NotNil(t, refreshed.Snapshot)
	assert.Equal(t, run.PinnedSource.ManifestRevision, refreshed.Snapshot.ManifestRevision)

	record := run.Actions[len(run.Actions)-1]
	record.Voided = true
	delta := runDelta{
		EventVersion:     runEventVersion,
		ContractVersion:  ContractVersion,
		RunID:            run.ID,
		CurrentActionSet: true,
		ActionUpdates: []actionUpdate{{
			Index:  len(run.Actions) - 1,
			Record: record,
		}},
		PinnedSourceSet: true,
		PinnedSource:    refreshed.Snapshot,
		UpdatedAt:       run.UpdatedAt.Add(time.Second),
	}
	event, err := runstore.NewEvent("action_skipped", delta)
	require.NoError(t, err)

	replayed := runBeforeMutation(run)
	err = applyRunEvent(&replayed, event)
	require.Error(t, err)
	assert.ErrorContains(t, err, "pinned source projection can change only in a source_refreshed event")
}

func TestCandidateResolutionRequiresFreshOrientRecord(t *testing.T) {
	repository := newTestRepository(t)
	service := openTestService(t, repository)
	run := startIssueTestRun(t, service, 6)

	envelope := validSourceEnvelope()
	setEnvelopeSection(t, &envelope, "requirements", "\n# Requirements\n\n- Create a candidate.\n")
	setEnvelopeParentRequirementsRevision(t, &envelope, run.PinnedSource.RequirementsRevision)
	input := sourceCandidateForTest(t, envelope)
	paused, err := service.Resume(run.ID, ResumeOptions{RefreshedSource: &input})
	require.NoError(t, err)
	require.NotNil(t, paused.SourceCandidate)

	forged := runBeforeMutation(paused)
	forged.SourceCandidate = nil
	forged.State = RunActive
	forged.PauseReason = ""
	forged.Actions[0].Voided = false
	forged.CurrentAction = &forged.Actions[0].Action
	forged.LastResumeResult = &ResumeResult{
		Operation:     ResumeOperationSourcePinned,
		BudgetApplied: true,
		CandidateID:   paused.SourceCandidate.CandidateID,
	}
	forged.LastSourceChoice = &SourceChoiceReceipt{
		CandidateID:       paused.SourceCandidate.CandidateID,
		Choice:            SourceChoicePinned,
		ResultingActionID: forged.CurrentAction.ActionID,
		At:                time.Now().UTC(),
	}

	err = validateCandidateResolution(ResumeOperationSourcePinned, paused, forged)
	require.Error(t, err)
	assert.ErrorContains(t, err, "append exactly one fresh Orient action")
}
