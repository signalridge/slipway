package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCommandToolsNone(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		cmd := makeInitCmd()
		cmd.SetArgs([]string{"--tools", "none"})
		require.NoError(t, cmd.Execute())

		_, err := os.Stat(filepath.Join(root, ".slipway.yaml"))
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(root, ".claude"))
		assert.True(t, os.IsNotExist(err))
	})
}

func TestInitCommandToolsAll(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		cmd := makeInitCmd()
		cmd.SetArgs([]string{"--tools", "all", "--refresh"})
		require.NoError(t, cmd.Execute())

		// Adapter skills in tool dirs (namespace: slipway/<id>/)
		_, err := os.Stat(filepath.Join(root, ".claude", "skills", "slipway", "new", "SKILL.md"))
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(root, ".claude", "skills", "slipway", "next", "SKILL.md"))
		require.NoError(t, err)

		// Command entries in tool dirs (cursor: flat path, others: nested)
		_, err = os.Stat(filepath.Join(root, ".cursor", "commands", "slipway-new.md"))
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(root, ".opencode", "commands", "slipway", "next.md"))
		require.NoError(t, err)

		// Technique skills in tool dirs
		_, err = os.Stat(filepath.Join(root, ".codex", "skills", "slipway", "tdd", "SKILL.md"))
		require.NoError(t, err)

		// Init command skill exists
		_, err = os.Stat(filepath.Join(root, ".claude", "skills", "slipway", "init", "SKILL.md"))
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(root, ".claude", "commands", "slipway", "init.md"))
		require.NoError(t, err)

		// Governance skills ARE in tool adapter dirs (namespace: slipway/<name>/)
		_, err = os.Stat(filepath.Join(root, ".claude", "skills", "slipway", "worktree-preflight", "SKILL.md"))
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(root, ".claude", "skills", "slipway", "goal-verification", "SKILL.md"))
		require.NoError(t, err)

		// Agent definitions in tool dirs
		_, err = os.Stat(filepath.Join(root, ".claude", "agents", "slipway-analyst.md"))
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(root, ".claude", "agents", "slipway-orchestrator.md"))
		require.NoError(t, err)

		// Cursor should NOT have agents (AgentStyle is "")
		_, err = os.Stat(filepath.Join(root, ".cursor", "agents"))
		assert.True(t, os.IsNotExist(err), "cursor should not have agents")

		// Codex agents should be TOML
		_, err = os.Stat(filepath.Join(root, ".codex", "agents", "slipway-executor.toml"))
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(root, ".codex", "config.toml"))
		require.NoError(t, err)

		// Gemini commands should be TOML
		_, err = os.Stat(filepath.Join(root, ".gemini", "commands", "slipway", "new.toml"))
		require.NoError(t, err)

		// OpenCode commands should be nested markdown
		_, err = os.Stat(filepath.Join(root, ".opencode", "commands", "slipway", "new.md"))
		require.NoError(t, err)

		// Codex should NOT have project-local commands
		_, err = os.Stat(filepath.Join(root, ".codex", "commands"))
		assert.True(t, os.IsNotExist(err), "codex should not have project-local commands")

		// No hidden .slipway/ directory contract remains.
		_, err = os.Stat(filepath.Join(root, ".slipway"))
		assert.True(t, os.IsNotExist(err))

	})
}

func TestInitCommandFromLinkedWorktreeMakesCurrentWorkspaceUsable(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")

		worktreeRoot := filepath.Join(t.TempDir(), "linked-worktree")
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", "feat/init-linked-worktree", "HEAD")

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(worktreeRoot))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		cmd := makeInitCmd()
		cmd.SetArgs([]string{"--tools", "none"})
		require.NoError(t, cmd.Execute())

		resolvedRoot, err := projectRootFromWD()
		require.NoError(t, err)
		expectedRoot, err := filepath.EvalSymlinks(root)
		require.NoError(t, err)
		assert.Equal(t, expectedRoot, resolvedRoot)
	})
}
