package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateIncludesScopeContractDriftReport(t *testing.T) {
	t.Parallel()

	root, slug := writeScopeContractDriftFixture(t)

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	require.NotNil(t, view.ScopeContract)
	assert.Equal(t, "fail", view.ScopeContract.Status)
	assert.Equal(t, []string{"cmd/review.go"}, view.ScopeContract.OutOfScopeFiles)
	assert.NotContains(t, model.ReasonSpecs(view.Blockers), "scope_contract_drift:cmd/review.go")
}

func TestValidateAndNextTreatS3ScopeContractDriftAsReviewInput(t *testing.T) {
	t.Parallel()

	root, slug := writeScopeContractDriftFixtureInState(t, model.StateS3Review)

	validateView, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	assert.NotContains(t, model.ReasonSpecs(validateView.Blockers), "scope_contract_drift:cmd/review.go")
	diagnostics := strings.Join(validateView.Diagnostics, "\n")
	assert.Contains(t, diagnostics, "scope_contract_recovery_guidance")
	assert.Contains(t, diagnostics, "target_files in tasks.md")
	assert.Contains(t, diagnostics, "same-intent")
	assert.Contains(t, diagnostics, "scope amendment")
	assert.Contains(t, diagnostics, "S3 review")

	nextView, err := buildNextView(root, changeRef{Slug: slug}, "", true, false, false)
	require.NoError(t, err)
	assert.NotContains(t, model.ReasonSpecs(nextView.Blockers), "scope_contract_drift:cmd/review.go")
	require.NotNil(t, nextView.ReviewBatch)
	assert.Equal(t, "parallel", nextView.ReviewBatch.Mode)
	warnings := strings.Join(nextView.Warnings, "\n")
	assert.Contains(t, warnings, "scope_contract_recovery_guidance")
	assert.Contains(t, warnings, "target_files in tasks.md")
	assert.Contains(t, warnings, "same-intent")
	assert.Contains(t, warnings, "scope amendment")
	assert.Contains(t, warnings, "S3 review")
}

func TestStatusSurfacesScopeContractDriftBlocker(t *testing.T) {
	t.Parallel()

	root, slug := writeScopeContractDriftFixture(t)
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotNil(t, view.ScopeContract)
	assert.Equal(t, "fail", view.ScopeContract.Status)
	assert.NotContains(t, model.ReasonSpecs(view.Blockers), "scope_contract_drift:cmd/review.go")
}

func TestReviewTreatsScopeContractDriftAsReviewInput(t *testing.T) {
	t.Parallel()

	root, slug := writeScopeContractDriftFixture(t)
	writePassingWaveEvidence(t, root, slug, 1)

	view, err := buildReviewViewForSlug(root, slug, false, "", nil)
	require.NoError(t, err)
	require.NotNil(t, view.ScopeContract)
	assert.Equal(t, "fail", view.Verdict)
	assert.Equal(t, "fail", view.ScopeContract.Status)
	assert.NotContains(t, model.ReasonSpecs(view.Blockers), "scope_contract_drift:cmd/review.go")
	assert.NotContains(t, strings.Join(view.Gaps.ArtifactToCode, "\n"), "scope_contract_drift")
}

// writeAdvanceableS2Fixture builds a governed change at S2_IMPLEMENT whose recorded
// execution + wave evidence is fresh and in-scope, recorded through the canonical
// `slipway evidence task` path so a plain `slipway run` reaches the Scope Contract
// advance gate (rather than blocking earlier on execution-summary freshness). The
// fixture wave-plan's single in-scope target is cmd/lifecycle_commands_test.go.
func writeAdvanceableS2Fixture(t *testing.T, root string) string {
	t.Helper()
	slug, _ := createEvidenceTaskFixture(t, root)
	capturedAt := time.Now().UTC()
	cmd := commandForRoot(t, root, makeEvidenceCmd())
	cmd.SetArgs([]string{
		"task", "--json",
		"--task-id", "t-01",
		"--run-summary-version", "1",
		"--task-kind", "verification",
		"--verdict", "pass",
		"--evidence-ref", "test:scope-drift",
		"--changed-file", "cmd/lifecycle_commands_test.go",
		"--target-file", "cmd/lifecycle_commands_test.go",
		"--captured-at", capturedAt.Format(time.RFC3339Nano),
		"--session-id", "session-a",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())
	writeSkillVerification(t, root, slug, progression.SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  capturedAt.Add(time.Second),
		RunVersion: 1,
		References: []string{"task:evidence:t-01"},
	})
	return slug
}

// An untracked, out-of-scope file in the worktree — the common developer case
// from issue #136 (a scratch file, or the dogfooded `slipway` build binary) —
// must block `slipway run` visibly with the real scope_contract_drift cause and
// must NOT clear the earned wave evidence. Before the fix, the gate rewrote S2
// in place and deleted wave-orchestration.yaml + execution-summary.yaml, which
// then masked the drift behind a "wave-orchestration missing" blocker.
func TestRunScopeContractDriftBlocksWithoutClearingWaveEvidence(t *testing.T) {
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := writeAdvanceableS2Fixture(t, root)

		waveOrchestrationPath := filepath.Join(
			state.VerificationDir(root, slug),
			progression.SkillWaveOrchestration+".yaml",
		)
		executionSummaryPath := filepath.Join(
			state.VerificationDir(root, slug),
			state.ExecutionSummaryFileName,
		)
		require.FileExists(t, waveOrchestrationPath)

		// Drop an untracked file outside the plan's target_files into the worktree.
		require.NoError(t, os.WriteFile(filepath.Join(root, "scratch.txt"), []byte("scratch\n"), 0o644))

		runCmd := commandForRoot(t, root, makeRunCmd())
		runCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var buf bytes.Buffer
		runCmd.SetOut(&buf)
		require.NoError(t, runCmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		// The real cause is surfaced (not masked) and the change stays in its
		// owning stage rather than being rewritten in place.
		assert.Equal(t, model.StateS2Implement, view.CurrentState)
		specs := strings.Join(model.ReasonSpecs(view.Blockers), "\n")
		assert.Contains(t, specs, "scope_contract_drift")
		assert.Contains(t, specs, "scratch.txt")

		// Non-destructive: the earned wave evidence survives (the legacy wipe path
		// would have deleted wave-orchestration.yaml and masked the cause).
		require.FileExists(t, waveOrchestrationPath)
		require.FileExists(t, executionSummaryPath)
		if view.Advanced != nil {
			assert.NotEqual(t, "stale_evidence_requires_review_alignment", view.Advanced.Reason,
				"scope drift must block directly without stale-evidence repair handoff")
			for _, effect := range view.Advanced.SideEffects {
				assert.NotEqual(t, "cleared_stale_generated_evidence", effect.Kind,
					"scope drift must not clear generated wave evidence")
			}
		}

		// Removing the out-of-scope file clears the drift and advancement resumes
		// on the preserved evidence (no re-run of wave-orchestration).
		require.NoError(t, os.Remove(filepath.Join(root, "scratch.txt")))
		resumeCmd := commandForRoot(t, root, makeRunCmd())
		resumeCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var resumeBuf bytes.Buffer
		resumeCmd.SetOut(&resumeBuf)
		require.NoError(t, resumeCmd.Execute())

		var resumeView nextView
		require.NoError(t, json.Unmarshal(resumeBuf.Bytes(), &resumeView))
		assert.Equal(t, model.StateS3Review, resumeView.CurrentState,
			"removing the out-of-scope file advances on the preserved evidence")
		assert.NotContains(t, strings.Join(model.ReasonSpecs(resumeView.Blockers), "\n"), "scope_contract_drift")
	})
}

func writeScopeContractDriftFixture(t *testing.T) (string, string) {
	t.Helper()

	return writeScopeContractDriftFixtureInState(t, model.StateS3Review)
}

func writeScopeContractDriftFixtureInState(t *testing.T, workflowState model.WorkflowState) (string, string) {
	t.Helper()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "scope contract drift fixture")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = workflowState
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement validation surface
  - depends_on: []
  - target_files: ["cmd/validate.go"]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))
	plan, err := state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	now := time.Now().UTC()
	summaryTasks := []model.ExecutionTaskSummary{
		{
			TaskID:       "t-01",
			Verdict:      model.TaskVerdictPass,
			TaskKind:     model.TaskKindCode,
			ChangedFiles: []string{"cmd/review.go"},
			TargetFiles:  []string{"cmd/validate.go"},
			CapturedAt:   now,
		},
	}
	writeExecutionSummary(t, root, slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        now,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01"},
		Tasks:             summaryTasks,
	})
	runs, err := state.BuildWaveRuns(plan, 1, summaryTasks, nil)
	require.NoError(t, err)
	require.NoError(t, state.SaveWaveRuns(root, slug, 1, runs))
	writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
		"changed_files": []string{"cmd/review.go"},
	})

	return root, slug
}
