package gate

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hasGateReasonCode(reasons []model.ReasonCode, code string) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func TestEvaluateGScope(t *testing.T) {
	t.Parallel()
	change := model.NewChange("slug")
	change.NeedsDiscovery = true
	change.WorktreePath = "/tmp/repo"
	change.WorktreeBranch = "main"

	research := `## Alternatives Considered
### Option A
Tradeoff A

### Option B
Tradeoff B

### Selected Direction
Use Option B

## Unknowns
No critical unknowns remaining.

## Assumptions
Standard deployment environment.

## Canonical References
Internal API docs, RFC-2024-auth.`

	eval := EvaluateGScope(change, research, true, nil)
	assert.Equal(t, model.GateStatusApproved, eval.Status)
	assert.Empty(t, eval.ReasonCodes)

	eval = EvaluateGScope(change, "", true, []model.ReasonCode{model.NewReasonCode("dedicated_worktree_branch_mismatch", "")})
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.True(t, hasGateReasonCode(eval.ReasonCodes, "dedicated_worktree_branch_mismatch"))
	require.NotEmpty(t, eval.ReasonCodes)
	assert.Equal(t, "dedicated_worktree_branch_mismatch", eval.ReasonCodes[0].Code)
	assert.Equal(t, model.ReasonSeverityError, eval.ReasonCodes[0].Severity)
}

func TestEvaluateGPlan(t *testing.T) {
	t.Parallel()
	eval := EvaluateGPlan(true, true, nil)
	assert.Equal(t, model.GateStatusApproved, eval.Status)

	eval = EvaluateGPlan(false, true, []model.ReasonCode{model.NewReasonCode("missing_spec", "")})
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.True(t, hasGateReasonCode(eval.ReasonCodes, "artifact_not_ready"))
}

func TestEvaluateGShip(t *testing.T) {
	t.Parallel()
	change := model.NewChange("slug")

	eval := EvaluateGShip(change, true, true, true, nil, nil)
	assert.Equal(t, model.GateStatusApproved, eval.Status)

	eval = EvaluateGShip(change, false, true, true, nil, nil)
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.True(t, hasGateReasonCode(eval.ReasonCodes, "artifact_not_ready"))
	require.NotEmpty(t, eval.ReasonCodes)
	assert.Equal(t, "artifact_not_ready", eval.ReasonCodes[0].Code)
}

func TestGuardrailHighRiskChecks(t *testing.T) {
	t.Parallel()
	required := RequiredHighRiskChecks("security_credentials")
	require.Equal(t, []string{"security_credentials.safety_baseline"}, required)
	assert.True(t, IsRegisteredCheckID("security_credentials.safety_baseline"))
}

func TestEvaluateGPivot(t *testing.T) {
	t.Parallel()
	eval := EvaluateGPivot(PivotKindReroute, true, model.StateS3Review)
	assert.Equal(t, model.GateStatusApproved, eval.Status)

	eval = EvaluateGPivot(PivotKindRescope, false, model.StateS2Execute)
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.True(t, hasGateReasonCode(eval.ReasonCodes, "pivot_not_approved"))

	// Rescope from S3_REVIEW is now valid (rescope valid from S1_PLAN or later)
	eval = EvaluateGPivot(PivotKindRescope, true, model.StateS3Review)
	assert.Equal(t, model.GateStatusApproved, eval.Status)
	assert.Empty(t, eval.ReasonCodes)

	// Rescope from S0_INTAKE is blocked (requires S1_PLAN or later)
	eval = EvaluateGPivot(PivotKindRescope, true, model.StateS0Intake)
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	require.NotEmpty(t, eval.ReasonCodes)
	assert.Equal(t, "rescope_requires_s1_or_later", eval.ReasonCodes[0].Code)
}
