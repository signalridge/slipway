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

// scopeContractMissingChangedFilesFixture seeds an S2_IMPLEMENT change whose single code
// task passed but recorded NO changed files, and materializes the matching
// execution-summary.yaml + passing wave-orchestration evidence. This is the
// self-wiping-mask scenario: scope contract flags scope_contract_changed_files_missing.
func scopeContractMissingChangedFilesFixture(t *testing.T) (string, model.Change) {
	t.Helper()
	root := t.TempDir()
	change := model.NewChange("scope-missing-s2")
	change.CurrentState = model.StateS2Implement
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

// At S2_IMPLEMENT, a scope_contract_changed_files_missing failure must block VISIBLY
// without wiping execution-summary.yaml / wave-orchestration.yaml. Wiping would
// mask the real blocker behind run_summary_missing and loop forever.
func TestAdvanceBlocksScopeContractMissingAtS2WithoutWiping(t *testing.T) {
	t.Parallel()
	root, change := scopeContractMissingChangedFilesFixture(t)

	summary, err := AdvanceGoverned(root, change.Slug)
	require.NoError(t, err)

	assert.Equal(t, "blocked", summary.Action, "advance must fail closed without wiping evidence")
	assert.Equal(t, model.StateS2Implement, summary.FromState)
	assert.NotEqual(t, "stale_evidence_requires_review_alignment", summary.Reason,
		"the in-S2 scope-contract miss must block in its owning stage")
	assert.Contains(t, model.ReasonSpecs(summary.Blockers),
		scopecontract.ReasonScopeContractChangedFilesMissing+":task-a",
		"the real scope_contract_changed_files_missing blocker must stay visible")

	// The change stays in S2_IMPLEMENT.
	reloaded, err := state.LoadChange(root, change.Slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS2Implement, reloaded.CurrentState)

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
