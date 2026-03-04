package gate

import (
	"testing"

	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMandatoryGatesForLevel(t *testing.T) {
	assert.Empty(t, MandatoryGatesForLevel(model.LevelL1))
	assert.Equal(t, []GateID{GatePlan, GatePivot, GateShip}, MandatoryGatesForLevel(model.LevelL2))
	assert.Equal(t, []GateID{GateScope, GatePlan, GatePivot, GateShip}, MandatoryGatesForLevel(model.LevelL3))
}

func TestMapGateDecision(t *testing.T) {
	status, err := MapGateDecision(model.GateDecisionApprove, []string{})
	require.NoError(t, err)
	assert.Equal(t, model.GateStatusApproved, status)

	status, err = MapGateDecision(model.GateDecisionReject, []string{"x"})
	require.NoError(t, err)
	assert.Equal(t, model.GateStatusBlocked, status)

	status, err = MapGateDecision(model.GateDecisionConditionalApprove, []string{"x"})
	require.NoError(t, err)
	assert.Equal(t, model.GateStatusPending, status)
}

func TestEvaluateGScope(t *testing.T) {
	change := model.NewChangeState(mustRequestID(t), "slug")
	change.Level = model.LevelL3
	change.WorktreePath = "/tmp/repo"
	change.WorktreeBranch = "main"

	explore := `## Objectives
One
## Unknowns
Two
## Assumptions
Three
## Scope Boundaries
Four
## Validation Plan
Five`

	eval := EvaluateGScope(change, explore, true, true, true)
	assert.Equal(t, model.GateStatusApproved, eval.Status)
	assert.Empty(t, eval.Reasons)

	eval = EvaluateGScope(change, "", true, true, true)
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.NotEmpty(t, eval.Reasons)
}

func TestEvaluateGPlan(t *testing.T) {
	eval := EvaluateGPlan(true, true, nil)
	assert.Equal(t, model.GateStatusApproved, eval.Status)

	eval = EvaluateGPlan(false, true, []string{"missing_spec"})
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.Contains(t, eval.Reasons, "artifact_not_ready")
}

func TestEvaluateGShip(t *testing.T) {
	change := model.NewChangeState(mustRequestID(t), "slug")
	change.Level = model.LevelL2

	eval := EvaluateGShip(change, true, true, true, nil, nil)
	assert.Equal(t, model.GateStatusApproved, eval.Status)

	eval = EvaluateGShip(change, false, true, true, nil, nil)
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.Contains(t, eval.Reasons, "artifact_not_ready")
}

func TestGuardrailHighRiskChecks(t *testing.T) {
	required := RequiredHighRiskChecks("security_credentials")
	require.Equal(t, []string{"security_credentials.baseline"}, required)
	assert.True(t, IsRegisteredCheckID("security_credentials.baseline"))

	reasons := EvaluateHighRiskChecks("security_credentials", map[string]bool{})
	assert.Contains(t, reasons, "high_risk_check_missing:security_credentials.baseline")
}

func TestEvaluateGPivot(t *testing.T) {
	eval := EvaluateGPivot(PivotKindReroute, true, model.StateS7Review, model.LevelL2)
	assert.Equal(t, model.GateStatusApproved, eval.Status)

	eval = EvaluateGPivot(PivotKindRescope, false, model.StateS6RunWaves, model.LevelL3)
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.Contains(t, eval.Reasons, "rescope_requires_approved_pivot")

	eval = EvaluateGPivot(PivotKindRescope, true, model.StateS7Review, model.LevelL3)
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.Contains(t, eval.Reasons, "rescope_requires_s6_state")
}

func mustRequestID(t *testing.T) string {
	t.Helper()
	id, err := model.NewRequestID()
	require.NoError(t, err)
	return id
}
