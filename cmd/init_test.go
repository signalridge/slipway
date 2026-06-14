package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/toolgen"
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

		// Adapter sentinel
		_, err := os.Stat(filepath.Join(root, ".claude", "slipway", ".adapter-generated"))
		require.NoError(t, err)

		// Command entries in tool dirs (Cursor/OpenCode: flat path, others: nested)
		_, err = os.Stat(filepath.Join(root, ".cursor", "commands", "slipway-new.md"))
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(root, ".opencode", "commands", "slipway-next.md"))
		require.NoError(t, err)

		// Host-visible technique skills in tool dirs
		_, err = os.Stat(filepath.Join(root, ".codex", "skills", "slipway-codebase-mapping", "SKILL.md"))
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(root, ".codex", "skills", "slipway-tdd", "SKILL.md"))
		assert.True(t, os.IsNotExist(err), "tdd should not be exported as a host auto-trigger skill")

		// Init command entry exists
		_, err = os.Stat(filepath.Join(root, ".claude", "commands", "slipway", "init.md"))
		require.NoError(t, err)

		// Governance skills ARE in tool adapter dirs (namespace: slipway/<name>/)
		_, err = os.Stat(filepath.Join(root, ".claude", "skills", "slipway-worktree-preflight", "SKILL.md"))
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(root, ".claude", "skills", "slipway-goal-verification", "SKILL.md"))
		require.NoError(t, err)

		// Exported agent files should not be generated for any adapter.
		_, err = os.Stat(filepath.Join(root, ".claude", "agents"))
		assert.True(t, os.IsNotExist(err), "claude should not have exported agents")
		_, err = os.Stat(filepath.Join(root, ".cursor", "agents"))
		assert.True(t, os.IsNotExist(err), "cursor should not have agents")
		_, err = os.Stat(filepath.Join(root, ".gemini", "agents"))
		assert.True(t, os.IsNotExist(err), "gemini should not have exported agents")
		_, err = os.Stat(filepath.Join(root, ".opencode", "agents"))
		assert.True(t, os.IsNotExist(err), "opencode should not have exported agents")
		_, err = os.Stat(filepath.Join(root, ".codex", "agents"))
		assert.True(t, os.IsNotExist(err), "codex should not have exported agents")

		// Codex should not create project-local agent config on fresh init.
		_, err = os.Stat(filepath.Join(root, ".codex", "config.toml"))
		assert.True(t, os.IsNotExist(err), "codex should not create project-local agent config on fresh init")

		// Gemini commands should be TOML
		_, err = os.Stat(filepath.Join(root, ".gemini", "commands", "slipway", "new.toml"))
		require.NoError(t, err)

		// OpenCode command names come from file names, so use flat markdown.
		_, err = os.Stat(filepath.Join(root, ".opencode", "commands", "slipway-new.md"))
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

func TestInitCommandRefreshWithoutSentinelizedToolsReturnsStructuredError(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		expectedRoot, err := state.NormalizePath(root)
		require.NoError(t, err)

		stdout, stderr, err := runRootCommand([]string{"init", "--refresh"})
		require.Error(t, err)
		assert.Empty(t, stdout)

		payload := decodeJSONMap(t, stderr)
		assert.Equal(t, string(categoryInvalidUsage), payload["category"])
		assert.Equal(t, "init_refresh_no_sentinelized_tools", payload["error_code"])
		assert.Equal(t, float64(exitCodeInvalidUsage), payload["exit_code"])

		details, ok := payload["details"].(map[string]any)
		require.True(t, ok, "details must be present")
		assert.Equal(t, "", details["tool"])
		assert.Equal(t, true, details["refresh"])
		assert.Equal(t, expectedRoot, details["workspace_root"])

		rawDetected, ok := details["detected_tools"].([]any)
		require.True(t, ok, "detected_tools must be a JSON array")
		assert.Len(t, rawDetected, 0)
		assert.Contains(t, payload["remediation"], "slipway init --tools <tool> --refresh")
	})
}

func TestInitCommandExplicitToolWithoutSentinelReturnsStructuredError(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		expectedRoot, err := state.NormalizePath(root)
		require.NoError(t, err)

		commandPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(commandPath), 0o755))
		require.NoError(t, os.WriteFile(commandPath, []byte("legacy command"), 0o644))

		stdout, stderr, err := runRootCommand([]string{"init", "--tools", "claude"})
		require.Error(t, err)
		assert.Empty(t, stdout)

		payload := decodeJSONMap(t, stderr)
		assert.Equal(t, string(categoryInvalidUsage), payload["category"])
		assert.Equal(t, "init_missing_sentinel_existing_tree", payload["error_code"])
		assert.Equal(t, float64(exitCodeInvalidUsage), payload["exit_code"])

		details, ok := payload["details"].(map[string]any)
		require.True(t, ok, "details must be present")
		assert.Equal(t, "claude", details["tool"])
		assert.Equal(t, false, details["refresh"])
		assert.Equal(t, expectedRoot, details["workspace_root"])
		assert.NotContains(t, details, "detected_tools")
		assert.Contains(t, payload["remediation"], "slipway init --tools claude --refresh")
	})
}

func TestWorkspaceAdapterSentinelContracts(t *testing.T) {
	t.Run("generated marker path", func(t *testing.T) {
		for _, cfg := range toolgen.Registry() {
			assert.Equal(t,
				filepath.Join(toolgen.ToolRootPath(cfg), "slipway", ".adapter-generated"),
				toolgen.GeneratedAdapterMarkerPath(cfg),
				"%s marker path drifted from tool root contract",
				cfg.ID,
			)
		}
	})

	t.Run("detect existing tools", func(t *testing.T) {
		root := t.TempDir()
		assert.Empty(t, toolgen.DetectExistingTools(root))

		require.NoError(t, os.MkdirAll(filepath.Join(root, ".claude"), 0o755))
		assert.Empty(t, toolgen.DetectExistingTools(root), "bare tool roots must not count as generated adapters")

		writeGeneratedAdapterMarkerForTest(t, root, "claude")
		writeGeneratedAdapterMarkerForTest(t, root, "cursor")
		assert.Equal(t, []string{"claude", "cursor"}, toolgen.DetectExistingTools(root))
	})

	t.Run("refresh auto-detects sentinelized adapters", func(t *testing.T) {
		root := t.TempDir()
		withWorkspace(t, root, func() {
			cmdPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")

			_, _, err := runRootCommand([]string{"init", "--tools", "claude"})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(cmdPath, []byte("tampered"), 0o644))

			_, _, err = runRootCommand([]string{"init", "--refresh"})
			require.NoError(t, err)

			content, readErr := os.ReadFile(cmdPath)
			require.NoError(t, readErr)
			assert.NotEqual(t, "tampered", string(content))
		})
	})

	t.Run("refresh without sentinel requires explicit tools", func(t *testing.T) {
		root := t.TempDir()
		withWorkspace(t, root, func() {
			_, stderr, err := runRootCommand([]string{"init", "--refresh"})
			require.Error(t, err)

			payload := decodeJSONMap(t, stderr)
			assert.Equal(t, "init_refresh_no_sentinelized_tools", payload["error_code"])
			assert.Contains(t, payload["remediation"], "slipway init --tools <tool> --refresh")
		})
	})

	t.Run("explicit tools none does not auto-detect", func(t *testing.T) {
		root := t.TempDir()
		withWorkspace(t, root, func() {
			cmdPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")

			_, _, err := runRootCommand([]string{"init", "--tools", "claude"})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(cmdPath, []byte("tampered"), 0o644))

			_, _, err = runRootCommand([]string{"init", "--tools", "none", "--refresh"})
			require.NoError(t, err)

			content, readErr := os.ReadFile(cmdPath)
			require.NoError(t, readErr)
			assert.Equal(t, "tampered", string(content), "--tools none must not refresh detected adapters")

			_, err = os.Stat(filepath.Join(root, ".cursor", "slipway", ".adapter-generated"))
			assert.True(t, os.IsNotExist(err), "explicit none must not generate other adapters")
		})
	})

	t.Run("clean tree no sentinel explicit tool succeeds", func(t *testing.T) {
		root := t.TempDir()
		withWorkspace(t, root, func() {
			_, _, err := runRootCommand([]string{"init", "--tools", "claude"})
			require.NoError(t, err)

			_, statErr := os.Stat(filepath.Join(root, ".claude", "slipway", ".adapter-generated"))
			require.NoError(t, statErr)
		})
	})

	t.Run("explicit tool without refresh rejects missing-sentinel existing tree", func(t *testing.T) {
		root := t.TempDir()
		withWorkspace(t, root, func() {
			commandPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")
			require.NoError(t, os.MkdirAll(filepath.Dir(commandPath), 0o755))
			require.NoError(t, os.WriteFile(commandPath, []byte("legacy command"), 0o644))

			_, stderr, err := runRootCommand([]string{"init", "--tools", "claude"})
			require.Error(t, err)

			payload := decodeJSONMap(t, stderr)
			assert.Equal(t, "init_missing_sentinel_existing_tree", payload["error_code"])
			assert.Contains(t, payload["remediation"], "slipway init --tools claude --refresh")
		})
	})

	t.Run("codex global prompts do not count as workspace-local existing tree", func(t *testing.T) {
		root := t.TempDir()
		withWorkspace(t, root, func() {
			codexHome := t.TempDir()
			t.Setenv("CODEX_HOME", codexHome)

			stalePrompt := filepath.Join(codexHome, "prompts", "slipway-new.md")
			require.NoError(t, os.MkdirAll(filepath.Dir(stalePrompt), 0o755))
			require.NoError(t, os.WriteFile(stalePrompt, []byte("stale global prompt"), 0o644))

			_, _, err := runRootCommand([]string{"init", "--tools", "codex"})
			require.NoError(t, err)

			_, statErr := os.Stat(filepath.Join(root, ".codex", "slipway", ".adapter-generated"))
			require.NoError(t, statErr)
		})
	})

	t.Run("codex legacy command prompts prune only after successful rewrite", func(t *testing.T) {
		root := t.TempDir()
		codexHome := t.TempDir()
		t.Setenv("CODEX_HOME", codexHome)

		require.NoError(t, toolgen.Generate(root, []string{"codex"}, true))

		legacyPrompt := filepath.Join(codexHome, "prompts", "slipway-new.md")
		userPrompt := filepath.Join(codexHome, "prompts", "slipway-stale.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(legacyPrompt), 0o755))
		require.NoError(t, os.WriteFile(legacyPrompt, []byte("legacy generated command prompt"), 0o644))
		require.NoError(t, os.WriteFile(userPrompt, []byte("user prompt"), 0o644))

		blockedSkillDir := filepath.Join(root, ".codex", "skills", "slipway-intake-clarification")
		require.NoError(t, os.RemoveAll(blockedSkillDir))
		require.NoError(t, os.WriteFile(blockedSkillDir, []byte("blocked"), 0o644))

		err := toolgen.Generate(root, []string{"codex"}, true)
		require.Error(t, err)

		_, statErr := os.Stat(legacyPrompt)
		require.NoError(t, statErr, "failed codex refresh must not prune legacy command prompts before rewrite succeeds")
		_, statErr = os.Stat(userPrompt)
		require.NoError(t, statErr, "failed codex refresh must not prune user prompts before rewrite succeeds")

		require.NoError(t, os.Remove(blockedSkillDir))
		require.NoError(t, toolgen.Generate(root, []string{"codex"}, true))

		_, statErr = os.Stat(legacyPrompt)
		assert.True(t, os.IsNotExist(statErr), "successful codex refresh must prune legacy generated command prompts")
		_, statErr = os.Stat(userPrompt)
		assert.NoError(t, statErr, "successful codex refresh must preserve user-owned slipway-* prompts")
	})

	t.Run("typed cli error for fail-closed init", func(t *testing.T) {
		root := t.TempDir()
		withWorkspace(t, root, func() {
			expectedRoot, err := state.NormalizePath(root)
			require.NoError(t, err)

			_, stderr, err := runRootCommand([]string{"init", "--refresh"})
			require.Error(t, err)
			payload := decodeJSONMap(t, stderr)
			assert.Equal(t, string(categoryInvalidUsage), payload["category"])
			assert.Equal(t, "init_refresh_no_sentinelized_tools", payload["error_code"])
			details, ok := payload["details"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, expectedRoot, details["workspace_root"])

			commandPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")
			require.NoError(t, os.MkdirAll(filepath.Dir(commandPath), 0o755))
			require.NoError(t, os.WriteFile(commandPath, []byte("legacy command"), 0o644))

			_, stderr, err = runRootCommand([]string{"init", "--tools", "claude"})
			require.Error(t, err)
			payload = decodeJSONMap(t, stderr)
			assert.Equal(t, string(categoryInvalidUsage), payload["category"])
			assert.Equal(t, "init_missing_sentinel_existing_tree", payload["error_code"])
		})
	})

	t.Run("refresh failure leaves no trusted command prompt surface", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, toolgen.Generate(root, []string{"claude"}, true))

		sentinelPath := filepath.Join(root, ".claude", "slipway", ".adapter-generated")
		commandPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")
		require.FileExists(t, sentinelPath)
		require.FileExists(t, commandPath)

		blockedSkillDir := filepath.Join(root, ".claude", "skills", "slipway-intake-clarification")
		require.NoError(t, os.RemoveAll(blockedSkillDir))
		require.NoError(t, os.WriteFile(blockedSkillDir, []byte("blocked"), 0o644))

		err := toolgen.Generate(root, []string{"claude"}, true)
		require.Error(t, err)

		_, statErr := os.Stat(sentinelPath)
		assert.True(t, os.IsNotExist(statErr), "failed refresh must leave the sentinel absent")
		_, statErr = os.Stat(commandPath)
		assert.True(t, os.IsNotExist(statErr), "failed refresh must not leave previously trusted command prompts in place")
	})
}

// TestInitCommandCodexPrintsInvocationSurface asserts the init success path
// surfaces the actual Codex invocation surface (issue #210), so a user does not
// have to inspect source or docs to discover how to invoke the generated skills.
func TestInitCommandCodexPrintsInvocationSurface(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())
	withWorkspace(t, root, func() {
		stdout, stderr, err := runRootCommand([]string{"init", "--tools", "codex"})
		require.NoError(t, err, "stderr: %s", stderr)

		cfg, ok := toolgen.LookupTool("codex")
		require.True(t, ok)
		assert.Contains(t, stdout, "codex: "+cfg.InvocationSummary())
		assert.Contains(t, stdout, "$slipway-<command>")
		assert.Contains(t, stdout, "/skills")
	})
}

func writeGeneratedAdapterMarkerForTest(t *testing.T, root, toolID string) {
	t.Helper()

	cfg, ok := toolgen.LookupTool(toolID)
	require.True(t, ok, "tool %q must exist", toolID)

	markerPath := filepath.Join(root, toolgen.GeneratedAdapterMarkerPath(cfg))
	require.NoError(t, os.MkdirAll(filepath.Dir(markerPath), 0o755))
	require.NoError(t, os.WriteFile(markerPath, []byte("generated\n"), 0o644))
}
