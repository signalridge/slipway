package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextReturnsSkillNameWhenWorkspaceIsCodexOnly(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))

		slug := createGovernedRequest(t, root, levelNonDiscovery, "codex-only next skill handoff")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, "plan-audit", view.NextSkill.Name)
	})
}

func TestNextSucceedsInMultiAdapterWorkspaceWithoutToolDisambiguation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude", "codex"}, false))

		slug := createGovernedRequest(t, root, levelNonDiscovery, "multi-adapter next handoff")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, "plan-audit", view.NextSkill.Name)
	})
}

func TestNextReturnsIntakeSkillNameAfterInit(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		create := makeNewCmd()
		create.SetArgs([]string{"--preset", "standard", "intake handoff should use skill name only"})
		require.NoError(t, create.Execute())

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, "intake-clarification", view.NextSkill.Name)
	})
}

// TestNextS0IntakeClarificationHandoffStatesFreshGateByDesign pins #357: the S0
// intake-clarification handoff is a fresh, non-delegable approved-summary gate.
// The gate is NOT relaxed; instead the next_action makes the policy explicit so a
// prior broad "continue" authorization is not mistaken for approval of the intent
// summary. It names the concrete next step: review and approve the Approved
// Summary, then record intake-clarification evidence.
func TestNextS0IntakeClarificationHandoffStatesFreshGateByDesign(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		create := makeNewCmd()
		create.SetArgs([]string{"--preset", "standard", "intake fresh-gate rationale should be explicit"})
		require.NoError(t, create.Execute())

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, "intake-clarification", view.NextSkill.Name)

		cr := view.ConfirmationRequirement
		assert.Equal(t, "skill_handoff:intake-clarification", cr.Reason)
		assert.True(t, cr.FreshConfirmationRequired, "intake approved-summary stays a fresh hard gate")
		assert.False(t, cr.PriorAuthorizationSufficient, "prior broad authorization must not satisfy the intake gate")
		assert.Contains(t, cr.NextAction, "FRESH HARD GATE BY DESIGN")
		assert.Contains(t, cr.NextAction, "review and approve the intake Approved Summary")
		assert.Contains(t, cr.NextAction, "does not substitute for explicit approval")
	})
}
