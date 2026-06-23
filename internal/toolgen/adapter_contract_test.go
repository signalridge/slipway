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
	frozenSurfaceRoot frozenSurfaceBase = "root"
)

type frozenHookContract struct {
	Event string
	Path  string
	// Registered is true when the host registers an inline `slipway hook ...`
	// command for this event in settings.json. Settings-capable hosts (claude,
	// gemini) register; file-by-path hosts (cursor, opencode) do not.
	Registered bool
	// InlineCommand is the bare inline command expected in settings.json for a
	// registered hook. Empty for unregistered (file-by-path) hooks.
	InlineCommand string
	// EmitsLauncher is true when the host writes the launcher file family
	// (extensionless + .ps1 + .cmd) for this hook. Settings-capable hosts emit
	// NO launcher files; only file-by-path hosts do.
	EmitsLauncher bool
}

type frozenToolContract struct {
	OwnershipRoot string
	CommandBase   frozenSurfaceBase
	CommandRoot   string
	CommandStyle  string
	CommandExt    string
	TriggerPrefix string
	TriggerStyle  string
	SettingsPath  string
	SettingsKind  string
	SessionHook   frozenHookContract
	PostToolHook  frozenHookContract
}

var frozenAdapterCommandIDs = []string{
	"abort",
	"cancel",
	"codebase-map",
	"delete",
	"done",
	"evidence",
	"fix",
	"handoff",
	"health",
	"implement",
	"init",
	"instructions",
	"intake",
	"new",
	"next",
	"plan",
	"preset",
	"repair",
	"review",
	"run",
	"status",
	"validate",
}

var frozenToolOrder = []string{
	"claude",
	"codex",
	"copilot",
	"cursor",
	"gemini",
	"kilo",
	"kiro",
	"opencode",
	"pi",
	"qwen",
	"windsurf",
}

var frozenToolContracts = map[string]frozenToolContract{
	"claude": {
		OwnershipRoot: ".claude",
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".claude/commands",
		CommandStyle:  "nested",
		CommandExt:    ".md",
		TriggerPrefix: "/slipway",
		TriggerStyle:  "slash-colon",
		SettingsPath:  ".claude/settings.json",
		SessionHook: frozenHookContract{
			Event:         "SessionStart",
			Path:          ".claude/hooks/slipway-session-start",
			Registered:    true,
			InlineCommand: sessionStartHookCommand,
			EmitsLauncher: false,
		},
		PostToolHook: frozenHookContract{
			Event:         "PostToolUse",
			Path:          ".claude/hooks/slipway-context-pressure-post-tool-use",
			Registered:    true,
			InlineCommand: contextPressureHookCommand,
			EmitsLauncher: false,
		},
	},
	"codex": {
		OwnershipRoot: ".codex",
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".codex/skills",
		CommandStyle:  "skill",
		CommandExt:    ".md",
		TriggerPrefix: "$slipway-",
		TriggerStyle:  "dollar-mention",
		SettingsPath:  ".codex/config.toml",
		SettingsKind:  settingsKindCodexHooks,
	},
	"copilot": {
		OwnershipRoot: ".github/copilot",
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".github/prompts",
		CommandStyle:  "flat",
		CommandExt:    ".prompt.md",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
	},
	"cursor": {
		OwnershipRoot: ".cursor",
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".cursor/commands",
		CommandStyle:  "flat",
		CommandExt:    ".md",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		SessionHook: frozenHookContract{
			Path:          ".cursor/hooks/slipway-session-start",
			Registered:    false,
			EmitsLauncher: true,
		},
	},
	"gemini": {
		OwnershipRoot: ".gemini",
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".gemini/commands",
		CommandStyle:  "nested",
		CommandExt:    ".toml",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		SettingsPath:  ".gemini/settings.json",
		SessionHook: frozenHookContract{
			Event:         "SessionStart",
			Path:          ".gemini/hooks/slipway-session-start",
			Registered:    true,
			InlineCommand: sessionStartHookCommand,
			EmitsLauncher: false,
		},
	},
	"kilo": {
		OwnershipRoot: ".kilocode",
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".kilocode/workflows",
		CommandStyle:  "flat",
		CommandExt:    ".md",
		TriggerPrefix: "/slipway",
		TriggerStyle:  "slash-colon",
	},
	"kiro": {
		OwnershipRoot: ".kiro",
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".kiro/skills",
		CommandStyle:  "skill",
		CommandExt:    ".md",
		TriggerPrefix: "@slipway",
		TriggerStyle:  "at-colon",
	},
	"opencode": {
		OwnershipRoot: ".opencode",
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".opencode/commands",
		CommandStyle:  "flat",
		CommandExt:    ".md",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		SessionHook: frozenHookContract{
			Path:          ".opencode/hooks/slipway-session-start",
			Registered:    false,
			EmitsLauncher: true,
		},
	},
	"pi": {
		OwnershipRoot: ".pi",
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".pi/prompts",
		CommandStyle:  "flat",
		CommandExt:    ".md",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		SettingsPath:  ".pi/settings.json",
		SettingsKind:  settingsKindPiRegistration,
	},
	"qwen": {
		OwnershipRoot: ".qwen",
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".qwen/skills",
		CommandStyle:  "skill",
		CommandExt:    ".md",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		SettingsPath:  ".qwen/settings.json",
		SessionHook: frozenHookContract{
			Event:         "SessionStart",
			Path:          ".qwen/hooks/slipway-session-start",
			Registered:    true,
			InlineCommand: sessionStartHookCommand,
			EmitsLauncher: false,
		},
	},
	"windsurf": {
		OwnershipRoot: ".windsurf",
		CommandBase:   frozenSurfaceRoot,
		CommandRoot:   ".windsurf/workflows",
		CommandStyle:  "flat",
		CommandExt:    ".md",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
	},
}

func TestAdapterContractsRemainStable(t *testing.T) {
	root := t.TempDir()

	require.NoError(t, Generate(root, frozenToolOrder, true))

	for _, toolID := range frozenToolOrder {
		toolID := toolID
		t.Run(toolID, func(t *testing.T) {
			contract := frozenToolContracts[toolID]
			assertFrozenOwnershipContract(t, root, toolID, contract)
			assertFrozenCommandSet(t, root, toolID, contract)
			assertFrozenHookContract(t, root, contract.SettingsPath, contract.SessionHook)
			assertFrozenHookContract(t, root, contract.SettingsPath, contract.PostToolHook)
			assertFrozenSettingsContract(t, root, contract)
		})
	}
}

func assertFrozenCommandSet(t *testing.T, root, toolID string, contract frozenToolContract) {
	t.Helper()

	// Codex exposes commands as per-command skills, not a flat command-surface
	// directory. The skills dir also holds host/governance skills, so assert each
	// frozen command id has its skill file rather than a strict directory match.
	if contract.CommandStyle == "skill" {
		for _, id := range frozenAdapterCommandIDs {
			relPath := contract.commandPath(id)
			absPath := contract.resolvePath(root, relPath)
			content, err := os.ReadFile(absPath)
			require.NoError(t, err, "missing generated command skill for %s/%s", toolID, id)
			assert.Contains(t, string(content), contract.commandContentMarker(id), "%s/%s contract marker drifted", toolID, id)
			assert.Contains(t, string(content), contract.commandTrigger(id), "%s/%s command trigger drifted", toolID, id)
		}
		return
	}

	expectedPaths := make([]string, 0, len(frozenAdapterCommandIDs))
	for _, id := range frozenAdapterCommandIDs {
		expectedPaths = append(expectedPaths, contract.commandPath(id))
	}

	actualPaths := collectRelativeFiles(t, contract.commandRootAbs(root), contract.CommandBase, root)
	assert.Equal(t, expectedPaths, actualPaths, "%s command paths drifted", toolID)

	for _, id := range frozenAdapterCommandIDs {
		relPath := contract.commandPath(id)
		absPath := contract.resolvePath(root, relPath)
		content, err := os.ReadFile(absPath)
		require.NoError(t, err, "missing generated command surface for %s/%s", toolID, id)
		assert.Contains(t, string(content), contract.commandContentMarker(id), "%s/%s contract marker drifted", toolID, id)
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

func (c frozenToolContract) commandRootAbs(root string) string {
	return filepath.Join(root, filepath.FromSlash(c.CommandRoot))
}

func (c frozenToolContract) resolvePath(root, relPath string) string {
	return filepath.Join(root, filepath.FromSlash(relPath))
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
	if c.TriggerStyle == "slash-colon" || c.TriggerStyle == "at-colon" {
		return c.TriggerPrefix + ":" + id
	}
	return c.TriggerPrefix + id
}

func assertFrozenOwnershipContract(t *testing.T, root, toolID string, contract frozenToolContract) {
	t.Helper()

	cfg, ok := toolRegistry[toolID]
	require.True(t, ok, "missing tool registry config for %s", toolID)
	assert.Equal(t, contract.OwnershipRoot, filepath.ToSlash(ToolRootPath(cfg)), "%s ownership root drifted", toolID)

	markerRel := filepath.ToSlash(filepath.Join(contract.OwnershipRoot, "slipway", ".adapter-generated"))
	manifestRel := filepath.ToSlash(filepath.Join(contract.OwnershipRoot, "slipway", ownershipManifestFileName))
	assert.Equal(t, markerRel, filepath.ToSlash(GeneratedAdapterMarkerPath(cfg)), "%s sentinel path drifted", toolID)
	assert.Equal(t, manifestRel, filepath.ToSlash(generatedOwnershipManifestPath(cfg)), "%s ownership manifest path drifted", toolID)
	assert.FileExists(t, filepath.Join(root, filepath.FromSlash(markerRel)), "%s missing ownership sentinel", toolID)
	assert.FileExists(t, filepath.Join(root, filepath.FromSlash(manifestRel)), "%s missing ownership manifest", toolID)

	if toolID == "copilot" {
		_, err := os.Stat(filepath.Join(root, ".github", "slipway", ".adapter-generated"))
		assert.True(t, os.IsNotExist(err), "copilot must not claim shared .github sentinel ownership")
		_, err = os.Stat(filepath.Join(root, ".github", "slipway", ownershipManifestFileName))
		assert.True(t, os.IsNotExist(err), "copilot must not claim shared .github manifest ownership")
	}
}

func collectRelativeFiles(t *testing.T, absRoot string, base frozenSurfaceBase, root string) []string {
	t.Helper()

	var relPaths []string
	err := filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
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
		assert.False(t, hook.EmitsLauncher)
		assert.Empty(t, hook.InlineCommand)
		return
	}

	// Launcher emission is keyed on whether the host owns a settings.json.
	// Settings-capable hosts (claude, gemini) emit NO launcher files; file-by-path
	// hosts (cursor, opencode) still emit the extensionless + .ps1 + .cmd family.
	for _, suffix := range []string{"", ".ps1", ".cmd", ".sh"} {
		p := filepath.Join(root, filepath.FromSlash(hook.Path+suffix))
		_, err := os.Stat(p)
		if hook.EmitsLauncher && suffix != ".sh" {
			require.NoErrorf(t, err, "missing generated launcher %s", hook.Path+suffix)
		} else {
			assert.Truef(t, os.IsNotExist(err),
				"unexpected launcher file %s (settings-capable hosts emit none; .sh is always retired)", hook.Path+suffix)
		}
	}

	if strings.TrimSpace(settingsPath) == "" {
		assert.False(t, hook.Registered, "hook without settings must not be marked registered")
		assert.Empty(t, hook.InlineCommand, "hook without settings must not declare an inline command")
		return
	}

	settingsAbsPath := filepath.Join(root, filepath.FromSlash(settingsPath))
	content, err := os.ReadFile(settingsAbsPath)
	require.NoError(t, err, "missing settings file %s", settingsPath)
	settings := string(content)

	// Settings-capable hosts register the bare inline command and never reference
	// a launcher path.
	if hook.Registered {
		require.NotEmpty(t, hook.InlineCommand, "registered hook must declare its inline command")
		assert.NotContains(t, settings, hook.Path,
			"settings must not reference launcher path for %s", hook.Path)
		assertShellNeutralHookCommand(t, hook.InlineCommand)
	}

	registered, err := hookCommandRegistered(settingsAbsPath, hook.Event, hook.InlineCommand)
	require.NoError(t, err)
	assert.Equal(t, hook.Registered, registered, "hook registration drifted for %s", hook.Path)
}

func assertFrozenSettingsContract(t *testing.T, root string, contract frozenToolContract) {
	t.Helper()

	switch contract.SettingsKind {
	case "":
		return
	case settingsKindPiRegistration:
		require.NotEmpty(t, contract.SettingsPath, "Pi registration settings require a settings path")
		assertPiRegistrationSettings(t, filepath.Join(root, filepath.FromSlash(contract.SettingsPath)))
	case settingsKindCodexHooks:
		require.NotEmpty(t, contract.SettingsPath, "Codex hook settings require a config path")
		assertCodexHooksConfig(t, filepath.Join(root, filepath.FromSlash(contract.SettingsPath)))
	default:
		t.Fatalf("unsupported frozen settings kind %q", contract.SettingsKind)
	}
}

func assertCodexHooksConfig(t *testing.T, settingsPath string) {
	t.Helper()

	content, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "missing Codex config file")
	settings := string(content)
	assert.Contains(t, settings, "[[hooks.SessionStart]]")
	assert.Contains(t, settings, `slipway hook session-start --tool codex`)
	assert.Contains(t, settings, "[[hooks.UserPromptSubmit]]")
	assert.Contains(t, settings, `slipway hook context-pressure --tool codex`)
	assert.Contains(t, settings, "inert until Codex trusts this repo and each hook")
	assert.Contains(t, settings, "Slipway never edits global Codex trust")
}

func assertShellNeutralHookCommand(t *testing.T, command string) {
	t.Helper()

	assert.NotContains(t, command, "||")
	assert.NotContains(t, command, "&&")
	assert.NotContains(t, command, ";")
	assert.NotContains(t, command, "|")
	assert.NotContains(t, command, "&")
	assert.NotContains(t, command, " exit ")
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
