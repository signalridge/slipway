package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, ".codex/skills/slipway-plan-audit/SKILL.md", view.NextSkill.PromptPath)
		assert.Equal(t, ".codex/agents/slipway-auditor.toml", view.NextSkill.AgentDefinitionPath)
	})
}

func TestNextUsesCurrentLinkedWorktreeAdaptersForSkillPaths(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")

		worktreeRoot := filepath.Join(t.TempDir(), "linked-worktree")
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", "feat/next-tool-path", "HEAD")

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(worktreeRoot))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		require.NoError(t, bootstrap.InitWorkspace(worktreeRoot, []string{"codex"}, false))

		slug := createGovernedRequest(t, root, "L2", "linked worktree adapter paths")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		_, err = os.Stat(filepath.Join(root, ".codex"))
		assert.True(t, os.IsNotExist(err), "main scope should not need codex adapters for this regression")
		_, err = os.Stat(filepath.Join(worktreeRoot, ".codex", "skills", "slipway-plan-audit", "SKILL.md"))
		require.NoError(t, err)

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, ".codex/skills/slipway-plan-audit/SKILL.md", view.NextSkill.PromptPath)
		assert.Equal(t, ".codex/agents/slipway-auditor.toml", view.NextSkill.AgentDefinitionPath)
	})
}

func TestNextUsesExistingPromptPathForIntakeHost(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		create := makeNewCmd()
		create.SetArgs([]string{"--preset", "standard", "intake prompt path must exist after init"})
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
		assert.Equal(t, ".claude/skills/slipway-intake-clarification/SKILL.md", view.NextSkill.PromptPath)
		_, err := os.Stat(filepath.Join(root, view.NextSkill.PromptPath))
		require.NoError(t, err, "next should not point to a non-existent exported skill prompt")
	})
}
