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

// issue227TwoWavePlan seeds an S2_EXECUTE change with a two-wave plan: wave 1
// holds task-a, wave 2 holds task-b (which depends on task-a).
func issue227TwoWavePlan(t *testing.T) (string, model.Change) {
	t.Helper()
	root := t.TempDir()
	change := model.NewChange("wave-boundary")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	writeTasksAndMaterializeWavePlan(t, root, change, "# Tasks\n\n"+
		"- [ ] `task-a` first wave task\n"+
		"  - depends_on: []\n"+
		"  - target_files: [\"cmd/checkpoint.go\"]\n"+
		"  - task_kind: code\n\n"+
		"- [ ] `task-b` second wave task\n"+
		"  - depends_on: [\"task-a\"]\n"+
		"  - target_files: [\"cmd/evidence.go\"]\n"+
		"  - task_kind: code\n")
	return root, change
}

func issue227WriteTaskEvidence(t *testing.T, root string, change model.Change, taskID string, changedFiles []string) {
	t.Helper()
	payload := map[string]any{
		"task_id":             taskID,
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       changedFiles,
		"target_files":        changedFiles,
		"evidence_ref":        "test:" + taskID,
		"captured_at":         time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, taskID),
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	path := filepath.Join(state.EvidenceTasksDir(root, change.Slug), taskID+".json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, raw, 0o644))
}

// Without any per-task evidence, the resolver reports "no usable evidence" so the
// caller keeps its wave-1 default (issue #227a).
func TestIssue227ResumeWaveIndexFromTaskEvidenceNoEvidence(t *testing.T) {
	t.Parallel()
	root, change := issue227TwoWavePlan(t)
	plan, err := state.LoadWavePlanForChange(root, change)
	require.NoError(t, err)

	index, derived, err := ResumeWaveIndexFromTaskEvidence(root, change, plan)
	require.NoError(t, err)
	assert.False(t, derived, "no task evidence means no evidence-derived index")
	assert.Equal(t, 0, index)
}

// With only wave 1's task recorded as passing, the resolver derives wave 2 as the
// current incomplete wave — the boundary that the documented per-task-evidence
// flow must be able to checkpoint before any run summary exists (issue #227a).
func TestIssue227ResumeWaveIndexFromTaskEvidenceCompletesFirstWave(t *testing.T) {
	t.Parallel()
	root, change := issue227TwoWavePlan(t)
	issue227WriteTaskEvidence(t, root, change, "task-a", []string{"cmd/checkpoint.go"})
	plan, err := state.LoadWavePlanForChange(root, change)
	require.NoError(t, err)

	index, derived, err := ResumeWaveIndexFromTaskEvidence(root, change, plan)
	require.NoError(t, err)
	assert.True(t, derived, "recorded task evidence must drive the resume index")
	assert.Equal(t, 2, index, "a fully-passed wave 1 makes wave 2 the current incomplete wave")
}

// When every wave's task is recorded passing, the resolver reports index 0
// (all-waves-passed), distinct from the no-evidence case via the derived flag.
func TestIssue227ResumeWaveIndexFromTaskEvidenceAllWavesPassed(t *testing.T) {
	t.Parallel()
	root, change := issue227TwoWavePlan(t)
	issue227WriteTaskEvidence(t, root, change, "task-a", []string{"cmd/checkpoint.go"})
	issue227WriteTaskEvidence(t, root, change, "task-b", []string{"cmd/evidence.go"})
	plan, err := state.LoadWavePlanForChange(root, change)
	require.NoError(t, err)

	index, derived, err := ResumeWaveIndexFromTaskEvidence(root, change, plan)
	require.NoError(t, err)
	assert.True(t, derived, "recorded task evidence must drive the resume index")
	assert.Equal(t, 0, index, "all waves passed yields index 0")
}

// A malformed task-evidence file must be soft-tolerated, exactly like the
// read-only sibling surfaces (LoadExecutionTasksFromEvidence), instead of making
// checkpoint hard-fail. The resolver reports the safe wave-1 default
// (derived=false) with no error (issue #227a review follow-up).
func TestIssue227ResumeWaveIndexFromTaskEvidenceToleratesMalformedFile(t *testing.T) {
	t.Parallel()
	root, change := issue227TwoWavePlan(t)
	// A valid wave-1 record plus a corrupt sibling file in the same directory.
	issue227WriteTaskEvidence(t, root, change, "task-a", []string{"cmd/checkpoint.go"})
	corrupt := filepath.Join(state.EvidenceTasksDir(root, change.Slug), "task-b.json")
	require.NoError(t, os.WriteFile(corrupt, []byte("{not valid json"), 0o644))

	plan, err := state.LoadWavePlanForChange(root, change)
	require.NoError(t, err)

	index, derived, err := ResumeWaveIndexFromTaskEvidence(root, change, plan)
	require.NoError(t, err, "a malformed evidence file must not hard-fail wave derivation")
	assert.False(t, derived, "unclean evidence yields the safe wave-1 default, not a guess")
	assert.Equal(t, 0, index)
}

// issue227ScopeContractMissingChange seeds an S2_EXECUTE change whose single code
// task passed but recorded NO changed files, and materializes the matching
// execution-summary.yaml + passing wave-orchestration evidence. This is the
// self-wiping-mask scenario: scope contract flags scope_contract_changed_files_missing.
func issue227ScopeContractMissingChange(t *testing.T) (string, model.Change) {
	t.Helper()
	root := t.TempDir()
	change := model.NewChange("scope-missing-s2")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.WorkflowPreset = model.WorkflowPresetLight
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"),
		[]byte("# Intent\n\n## Summary\nProbe scope contract.\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"),
		[]byte("# Requirements\n\n### Requirement: Scope\nREQ-001: Probe.\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"),
		[]byte("# Decision\n\n## Selected Approach\nProbe.\n"), 0o644))

	writeTasksAndMaterializeWavePlan(t, root, change, "# Tasks\n\n"+
		"- [x] `task-a` Implement task A\n"+
		"  - target_files: [\"cmd/next.go\"]\n"+
		"  - task_kind: code\n")

	recordedAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	writeVerificationForTest(t, root, change.Slug, SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  recordedAt,
		RunVersion: 1,
	})

	freshness := expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a")
	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		// No changed_files: this is the offending record.
		"target_files":     []string{"cmd/next.go"},
		"evidence_ref":     "test:task-a",
		"captured_at":      recordedAt.Format(time.RFC3339Nano),
		"freshness_inputs": freshness,
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, change.Slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        recordedAt,
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:      "task-a",
			Verdict:     model.TaskVerdictPass,
			TaskKind:    model.TaskKindCode,
			TargetFiles: []string{"cmd/next.go"},
			EvidenceRef: "test:task-a",
			CapturedAt:  recordedAt,
		}},
	}
	summary.SyncDerivedFields()
	summary.Normalize()
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	return root, change
}

// At S2_EXECUTE, a scope_contract_changed_files_missing failure must block VISIBLY
// without wiping execution-summary.yaml / wave-orchestration.yaml. Wiping would
// mask the real blocker behind run_summary_missing and loop forever (issue #227b).
func TestIssue227AdvanceBlocksScopeContractMissingAtS2WithoutWiping(t *testing.T) {
	t.Parallel()
	root, change := issue227ScopeContractMissingChange(t)

	summary, err := AdvanceGoverned(root, change.Slug)
	require.NoError(t, err)

	assert.Equal(t, "blocked", summary.Action, "advance must fail closed, not reopen-and-wipe")
	assert.Equal(t, model.StateS2Execute, summary.FromState)
	assert.NotEqual(t, "stale_evidence_recovery_started", summary.Reason,
		"the in-S2 scope-contract miss must not route through the summary-wiping reopen")
	assert.Contains(t, model.ReasonSpecs(summary.Blockers),
		scopecontract.ReasonScopeContractChangedFilesMissing+":task-a",
		"the real scope_contract_changed_files_missing blocker must stay visible")

	// The change stays in S2_EXECUTE.
	reloaded, err := state.LoadChange(root, change.Slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS2Execute, reloaded.CurrentState)

	// And, critically, the engine-owned evidence is preserved so the next read of
	// validate/status/next still surfaces the real blocker instead of being reset
	// to the masking run_summary_missing.
	preservedSummary, err := state.LoadOptionalExecutionSummary(root, change.Slug)
	require.NoError(t, err)
	require.NotNil(t, preservedSummary, "execution-summary.yaml must NOT be wiped")
	assert.Equal(t, 1, preservedSummary.RunSummaryVersion)

	waveRec, found, err := LatestPassingWaveEvidence(root, change.Slug)
	require.NoError(t, err)
	assert.True(t, found, "wave-orchestration.yaml must NOT be wiped")
	assert.Equal(t, 1, waveRec.RunVersion)
}
