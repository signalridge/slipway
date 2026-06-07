package cmd

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The compact next/run handoff must carry the recovery object so the first
// surface a host hits is not left without a next action (REQ-003, REQ-004).
func TestNextHandoffViewCarriesRecovery(t *testing.T) {
	t.Parallel()

	view := nextView{
		Blockers: model.ReasonCodesFromSpecs([]string{"required_skill_stale:plan-audit:assurance.md"}),
	}
	handoff := buildNextHandoffView(view)

	require.NotNil(t, handoff.Recovery, "compact handoff must preserve the recovery object")
	assert.NotEmpty(t, handoff.Recovery.PrimaryCommand, "primary recovery command must survive the compact projection")
	require.NotEmpty(t, handoff.Recovery.Steps)
	assert.Equal(t, "required_skill_stale", handoff.Recovery.Steps[0].Code)
	assert.Equal(t, "plan-audit", handoff.Recovery.Steps[0].Subject)
	assert.Equal(t, []string{"assurance.md"}, handoff.Recovery.Steps[0].Details)
	assert.NotEmpty(t, handoff.Recovery.Steps[0].Remediation, "every rendered blocker must carry a remediation")
}

// A clean state (only informational blockers) must omit the recovery object so
// existing healthy outputs stay byte-identical (REQ-007 omitempty).
func TestNextHandoffViewOmitsRecoveryOnCleanState(t *testing.T) {
	t.Parallel()

	view := nextView{
		Blockers: model.ReasonCodesFromSpecs([]string{"no_skill_required:S2_EXECUTE"}),
	}
	handoff := buildNextHandoffView(view)
	assert.Nil(t, handoff.Recovery)
}

// CLIError must surface the same recovery the views do, built from its reasons
// by the shared model.BuildRecovery (REQ-006).
func TestGovernanceBlockedErrorCarriesRecovery(t *testing.T) {
	t.Parallel()

	reasons := model.ReasonCodesFromSpecs([]string{"required_skill_stale:plan-audit:assurance.md"})
	err := newGovernanceBlockedErrorWithReasons("governance_blocked", "blocked", "remediation", "slug", reasons, nil)

	require.NotNil(t, err.Recovery)
	assert.NotEmpty(t, err.Recovery.PrimaryCommand)
	assert.Equal(t, model.BuildRecovery(reasons).PrimaryCommand, err.Recovery.PrimaryCommand,
		"CLIError recovery must match what the views produce for the same reasons")
}

// validate --json must carry a primary recovery command for blocked states
// (REQ-003); buildValidateViewBase is the seam both validate paths share.
func TestValidateViewBaseCarriesRecovery(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("recovery-validate")
	change.CurrentState = model.StateS1Plan
	blockers := model.ReasonCodesFromSpecs([]string{"plan_audit_failed"})

	view := buildValidateViewBase(root, change, nil, blockers, nil, nil)

	require.NotNil(t, view.Recovery)
	assert.NotEmpty(t, view.Recovery.PrimaryCommand)
}

func TestValidateRecoveryIncludesGateDetailBlockers(t *testing.T) {
	t.Parallel()

	recovery := buildValidateRecovery(nil, map[string]model.GateRecord{
		"G_plan": {
			GateID: "G_plan",
			Status: model.GateStatusBlocked,
			ReasonCodes: model.ReasonCodesFromSpecs([]string{
				"required_skill_stale:plan-audit:requirements.md",
				"required_skill_stale:plan-audit:tasks.md",
			}),
		},
	})

	require.NotNil(t, recovery, "gate_details blockers must contribute to validate recovery")
	var step *model.RecoveryStep
	for i := range recovery.Steps {
		if recovery.Steps[i].Code == "required_skill_stale" && recovery.Steps[i].Subject == "plan-audit" {
			step = &recovery.Steps[i]
			break
		}
	}
	require.NotNil(t, step, "plan-audit gate_details blocker must render a recovery step")
	assert.Equal(t, []string{"requirements.md", "tasks.md"}, step.Details)
	assert.Equal(t, "slipway run", step.Command)
	assert.NotContains(t, step.Command, "--skill")
	assert.NotEmpty(t, step.Remediation)
}

func TestValidateViewBaseOmitsRecoveryWhenClean(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("clean-validate")
	change.CurrentState = model.StateS1Plan

	view := buildValidateViewBase(root, change, nil, nil, nil, nil)
	assert.Nil(t, view.Recovery)
}

func recoveryStepCodes(recovery *model.RecoverySummary) []string {
	if recovery == nil {
		return nil
	}
	codes := make([]string, 0, len(recovery.Steps))
	for _, step := range recovery.Steps {
		codes = append(codes, step.Code)
	}
	return codes
}
