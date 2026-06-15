package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func TestSessionStartHookBareCommandOmitsUnknownToolAttribute(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "session-start bare command")

		cmd := makeHookCmd()
		cmd.SetArgs([]string{"session-start"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		body := out.String()
		assert.Contains(t, body, `<slipway-session-start>`)
		assert.NotContains(t, body, `tool="unknown"`)
		assert.NotContains(t, body, `tool=""`)
		assert.Contains(t, body, `"slug": "`+slug+`"`)
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

// TestSessionStartHookFailsSilentOnUnusableInput pins REQ-003: the SessionStart
// hook is inlined into automatic host hooks, so it must always exit 0 (Execute
// returns nil, never panics) even when stdin is empty or malformed garbage. The
// subcommand does not consume stdin, but the fail-silent contract must hold for
// any host-supplied input.
func TestSessionStartHookFailsSilentOnUnusableInput(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createGovernedRequest(t, root, "L2", "session-start fail-silent input")

		tests := []struct {
			name  string
			stdin string
		}{
			{
				name:  "empty stdin",
				stdin: "",
			},
			{
				name:  "whitespace only stdin",
				stdin: "   \n\t  \n",
			},
			{
				name:  "garbage non-json stdin",
				stdin: "this is not json at all <<>>",
			},
			{
				name:  "truncated json",
				stdin: `{"hook_event_name":"SessionStart",`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cmd := makeHookCmd()
				cmd.SetArgs([]string{"session-start", "--tool", "claude"})
				cmd.SetIn(strings.NewReader(tt.stdin))
				var out bytes.Buffer
				cmd.SetOut(&out)

				require.NotPanics(t, func() {
					require.NoError(t, cmd.Execute())
				})
			})
		}
	})
}
