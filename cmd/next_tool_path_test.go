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

func TestNextUsesCodexPathsWhenWorkspaceIsCodexOnly(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"codex"}, false))

		slug := createGovernedRequest(t, root, "L2", "codex-only handoff paths")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, ".codex/skills/slipway/plan-audit/SKILL.md", view.NextSkill.PromptPath)
		assert.Equal(t, ".codex/agents/slipway-auditor.toml", view.NextSkill.AgentDefinitionPath)
	})
}
