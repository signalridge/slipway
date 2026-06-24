package cmd

import (
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderStatusTextIncludesProgressAndNextAction(t *testing.T) {
	t.Parallel()

	rendered := renderStatusText(statusView{
		ExecutionMode:     "governed",
		Slug:              "req-123",
		Phase:             model.PhaseBuilding,
		LifecycleStatus:   "active",
		CurrentState:      model.StateS2Implement,
		NextReadyActions:  []string{"slipway next", "slipway review"},
		EvidenceFreshness: "fresh",
		Progress: &statusProgress{
			Percentage:     55,
			StageIndex:     4,
			StageTotal:     8,
			StageName:      string(model.StateS2Implement),
			TasksCompleted: 2,
			TasksTotal:     4,
			TasksByVerdict: map[string]int{"pass": 2, "pending": 2},
		},
		ArtifactDAG: []artifactDAGNode{
			{Name: "intent.md", State: string(model.ArtifactLifecycleApproved), Ready: true},
		},
	})
	assert.Contains(t, rendered, "# req-123")
	assert.Contains(t, rendered, "Phase: Building")
	assert.Contains(t, rendered, "Progress:")
	assert.Contains(t, rendered, "What's Next:")
	assert.Contains(t, rendered, "Also available: slipway review")
}

func TestRenderMultiChangeTextUsesPlaceholders(t *testing.T) {
	t.Parallel()

	rendered := renderMultiChangeText(multiChangeSummaryView{
		ActiveCount: 1,
		ActiveChanges: []multiChangeSummaryEntry{
			{Slug: "req-123", ExecMode: "direct", CurrentState: "S1_PLAN"},
		},
		Hint: "Use --change",
	})
	assert.Contains(t, rendered, "Active Changes: 1")
	assert.Contains(t, rendered, "req-123")
	assert.Contains(t, rendered, "-")
	assert.Contains(t, rendered, "Use --change")
}

func TestRenderStatusTextSortsGateOutput(t *testing.T) {
	t.Parallel()

	rendered := renderStatusText(statusView{
		ExecutionMode:     "governed",
		Slug:              "req-123",
		Phase:             model.PhaseReviewing,
		LifecycleStatus:   "active",
		CurrentState:      model.StateS3Review,
		EvidenceFreshness: "fresh",
		GateStatus: map[string]model.GateRecord{
			"G_ship":  {Status: model.GateStatusApproved},
			"G_plan":  {Status: model.GateStatusBlocked},
			"G_scope": {Status: model.GateStatusApproved},
		},
	})

	planIndex := strings.Index(rendered, "G_plan: blocked")
	scopeIndex := strings.Index(rendered, "G_scope: approved")
	shipIndex := strings.Index(rendered, "G_ship: approved")

	require.NotEqual(t, -1, planIndex)
	require.NotEqual(t, -1, scopeIndex)
	require.NotEqual(t, -1, shipIndex)
	assert.Less(t, planIndex, scopeIndex)
	assert.Less(t, scopeIndex, shipIndex)
}

func TestRenderStatusTextUsesStructuredBlockerMessages(t *testing.T) {
	t.Parallel()

	rendered := renderStatusText(statusView{
		ExecutionMode:     "governed",
		Slug:              "req-123",
		Phase:             model.PhaseReviewing,
		LifecycleStatus:   "active",
		CurrentState:      model.StateS3Review,
		EvidenceFreshness: "fresh",
		Blockers:          model.ReasonCodesFromSpecs([]string{"required_skill_missing:ship-verification"}),
	})
	assert.Contains(t, rendered, "required_skill_missing: Required governance skill evidence is missing: ship-verification")
}

func TestRenderStatusTextExplainsStaleExecutionEvidenceRemediation(t *testing.T) {
	t.Parallel()

	rendered := renderStatusText(statusView{
		ExecutionMode:     "governed",
		Slug:              "req-stale",
		Phase:             model.PhaseBuilding,
		LifecycleStatus:   "active",
		CurrentState:      model.StateS2Implement,
		EvidenceFreshness: "stale",
		Blockers:          model.ReasonCodesFromSpecs([]string{state.StaleExecutionEvidenceBlockerToken}),
	})
	assert.Contains(t, rendered, "stale_execution_evidence: Execution evidence is stale; rerun wave-orchestration for affected tasks")
}

func TestRenderStatusTextShowsPlanningSubStepAndRecoveryNote(t *testing.T) {
	t.Parallel()

	rendered := renderStatusText(statusView{
		ExecutionMode:     "governed",
		Slug:              "req-validate",
		Phase:             model.PhasePlanning,
		LifecycleStatus:   "active",
		CurrentState:      model.StateS1Plan,
		PlanSubStep:       model.PlanSubStepValidate,
		PlanningNote:      "This is a recovery-only planning state entered after post-audit machine validation failed.",
		EvidenceFreshness: "stale",
	})
	assert.Contains(t, rendered, "State: S1_PLAN/validate")
	assert.Contains(t, rendered, "Planning Note: This is a recovery-only planning state entered after post-audit machine validation failed.")
}

func TestRenderStatusTextHighlightsExecutionSummaryIssuesSeparately(t *testing.T) {
	t.Parallel()

	rendered := renderStatusText(statusView{
		ExecutionMode:     "governed",
		Slug:              "req-summary",
		Phase:             model.PhaseBuilding,
		LifecycleStatus:   "active",
		CurrentState:      model.StateS2Implement,
		EvidenceFreshness: "fresh",
		SummaryBlockers:   model.ReasonCodesFromSpecs([]string{"session_isolation_warning:session_id=abc:shared_by=task-a,task-b"}),
		Blockers: model.ReasonCodesFromSpecs([]string{
			"session_isolation_warning:session_id=abc:shared_by=task-a,task-b",
			"required_skill_missing:wave-orchestration",
		}),
	})
	assert.Contains(t, rendered, "Execution Summary Issues:")
	assert.Contains(t, rendered, "session_isolation_warning")
	assert.Contains(t, rendered, "Blockers:")
	assert.Contains(t, rendered, "required_skill_missing")
}
