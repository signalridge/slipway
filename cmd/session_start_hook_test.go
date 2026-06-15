package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStartHookEmitsCompiledHandoff(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "session-start compiled handoff")

		handoffPath := filepath.Join(state.GitStateDir(root), "runtime", "handoff.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(handoffPath), 0o755))
		require.NoError(t, os.WriteFile(handoffPath, []byte("handoff body must not be embedded"), 0o644))

		cmd := makeHookCmd()
		cmd.SetArgs([]string{"session-start", "--tool", "claude"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		body := out.String()
		assert.Contains(t, body, `<slipway-session-start tool="claude">`)
		assert.Contains(t, body, `"slug": "`+slug+`"`)
		assert.Contains(t, body, "slipway_entry_skill:")
		assert.Contains(t, body, "session_handoff_present: true")
		assert.Contains(t, body, "session_handoff_path: "+handoffPath)
		assert.NotContains(t, body, "handoff body must not be embedded")
	})
}

func TestSessionStartHookTreatsBoundWorktreeChangeAsInformational(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		worktreePath := filepath.Join(t.TempDir(), "bound-worktree")
		runGit(t, root, "worktree", "add", worktreePath, "-b", "bound-worktree")
		change := model.NewChange("bound-change")
		change.WorktreePath = worktreePath
		require.NoError(t, state.SaveChange(root, change))
		normalizedWorktreePath, err := state.NormalizePath(worktreePath)
		require.NoError(t, err)

		cmd := makeHookCmd()
		cmd.SetArgs([]string{"session-start", "--tool", "claude"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		body := out.String()
		assert.Contains(t, body, "session_handoff_info: no active change in this worktree")
		assert.Contains(t, body, "active change bound-change is bound to "+normalizedWorktreePath)
		assert.Contains(t, body, "use --change bound-change to act")
		assert.NotContains(t, body, "hook_diagnostic: slipway next --json failed:")
	})
}

func TestSessionStartHookSurfacesRootFailureDiagnostic(t *testing.T) {
	root := t.TempDir()
	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	defer func() {
		_ = os.Chdir(previousWD)
	}()

	cmd := makeHookCmd()
	cmd.SetArgs([]string{"session-start", "--tool", `bad"tool`})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())

	body := out.String()
	assert.Contains(t, body, `<slipway-session-start tool="bad&quot;tool">`)
	assert.Contains(t, body, "hook_diagnostic: slipway root failed:")
}
