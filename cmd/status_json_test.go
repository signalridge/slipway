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
	// blocked_by comes only from authoritative governance_action_required
	// blockers. domain-review is one; independent-review is an unsatisfied
	// blocking action but is NOT a governance_action_required blocker here, so
	// it stays in required_actions (the pending queue) without inflating
	// blocked_by.
	assert.Equal(t, []any{"domain-review"}, summary["blocked_by"])
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

func TestStatusJSONResponseIncludesAttributedSatisfiedActions(t *testing.T) {
	t.Parallel()

	view := statusView{
		ExecutionMode:     governedExecutionMode,
		Slug:              "domain-review-mapping",
		Phase:             model.PhaseReviewing,
		LifecycleStatus:   string(model.ChangeStatusActive),
		CurrentState:      model.StateS3Review,
		EvidenceFreshness: "fresh",
		RequiredActions: []governanceActionView{
			{
				ControlID:   "domain-review",
				Mode:        "blocking",
				Description: "run domain-aware compliance review and attach review evidence",
				Satisfied:   true,
				SatisfiedBy: []governanceActionSatisfactionView{
					{
						Kind:        "skill_evidence",
						Name:        "spec-compliance-review",
						EvidenceRef: "artifacts/changes/domain-review-mapping/verification/spec-compliance-review.yaml",
						Reason:      "spec-compliance-review provides the domain-aware review evidence for domain-review",
					},
				},
			},
		},
	}

	raw, err := json.Marshal(buildStatusJSONResponse(view))
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.NotContains(t, payload, "required_actions")
	summary, ok := payload["governance_summary"].(map[string]any)
	require.True(t, ok)
	actions, ok := summary["satisfied_actions"].([]any)
	require.True(t, ok)
	require.Len(t, actions, 1)
	action, ok := actions[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "domain-review", action["control_id"])
	satisfiedBy, ok := action["satisfied_by"].([]any)
	require.True(t, ok)
	require.Len(t, satisfiedBy, 1)
	source, ok := satisfiedBy[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "skill_evidence", source["kind"])
	assert.Equal(t, "spec-compliance-review", source["name"])
	assert.Equal(t, "artifacts/changes/domain-review-mapping/verification/spec-compliance-review.yaml", source["evidence_ref"])
	assert.Equal(t, "spec-compliance-review provides the domain-aware review evidence for domain-review", source["reason"])
}

func TestStatusJSONResponseDoesNotTreatNonGovernanceBlockersAsGovernanceSummary(t *testing.T) {
	t.Parallel()

	view := statusView{
		ExecutionMode:     governedExecutionMode,
		Slug:              "execution-blocked",
		Phase:             model.PhaseBuilding,
		LifecycleStatus:   string(model.ChangeStatusActive),
		CurrentState:      model.StateS2Implement,
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

func TestStatusJSONResponseExcludesAdvisoryActionFromBlockedBy(t *testing.T) {
	t.Parallel()

	// Mirrors the Lattice S2 report (issue #36, comment 1): the real blocker is
	// the missing execution host (wave-orchestration); independent-review is an
	// unsatisfied review-scope action that does not gate S2. The pending action
	// must surface as a required_action with post-execution wording, but must NOT
	// be reported as blocked_by, since no governance_action_required blocker
	// names it.
	view := statusView{
		ExecutionMode:     governedExecutionMode,
		Slug:              "advisory-pending",
		Phase:             model.PhaseBuilding,
		LifecycleStatus:   string(model.ChangeStatusActive),
		CurrentState:      model.StateS2Implement,
		EvidenceFreshness: "fresh",
		Blockers: []model.ReasonCode{
			model.NewReasonCode("required_skill_missing", "wave-orchestration"),
		},
		ActiveControls: []governanceControlView{
			{ControlID: "independent-review", Mode: "advisory", Scope: "review"},
		},
		RequiredActions: []governanceActionView{
			{
				ControlID:   "independent-review",
				Mode:        "advisory",
				Description: "run independent review after wave execution produces execution evidence",
				Satisfied:   false,
			},
		},
	}

	raw, err := json.Marshal(buildStatusJSONResponse(view))
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	summary, ok := payload["governance_summary"].(map[string]any)
	require.True(t, ok)
	// Advisory independent-review is a pending action, not a blocker, and its
	// wording must not claim it runs before execution.
	assert.NotContains(t, summary, "blocked_by")
	actions, ok := summary["required_actions"].([]any)
	require.True(t, ok)
	require.Len(t, actions, 1)
	assert.Equal(t, "run independent review after wave execution produces execution evidence", actions[0])
	assert.NotContains(t, actions[0], "before further execution")
}

func TestStatusJSONResponseBuildsGovernanceSummaryFromGovernanceBlocker(t *testing.T) {
	t.Parallel()

	view := statusView{
		ExecutionMode:     governedExecutionMode,
		Slug:              "governance-blocked",
		Phase:             model.PhaseBuilding,
		LifecycleStatus:   string(model.ChangeStatusActive),
		CurrentState:      model.StateS2Implement,
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
