package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildLearnViewAggregatesLifecycleSignalsIntoPreviewProposals(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("learn-plan-audit")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	change.PlanAuditIterations = 3
	require.NoError(t, state.SaveChange(root, change))
	_, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType:   "gate.blocked",
		Action:      "blocked",
		BeforeState: model.StateS1Plan,
		AfterState:  model.StateS1Plan,
		Blockers: []model.ReasonCode{
			model.NewReasonCode("plan_audit_stalled", ""),
			model.NewReasonCode("plan_audit_budget_exhausted", ""),
		},
	})
	require.NoError(t, err)
	_, err = state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType: "abort.marked",
		Action:    "execution_interrupted",
		Result:    "interrupted",
	})
	require.NoError(t, err)
	_, err = state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType: "resume.succeeded",
		Action:    "resumed",
		Result:    "success",
	})
	require.NoError(t, err)

	view, err := buildLearnView(root, time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	assert.True(t, view.Preview)
	assert.False(t, view.AutoApply)
	assert.Equal(t, 1, view.AnalyzedChanges)
	assert.Equal(t, 3, view.Signals.LifecycleEventCount)
	assert.Equal(t, 1, view.Signals.PlanAuditStalled)
	assert.Equal(t, 1, view.Signals.PlanAuditBudgetExhausted)
	assert.Equal(t, 3, view.Signals.PlanAuditIterations)
	assert.Equal(t, map[string]int{"3": 1}, view.Signals.PlanAuditIterationDistribution)
	assert.Equal(t, 1.0, view.Signals.PlanAuditStallRate)
	assert.Equal(t, 1, view.Signals.InterruptionCount)
	assert.Equal(t, 1, view.Signals.InterruptionResumeSuccesses)
	assert.Equal(t, 1.0, view.Signals.InterruptionResumeSuccessRate)
	require.NotEmpty(t, view.Proposals)
	assert.Equal(t, "plan-audit-loop-review", view.Proposals[0].ID)
	assert.Equal(t, "learn-2026-05-24-plan-audit-loop-review", view.Proposals[0].ProposalID)
	assert.Equal(t, "template_adjustment", view.Proposals[0].Kind)
	assert.True(t, view.Proposals[0].RequiresHumanApproval)
	assert.Contains(t, view.Proposals[0].Changes, change.Slug)
	assert.Equal(t, 1.0, view.Proposals[0].Metrics["plan_audit_stall_rate"])
	assert.NotEmpty(t, view.Proposals[0].RecommendedAction)
	assert.NotEmpty(t, view.Proposals[0].Risk)
}

func TestBuildLearnViewSplitsManualAndAutoCheckpointResolutionSignals(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("learn-checkpoint-resolution-signals")
	require.NoError(t, state.SaveChange(root, change))
	for range 2 {
		_, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
			EventType: "checkpoint.opened",
			Action:    "opened",
		})
		require.NoError(t, err)
	}
	_, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType: "checkpoint.resolved",
		Action:    "resolved",
		SideEffects: []state.LifecycleSideEffect{
			{Kind: "active_checkpoint_cleared"},
		},
	})
	require.NoError(t, err)
	_, err = state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType: "checkpoint.resolved",
		Action:    "resolved",
		SideEffects: []state.LifecycleSideEffect{
			{Kind: "active_checkpoint_cleared"},
			{Kind: autoCheckpointAcknowledgedSideEffect},
		},
	})
	require.NoError(t, err)

	view, err := buildLearnView(root, time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	assert.Equal(t, 4, view.Signals.LifecycleEventCount)
	assert.Equal(t, 2, view.Signals.CheckpointOpened)
	assert.Equal(t, 2, view.Signals.CheckpointResolved)
	assert.Equal(t, 1, view.Signals.CheckpointResolvedManual)
	assert.Equal(t, 1, view.Signals.CheckpointResolvedAuto)
	// The manual/auto counts must partition the total resolved count exactly.
	assert.Equal(t, view.Signals.CheckpointResolved,
		view.Signals.CheckpointResolvedManual+view.Signals.CheckpointResolvedAuto)
	// checkpoint_resolution_rate keeps its historical TOTAL meaning (resolved/opened);
	// the manual/auto rates break that total down by attribution.
	assert.Equal(t, 1.0, view.Signals.CheckpointResolutionRate)
	assert.Equal(t, 0.5, view.Signals.CheckpointManualResolutionRate)
	assert.Equal(t, 0.5, view.Signals.CheckpointAutoResolutionRate)
}

func TestBuildLearnViewToleratesArchivedChangeWithoutLifecycleEvents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("archived-no-events")
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, os.WriteFile(filepath.Join(root, "artifacts", "changes", change.Slug, "intent.md"), []byte("# Intent\n"), 0o644))

	_, err := state.ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	view, err := buildLearnView(root, time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	assert.Equal(t, 1, view.Signals.ArchivedChanges)
	assert.Equal(t, 1, view.AnalyzedChanges)
	assert.Empty(t, view.Signals.MissingLifecycleLogs)
	assert.Empty(t, view.IntegrityIssues)
}

func TestLearnPreviewCommandReturnsReadOnlyPreviewFlags(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cmd := makeLearnCmd()
		cmd.SetArgs([]string{"--preview", "--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		require.NoError(t, cmd.Execute())

		var view learnView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.True(t, view.Preview)
		assert.False(t, view.AutoApply)
	})
}

func TestLearnApplyPathIsUnsupported(t *testing.T) {
	cmd := makeLearnCmd()
	cmd.SetArgs([]string{"--preview=false"})

	err := cmd.Execute()
	require.Error(t, err)

	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "learn_apply_unsupported", cliErr.ErrorCode)
	assert.Equal(t, exitCodeInvalidUsage, cliErr.ExitCode)
}
