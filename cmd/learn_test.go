package cmd

import (
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
