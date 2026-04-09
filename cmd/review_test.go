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
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasIntentDriftSignal_BlockerExactMatch(t *testing.T) {
	t.Parallel()
	empty := model.VerificationRecord{}

	assert.True(t, hasIntentDriftSignal(model.ReasonCodesFromSpecs([]string{"pivot_required:intent_drift"}), empty))
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
		model.NewReasonCode("task_blockers", "task-a__rv1"),
		model.NewReasonCode("required_skill_missing", "spec-compliance-review"),
	})
	require.NotNil(t, gaps)
	assert.Equal(t, []string{"task_blockers:task-a__rv1"}, gaps.CodeToArtifact)
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

func TestReviewRejectsUnsupportedArtifactFlag(t *testing.T) {
	t.Parallel()

	err := func() error {
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--artifact", "artifacts/changes/example/requirements.md"})
		return cmd.Execute()
	}()
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "unsupported_flag", cliErr.ErrorCode)
	assert.Equal(t, categoryInvalidUsage, cliErr.Category)
	assert.Equal(t, exitCodeInvalidUsage, cliErr.ExitCode)
}

func TestEnsureReviewEntryStateRequiresRunSummary(t *testing.T) {
	t.Parallel()

	err := ensureReviewEntryState(model.StateS2Execute, nil)
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

	require.NoError(t, ensureReviewEntryState(model.StateS2Execute, summary))
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
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review inactive governed change")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.Status = model.ChangeStatusCancelled
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--change", slug})
		err = cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "not_active", cliErr.ErrorCode)
		assert.Equal(t, categoryPrecondition, cliErr.Category)
		assert.Equal(t, exitCodePrecondition, cliErr.ExitCode)
	})
}

func TestReviewRequiresExecutionSummaryEvenWhenChecklistIsComplete(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review requires frozen execution summary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [x] `+"`t-01`"+` checked checklist must not unlock review
  - depends_on: []
  - target_files: ["cmd/review.go"]
  - task_kind: verification
  - covers: [REQ-001]
`), 0o644))

		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--change", slug})
		err = cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "missing_run_summary", cliErr.ErrorCode)
		assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
		assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
	})
}

func TestReviewPassFromS7VerifyPreservesGovernedState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review should preserve governed done-ready state")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		change.Artifacts = map[string]model.ArtifactState{}
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` preserve review contract
  - depends_on: []
  - target_files: ["cmd/review.go"]
  - task_kind: verification
  - covers: [REQ-001]
`), 0o644))
		specPath := artifact.ResolveArtifactPath(bundlePath, slug, "requirements.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(specPath), 0o755))
		require.NoError(t, os.WriteFile(specPath, []byte(`## Requirements

### Requirement: ReviewContract

REQ-001: The system must preserve governed verify-state when review prerequisites remain valid.
`), 0o644))

		now := time.Now().UTC()
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        now,
			OverallVerdict:    model.ExecutionVerdictPass,
			CompletedTasks:    []string{"t-01"},
			Tasks: []model.ExecutionTaskSummary{
				{
					TaskID:       "t-01",
					Verdict:      model.TaskVerdictPass,
					TaskKind:     model.TaskKindVerification,
					ChangedFiles: []string{"cmd/review.go"},
					CapturedAt:   now,
				},
			},
		})

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
		cmd.SetArgs([]string{"--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "pass", view.Verdict)
		assert.Equal(t, string(model.StateS4Verify), view.CurrentState)

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS4Verify, change.CurrentState)
	})
}

func TestReviewPassesFromExecutionSummaryWithoutStoredRuns(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review should use execution summary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
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
		specPath := artifact.ResolveArtifactPath(bundlePath, slug, "requirements.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(specPath), 0o755))
		require.NoError(t, os.WriteFile(specPath, []byte(`## Requirements

### Requirement: ReviewContract

REQ-001: The system must preserve governed verify-state when review prerequisites remain valid.
`), 0o644))

		now := time.Now().UTC()
		require.NoError(t, state.SaveExecutionSummary(root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        now,
			OverallVerdict:    model.ExecutionVerdictPass,
			CompletedTasks:    []string{"t-01"},
			Tasks: []model.ExecutionTaskSummary{
				{
					TaskID:       "t-01",
					Verdict:      model.TaskVerdictPass,
					TaskKind:     model.TaskKindVerification,
					ChangedFiles: []string{"cmd/review.go"},
					CapturedAt:   now,
				},
			},
		}))

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
		cmd.SetArgs([]string{"--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "pass", view.Verdict)
		assert.Equal(t, string(model.StateS4Verify), view.CurrentState)
	})
}

func TestReviewFailsWhenExecutionEvidenceIsStale(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review should fail on stale evidence")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.Artifacts = map[string]model.ArtifactState{}
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

		// Write execution summary with CapturedAt = now.
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		// Write a bundle artifact AFTER the summary to make evidence stale.
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "intent.md"), []byte("# Intent (modified after execution)\n\nUpdated intent."), 0o644))

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "fail", view.Verdict)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "stale_execution_evidence")
	})
}

func TestReviewChangedOnlyUsesInMemoryArtifactReconcile(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review changed-only should follow stale artifact projection")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
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
		require.NoError(t, artifact.ReconcileFromFilesystem(root, &change))
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
		cmd.SetArgs([]string{"--change", slug})
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
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review changed-only should include non-required runtime artifacts")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
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
		require.NoError(t, artifact.ReconcileFromFilesystem(root, &change))
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
		cmd.SetArgs([]string{"--changed-only", "--change", slug})
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
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review should fail when requirement coverage drifts")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
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
		specPath := artifact.ResolveArtifactPath(bundlePath, slug, "requirements.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(specPath), 0o755))
		require.NoError(t, os.WriteFile(specPath, []byte(`## Requirements

### Requirement: Auth

REQ-001: The system must authenticate requests.

### Requirement: Logging

REQ-002: The system must emit audit logs.
`), 0o644))

		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "fail", view.Verdict)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "plan_dimension_coverage_missing_requirement:REQ-002")

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS2Execute, change.CurrentState)
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
	verdict, blockers := evaluateReviewVerdict(executionContext{
		Summary:          summary,
		LatestRunVersion: summary.RunSummaryVersion,
		Ready:            false, // empty tasks → not ready
	})

	assert.Equal(t, "fail", verdict)
	assert.Contains(t, model.ReasonSpecs(blockers), "missing_run_summary")
}

func TestEvaluateReviewVerdictRejectsNilSummary(t *testing.T) {
	t.Parallel()

	verdict, blockers := evaluateReviewVerdict(executionContext{})

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
	verdict, blockers := evaluateReviewVerdict(executionContext{
		Summary:          summary,
		LatestRunVersion: 1,
		Ready:            true,
		SummaryBlockers:  summary.OpenBlockers,
	})

	assert.Equal(t, "fail", verdict)
	assert.Contains(t, model.ReasonSpecs(blockers), "session_isolation_warning:session_id=abc:shared_by=task-a,task-b")
}

func TestEvaluateReviewVerdictSurfacesInvalidTaskRunKey(t *testing.T) {
	t.Parallel()

	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictFail,
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:     "task-a__rvshadow",
				Verdict:    model.TaskVerdictPass,
				TaskKind:   model.TaskKindCode,
				Blockers:   []model.ReasonCode{model.NewReasonCode("lint_failed", "")},
				CapturedAt: time.Now().UTC(),
			},
		},
	}
	verdict, blockers := evaluateReviewVerdict(executionContext{
		Summary:          summary,
		LatestRunVersion: 1,
		Ready:            true,
	})

	assert.Equal(t, "fail", verdict)
	assert.Contains(t, model.ReasonSpecs(blockers), "task_blockers_invalid_key:task-a__rvshadow")
	assert.NotContains(t, model.ReasonSpecs(blockers), "task_blockers:task-a__rvshadow")
}
