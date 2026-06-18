package progression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/scopecontract"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// codeTaskSummary builds a ready execution summary with a single passing code
// task whose changed-files set is supplied by the caller.
func codeTaskSummary(changedFiles []string) *model.ExecutionSummary {
	return &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:       "task-a",
			Verdict:      model.TaskVerdictPass,
			TaskKind:     model.TaskKindCode,
			ChangedFiles: changedFiles,
			TargetFiles:  []string{"cmd/next.go"},
			EvidenceRef:  "test:task-a",
		}},
	}
}

func seedScopeContractChange(t *testing.T, state2 model.WorkflowState) (string, model.Change) {
	t.Helper()
	root := t.TempDir()
	slug := "scope-repair"
	change := model.NewChange(slug)
	change.CurrentState = state2
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	writeTasksAndMaterializeWavePlan(t, root, change, "# Tasks\n\n"+
		"- [x] `task-a` Implement task A\n"+
		"  - target_files: [\"cmd/next.go\"]\n"+
		"  - task_kind: code\n")
	return root, change
}

// A code task recorded without changed files fails the Scope Contract. The
// target identifies the owning implementation authority without mutating
// lifecycle state.
func TestScopeContractRepairTargetReturnsS2RepairWhenChangedFilesMissing(t *testing.T) {
	t.Parallel()
	root, change := seedScopeContractChange(t, model.StateS3Review)

	target, err := scopeContractRepairTarget(root, change, codeTaskSummary(nil))
	require.NoError(t, err)

	assert.Equal(t, SkillWaveOrchestration, target.SkillName)
	assert.Equal(t, model.StateS2Implement, target.State)
	require.NotEmpty(t, target.Blockers, "scope-contract failure must carry blockers into the repair target")
	found := false
	for _, b := range target.Blockers {
		if b.Code == scopecontract.ReasonScopeContractChangedFilesMissing {
			found = true
		}
	}
	assert.True(t, found, "repair target should surface scope_contract_changed_files_missing")
}

// When the same task records its changed files, the Scope Contract is satisfied
// and the gate must not request repair (advance proceeds normally).
func TestScopeContractRepairTargetEmptyWhenContractSatisfied(t *testing.T) {
	t.Parallel()
	root, change := seedScopeContractChange(t, model.StateS3Review)

	target, err := scopeContractRepairTarget(root, change, codeTaskSummary([]string{"cmd/next.go"}))
	require.NoError(t, err)
	assert.Empty(t, target.SkillName, "a satisfied scope contract must not trigger a repair")
}

// The gate is a no-op before the change has produced a ready execution summary.
func TestScopeContractRepairTargetEmptyWhenSummaryNotReady(t *testing.T) {
	t.Parallel()
	root, change := seedScopeContractChange(t, model.StateS3Review)

	target, err := scopeContractRepairTarget(root, change, nil)
	require.NoError(t, err)
	assert.Empty(t, target.SkillName, "no repair without a ready execution summary")
}

// Out-of-scope drift (a changed file outside the plan) is non-destructive and
// must block visibly instead of clearing wave evidence; missing task
// changed-file evidence routes to explicit repair (issue #136).
func TestScopeContractDriftOnly(t *testing.T) {
	t.Parallel()

	drift := model.NewReasonCode(scopecontract.ReasonScopeContractDrift, "scratch.txt")
	missing := model.NewReasonCode(scopecontract.ReasonScopeContractChangedFilesMissing, "t-01")

	assert.True(t, scopeContractDriftOnly([]model.ReasonCode{drift}),
		"a sole drift blocker is drift-only")
	assert.True(t, scopeContractDriftOnly([]model.ReasonCode{drift, drift}),
		"multiple drift blockers are still drift-only")
	assert.False(t, scopeContractDriftOnly([]model.ReasonCode{drift, missing}),
		"a mix with missing-evidence is not drift-only")
	assert.False(t, scopeContractDriftOnly([]model.ReasonCode{missing}),
		"missing-evidence alone is not drift-only")
	assert.False(t, scopeContractDriftOnly(nil),
		"no blockers is not drift-only")
}

func TestSensitiveEvidenceRepairTargetReturnsS2RepairWhenMarkerMissing(t *testing.T) {
	t.Parallel()

	root, change := seedSensitiveEvidenceExecution(t, model.StateS3Review, "go-test:./...")

	target, err := sensitiveEvidenceRepairTarget(root, change, sensitiveMigrationSummary("go-test:./..."))
	require.NoError(t, err)

	assert.Equal(t, SkillWaveOrchestration, target.SkillName)
	assert.Equal(t, model.StateS2Implement, target.State)
	assert.Contains(
		t,
		model.ReasonSpecs(target.Blockers),
		"sensitive_evidence_missing:schema_migration:db/migrations/001_create_users.sql",
	)
}

func TestAdvanceGovernedBlocksSensitiveEvidenceAtS2WithoutAdvancing(t *testing.T) {
	t.Parallel()

	root, change := seedSensitiveEvidenceExecution(t, model.StateS2Implement, "go-test:./...")

	summary, err := AdvanceGoverned(root, change.Slug)
	require.NoError(t, err)

	assert.Equal(t, "blocked", summary.Action)
	assert.Equal(t, model.StateS2Implement, summary.FromState)
	assert.Contains(
		t,
		model.ReasonSpecs(summary.Blockers),
		"sensitive_evidence_missing:schema_migration:db/migrations/001_create_users.sql",
	)

	reloaded, err := state.LoadChange(root, change.Slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS2Implement, reloaded.CurrentState)
}

func TestAdvanceGovernedBlocksSensitiveEvidenceFromS3ForReviewAlignment(t *testing.T) {
	t.Parallel()

	root, change := seedSensitiveEvidenceExecution(t, model.StateS3Review, "go-test:./...")

	summary, err := AdvanceGoverned(root, change.Slug)
	require.NoError(t, err)

	assert.Equal(t, "blocked", summary.Action)
	assert.Equal(t, "stale_evidence_requires_review_alignment", summary.Reason)
	assert.Equal(t, model.StateS3Review, summary.FromState)
	assert.Empty(t, summary.ToState)
	assert.Contains(
		t,
		model.ReasonSpecs(summary.Blockers),
		"sensitive_evidence_missing:schema_migration:db/migrations/001_create_users.sql",
	)

	reloaded, err := state.LoadChange(root, change.Slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS3Review, reloaded.CurrentState)
}

func TestSensitiveEvidenceRepairTargetEmptyWhenMarkerPresent(t *testing.T) {
	t.Parallel()

	root, change := seedSensitiveEvidenceExecution(t, model.StateS3Review, "migration-applied:goose up")

	target, err := sensitiveEvidenceRepairTarget(root, change, sensitiveMigrationSummary("migration-applied:goose up"))
	require.NoError(t, err)
	assert.Empty(t, target.SkillName, "matching sensitive evidence must not trigger a repair")
}

func seedSensitiveEvidenceExecution(
	t *testing.T,
	workflowState model.WorkflowState,
	evidenceRef string,
) (string, model.Change) {
	t.Helper()

	root := t.TempDir()
	change := model.NewChange("sensitive-evidence")
	change.CurrentState = workflowState
	change.PlanSubStep = model.PlanSubStepNone
	change.WorkflowPreset = model.WorkflowPresetLight
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(bundleDir, "intent.md"),
		[]byte("# Intent\n\n## Summary\nProbe sensitive evidence.\n"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(bundleDir, "requirements.md"),
		[]byte("# Requirements\n\n### Requirement: Sensitive evidence\nREQ-001: Probe.\n"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(bundleDir, "decision.md"),
		[]byte("# Decision\n\n## Selected Approach\nProbe.\n"),
		0o644,
	))

	const migration = "db/migrations/001_create_users.sql"
	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [x] `+"`t-01`"+` apply schema migration
  - target_files: ["`+migration+`"]
  - task_kind: code
`)

	recordedAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	writeVerificationForTest(t, root, change.Slug, SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  recordedAt,
		RunVersion: 1,
	})

	freshnessInputs := expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "t-01")
	taskEvidence := map[string]any{
		"task_id":             "t-01",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{migration},
		"target_files":        []string{migration},
		"blockers":            []model.ReasonCode{},
		"evidence_ref":        evidenceRef,
		"captured_at":         recordedAt.Format(time.RFC3339Nano),
		"freshness_inputs":    freshnessInputs,
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, change.Slug), "t-01.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *sensitiveMigrationSummary(evidenceRef)))

	return root, change
}

func sensitiveMigrationSummary(evidenceRef string) *model.ExecutionSummary {
	const migration = "db/migrations/001_create_users.sql"
	recordedAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        recordedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:       "t-01",
			Verdict:      model.TaskVerdictPass,
			TaskKind:     model.TaskKindCode,
			ChangedFiles: []string{migration},
			TargetFiles:  []string{migration},
			EvidenceRef:  evidenceRef,
			CapturedAt:   recordedAt,
		}},
	}
	summary.SyncDerivedFields()
	summary.Normalize()
	return summary
}
