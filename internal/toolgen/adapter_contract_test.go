package toolgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type frozenSurfaceBase string

const (
	frozenSurfaceRoot      frozenSurfaceBase = "root"
	frozenSurfaceCodexHome frozenSurfaceBase = "codex_home"
)

type frozenHookContract struct {
	Event      string
	Path       string
	Registered bool
}

type frozenToolContract struct {
	CommandBase   frozenSurfaceBase
	CommandRoot   string
	CommandStyle  string
	CommandExt    string
	TriggerPrefix string
	TriggerStyle  string
	SettingsPath  string
	SessionHook   frozenHookContract
	PostToolHook  frozenHookContract
}

var frozenAdapterCommandIDs = []string{
	"abort",
	"cancel",
	"checkpoint",
	"codebase-map",
	"delete",
	"done",
	"evidence",
	"health",
	"init",
	"instructions",
	"learn",
	"new",
	"next",
	"pivot",
	"preset",
	"repair",
	"review",
	"run",
	"stats",
	"status",
	"validate",
}

var frozenToolOrder = []string{
	"claude",
	"codex",
	"cursor",
	"gemini",
	"opencode",
}

var frozenToolContracts = map[string]frozenToolContract{
	"claude": {
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".claude/commands",
		CommandStyle:  "nested",
		CommandExt:    ".md",
		TriggerPrefix: "/slipway",
		TriggerStyle:  "slash-colon",
		SettingsPath:  ".claude/settings.json",
		SessionHook: frozenHookContract{
			Event:      "SessionStart",
			Path:       ".claude/hooks/slipway-session-start",
			Registered: true,
		},
		PostToolHook: frozenHookContract{
			Event:      "PostToolUse",
			Path:       ".claude/hooks/slipway-context-pressure-post-tool-use",
			Registered: true,
		},
	},
	"codex": {
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".codex/skills",
		CommandStyle:  "skill",
		CommandExt:    ".md",
		TriggerPrefix: "$slipway-",
		TriggerStyle:  "dollar-mention",
	},
	"cursor": {
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".cursor/commands",
		CommandStyle:  "flat",
		CommandExt:    ".md",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		SessionHook: frozenHookContract{
			Path:       ".cursor/hooks/slipway-session-start",
			Registered: false,
		},
	},
	"gemini": {
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".gemini/commands",
		CommandStyle:  "nested",
		CommandExt:    ".toml",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		SettingsPath:  ".gemini/settings.json",
		SessionHook: frozenHookContract{
			Event:      "SessionStart",
			Path:       ".gemini/hooks/slipway-session-start",
			Registered: true,
		},
	},
	"opencode": {
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".opencode/commands",
		CommandStyle:  "flat",
		CommandExt:    ".md",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		SessionHook: frozenHookContract{
			Path:       ".opencode/hooks/slipway-session-start",
			Registered: false,
		},
	},
}

func TestAdapterContractsRemainStable(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	require.NoError(t, Generate(root, frozenToolOrder, true))

	for _, toolID := range frozenToolOrder {
		toolID := toolID
		t.Run(toolID, func(t *testing.T) {
			contract := frozenToolContracts[toolID]
			assertFrozenCommandSet(t, root, codexHome, toolID, contract)
			assertFrozenHookContract(t, root, contract.SettingsPath, contract.SessionHook)
			assertFrozenHookContract(t, root, contract.SettingsPath, contract.PostToolHook)
		})
	}
}

func assertFrozenCommandSet(t *testing.T, root, codexHome, toolID string, contract frozenToolContract) {
	t.Helper()

	// Codex exposes commands as per-command skills, not a flat command-surface
	// directory. The skills dir also holds host/governance skills, so assert each
	// frozen command id has its skill file rather than a strict directory match,
	// and assert no retired generated command prompts remain under
	// $CODEX_HOME/prompts.
	if contract.CommandStyle == "skill" {
		for _, id := range frozenAdapterCommandIDs {
			relPath := contract.commandPath(id)
			absPath := contract.resolvePath(root, codexHome, relPath)
			content, err := os.ReadFile(absPath)
			require.NoError(t, err, "missing generated command skill for %s/%s", toolID, id)
			assert.Contains(t, string(content), contract.commandContentMarker(id), "%s/%s contract marker drifted", toolID, id)
		}
		assertNoFrozenCodexCommandPrompts(t, codexHome)
		return
	}

	expectedPaths := make([]string, 0, len(frozenAdapterCommandIDs))
	for _, id := range frozenAdapterCommandIDs {
		expectedPaths = append(expectedPaths, contract.commandPath(id))
	}

	actualPaths := collectRelativeFiles(t, contract.commandRootAbs(root, codexHome), contract.CommandBase, root, codexHome)
	assert.Equal(t, expectedPaths, actualPaths, "%s command paths drifted", toolID)

	for _, id := range frozenAdapterCommandIDs {
		relPath := contract.commandPath(id)
		absPath := contract.resolvePath(root, codexHome, relPath)
		content, err := os.ReadFile(absPath)
		require.NoError(t, err, "missing generated command surface for %s/%s", toolID, id)
		assert.Contains(t, string(content), contract.commandContentMarker(id), "%s/%s contract marker drifted", toolID, id)
	}
}

// assertNoFrozenCodexCommandPrompts asserts the retired generated Codex command
// prompt files are absent. It deliberately checks the command registry filenames
// rather than the whole slipway-* namespace because $CODEX_HOME/prompts is user
// state and may contain unrelated prompts with the same prefix.
func assertNoFrozenCodexCommandPrompts(t *testing.T, codexHome string) {
	t.Helper()
	promptsDir := filepath.Join(codexHome, "prompts")
	for _, id := range frozenAdapterCommandIDs {
		_, err := os.Stat(filepath.Join(promptsDir, "slipway-"+id+".md"))
		assert.True(t, os.IsNotExist(err), "codex must not write retired generated command prompt for %s", id)
	}
}

func (c frozenToolContract) commandPath(id string) string {
	filename := id + c.CommandExt
	switch c.CommandStyle {
	case "skill":
		return filepath.ToSlash(filepath.Join(c.CommandRoot, "slipway-"+id, "SKILL.md"))
	case "flat", "global":
		return filepath.ToSlash(filepath.Join(c.CommandRoot, "slipway-"+filename))
	default:
		return filepath.ToSlash(filepath.Join(c.CommandRoot, "slipway", filename))
	}
}

func (c frozenToolContract) commandRootAbs(root, codexHome string) string {
	base := root
	if c.CommandBase == frozenSurfaceCodexHome {
		base = codexHome
	}
	return filepath.Join(base, filepath.FromSlash(c.CommandRoot))
}

func (c frozenToolContract) resolvePath(root, codexHome, relPath string) string {
	base := root
	if c.CommandBase == frozenSurfaceCodexHome {
		base = codexHome
	}
	return filepath.Join(base, filepath.FromSlash(relPath))
}

func (c frozenToolContract) commandContentMarker(id string) string {
	switch c.CommandStyle {
	case "skill":
		// Codex command skills carry the injected adapter name frontmatter.
		return "name: slipway-" + id
	case "global":
		return "surface: \"adapter\""
	default:
		return c.commandTrigger(id)
	}
}

func (c frozenToolContract) commandTrigger(id string) string {
	if c.TriggerStyle == "slash-colon" {
		return c.TriggerPrefix + ":" + id
	}
	return c.TriggerPrefix + id
}

func collectRelativeFiles(t *testing.T, absRoot string, base frozenSurfaceBase, root, codexHome string) []string {
	t.Helper()

	var relPaths []string
	err := filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		var rel string
		switch base {
		case frozenSurfaceCodexHome:
			rel, err = filepath.Rel(codexHome, path)
		default:
			rel, err = filepath.Rel(root, path)
		}
		if err != nil {
			return err
		}
		relPaths = append(relPaths, filepath.ToSlash(rel))
		return nil
	})
	require.NoError(t, err)

	sort.Strings(relPaths)
	return relPaths
}

func assertFrozenHookContract(t *testing.T, root, settingsPath string, hook frozenHookContract) {
	t.Helper()

	if strings.TrimSpace(hook.Path) == "" {
		assert.Empty(t, hook.Event)
		assert.False(t, hook.Registered)
		return
	}

	absHookPath := filepath.Join(root, filepath.FromSlash(hook.Path))
	_, err := os.Stat(absHookPath)
	require.NoError(t, err, "missing generated hook %s", hook.Path)
	assert.FileExists(t, filepath.Join(root, filepath.FromSlash(hook.Path+".ps1")))
	assert.FileExists(t, filepath.Join(root, filepath.FromSlash(hook.Path+".cmd")))

	if strings.TrimSpace(settingsPath) == "" {
		assert.False(t, hook.Registered, "hook without settings must not be marked registered")
		return
	}

	settingsAbsPath := filepath.Join(root, filepath.FromSlash(settingsPath))
	_, err = os.Stat(settingsAbsPath)
	require.NoError(t, err, "missing settings file %s", settingsPath)

	registered, err := hookCommandRegistered(settingsAbsPath, hook.Event, hookInvocationCommand(hook.Path))
	require.NoError(t, err)
	assert.Equal(t, hook.Registered, registered, "hook registration drifted for %s", hook.Path)
}

func hookInvocationCommand(hookPath string) string {
	return hookLauncherCommand(nativeHookPath(hookPath))
}

func hookCommandRegistered(settingsPath, eventName, command string) (bool, error) {
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		return false, err
	}

	settings := map[string]any{}
	if err := json.Unmarshal(content, &settings); err != nil {
		return false, err
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false, nil
	}
	entries, ok := hooks[eventName].([]any)
	if !ok {
		return false, nil
	}

	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		rawHooks, ok := entryMap["hooks"].([]any)
		if !ok {
			continue
		}
		for _, hook := range rawHooks {
			hookMap, ok := hook.(map[string]any)
			if ok && hookMap["command"] == command {
				return true, nil
			}
		}
	}
	return false, nil
}
