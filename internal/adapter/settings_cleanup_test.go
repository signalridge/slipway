package adapter

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONSettingsCleanupRemovesOnlyRetiredSlipwayHooks(t *testing.T) {
	t.Parallel()
	root := filepath.ToSlash(filepath.Join(t.TempDir(), "repo"))
	otherRoot := filepath.ToSlash(filepath.Join(t.TempDir(), "other"))
	raw := []byte(fmt.Sprintf(`{
  "theme": "dark",
  "hooks": {
    "SessionStart": [
      {"matcher":"startup","hooks":[
        {"type":"command","command":"slipway hook session-start --tool claude"},
        {"type":"command","command":"echo keep"}
      ]},
      {"hooks":[{"type":"command","command":"bash .claude/hooks/slipway-session-start.sh"}]},
      {"hooks":[{"type":"command","command":"/opt/custom/slipway-session-start.sh"}]},
      {"hooks":[{"type":"command","command":"slipway hook context-pressure"}]},
      {"hooks":[{"type":"command","command":"slipway hook session-start --tool qwen"}]},
      {"hooks":[{"type":"command","command":"bash .claude/hooks/slipway-session-startup.sh"}]},
      {"hooks":[{"type":"command","command":"echo slipway hook session-start"}]},
      {"hooks":[{"type":"command","command":"bash -lc \"echo .claude/hooks/slipway-session-start\""}]},
      {"note":"slipway hook session-start"},
      {"hooks":[{"type":"command","command":"go -C %s run . hook session-start --tool claude"}]},
      {"hooks":[{"type":"command","command":"go -C %s run . hook session-start"}]}
    ],
    "PostToolUse": [
      {"hooks":[{"type":"command","command":"slipway hook context-pressure || exit 0"}]},
      {"hooks":[{"type":"command","command":"echo post"}]}
    ],
    "PreToolUse": [{"hooks":[{"type":"command","command":"slipway hook session-start"}]}]
  }
}`, root, otherRoot))
	updated, changed, err := cleanJSONHooks(raw, "claude", root)
	require.NoError(t, err)
	require.True(t, changed)
	text := string(updated)
	assert.NotContains(t, text, "hook session-start --tool claude")
	assert.NotContains(t, text, "slipway hook context-pressure || exit 0")
	assert.Contains(t, text, "echo keep")
	assert.Contains(t, text, "slipway-session-startup.sh")
	assert.Contains(t, text, "/opt/custom/slipway-session-start.sh")
	assert.Contains(t, text, "slipway hook context-pressure")
	assert.Contains(t, text, "slipway hook session-start --tool qwen")
	assert.Contains(t, text, "echo slipway hook session-start")
	assert.Contains(t, text, `bash -lc \"echo .claude/hooks/slipway-session-start\"`)
	assert.Contains(t, text, `"note": "slipway hook session-start"`)
	assert.NotContains(t, text, "go -C "+root+" run . hook session-start")
	assert.Contains(t, text, "go -C "+otherRoot+" run . hook session-start")
	assert.Contains(t, text, "echo post")
	assert.Contains(t, text, "PreToolUse")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(updated, &parsed))
	_, changedAgain, err := cleanJSONHooks(updated, "claude", root)
	require.NoError(t, err)
	assert.False(t, changedAgain)
}

func TestRetiredHookMatcherRequiresCompleteManagedCommand(t *testing.T) {
	t.Parallel()
	root := filepath.ToSlash(filepath.Join(t.TempDir(), "repo"))
	otherRoot := filepath.ToSlash(filepath.Join(t.TempDir(), "other"))
	cases := []struct {
		name    string
		host    string
		event   string
		command string
		retired bool
	}{
		{name: "direct hook", host: "claude", event: "SessionStart", command: "slipway hook session-start --tool claude", retired: true},
		{name: "qwen direct hook", host: "qwen", event: "SessionStart", command: "slipway hook session-start --tool qwen", retired: true},
		{name: "context no-op wrapper", host: "claude", event: "PostToolUse", command: "slipway hook context-pressure || exit 0", retired: true},
		{name: "custom absolute slipway executable", host: "claude", event: "SessionStart", command: "/opt/custom/slipway hook session-start --tool claude"},
		{name: "custom Windows slipway executable", host: "claude", event: "SessionStart", command: `C:\custom\slipway.exe hook session-start --tool claude`},
		{name: "canonical go hook", host: "claude", event: "SessionStart", command: "go -C " + root + " run . hook session-start --tool claude", retired: true},
		{name: "shell launcher", host: "claude", event: "SessionStart", command: "bash .claude/hooks/slipway-session-start.sh", retired: true},
		{name: "absolute managed launcher", host: "claude", event: "SessionStart", command: filepath.ToSlash(filepath.Join(root, ".claude/hooks/slipway-session-start.cmd")), retired: true},
		{name: "project variable launcher", host: "claude", event: "SessionStart", command: `$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.sh`, retired: true},
		{name: "cross root absolute launcher", host: "claude", event: "SessionStart", command: filepath.ToSlash(filepath.Join(otherRoot, ".claude/hooks/slipway-session-start.sh"))},
		{name: "direct launcher", host: "claude", event: "SessionStart", command: ".claude/hooks/slipway-session-start.sh", retired: true},
		{name: "Windows launcher path", host: "claude", event: "SessionStart", command: `.claude\hooks\slipway-session-start.cmd`, retired: true},
		{name: "cross host tool", host: "claude", event: "SessionStart", command: "slipway hook session-start --tool qwen"},
		{name: "wrong event command", host: "claude", event: "SessionStart", command: "slipway hook context-pressure"},
		{name: "wrong event launcher", host: "claude", event: "PostToolUse", command: ".claude/hooks/slipway-session-start.sh"},
		{name: "cross host launcher", host: "claude", event: "SessionStart", command: ".qwen/hooks/slipway-session-start.sh"},
		{name: "custom same basename", host: "claude", event: "SessionStart", command: "/opt/custom/slipway-session-start.sh"},
		{name: "chained slipway", host: "claude", event: "SessionStart", command: "slipway hook session-start && echo keep"},
		{name: "semicolon go", host: "claude", event: "SessionStart", command: "go -C " + root + " run . hook session-start; echo keep"},
		{name: "piped shell", host: "claude", event: "SessionStart", command: "bash .claude/hooks/slipway-session-start.sh | tee keep"},
		{name: "background launcher", host: "claude", event: "SessionStart", command: ".claude/hooks/slipway-session-start.sh & echo keep"},
		{name: "wrapper followed by chain", host: "claude", event: "PostToolUse", command: "slipway hook context-pressure || exit 0 && echo keep"},
		{name: "unknown tool", host: "claude", event: "SessionStart", command: "slipway hook session-start --tool user-script"},
		{name: "unknown suffix", host: "claude", event: "PostToolUse", command: "slipway hook context-pressure --force"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.retired, isRetiredHook(test.command, root, test.host, test.event))
		})
	}
}

func TestJSONSettingsCleanupPreservesCrossRootLauncher(t *testing.T) {
	t.Parallel()
	root := filepath.ToSlash(filepath.Join(t.TempDir(), "current"))
	other := filepath.ToSlash(filepath.Join(t.TempDir(), "other"))
	raw := []byte(fmt.Sprintf(`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"%s/.claude/hooks/slipway-session-start.sh"}]}]}}`, other))
	updated, changed, err := cleanJSONHooks(raw, "claude", root)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Nil(t, updated)
}

func TestJSONSettingsCleanupPreservesChainedCommands(t *testing.T) {
	t.Parallel()
	root := filepath.ToSlash(filepath.Join(t.TempDir(), "repo"))
	commands := []string{
		"slipway hook session-start && echo keep",
		"go -C " + root + " run . hook session-start; echo keep",
		"bash .claude/hooks/slipway-session-start.sh | tee keep",
		".claude/hooks/slipway-session-start.sh & echo keep",
		"slipway hook context-pressure || exit 0 && echo keep",
	}
	for _, command := range commands {
		raw, err := json.Marshal(map[string]any{
			"hooks": map[string]any{
				"SessionStart": []any{map[string]any{
					"hooks": []any{map[string]any{"type": "command", "command": command}},
				}},
			},
		})
		require.NoError(t, err)
		updated, changed, err := cleanJSONHooks(raw, "claude", root)
		require.NoError(t, err)
		assert.False(t, changed, command)
		assert.Nil(t, updated, command)
	}
}

func TestQwenCleanupLeavesPostToolUseUntouched(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"hooks":{"PostToolUse":[{"command":"slipway hook context-pressure"}],"SessionStart":[{"command":"slipway hook session-start"}]}}`)
	updated, changed, err := cleanJSONHooks(raw, "qwen", "/repo")
	require.NoError(t, err)
	require.True(t, changed)
	assert.Contains(t, string(updated), "PostToolUse")
	assert.Contains(t, string(updated), "context-pressure")
	assert.NotContains(t, string(updated), "SessionStart")
}

func TestJSONSettingsCleanupRejectsTrailingDataWithoutRewriting(t *testing.T) {
	raw := []byte(`{"hooks":{"SessionStart":[{"command":"slipway hook session-start"}]}} trailing`)
	updated, changed, err := cleanJSONHooks(raw, "claude", "/repo")
	require.Error(t, err)
	assert.False(t, changed)
	assert.Nil(t, updated)
	assert.Contains(t, err.Error(), "trailing JSON data")
}

func TestCodexCleanupRemovesExactlyOneManagedBlock(t *testing.T) {
	t.Parallel()
	raw := []byte("model = \"gpt\"\n" + codexBegin + "\nnotify = [\"slipway\"]\n" + codexEnd + "\napproval_policy = \"ask\"\n")
	updated, changed, err := removeManagedBlock(raw)
	require.NoError(t, err)
	require.True(t, changed)
	assert.Equal(t, "model = \"gpt\"\napproval_policy = \"ask\"\n", string(updated))

	_, changed, err = removeManagedBlock(updated)
	require.NoError(t, err)
	assert.False(t, changed)

	_, _, err = removeManagedBlock([]byte(strings.Repeat(codexBegin+"\n", 2) + codexEnd))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unbalanced")
}

func TestCodexCleanupPreservesMarkerTextThatIsNotAnExactLine(t *testing.T) {
	raw := []byte("note = \"" + codexBegin + "\"\nmessage = \"" + codexEnd + "\"\n")
	updated, changed, err := removeManagedBlock(raw)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Nil(t, updated)
}
