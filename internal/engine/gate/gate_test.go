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
	eval := EvaluateGPlan(true, nil)
	assert.Equal(t, model.GateStatusApproved, eval.Status)

	eval = EvaluateGPlan(false, []model.ReasonCode{model.NewReasonCode("missing_spec", "")})
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.True(t, hasGateReasonCode(eval.ReasonCodes, "artifact_not_ready"))
}

func TestEvaluateGShip(t *testing.T) {
	t.Parallel()
	change := model.NewChange("slug")
	change.CurrentState = model.StateS3Review

	eval := EvaluateGShip(change, true, true, true, nil, nil)
	assert.Equal(t, model.GateStatusApproved, eval.Status)

	eval = EvaluateGShip(change, false, true, true, nil, nil)
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.True(t, hasGateReasonCode(eval.ReasonCodes, "artifact_not_ready"))
	require.NotEmpty(t, eval.ReasonCodes)
	assert.Equal(t, "artifact_not_ready", eval.ReasonCodes[0].Code)
}

func TestEvaluateGShipMissingVerificationEvidenceRoutesS4Recovery(t *testing.T) {
	t.Parallel()

	eval := EvaluateGShip(model.NewChange("slug"), true, false, true, nil, nil)
	require.NotEmpty(t, eval.ReasonCodes)
	reason := findGateReasonCode(t, eval.ReasonCodes, "ship_verification_evidence_missing")
	assert.Equal(t, model.ReasonSeverityError, reason.Severity)
	recovery := model.BuildRecovery(eval.ReasonCodes)
	require.NotNil(t, recovery)
	assert.Equal(t, model.RecoveryClassReviewAlignment, recovery.RecoveryClass)
	assert.Equal(t, "slipway review", recovery.PrimaryCommand)
}

func TestGuardrailHighRiskChecks(t *testing.T) {
	t.Parallel()
	required := RequiredHighRiskChecks("security_credentials")
	require.Equal(t, []string{"security_credentials.safety_baseline"}, required)
	assert.True(t, IsRegisteredCheckID("security_credentials.safety_baseline"))
}

func findGateReasonCode(t *testing.T, reasons []model.ReasonCode, code string) model.ReasonCode {
	t.Helper()

	for _, reason := range reasons {
		if reason.Code == code {
			return reason
		}
	}
	require.Failf(t, "reason code not found", "missing %s in %#v", code, reasons)
	return model.ReasonCode{}
}
