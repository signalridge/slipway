package progression

import (
	"testing"

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
	slug := "scope-reopen"
	change := model.NewChange(slug)
	change.CurrentState = state2
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	writeTasksAndMaterializeWavePlan(t, root, change, "# Tasks\n\n"+
		"- [x] `task-a` Implement task A\n"+
		"  - target_files: [\"cmd/next.go\"]\n"+
		"  - wave: 1\n"+
		"  - task_kind: code\n")
	return root, change
}

// A code task recorded without changed files fails the Scope Contract. Because
// the failure can only be repaired by re-recording task evidence in S2_EXECUTE,
// the gate must reopen there instead of leaving the change stranded downstream.
func TestScopeContractReopenTargetReopensToS2WhenChangedFilesMissing(t *testing.T) {
	t.Parallel()
	root, change := seedScopeContractChange(t, model.StateS3Review)

	target, err := scopeContractReopenTarget(root, change, codeTaskSummary(nil))
	require.NoError(t, err)

	assert.Equal(t, SkillWaveOrchestration, target.SkillName)
	assert.Equal(t, model.StateS2Execute, target.State)
	require.NotEmpty(t, target.Blockers, "scope-contract failure must carry blockers into the reopen target")
	found := false
	for _, b := range target.Blockers {
		if b.Code == scopecontract.ReasonScopeContractChangedFilesMissing {
			found = true
		}
	}
	assert.True(t, found, "reopen target should surface scope_contract_changed_files_missing")
}

// When the same task records its changed files, the Scope Contract is satisfied
// and the gate must not reopen (advance proceeds normally).
func TestScopeContractReopenTargetEmptyWhenContractSatisfied(t *testing.T) {
	t.Parallel()
	root, change := seedScopeContractChange(t, model.StateS3Review)

	target, err := scopeContractReopenTarget(root, change, codeTaskSummary([]string{"cmd/next.go"}))
	require.NoError(t, err)
	assert.Empty(t, target.SkillName, "a satisfied scope contract must not trigger a reopen")
}

// The gate is a no-op before the change has produced a ready execution summary.
func TestScopeContractReopenTargetEmptyWhenSummaryNotReady(t *testing.T) {
	t.Parallel()
	root, change := seedScopeContractChange(t, model.StateS3Review)

	target, err := scopeContractReopenTarget(root, change, nil)
	require.NoError(t, err)
	assert.Empty(t, target.SkillName, "no reopen without a ready execution summary")
}

// Out-of-scope drift (a changed file outside the plan) is non-destructive and
// must block visibly instead of clearing wave evidence; missing task
// changed-file evidence still requires a re-record reopen (issue #136).
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
