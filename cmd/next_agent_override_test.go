package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIEndToEndNextJSONReflectsConfiguredAgentOverride(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		cfg, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		cfg.Agents.Mappings["research-orchestration"] = "slipway-planner"
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		slug := createGovernedRequest(t, root, "L3", "next should reflect configured agent override")

		stdout, stderr, err := runRootCommand([]string{"next", "--json", "--change", slug})
		require.NoError(t, err)
		assert.Empty(t, stderr)

		nextPayload := decodeJSONMap(t, stdout)
		nextSkill, ok := nextPayload["next_skill"].(map[string]any)
		require.True(t, ok, "expected next_skill in next output")
		assert.Equal(t, "research-orchestration", nextSkill["name"])
		assert.Equal(t, "slipway-planner", nextSkill["agent_hint"])
		assert.Contains(t, nextSkill["agent_definition_path"], "slipway-planner")
	})
}

func TestCLIEndToEndNextJSONRejectsManualOnlyConfiguredAgentOverride(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		cfg, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		cfg.Agents.Mappings["research-orchestration"] = "slipway-executor"
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		slug := createGovernedRequest(t, root, "L3", "next should reject manual-only configured agent override")

		_, _, err = runRootCommand([]string{"next", "--json", "--change", slug})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "manual-only")
	})
}

func TestCLIEndToEndNextJSONUsesCanonicalScopeConfigFromBoundWorktree(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", "README.md")
		runGit(t, root, "commit", "-m", "init")
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		cfg, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		cfg.Agents.Mappings["research-orchestration"] = "slipway-planner"
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		slug := createGovernedRequest(t, root, "L3", "next should ignore drifted worktree config mirror")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		worktreeRoot := filepath.Join(t.TempDir(), slug)
		branch := "feat/" + slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")
		require.NoError(t, bootstrap.InitWorkspace(worktreeRoot, []string{"claude"}, false))

		bound := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&bound, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, change, bound))
		require.NoError(t, state.SaveChange(root, bound))

		worktreeCfg, err := model.LoadConfig(state.ConfigPath(worktreeRoot))
		require.NoError(t, err)
		worktreeCfg.Agents.Mappings["research-orchestration"] = "slipway-reviewer"
		require.NoError(t, model.SaveConfig(state.ConfigPath(worktreeRoot), worktreeCfg))

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(worktreeRoot))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		stdout, stderr, err := runRootCommand([]string{"next", "--json", "--change", slug})
		require.NoError(t, err)
		assert.Empty(t, stderr)

		nextPayload := decodeJSONMap(t, stdout)
		nextSkill, ok := nextPayload["next_skill"].(map[string]any)
		require.True(t, ok, "expected next_skill in next output")
		assert.Equal(t, "research-orchestration", nextSkill["name"])
		assert.Equal(t, "slipway-planner", nextSkill["agent_hint"])
		assert.Contains(t, nextSkill["agent_definition_path"], "slipway-planner")
	})
}
