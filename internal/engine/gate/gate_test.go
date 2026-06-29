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

	eval := EvaluateGScope(change, research, true, nil, false, false)
	assert.Equal(t, model.GateStatusApproved, eval.Status)
	assert.Empty(t, eval.ReasonCodes)

	eval = EvaluateGScope(change, "", true, []model.ReasonCode{model.NewReasonCode("dedicated_worktree_branch_mismatch", "")}, false, false)
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.True(t, hasGateReasonCode(eval.ReasonCodes, "dedicated_worktree_branch_mismatch"))
	require.NotEmpty(t, eval.ReasonCodes)
	assert.Equal(t, "dedicated_worktree_branch_mismatch", eval.ReasonCodes[0].Code)
	assert.Equal(t, model.ReasonSeverityError, eval.ReasonCodes[0].Severity)
}

// TestEvaluateGScopePresentButStaleDiscoveryEvidence pins the honest-wording
// distinction (#377), mirroring the EvaluateGShip stale taxonomy: when a
// research-orchestration record EXISTS but its certified discovery inputs went
// stale after the verdict (discoveryRecordPresent=true, discoveryRecordStale=true,
// discoveryEvidenceOK=false), G_scope must NOT report the misleading
// missing_discovery_evidence — the merged required_skill_stale carries the
// present-but-stale state instead.
func TestEvaluateGScopePresentButStaleDiscoveryEvidence(t *testing.T) {
	t.Parallel()

	change := model.NewChange("slug")
	change.NeedsDiscovery = true

	eval := EvaluateGScope(change, "", false, nil, true, true)
	assert.False(t, hasGateReasonCode(eval.ReasonCodes, "missing_discovery_evidence"),
		"a present-but-stale discovery record must not be reported as missing")
}

// TestEvaluateGScopePresentButFailedDiscoveryEvidence proves a research-orchestration
// record that is PRESENT but failed on its own merits (not stale,
// discoveryRecordPresent=true, discoveryRecordStale=false) is not relabeled missing:
// the specific required-skill blocker carries the block. It also pins the
// genuinely-absent case (present=false, stale=false) which STILL reserves the
// missing_discovery_evidence code, so the three arms stay distinct (#377).
func TestEvaluateGScopePresentButFailedDiscoveryEvidence(t *testing.T) {
	t.Parallel()

	change := model.NewChange("slug")
	change.NeedsDiscovery = true

	present := EvaluateGScope(change, "", false, nil, true, false)
	assert.False(t, hasGateReasonCode(present.ReasonCodes, "missing_discovery_evidence"),
		"a present-but-failed discovery record must not be reported as missing")

	// Genuinely absent: no record present, not stale -> the _missing code is reserved.
	absent := EvaluateGScope(change, "", false, nil, false, false)
	assert.True(t, hasGateReasonCode(absent.ReasonCodes, "missing_discovery_evidence"),
		"a genuinely absent discovery record must still report missing_discovery_evidence")
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

	eval := EvaluateGShip(change, true, true, true, nil, nil, false, false)
	assert.Equal(t, model.GateStatusApproved, eval.Status)

	eval = EvaluateGShip(change, false, true, true, nil, nil, false, false)
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.True(t, hasGateReasonCode(eval.ReasonCodes, "artifact_not_ready"))
	require.NotEmpty(t, eval.ReasonCodes)
	assert.Equal(t, "artifact_not_ready", eval.ReasonCodes[0].Code)
}

func TestEvaluateGShipMissingVerificationEvidenceRoutesS4Recovery(t *testing.T) {
	t.Parallel()

	// No ship-verification record present -> the genuinely-absent code.
	eval := EvaluateGShip(model.NewChange("slug"), true, false, true, nil, nil, false, false)
	require.NotEmpty(t, eval.ReasonCodes)
	reason := findGateReasonCode(t, eval.ReasonCodes, "ship_verification_evidence_missing")
	assert.Equal(t, model.ReasonSeverityError, reason.Severity)
	assert.False(t, hasGateReasonCode(eval.ReasonCodes, "ship_verification_evidence_stale"))
	recovery := model.BuildRecovery(eval.ReasonCodes)
	require.NotNil(t, recovery)
	assert.Equal(t, model.RecoveryClassReviewAlignment, recovery.RecoveryClass)
	assert.Equal(t, "slipway review", recovery.PrimaryCommand)
}

// TestEvaluateGShipPresentButStaleVerificationEvidence pins the honest-wording
// distinction (#344): when the ship-verification record EXISTS but is no longer
// fresh/passing (shipRecordPresent=true, verificationReady=false), the gate reports
// ship_verification_evidence_stale, never the misleading _missing, and its recovery
// names a refresh rather than a first-time run.
func TestEvaluateGShipPresentButStaleVerificationEvidence(t *testing.T) {
	t.Parallel()

	change := model.NewChange("slug")
	change.CurrentState = model.StateS3Review

	eval := EvaluateGShip(change, true, false, true, nil, nil, true, true)
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	stale := findGateReasonCode(t, eval.ReasonCodes, "ship_verification_evidence_stale")
	assert.Equal(t, model.ReasonSeverityError, stale.Severity)
	assert.False(t, hasGateReasonCode(eval.ReasonCodes, "ship_verification_evidence_missing"),
		"a present-but-stale ship record must not be reported as missing")
}

// A ship-verification record that is PRESENT but failed on its own merits (not
// stale) must NOT be relabeled stale or missing; the specific required-skill
// blocker carries the block. Guards against the generic reason contradicting the
// specific one in the same response.
func TestEvaluateGShipPresentButFailedVerificationEvidence(t *testing.T) {
	t.Parallel()
	change := model.NewChange("slug")
	change.CurrentState = model.StateS3Review
	// The present-but-failed record's specific blocker travels via unresolvedBlockers
	// (the channel the authority routes required_skill_not_passed through); the gate
	// stays blocked on it while the present-but-failed switch arm adds no generic
	// reason, so neither _stale nor _missing can contradict it.
	specific := []model.ReasonCode{model.NewReasonCode("required_skill_not_passed", "ship-verification")}
	eval := EvaluateGShip(change, true, false, true, specific, nil, true, false)
	assert.Equal(t, model.GateStatusBlocked, eval.Status)
	assert.True(t, hasGateReasonCode(eval.ReasonCodes, "required_skill_not_passed"),
		"the specific failed-skill blocker must carry the block")
	assert.False(t, hasGateReasonCode(eval.ReasonCodes, "ship_verification_evidence_stale"),
		"a present-but-failed ship record must not be reported as stale")
	assert.False(t, hasGateReasonCode(eval.ReasonCodes, "ship_verification_evidence_missing"),
		"a present-but-failed ship record must not be reported as missing")
}

func TestGuardrailHighRiskChecks(t *testing.T) {
	t.Parallel()
	required := RequiredHighRiskChecks("security_credentials")
	require.Equal(t, []string{"security_credentials.safety_baseline"}, required)
	assert.True(t, IsRegisteredCheckID("security_credentials.safety_baseline"))
}

// TestEvaluateGShipHighRiskChecks drives the high-risk safety gate end-to-end
// through EvaluateGShip for a guardrail-domain change: a required SAST baseline
// that is absent must block with high_risk_check_missing, an explicit failure
// must block with high_risk_check_failed, and only a recorded pass clears the
// gate. The catalog/unit legs are covered above; this pins the wired G_ship leg
// so a sensitive-domain change cannot reach the ship decision without the SAST
// baseline actually passing.
func TestEvaluateGShipHighRiskChecks(t *testing.T) {
	t.Parallel()

	change := model.NewChange("slug")
	change.CurrentState = model.StateS3Review
	change.GuardrailDomain = model.GuardrailDomainAuthAuthZ
	const baseline = "auth_authz.safety_baseline"

	// Missing baseline -> fail closed; every other ship input is ready so the
	// high-risk reason is the sole blocker.
	missing := EvaluateGShip(change, true, true, true, nil, nil, false, false)
	assert.Equal(t, model.GateStatusBlocked, missing.Status)
	reason := findGateReasonCode(t, missing.ReasonCodes, "high_risk_check_missing")
	assert.Equal(t, baseline, reason.Detail)

	// Recorded explicit failure -> fail closed with the failed code.
	failed := EvaluateGShip(change, true, true, true, nil, map[string]bool{baseline: false}, false, false)
	assert.Equal(t, model.GateStatusBlocked, failed.Status)
	reason = findGateReasonCode(t, failed.ReasonCodes, "high_risk_check_failed")
	assert.Equal(t, baseline, reason.Detail)

	// Recorded pass -> the gate clears (no high-risk reason remains).
	passed := EvaluateGShip(change, true, true, true, nil, map[string]bool{baseline: true}, false, false)
	assert.Equal(t, model.GateStatusApproved, passed.Status)
	assert.False(t, hasGateReasonCode(passed.ReasonCodes, "high_risk_check_missing"))
	assert.False(t, hasGateReasonCode(passed.ReasonCodes, "high_risk_check_failed"))
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
