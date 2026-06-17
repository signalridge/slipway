package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsCommandSummarizesRepoWideSignals(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "stats view should summarize workflow state")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.QualityMode = model.QualityModeFull
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := makeStatsCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view statsView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, 1, view.ActiveCount)
		assert.Contains(t, view.MissingReviewEvidence, slug)
		assert.Contains(t, view.CloseoutFreshness.Missing, slug)
	})
}

func TestStatsUsesExecutionSummaryForFrozenEvidenceFreshness(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := "stats-execution-summary"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.NeedsDiscovery = false
	change.GuardrailDomain = ""
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))
	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`task-a`"+` keep execution summary fresh
  - target_files: ["cmd/stats.go"]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`## Requirements

### Requirement: StatsExecutionSummary

REQ-001: Stats must treat execution-summary.yaml as the frozen execution evidence source.
`), 0o644))

	now := time.Now().UTC()
	require.NoError(t, state.SaveExecutionSummary(root, slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        now,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{
			{TaskID: "task-a", Verdict: model.TaskVerdictPass, TaskKind: model.TaskKindCode, ChangedFiles: []string{"cmd/stats.go"}, CapturedAt: now},
		},
	}))
	writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  now,
		RunVersion: 1,
	})

	statusView, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.NotContains(t, statusView.Blockers, "required_skill_missing:wave-orchestration")
	assert.NotEqual(t, "unknown", statusView.EvidenceFreshness)

	view, err := buildStatsView(root, now)
	require.NoError(t, err)
	assert.NotContains(t, view.StaleRunSummaries, slug)
}

func TestStatsDoesNotTreatMissingReviewEvidenceAsStaleRunSummary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "stats should separate stale execution evidence from missing review evidence")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)

	view, err := buildStatsView(root, time.Now().UTC())
	require.NoError(t, err)

	assert.Contains(t, view.MissingReviewEvidence, slug)
	assert.NotContains(t, view.StaleRunSummaries, slug)
}

func TestStatsMarksStaleRunSummaryWhenExecutionEvidenceDrifts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "stats should still report stale execution evidence")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writePassingReviewEvidencePack(t, root, slug, 1)

	change.GuardrailDomain = model.GuardrailDomainExternalAPIContracts
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildStatsView(root, time.Now().UTC())
	require.NoError(t, err)

	assert.Contains(t, view.StaleRunSummaries, slug)
}

func TestStatsIgnoresBrokenExecutionSummaryOutsideExecutionStates(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "stats should ignore irrelevant broken summary")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	require.NoError(t, state.SaveChange(root, change))

	summaryPath := executionSummaryPathForTest(root, slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(summaryPath), 0o755))
	require.NoError(t, os.WriteFile(summaryPath, []byte("version: ["), 0o644))

	view, err := buildStatsView(root, time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, 1, view.ActiveCount)
	assert.NotContains(t, view.StaleRunSummaries, slug)
}

func TestStatsReportsBrokenExecutionSummaryInExecutionStatesWithoutFailing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "stats should degrade when execution summary is corrupt")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	summaryPath := executionSummaryPathForTest(root, slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(summaryPath), 0o755))
	require.NoError(t, os.WriteFile(summaryPath, []byte("version: ["), 0o644))

	view, err := buildStatsView(root, time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, 1, view.ActiveCount)
	require.Len(t, view.IntegrityIssues, 1)
	assert.Contains(t, view.IntegrityIssues[0], slug)
	assert.Contains(t, view.IntegrityIssues[0], "execution_summary_load_failed")
}

func TestStatsCountsArchivedOwnersAlongsideHiddenBoundWorktreeChanges(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	initGitRepoForWorktreeTests(t, root)

	slug := createGovernedRequest(t, root, "L3", "stats should still see hidden siblings and archived owners")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	worktreeRoot := filepath.Join(t.TempDir(), change.Slug)
	branch := "feat/" + change.Slug
	runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)
	normalizedWT, err := state.NormalizePath(worktreeRoot)
	require.NoError(t, err)

	changeBeforeWT := change
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = normalizedWT
	change.WorktreeBranch = branch
	require.NoError(t, state.RelocateGovernedBundle(root, changeBeforeWT, change))
	require.NoError(t, state.SaveChange(root, change))

	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	require.NoError(t, os.Remove(filepath.Join(normalizedWT, ".slipway.yaml")))

	archived := model.NewChange("archived-owner")
	archived.Description = "stats archived owner"
	require.NoError(t, state.SaveChange(root, archived))
	_, err = state.ArchiveChange(root, archived, model.ChangeStatusDone)
	require.NoError(t, err)

	view, err := buildStatsView(root, time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, 1, view.ActiveCount)
	assert.Equal(t, 1, view.ArchiveCount)
	assert.Containsf(t, view.MissingReviewEvidence, slug, "view=%+v", view)
}

func TestStatsUsesAuthoritativeVerificationForHiddenBoundWorktreeCloseoutFreshness(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	initGitRepoForWorktreeTests(t, root)

	slug := createGovernedRequest(t, root, "L3", "stats should keep hidden worktree closeout freshness authoritative")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	worktreeRoot := filepath.Join(t.TempDir(), change.Slug)
	branch := "feat/" + change.Slug
	runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)
	normalizedWT, err := state.NormalizePath(worktreeRoot)
	require.NoError(t, err)

	changeBeforeWT := change
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	change.QualityMode = model.QualityModeFull
	change.WorktreePath = normalizedWT
	change.WorktreeBranch = branch
	require.NoError(t, state.RelocateGovernedBundle(root, changeBeforeWT, change))
	require.NoError(t, state.SaveChange(root, change))

	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writePassingReviewEvidencePack(t, root, slug, 1)
	writePassingIndependentReviewEvidence(t, root, slug, 1)
	writePassingGoalVerificationEvidence(t, root, slug, 1)
	writeSkillVerification(t, root, slug, "final-closeout", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
		References: []string{"closeout:pass"},
	})

	require.NoError(t, os.Remove(filepath.Join(normalizedWT, ".slipway.yaml")))

	view, err := buildStatsView(root, time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, 1, view.ActiveCount)
	assert.NotContainsf(t, view.MissingReviewEvidence, slug, "view=%+v", view)
	assert.Containsf(t, view.CloseoutFreshness.Fresh, slug, "view=%+v", view)
	assert.NotContainsf(t, view.CloseoutFreshness.Missing, slug, "view=%+v", view)
	assert.NotContainsf(t, view.CloseoutFreshness.Stale, slug, "view=%+v", view)
}

func TestStatsCountsMissingMandatoryIndependentReviewEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "stats should count missing independent review evidence")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
		References: []string{"layer:R0=pass"},
	})
	writeSkillVerification(t, root, slug, progression.SkillCodeQualityReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
		References: []string{"layer:IR1=pass"},
	})

	view, err := buildStatsView(root, time.Now().UTC())
	require.NoError(t, err)
	assert.Containsf(t, view.MissingReviewEvidence, slug, "view=%+v", view)
}

func TestStatsCountsMissingSelectedSecurityReviewEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "stats should count selected security review evidence")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.WorkflowPreset = model.WorkflowPresetStrict
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))
	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` strict medium review
  - target_files: ["cmd/a.go", "cmd/b.go", "cmd/c.go", "cmd/d.go", "cmd/e.go"]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))

	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
		References: []string{"layer:R0=pass"},
	})
	writeSkillVerification(t, root, slug, progression.SkillCodeQualityReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
		References: []string{"layer:IR1=pass"},
	})
	writePassingIndependentReviewEvidence(t, root, slug, 1)

	view, err := buildStatsView(root, time.Now().UTC())
	require.NoError(t, err)
	assert.Containsf(t, view.MissingReviewEvidence, slug, "view=%+v", view)
}

func writePassingIndependentReviewEvidence(t *testing.T, root, slug string, runSummaryVersion int) {
	t.Helper()
	writeSkillVerification(t, root, slug, progression.SkillIndependentReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: runSummaryVersion,
		References: []string{"independent-review:pass", model.ContextOriginReferencePrefix + model.StageContextReview + "=stats-independent-reviewer"},
	})
	refreshPassingSkillDigestsForTest(t, root, slug, progression.SkillIndependentReview)
}
