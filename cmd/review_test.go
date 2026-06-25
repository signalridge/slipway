package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasIntentDriftSignal_BlockerExactMatch(t *testing.T) {
	t.Parallel()
	empty := model.VerificationRecord{}

	assert.True(t, hasIntentDriftSignal(model.ReasonCodesFromSpecs([]string{"new_change_required:intent_conflict"}), empty))
	assert.True(t, hasIntentDriftSignal(model.ReasonCodesFromSpecs([]string{"intent_drift"}), empty))
	assert.True(t, hasIntentDriftSignal(model.ReasonCodesFromSpecs([]string{"intent_drift:severe"}), empty))
}

func TestHasIntentDriftSignal_BlockerSubstringRejected(t *testing.T) {
	t.Parallel()
	empty := model.VerificationRecord{}

	// Loose substrings that previously matched should NOT match now.
	assert.False(t, hasIntentDriftSignal(model.ReasonCodesFromSpecs([]string{"some_intent_drift_thing"}), empty))
	assert.False(t, hasIntentDriftSignal(model.ReasonCodesFromSpecs([]string{"no_intent_drift_here"}), empty))
	assert.False(t, hasIntentDriftSignal(model.ReasonCodesFromSpecs([]string{"prefix_intent_drift"}), empty))
}

func TestHasIntentDriftSignal_ReferenceMatch(t *testing.T) {
	t.Parallel()
	record := model.VerificationRecord{
		References: []string{"intent_drift:true"},
	}
	assert.True(t, hasIntentDriftSignal(nil, record))

	record.References = []string{"intent_drift:yes"}
	assert.True(t, hasIntentDriftSignal(nil, record))
}

func TestHasIntentDriftSignal_ReferenceNoMatch(t *testing.T) {
	t.Parallel()
	record := model.VerificationRecord{
		References: []string{"intent_drift:false"},
	}
	assert.False(t, hasIntentDriftSignal(nil, record))

	record.References = []string{"some_other_ref"}
	assert.False(t, hasIntentDriftSignal(nil, record))
}

func TestHasIntentDriftSignal_EmptyInputs(t *testing.T) {
	t.Parallel()
	assert.False(t, hasIntentDriftSignal(nil, model.VerificationRecord{}))
	assert.False(t, hasIntentDriftSignal([]model.ReasonCode{}, model.VerificationRecord{}))
}

func TestClassifyReviewGapsSeparatesArtifactAndCodeBlockers(t *testing.T) {
	t.Parallel()

	gaps := classifyReviewGaps([]model.ReasonCode{
		model.NewReasonCode("task_blockers", "task-a"),
		model.NewReasonCode("required_skill_missing", "spec-compliance-review"),
	})
	require.NotNil(t, gaps)
	assert.Equal(t, []string{"task_blockers:task-a"}, gaps.CodeToArtifact)
	assert.Equal(t, []string{"required_skill_missing:spec-compliance-review"}, gaps.ArtifactToCode)
}

func TestReviewRejectsMutuallyExclusiveFlags(t *testing.T) {
	t.Parallel()

	err := func() error {
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--all", "--changed-only"})
		return cmd.Execute()
	}()
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "mutually_exclusive_flags", cliErr.ErrorCode)
	assert.Equal(t, categoryInvalidUsage, cliErr.Category)
	assert.Equal(t, exitCodeInvalidUsage, cliErr.ExitCode)
}

func TestReviewAllowsAllWhenChangedOnlyIsExplicitlyFalse(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--all", "--changed-only=false", "--json"})

		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "no_active_change", cliErr.ErrorCode)
		assert.NotEqual(t, "mutually_exclusive_flags", cliErr.ErrorCode)
	})
}

func TestReviewHelpDoesNotExposeArtifactFlag(t *testing.T) {
	t.Parallel()

	cmd := makeReviewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.Help())
	assert.NotContains(t, out.String(), "--artifact")
}

func TestReviewRejectsHydrateWithJSONWithoutMutatingState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, true))

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review hydrate/json rejection must be side-effect free")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeShipReadyGovernedBundle(t, root, change)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)

		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--hydrate", "--change", slug})
		err = cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "mutually_exclusive_flags", cliErr.ErrorCode)

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS2Implement, after.CurrentState, "invalid hydrate/json request must not advance review state")
		assert.Zero(t, after.ReviewIntentDriftFailures, "invalid hydrate/json request must not mutate review counters")
	})
}

func TestReviewRejectsUnexpectedArgs(t *testing.T) {
	t.Parallel()

	err := func() error {
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"unexpected"})
		return cmd.Execute()
	}()
	require.Error(t, err)
	assertUnexpectedArgError(t, err)
}

func TestEnsureReviewEntryStateRequiresRunSummary(t *testing.T) {
	t.Parallel()

	err := ensureReviewEntryState(model.StateS3Review, nil)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "missing_run_summary", cliErr.ErrorCode)
	assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
	assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
}

func TestEnsureReviewEntryStateAcceptsSummaryLevelBlockersWithoutTasks(t *testing.T) {
	t.Parallel()

	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictFail,
		OpenBlockers:      model.ReasonCodesFromSpecs([]string{"session_isolation_warning:session_id=abc:shared_by=task-a,task-b"}),
	}

	require.NoError(t, ensureReviewEntryState(model.StateS3Review, summary))
}

func TestEnsureReviewEntryStateRejectsS2Implement(t *testing.T) {
	t.Parallel()

	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "task-a",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: time.Now().UTC(),
		}},
	}

	err := ensureReviewEntryState(model.StateS2Implement, summary)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "review_state_invalid", cliErr.ErrorCode)
	assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
	assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
	require.NotNil(t, cliErr.Details)
	assert.Equal(t, "slipway implement", cliErr.Details["next_command"])
}

func TestEnsureReviewEntryStateRejectsEarlierState(t *testing.T) {
	t.Parallel()

	err := ensureReviewEntryState(model.StateS1Plan, nil)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "review_state_invalid", cliErr.ErrorCode)
	assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
	assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
}

func TestReviewExplicitRequestRejectsInactiveGovernedChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review inactive governed change")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.Status = model.ChangeStatusCancelled
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		err = cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "not_active", cliErr.ErrorCode)
		assert.Equal(t, categoryPrecondition, cliErr.Category)
		assert.Equal(t, exitCodePrecondition, cliErr.ExitCode)
	})
}

func TestReviewRejectsS2ImplementWithoutMutatingState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review should not advance S2")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		err = cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "review_state_invalid", cliErr.ErrorCode)
		assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
		require.NotNil(t, cliErr.Details)
		assert.Equal(t, "slipway implement", cliErr.Details["next_command"])

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS2Implement, after.CurrentState)
	})
}

func TestReviewDiagnosticsFlagEmitsJSON(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review diagnostics json")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeShipReadyGovernedBundle(t, root, change)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--diagnostics", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, string(model.StateS3Review), view.CurrentState)
	})
}

func TestReviewRequiresExecutionSummaryEvenWhenChecklistIsComplete(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review requires frozen execution summary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [x] `+"`t-01`"+` checked checklist must not unlock review
  - depends_on: []
  - target_files: ["cmd/review.go"]
  - task_kind: verification
  - covers: [REQ-001]
`), 0o644))

		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		err = cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "missing_run_summary", cliErr.ErrorCode)
		assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
		assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
	})
}

func TestReviewPassFromS3ReviewPreservesGovernedState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review should preserve governed done-ready state")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.Artifacts = map[string]model.ArtifactState{}
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` preserve review contract
  - depends_on: []
  - target_files: ["cmd/review.go"]
  - task_kind: verification
  - covers: [REQ-001]
`), 0o644))
		specPath := artifact.ResolveArtifactPath(bundlePath, "requirements.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(specPath), 0o755))
		require.NoError(t, os.WriteFile(specPath, []byte(`## Requirements

### Requirement: ReviewContract

REQ-001: The system MUST preserve governed review-state when review prerequisites remain valid.

#### Scenario: Review-state preserved on valid review
GIVEN a governed change at S3_REVIEW with valid review prerequisites
WHEN the review runs
THEN the governed review-state is preserved.
`), 0o644))

		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingSelectedReviewEvidencePack(t, root, slug, 1)
		writePassingShipVerificationEvidence(t, root, slug, 1)

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "pass", view.Verdict, model.ReasonSpecs(view.Blockers))
		assert.Equal(t, string(model.StateS3Review), view.CurrentState)
		assert.ElementsMatch(t, []string{
			progression.SkillSpecComplianceReview,
			progression.SkillCodeQualityReview,
			progression.SkillIndependentReview,
		}, view.SelectedReviewSkills)

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS3Review, change.CurrentState)
	})
}

func writePassingSelectedReviewEvidencePack(t *testing.T, root, slug string, runSummaryVersion int) {
	t.Helper()
	reviewStampedAt := time.Now().UTC()
	writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  reviewStampedAt,
		RunVersion: runSummaryVersion,
		References: []string{
			"layer:R0=pass",
			"layer:R3=pass",
			model.ContextOriginReferencePrefix + model.StageContextReview + "=review-spec",
		},
	})
	writeSkillVerification(t, root, slug, progression.SkillCodeQualityReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  reviewStampedAt,
		RunVersion: runSummaryVersion,
		References: []string{
			"layer:IR1=pass",
			"layer:IR3=pass",
			"layer:QUALITY=pass",
			model.ContextOriginReferencePrefix + model.StageContextReview + "=review-code",
		},
	})
	writeSkillVerification(t, root, slug, progression.SkillIndependentReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  reviewStampedAt,
		RunVersion: runSummaryVersion,
		References: []string{
			"independent-review:pass",
			model.ContextOriginReferencePrefix + model.StageContextReview + "=review-independent",
		},
	})
	refreshPassingSkillDigestsForTest(
		t,
		root,
		slug,
		progression.SkillSpecComplianceReview,
		progression.SkillCodeQualityReview,
		progression.SkillIndependentReview,
	)
}

func TestReviewRequiresStoredWaveRunsForExecutionSummary(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review should use execution summary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.Artifacts = map[string]model.ArtifactState{}
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.MkdirAll(bundlePath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [x] `+"`t-01`"+` preserve review contract
  - depends_on: []
  - target_files: ["cmd/review.go"]
  - task_kind: verification
  - covers: [REQ-001]
`), 0o644))
		specPath := artifact.ResolveArtifactPath(bundlePath, "requirements.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(specPath), 0o755))
		require.NoError(t, os.WriteFile(specPath, []byte(`## Requirements

### Requirement: ReviewContract

REQ-001: The system MUST preserve governed review-state when review prerequisites remain valid.

#### Scenario: Review-state preserved on valid review
GIVEN a governed change at S3_REVIEW with valid review prerequisites
WHEN the review runs
THEN the governed review-state is preserved.
`), 0o644))

		now := time.Now().UTC()
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  now,
			RunVersion: 1,
		})
		writeSkillVerification(t, root, slug, "spec-compliance-review", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  now.Add(time.Second),
			RunVersion: 1,
			References: []string{"layer:R0=pass", "layer:IR1=pass"},
		})
		writeSkillVerification(t, root, slug, "code-quality-review", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  now.Add(2 * time.Second),
			RunVersion: 1,
		})

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		cmd.SetOut(&out)
		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "wave_runs_missing", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
	})
}

func TestReviewFailsClosedOnWaveRunsMissingEvenWhenReadinessIsAlreadyBlocked(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review should fail closed when wave runs are missing")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.Artifacts = map[string]model.ArtifactState{}
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.MkdirAll(bundlePath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [x] `+"`t-01`"+` preserve review contract
  - depends_on: []
  - target_files: ["cmd/review.go"]
  - task_kind: verification
  - covers: [REQ-001]
`), 0o644))
		specPath := artifact.ResolveArtifactPath(bundlePath, "requirements.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(specPath), 0o755))
		require.NoError(t, os.WriteFile(specPath, []byte(`## Requirements

### Requirement: ReviewContract

REQ-001: The system MUST preserve governed review-state when review prerequisites remain valid.

#### Scenario: Review-state preserved on valid review
GIVEN a governed change at S3_REVIEW with valid review prerequisites
WHEN the review runs
THEN the governed review-state is preserved.
`), 0o644))

		now := time.Now().UTC()
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  now,
			RunVersion: 1,
		})
		// Intentionally omit spec-compliance-review so readiness is already blocked.

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		cmd.SetOut(&out)
		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "wave_runs_missing", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
	})
}

func TestReviewFailsWhenWaveTaskLinkageIsMismatched(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "review should reject mismatched wave linkage")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [x] `+"`t-01`"+` preserve first review wave
  - depends_on: []
  - target_files: ["cmd/review.go"]
  - task_kind: verification
  - covers: [REQ-001]

- [x] `+"`t-02`"+` preserve second review wave
  - depends_on: ["t-01"]
  - target_files: ["cmd/review.go"]
  - task_kind: verification
`), 0o644))

	now := time.Now().UTC()
	require.NoError(t, state.SaveExecutionSummary(root, slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        now,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01", "t-02"},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:       "t-01",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindVerification,
				ChangedFiles: []string{"cmd/review.go"},
				CapturedAt:   now,
			},
			{
				TaskID:       "t-02",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindVerification,
				ChangedFiles: []string{"cmd/review.go"},
				CapturedAt:   now.Add(time.Second),
			},
		},
	}))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	require.NoError(t, state.SaveWaveRuns(root, slug, 1, []model.WaveRun{
		{
			WaveIndex:         1,
			RunSummaryVersion: 1,
			TaskRuns: []model.TaskRunRef{{
				TaskID:            "t-02",
				RunSummaryVersion: 1,
			}},
			Verdict: model.WaveVerdictPass,
		},
		{
			WaveIndex:         2,
			RunSummaryVersion: 1,
			TaskRuns: []model.TaskRunRef{{
				TaskID:            "t-01",
				RunSummaryVersion: 1,
			}},
			Verdict: model.WaveVerdictPass,
		},
	}))

	change, err = state.LoadChange(root, slug)
	require.NoError(t, err)
	execCtx, err := loadExecutionContext(root, change)
	require.NoError(t, err)

	_, err = loadAuthoritativeWaveExecution(root, change, execCtx.LatestRunVersion, "review")
	require.Error(t, err)

	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "wave_task_linkage_mismatch", cliErr.ErrorCode)
	assert.Equal(t, categoryStateIntegrity, cliErr.Category)
}

func TestReviewDoesNotPreGateOnStaleExecutionEvidence(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review should fail on stale evidence")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.Artifacts = map[string]model.ArtifactState{}
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))

		// Write execution summary with CapturedAt = now.
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		materializeWaveExecutionForSummary(t, root, slug)

		// Change the semantic task plan after the summary to make planning evidence stale.
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` review should fail on stale evidence
  - depends_on: []
  - target_files: ["cmd/review.go"]
  - task_kind: verification
  - covers: [REQ-001]
`), 0o644))

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "fail", view.Verdict)
		assert.NotContains(t, model.ReasonSpecs(view.Blockers), "stale_planning_evidence")
		require.NotEmpty(t, view.Waves, "review should still surface wave status on blocked paths when wave execution data is available")
		assert.Equal(t, "pass", view.Waves[0].Verdict)
	})
}

func TestReviewAllDoesNotTreatImplementationEvidenceAsArtifactEvidence(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review should keep review evidence roles named")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.GuardrailDomain = string(model.GuardrailDomainExternalAPIContracts)
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))

		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		materializeWaveExecutionForSummary(t, root, slug)
		writeSkillVerification(t, root, slug, progression.SkillCodeQualityReview, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
			References: []string{"layer:IR1=pass", "layer:IR3=pass"},
		})

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--all", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		specs := model.ReasonSpecs(view.Blockers)
		assert.Contains(t, specs, "required_skill_missing:spec-compliance-review")
		assert.NotContains(t, specs, "review_layer_missing:R0")
		assert.NotContains(t, specs, "review_layer_missing:R3")
	})
}

func TestReviewChangedOnlyUsesInMemoryArtifactReconcile(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review changed-only should follow stale artifact projection")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.GuardrailDomain = string(model.GuardrailDomainAuthAuthZ)
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "intent.md", []byte(`# Intent
INT-001: protect auth review scope
## Open Questions
(none)
`)))
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "requirements.md", []byte(`# Requirements
### Requirement: AuthReview
REQ-001: Changed auth artifacts must trigger guardrail review depth.
`)))
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "decision.md", []byte(`# Decision
## Alternatives Considered
### Option A
Keep the current auth flow.
### Option B
Adjust the auth flow with explicit review coverage.

## Selected Approach
Option B.

## Interfaces and Data Flow
Auth entrypoints preserve the MFA contract.

## Rollout and Rollback
Roll forward behind verification and roll back by restoring the prior auth handler.

## Risk
Auth regressions require guardrail review.
`)))
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` verify review scope
  - depends_on: []
  - target_files: ["cmd/review.go"]
  - task_kind: verification
  - covers: [REQ-001]
`)))
		writeAssuranceMD(t, root, slug, validAssuranceContent())

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		_, reconcileErr := artifact.ReconcileFromFilesystem(root, &change)
		require.NoError(t, reconcileErr)
		for id, artState := range change.Artifacts {
			artState.State = model.ArtifactLifecycleApproved
			change.Artifacts[id] = artState
		}
		require.NoError(t, state.SaveChange(root, change))

		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "intent.md", []byte(`# Intent
INT-001: protect auth review scope
## Open Questions
How do we prove changed-only review still sees stale downstream artifacts?
`)))

		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		materializeWaveExecutionForSummary(t, root, slug)
		writeSkillVerification(t, root, slug, "spec-compliance-review", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
			References: []string{"layer:R0=pass", "layer:IR1=pass", "layer:IR3=pass"},
		})
		writeSkillVerification(t, root, slug, "code-quality-review", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC().Add(time.Second),
			RunVersion: 1,
		})

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "fail", view.Verdict)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "review_layer_missing:R3")
	})
}

func TestReviewChangedOnlyIncludesNonRequiredRuntimeArtifacts(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review changed-only should include non-required runtime artifacts")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.GuardrailDomain = string(model.GuardrailDomainAuthAuthZ)
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "intent.md", []byte(`# Intent
INT-001: protect auth review scope
## Open Questions
(none)
`)))
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "requirements.md", []byte(`# Requirements
### Requirement: AuthReview
REQ-001: Changed governed artifacts must keep auth review depth aligned.
`)))
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "decision.md", []byte(`# Decision
## Alternatives Considered
### Option A
Ignore optional runtime artifacts during review.
### Option B
Project them into changed-only review scope.

## Selected Approach
Option B.

## Interfaces and Data Flow
Review scope is derived from runtime artifact projection.

## Rollout and Rollback
Roll forward by keeping changed-only review aligned with runtime state.

## Risk
Missing optional artifacts can silently weaken guardrail review coverage.
`)))
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` verify optional review scope
  - depends_on: []
  - target_files: ["cmd/review.go"]
  - task_kind: verification
  - covers: [REQ-001]
`)))
		writeAssuranceMD(t, root, slug, validAssuranceContent())

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		_, reconcileErr := artifact.ReconcileFromFilesystem(root, &change)
		require.NoError(t, reconcileErr)
		for id, artState := range change.Artifacts {
			artState.State = model.ArtifactLifecycleApproved
			change.Artifacts[id] = artState
		}
		change.Artifacts["notes"] = model.ArtifactState{
			ID:    "notes",
			Path:  filepath.Join(bundlePath, "notes.md"),
			State: model.ArtifactLifecycleStale,
		}
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "notes.md", []byte(`# Notes
Auth-specific follow-up still needs review.
`)))

		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		materializeWaveExecutionForSummary(t, root, slug)
		writeSkillVerification(t, root, slug, "spec-compliance-review", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
			References: []string{"layer:R0=pass", "layer:IR1=pass", "layer:IR3=pass"},
		})
		writeSkillVerification(t, root, slug, "code-quality-review", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC().Add(time.Second),
			RunVersion: 1,
		})

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--changed-only", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "fail", view.Verdict)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "review_layer_missing:R3")
	})
}

func TestReviewFailsWhenTasksChecklistCoverageDrifts(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review should fail when requirement coverage drifts")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.Artifacts = map[string]model.ArtifactState{}
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement auth
  - depends_on: []
  - target_files: ["cmd/review.go"]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))
		specPath := artifact.ResolveArtifactPath(bundlePath, "requirements.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(specPath), 0o755))
		require.NoError(t, os.WriteFile(specPath, []byte(`## Requirements

### Requirement: Auth

REQ-001: The system must authenticate requests.

### Requirement: Logging

REQ-002: The system must emit audit logs.
`), 0o644))

		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		materializeWaveExecutionForSummary(t, root, slug)
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "fail", view.Verdict)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "plan_dimension_coverage_missing_requirement:REQ-002")

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS3Review, change.CurrentState)
	})
}

func TestEvaluateReviewVerdictRejectsEmptyExecutionSummaryWithoutChecklistFallback(t *testing.T) {
	t.Parallel()

	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		Tasks:             []model.ExecutionTaskSummary{},
	}
	verdict, blockers, _ := evaluateReviewVerdict(executionContext{
		Summary:          summary,
		LatestRunVersion: summary.RunSummaryVersion,
		Ready:            false, // empty tasks → not ready
	}, nil)

	assert.Equal(t, "fail", verdict)
	assert.Contains(t, model.ReasonSpecs(blockers), "missing_run_summary")
}

func TestEvaluateReviewVerdictRejectsNilSummary(t *testing.T) {
	t.Parallel()

	verdict, blockers, _ := evaluateReviewVerdict(executionContext{}, nil)

	assert.Equal(t, "fail", verdict)
	assert.Contains(t, model.ReasonSpecs(blockers), "missing_run_summary")
}

func TestEvaluateReviewVerdictSurfacesSummaryLevelBlockers(t *testing.T) {
	t.Parallel()

	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictFail,
		CompletedTasks:    []string{"task-a"},
		OpenBlockers:      model.ReasonCodesFromSpecs([]string{"session_isolation_warning:session_id=abc:shared_by=task-a,task-b"}),
		Tasks: []model.ExecutionTaskSummary{
			{TaskID: "task-a", Verdict: model.TaskVerdictPass, TaskKind: model.TaskKindCode, CapturedAt: time.Now().UTC()},
		},
	}
	verdict, blockers, _ := evaluateReviewVerdict(executionContext{
		Summary:          summary,
		LatestRunVersion: 1,
		Ready:            true,
		SummaryBlockers:  summary.OpenBlockers,
	}, nil)

	assert.Equal(t, "fail", verdict)
	assert.Contains(t, model.ReasonSpecs(blockers), "session_isolation_warning:session_id=abc:shared_by=task-a,task-b")
}

func TestEvaluateReviewVerdictSurfacesTaskBlockerByTaskID(t *testing.T) {
	t.Parallel()

	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictFail,
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:     "task-a",
				Verdict:    model.TaskVerdictPass,
				TaskKind:   model.TaskKindCode,
				Blockers:   []model.ReasonCode{model.NewReasonCode("lint_failed", "")},
				CapturedAt: time.Now().UTC(),
			},
		},
	}
	verdict, blockers, _ := evaluateReviewVerdict(executionContext{
		Summary:          summary,
		LatestRunVersion: 1,
		Ready:            true,
	}, nil)

	assert.Equal(t, "fail", verdict)
	assert.Contains(t, model.ReasonSpecs(blockers), "task_blockers:task-a")
}

func materializeWaveExecutionForSummary(t *testing.T, root, slug string) {
	t.Helper()

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)

	plan, err := state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	runs, err := state.BuildWaveRuns(plan, summary.RunSummaryVersion, summary.Tasks, nil)
	require.NoError(t, err)
	require.NoError(t, state.SaveWaveRuns(root, slug, summary.RunSummaryVersion, runs))
}
