package cmd

import (
	"encoding/json"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusJSONResponseShapesGovernanceSummary(t *testing.T) {
	t.Parallel()

	view := statusView{
		ExecutionMode:     governedExecutionMode,
		Slug:              "auth-change",
		Phase:             model.PhaseReviewing,
		LifecycleStatus:   string(model.ChangeStatusActive),
		CurrentState:      model.StateS3Review,
		EvidenceFreshness: "stale",
		SourceStateFile:   "artifacts/changes/auth-change/change.yaml",
		Blockers: []model.ReasonCode{
			model.NewReasonCode("governance_action_required", "domain-review"),
		},
		GovernanceSignals: &governanceSignalView{
			Domains:     []string{"auth_authz"},
			BlastRadius: "medium",
		},
		ActiveControls: []governanceControlView{
			{ControlID: "domain-review", Mode: "blocking", Scope: "change"},
			{ControlID: "independent-review", Mode: "blocking", Scope: "change"},
		},
		RequiredActions: []governanceActionView{
			{
				ControlID:   "domain-review",
				Mode:        "blocking",
				Description: "record domain review evidence",
				Satisfied:   false,
			},
			{
				ControlID:   "independent-review",
				Mode:        "blocking",
				Description: "record independent review evidence",
				Satisfied:   false,
			},
			{
				ControlID:   "worktree-isolation",
				Mode:        "blocking",
				Description: "confirm dedicated worktree",
				Satisfied:   true,
			},
		},
	}

	raw, err := json.Marshal(buildStatusJSONResponse(view))
	require.NoError(t, err)
	s := string(raw)
	assert.NotContains(t, s, "governance_signals")
	assert.NotContains(t, s, "active_controls")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.NotContains(t, payload, "required_actions")
	summary, ok := payload["governance_summary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []any{"domain-review", "independent-review"}, summary["blocked_by"])
	assert.Equal(t, []any{"record domain review evidence", "record independent review evidence"}, summary["required_actions"])
	assert.Equal(t, []any{
		"artifacts/changes/auth-change/change.yaml",
		"slipway health --governance --json --change auth-change",
	}, summary["authority_refs"])
}

func TestStatusJSONResponseOmitsGovernanceSummaryWhenNoGovernanceActions(t *testing.T) {
	t.Parallel()

	view := statusView{
		ExecutionMode:     governedExecutionMode,
		Slug:              "plain-change",
		Phase:             model.PhasePlanning,
		LifecycleStatus:   string(model.ChangeStatusActive),
		CurrentState:      model.StateS1Plan,
		EvidenceFreshness: "fresh",
	}

	raw, err := json.Marshal(buildStatusJSONResponse(view))
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.NotContains(t, payload, "governance_summary")
	assert.NotContains(t, payload, "governance_signals")
	assert.NotContains(t, payload, "active_controls")
	assert.NotContains(t, payload, "required_actions")
}

func TestStatusJSONResponseDoesNotTreatNonGovernanceBlockersAsGovernanceSummary(t *testing.T) {
	t.Parallel()

	view := statusView{
		ExecutionMode:     governedExecutionMode,
		Slug:              "execution-blocked",
		Phase:             model.PhaseBuilding,
		LifecycleStatus:   string(model.ChangeStatusActive),
		CurrentState:      model.StateS2Execute,
		EvidenceFreshness: "fresh",
		Blockers: []model.ReasonCode{
			model.NewReasonCode("wave_execution_blocked", "task-01 failed"),
		},
		ActiveControls: []governanceControlView{
			{ControlID: "domain-review", Mode: "blocking", Scope: "change"},
		},
		RequiredActions: []governanceActionView{
			{
				ControlID:   "domain-review",
				Mode:        "blocking",
				Description: "record domain review evidence",
				Satisfied:   true,
			},
		},
	}

	raw, err := json.Marshal(buildStatusJSONResponse(view))
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.NotContains(t, payload, "governance_summary")
	assert.NotContains(t, payload, "active_controls")
	assert.NotContains(t, payload, "required_actions")
}

func TestStatusJSONResponseBuildsGovernanceSummaryFromGovernanceBlocker(t *testing.T) {
	t.Parallel()

	view := statusView{
		ExecutionMode:     governedExecutionMode,
		Slug:              "governance-blocked",
		Phase:             model.PhaseBuilding,
		LifecycleStatus:   string(model.ChangeStatusActive),
		CurrentState:      model.StateS2Execute,
		EvidenceFreshness: "fresh",
		Blockers: []model.ReasonCode{
			model.NewReasonCode("governance_action_required", "domain-review: run domain-aware compliance review"),
		},
	}

	raw, err := json.Marshal(buildStatusJSONResponse(view))
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	summary, ok := payload["governance_summary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []any{"domain-review"}, summary["blocked_by"])
	assert.Equal(t, []any{"run domain-aware compliance review"}, summary["required_actions"])
}
