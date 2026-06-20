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
