package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertNoSessionStartChangeState pins REQ-004: the SessionStart hook must not
// auto-inject any per-session change-state — neither the active-worktree
// `next --json` view, nor the bound-elsewhere pointer, nor the handoff summary.
func assertNoSessionStartChangeState(t *testing.T, body string) {
	t.Helper()
	assert.NotContains(t, body, "session_handoff")
	assert.NotContains(t, body, "session_handoff_info")
	assert.NotContains(t, body, "session_handoff_present")
	assert.NotContains(t, body, "session_handoff_path")
	assert.NotContains(t, body, `"current_state"`)
	assert.NotContains(t, body, `"next_skill"`)
	assert.NotContains(t, body, "bound to")
	assert.NotContains(t, body, "--change")
}

func TestSessionStartHookEmitsOnlyEntrySkillPointer(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		// An active change of this worktree must NOT cause any change-state
		// auto-injection; only the entry-skill routing pointer is emitted.
		createGovernedRequest(t, root, levelNonDiscovery, "session-start entry-skill only")

		cmd := makeHookCmd()
		cmd.SetArgs([]string{"session-start", "--tool", "claude"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		body := out.String()
		assert.Contains(t, body, `<slipway-session-start tool="claude">`)
		assert.Contains(t, body, "slipway_entry_skill:")
		assert.Contains(t, body, `load the "slipway" skill`)
		assertNoSessionStartChangeState(t, body)
		assert.NotContains(t, body, "hook_diagnostic:")
	})
}

func TestSessionStartHookEmitsCodexAdditionalContext(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createGovernedRequest(t, root, levelNonDiscovery, "codex session-start entry skill")

		cmd := makeHookCmd()
		cmd.SetArgs([]string{"session-start", "--tool", "codex"})
		cmd.SetIn(strings.NewReader(`{"hook_event_name":"SessionStart","source":"compact"}`))
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		body := out.String()
		assert.NotContains(t, body, "<slipway-session-start")
		assert.Contains(t, body, `"hookEventName":"SessionStart"`)
		assert.Contains(t, body, "additionalContext")
		var payload map[string]map[string]string
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		additionalContext := payload["hookSpecificOutput"]["additionalContext"]
		assert.Contains(t, additionalContext, "slipway_entry_skill:")
		assertNoSessionStartChangeState(t, additionalContext)
	})
}

func TestSessionStartHookBoundElsewhereEmitsOnlyEntrySkill(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		// A change bound to another worktree must no longer produce a
		// "session_handoff_info: ... bound to <worktree>" pointer; the hook emits
		// only the entry-skill routing pointer.
		worktreePath := t.TempDir() + "/bound-worktree"
		runGit(t, root, "worktree", "add", worktreePath, "-b", "bound-worktree")

		cmd := makeHookCmd()
		cmd.SetArgs([]string{"session-start", "--tool", "claude"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		body := out.String()
		assert.Contains(t, body, "slipway_entry_skill:")
		assertNoSessionStartChangeState(t, body)
		assert.NotContains(t, body, "hook_diagnostic: slipway next --json failed:")
	})
}

func TestSessionStartHookBareCommandOmitsUnknownToolAttribute(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createGovernedRequest(t, root, levelNonDiscovery, "session-start bare command")

		cmd := makeHookCmd()
		cmd.SetArgs([]string{"session-start"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		body := out.String()
		assert.Contains(t, body, `<slipway-session-start>`)
		assert.NotContains(t, body, `tool="unknown"`)
		assert.NotContains(t, body, `tool=""`)
		assert.Contains(t, body, "slipway_entry_skill:")
		assertNoSessionStartChangeState(t, body)
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
	assert.Contains(t, body, "slipway_entry_skill:")
	assert.Contains(t, body, "hook_diagnostic: slipway root failed:")
}

// TestSessionStartHookFailsSilentOnUnusableInput pins the fail-silent contract:
// the SessionStart hook is inlined into automatic host hooks, so it must always
// exit 0 (Execute returns nil, never panics) even when stdin is empty or
// malformed garbage. The subcommand does not consume stdin, but the contract
// must hold for any host-supplied input.
func TestSessionStartHookFailsSilentOnUnusableInput(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createGovernedRequest(t, root, levelNonDiscovery, "session-start fail-silent input")

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
