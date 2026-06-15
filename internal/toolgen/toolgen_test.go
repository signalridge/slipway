package toolgen

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/engine/capability"
	"github.com/signalridge/slipway/internal/tmpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type hydrateReferenceEntry struct {
	Name   string `yaml:"name"`
	Reason string `yaml:"reason"`
}

// loadHydrateReferencesFromFS parses a SKILL.md frontmatter block from an fs.FS
// and returns the declared hydrate_references[] records (empty slice when absent).
func loadHydrateReferencesFromFS(t *testing.T, src fs.FS, name string) []hydrateReferenceEntry {
	t.Helper()
	raw, err := fs.ReadFile(src, name)
	require.NoError(t, err)
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	if start >= len(lines) || strings.TrimSpace(lines[start]) != "---" {
		return nil
	}
	end := -1
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		t.Fatalf("%s: unterminated frontmatter", name)
	}
	block := strings.Join(lines[start+1:end], "\n")
	var fm struct {
		HydrateReferences []hydrateReferenceEntry `yaml:"hydrate_references"`
	}
	require.NoErrorf(t, yaml.Unmarshal([]byte(block), &fm), "%s: parse frontmatter", name)
	return fm.HydrateReferences
}

func TestRegistryHasFiveTools(t *testing.T) {
	t.Parallel()
	registry := Registry()
	require.Len(t, registry, 5)

	ids := make([]string, len(registry))
	for i, cfg := range registry {
		ids[i] = cfg.ID
	}
	assert.Equal(t, []string{"claude", "codex", "cursor", "gemini", "opencode"}, ids)
}

func TestResolveTools(t *testing.T) {
	t.Parallel()
	all, err := ResolveTools("all")
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "codex", "cursor", "gemini", "opencode"}, all)

	none, err := ResolveTools("none")
	require.NoError(t, err)
	assert.Nil(t, none)

	selected, err := ResolveTools("cursor,claude,cursor")
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "cursor"}, selected)

	_, err = ResolveTools("unknown")
	require.Error(t, err)
}

func TestCommandRegistryContainsAllAdapterSkillIDs(t *testing.T) {
	t.Parallel()
	// Verify registry has 22 commands (5 core + 12 situational + 5 diagnostics).
	assert.Len(t, commandRegistry, 22)

	// Verify all registry entries have the required fields.
	for _, def := range commandRegistry {
		assert.NotEmpty(t, def.ID, "registry entry missing ID")
		assert.Contains(t, []CommandClass{CommandClassQuery, CommandClassMutation}, def.Class,
			"registry entry %s has invalid Class %q", def.ID, def.Class)
		assert.NotEmpty(t, def.Description, "registry entry %s missing Description", def.ID)
		assert.NotEmpty(t, def.Tier, "registry entry %s missing Tier", def.ID)
		assert.True(t, def.Tier == "core" || def.Tier == "situational" || def.Tier == "diagnostics",
			"registry entry %s has invalid Tier %q", def.ID, def.Tier)
	}

	// Count tiers.
	core, sit, diag := 0, 0, 0
	query, mutation := 0, 0
	for _, def := range commandRegistry {
		switch def.Class {
		case CommandClassQuery:
			query++
		case CommandClassMutation:
			mutation++
		}
		switch def.Tier {
		case "core":
			core++
		case "situational":
			sit++
		case "diagnostics":
			diag++
		}
	}
	assert.Equal(t, 5, core, "expected 5 core commands")
	assert.Equal(t, 12, sit, "expected 12 situational commands")
	assert.Equal(t, 5, diag, "expected 5 diagnostics commands")
	assert.Equal(t, 7, query, "expected 7 query commands")
	assert.Equal(t, 15, mutation, "expected 15 mutation commands")

	// Verify commandIDs() returns a sorted list covering every command that
	// ships a prompt surface. CLI-only helpers such as `tool` remain registered
	// but intentionally do not generate host prompt wrappers.
	ids := commandIDs()
	assert.Len(t, ids, 21)
	for i := 1; i < len(ids); i++ {
		assert.True(t, ids[i-1] < ids[i], "commandIDs not sorted: %s >= %s", ids[i-1], ids[i])
	}
	assert.NotContains(t, ids, "tool")
}

func TestGovernanceSurfaceExportSetsStayComplete(t *testing.T) {
	t.Parallel()

	exported := map[string]struct{}{}
	for _, name := range GovernanceSkillNames {
		exported[name] = struct{}{}
	}
	for _, name := range standaloneGovernanceNames {
		exported[name] = struct{}{}
	}
	for _, name := range TemplatedGovernanceSkillNames {
		exported[name] = struct{}{}
	}

	workflowGovernanceNames := governanceSurfaceIDs(func(desc governanceSurfaceDescriptor) bool {
		return desc.WorkflowOwned
	})
	for _, name := range workflowGovernanceNames {
		_, ok := exported[name]
		assert.Truef(t, ok, "workflow governance host %q is not exported by toolgen", name)
	}

	extraExportedGovernanceNames := governanceSurfaceIDs(func(desc governanceSurfaceDescriptor) bool {
		return desc.ExportOnlyExtra
	})
	for _, name := range extraExportedGovernanceNames {
		_, ok := exported[name]
		assert.Truef(t, ok, "extra exported governance surface %q is not exported by toolgen", name)
	}
	assert.Len(t, exported, len(governanceSurfaceDescriptors), "governance export union drifted from descriptor table")
}

func TestGeneratedHostSkillSetEqualsAllowlist(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())
	require.NoError(t, Generate(root, []string{"codex"}, true))

	cfg := toolRegistry["codex"]
	skillsRoot := filepath.Join(root, cfg.SkillsDir)
	entries, err := os.ReadDir(skillsRoot)
	require.NoError(t, err)

	// Codex also renders one command skill per Slipway command under the same
	// SkillsDir. Separate those from the host-skill allowlist so the slim
	// exported-surface contract still measures only host skills.
	commandSkillSet := map[string]struct{}{}
	for _, name := range commandSkillDirNames() {
		commandSkillSet[name] = struct{}{}
	}

	var got []string
	var gotCommandSkills []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(skillsRoot, entry.Name(), "SKILL.md")); err != nil {
			continue
		}
		if _, ok := commandSkillSet[entry.Name()]; ok {
			gotCommandSkills = append(gotCommandSkills, entry.Name())
			continue
		}
		got = append(got, entry.Name())
	}

	expectedSet := map[string]struct{}{}
	for _, names := range [][]string{
		GovernanceSkillNames,
		standaloneGovernanceNames,
		TemplatedGovernanceSkillNames,
		standaloneNames,
		techniqueNames,
		catalogSkillIDs,
	} {
		for _, name := range names {
			if shouldExportAsHostSkill(name) {
				expectedSet[adapterSkillName(name)] = struct{}{}
			}
		}
	}
	var expected []string
	for name := range expectedSet {
		expected = append(expected, name)
	}
	assert.ElementsMatch(t, expected, got)
	assert.Len(t, got, 23, "host skill count should stay within the slim exported surface target")

	// Every Slipway command must have its own discoverable Codex command skill.
	assert.ElementsMatch(t, commandSkillDirNames(), gotCommandSkills,
		"codex command skill dirs must cover exactly the command set")
}

func TestNonExportedRegistrySkillsDoNotEmitAgentFacingCatalogArtifacts(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())
	require.NoError(t, Generate(root, []string{"codex"}, true))

	cfg := toolRegistry["codex"]
	for _, id := range []string{"threat-modeling", "sast-orchestration", "review-comment-triage"} {
		_, err := os.Stat(filepath.Join(root, SkillPath(cfg, id)))
		assert.True(t, os.IsNotExist(err), "%s should not be emitted as a host SKILL.md", id)

		_, err = os.Stat(filepath.Join(root, cfg.SkillsDir, "slipway", "references", "catalog", id+".md"))
		assert.True(t, os.IsNotExist(err), "%s should not be emitted as a catalog route card", id)
		_, err = os.Stat(filepath.Join(root, cfg.SkillsDir, "slipway", "references", "catalog", id))
		assert.True(t, os.IsNotExist(err), "%s should not copy support files under workflow catalog references", id)
	}

	_, err := os.Stat(filepath.Join(root, SkillIndexPath(cfg)))
	assert.NoError(t, err, "workflow-owned skill index should still be generated")
}

func TestResolveNextSkillOutputsMapToExportedHostSkills(t *testing.T) {
	t.Parallel()

	for _, id := range []string{
		"intake-clarification",
		"research-orchestration",
		"worktree-preflight",
		"plan-audit",
		"wave-orchestration",
		"spec-compliance-review",
		"code-quality-review",
		"goal-verification",
		"final-closeout",
		"tdd-governance",
	} {
		assert.Truef(t, shouldExportAsHostSkill(id), "next-skill output %q must map to an exported host skill", id)
	}
}

func TestDetectExistingTools(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Empty workspace: no tools detected.
	assert.Empty(t, DetectExistingTools(root))

	// Bare .claude/ directory (not generated by slipway): should NOT be detected.
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".claude"), 0o755))
	assert.Empty(t, DetectExistingTools(root), "bare .claude/ should not trigger detection")

	// Create slipway sentinel for claude.
	claudeCfg := toolRegistry["claude"]
	writeGeneratedAdapterMarker(t, root, claudeCfg)
	detected := DetectExistingTools(root)
	assert.Equal(t, []string{"claude"}, detected)

	// Add slipway sentinel for cursor.
	cursorCfg := toolRegistry["cursor"]
	writeGeneratedAdapterMarker(t, root, cursorCfg)
	detected = DetectExistingTools(root)
	assert.Equal(t, []string{"claude", "cursor"}, detected)
}

func TestHasGeneratedAdapter(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	assert.False(t, hasGeneratedAdapter(root, toolRegistry["claude"]))

	writeGeneratedAdapterMarker(t, root, toolRegistry["claude"])
	assert.True(t, hasGeneratedAdapter(root, toolRegistry["claude"]))
	assert.False(t, hasGeneratedAdapter(root, toolRegistry["cursor"]))
}

func TestGenerateProducesAllExpectedFiles(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())
	require.NoError(t, Generate(root, []string{"claude", "cursor", "codex", "gemini", "opencode"}, true))

	for _, toolID := range []string{"claude", "cursor", "codex", "gemini", "opencode"} {
		cfg := toolRegistry[toolID]

		// Sentinel marker
		sentinelPath := filepath.Join(root, GeneratedAdapterMarkerPath(cfg))
		_, err := os.Stat(sentinelPath)
		assert.NoError(t, err, "%s: missing adapter sentinel", toolID)

		// Governance skills (static) + standalone governance guidance skills
		allStaticGov := append([]string{}, GovernanceSkillNames...)
		allStaticGov = append(allStaticGov, standaloneGovernanceNames...)
		for _, name := range allStaticGov {
			path := filepath.Join(root, cfg.SkillsDir, "slipway-"+name, "SKILL.md")
			_, err := os.Stat(path)
			assert.NoError(t, err, "%s: missing governance skill %s", toolID, name)
		}

		// Templated governance skills (tool-aware)
		for _, name := range TemplatedGovernanceSkillNames {
			path := filepath.Join(root, cfg.SkillsDir, "slipway-"+name, "SKILL.md")
			_, err := os.Stat(path)
			assert.NoError(t, err, "%s: missing templated governance skill %s", toolID, name)
		}

		// Standalone exported skills
		for _, name := range standaloneNames {
			path := filepath.Join(root, cfg.SkillsDir, adapterSkillName(name), "SKILL.md")
			_, err := os.Stat(path)
			assert.NoError(t, err, "%s: missing standalone skill %s", toolID, name)
		}

		// Command entries — per-tool path and format assertions
		if cfg.CommandsDir != "" {
			ext := ".md"
			if cfg.CommandFormat == "toml" {
				ext = ".toml"
			}
			for _, id := range commandIDs() {
				var path string
				switch cfg.CommandStyle {
				case "flat":
					path = filepath.Join(root, cfg.CommandsDir, "slipway-"+id+ext)
				default:
					path = filepath.Join(root, cfg.CommandsDir, "slipway", id+ext)
				}
				_, err := os.Stat(path)
				assert.NoError(t, err, "%s: missing command entry %s", toolID, id)
			}
		} else {
			// Codex: no project-local commands
			commandsDir := filepath.Join(root, "."+toolID, "commands")
			_, err := os.Stat(commandsDir)
			assert.True(t, os.IsNotExist(err), "%s: unexpected commands directory generated", toolID)
		}

		// Technique skills
		for _, name := range techniqueNames {
			path := filepath.Join(root, cfg.SkillsDir, "slipway-"+name, "SKILL.md")
			_, err := os.Stat(path)
			if shouldExportAsHostSkill(name) {
				assert.NoError(t, err, "%s: missing technique %s", toolID, name)
			} else {
				assert.True(t, os.IsNotExist(err), "%s: unexpected host technique %s", toolID, name)
			}
		}

		// Registry-owned host skills. Non-exported registry skills remain
		// internal metadata and must not create workflow catalog route files.
		reg := capability.DefaultRegistry()
		for _, id := range catalogSkillIDs {
			skillDir := filepath.Join(root, cfg.SkillsDir, "slipway-"+id)
			skillPath := filepath.Join(skillDir, "SKILL.md")
			if shouldExportAsHostSkill(id) {
				body, err := os.ReadFile(skillPath)
				assert.NoError(t, err, "%s: missing registry host skill %s", toolID, id)

				fm := extractAdapterFrontmatter(t, string(body), toolID, id)
				assert.Equal(t, "slipway-"+id, fm["name"],
					"%s: registry skill %s: wrong name", toolID, id)
				assert.NotEmpty(t, fm["description"],
					"%s: registry skill %s: empty description", toolID, id)
				sk, ok := reg.Lookup(id)
				if assert.Truef(t, ok, "%s: registry skill %s missing from registry", toolID, id) {
					assert.Equal(t, sk.Summary, fm["description"],
						"%s: registry skill %s: description drifted from registry summary", toolID, id)
				}
			} else {
				_, err := os.Stat(skillPath)
				assert.True(t, os.IsNotExist(err), "%s: unexpected catalog-only host skill %s", toolID, id)
			}
			_, err := os.Stat(filepath.Join(root, cfg.SkillsDir, "slipway", "references", "catalog", id+".md"))
			assert.True(t, os.IsNotExist(err), "%s: unexpected catalog route artifact %s", toolID, id)
			_, err = os.Stat(filepath.Join(root, cfg.SkillsDir, "slipway", "references", "catalog", id))
			assert.True(t, os.IsNotExist(err), "%s: unexpected catalog support root %s", toolID, id)
		}

		// Workflow-owned skill index
		indexPath := filepath.Join(root, SkillIndexPath(cfg))
		indexBytes, err := os.ReadFile(indexPath)
		assert.NoError(t, err, "%s: missing workflow skill index", toolID)
		index := string(indexBytes)
		assert.Contains(t, index, "# Slipway Skill Index",
			"%s: skill index missing header", toolID)
		assert.Contains(t, index, "## Index",
			"%s: skill index missing dispatcher index", toolID)
		assert.NotContains(t, index, "references/catalog/",
			"%s: skill index must not expose catalog paths", toolID)
		for _, id := range catalogSkillIDs {
			if shouldExportAsHostSkill(id) {
				assert.Contains(t, index, filepath.ToSlash(SkillPath(cfg, id)),
					"%s: skill index missing host path for %s", toolID, id)
			} else {
				assert.NotContains(t, index, "slipway-"+id+"/SKILL.md",
					"%s: skill index should not list non-exported skill %s", toolID, id)
			}
		}

		// Hook emission is keyed on whether the host owns a settings.json.
		// Settings-capable hosts (claude, gemini) register a bare inline command
		// and emit NO launcher files. File-by-path hosts (cursor, opencode) still
		// emit the session-start launcher family. Codex has no session hook.
		switch {
		case cfg.SettingsPath != "":
			// Settings-capable hosts must not write any launcher file for either
			// hook (the extensionless POSIX entry or its .ps1/.cmd/.sh variants).
			for _, base := range []string{cfg.SessionHook, cfg.PostToolHook} {
				if base == "" {
					continue
				}
				for _, suffix := range []string{"", ".ps1", ".cmd", ".sh"} {
					p := filepath.Join(root, filepath.FromSlash(base+suffix))
					_, err := os.Stat(p)
					assert.True(t, os.IsNotExist(err),
						"%s: settings-capable host must not emit launcher file %s", toolID, base+suffix)
				}
			}
		case cfg.SessionHook != "":
			// File-by-path hosts keep emitting the session-start launcher family.
			hookPath := filepath.Join(root, cfg.SessionHook)
			_, err := os.Stat(hookPath)
			assert.NoError(t, err, "%s: missing session-start launcher", toolID)
			assert.FileExists(t, hookPath+".ps1", "%s: missing PowerShell session-start launcher", toolID)
			assert.FileExists(t, hookPath+".cmd", "%s: missing cmd session-start launcher", toolID)
		default:
			// Codex: no session hook at all.
			hookPath := filepath.Join(root, "."+toolID, "hooks", "slipway-session-start")
			_, err := os.Stat(hookPath)
			assert.True(t, os.IsNotExist(err), "%s: unexpected session hook generated", toolID)
		}

		switch {
		case cfg.SettingsPath != "":
			settingsPath := filepath.Join(root, cfg.SettingsPath)
			content, err := os.ReadFile(settingsPath)
			assert.NoError(t, err, "%s: missing settings file", toolID)
			settings := string(content)
			assert.Contains(t, settings, "SessionStart", "%s: missing session-start registration", toolID)
			// Both settings-capable hosts register the bare inline session-start
			// command with no launcher path or shell operator.
			assert.Contains(t, settings, sessionStartHookCommand,
				"%s: missing inline session-start command", toolID)
			assert.NotContains(t, settings, ".claude/hooks/",
				"%s: settings must not reference a launcher path", toolID)
			assert.NotContains(t, settings, "."+toolID+"/hooks/",
				"%s: settings must not reference a launcher path", toolID)
			assert.NotContains(t, settings, "--tool", "%s: settings must use the bare inline command", toolID)
			assert.NotContains(t, settings, "||", "%s: settings command must parse in Windows PowerShell 5.1", toolID)
			assert.NotContains(t, settings, "bash", "%s: settings must not require bash", toolID)
			if cfg.PostToolEvent != "" && cfg.PostToolHook != "" {
				assert.Contains(t, settings, "PostToolUse", "%s: missing post-tool registration", toolID)
				assert.Contains(t, settings, contextPressureHookCommand,
					"%s: missing inline context-pressure command", toolID)
			} else {
				assert.NotContains(t, settings, "PostToolUse", "%s: unexpected post-tool registration", toolID)
				assert.NotContains(t, settings, contextPressureHookCommand,
					"%s: unexpected context-pressure registration", toolID)
			}
		default:
			settingsPath := filepath.Join(root, "."+toolID, "settings.json")
			_, err := os.Stat(settingsPath)
			assert.True(t, os.IsNotExist(err), "%s: unexpected settings file generated", toolID)
		}
	}
}

// referenceSectionFor returns the command-reference body from header up to the
// next "### " command heading or "## " section heading (or end of file).
func referenceSectionFor(ref, header string) string {
	idx := strings.Index(ref, header)
	if idx < 0 {
		return ""
	}
	rest := ref[idx+len(header):]
	cut := len(rest)
	if next := strings.Index(rest, "\n### "); next >= 0 && next < cut {
		cut = next
	}
	if next := strings.Index(rest, "\n## "); next >= 0 && next < cut {
		cut = next
	}
	return rest[:cut]
}

// TestInstructionsCommandDeclaresNoFalsePrerequisites guards the issue-#91
// regression where the `instructions` registry entry omitted Prerequisites and
// commandPrerequisites leaked the catch-all default (run `slipway init` / an
// active change), both false for a command that reads only embedded templates.
func TestInstructionsCommandDeclaresNoFalsePrerequisites(t *testing.T) {
	prereqs := commandPrerequisites("instructions")
	require.NotEmpty(t, prereqs, "instructions must declare explicit prerequisites so the catch-all default cannot leak")
	joined := strings.Join(prereqs, "\n")
	assert.NotContains(t, joined, "an active change must exist", "instructions does not require an active change")
	assert.NotContains(t, joined, "run `slipway init`", "instructions reads embedded templates; it does not require slipway init")
}

func TestWorkflowSkillGenerationAndReference(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	require.NoError(t, Generate(root, []string{"claude", "codex", "cursor", "gemini", "opencode"}, true))

	for _, cfg := range Registry() {
		skillPath := filepath.Join(root, cfg.SkillsDir, "slipway", "SKILL.md")
		content, err := os.ReadFile(skillPath)
		require.NoError(t, err, "%s: missing workflow skill", cfg.ID)

		s := string(content)
		fm := extractAdapterFrontmatter(t, s, cfg.ID, "workflow")
		assert.Equal(t, "slipway", fm["name"], "%s: wrong workflow public name", cfg.ID)
		assert.Equal(t, "workflow", fm["skill_id"], "%s: wrong workflow skill_id", cfg.ID)
		assert.NotEmpty(t, fm["description"], "%s: missing workflow description", cfg.ID)
		assert.Equal(t, cfg.ID, fm["tool"], "%s: wrong workflow tool metadata", cfg.ID)
		assert.Contains(t, s, "Use `slipway` as the single exported entry skill for Slipway.", "%s: missing canonical entry skill name", cfg.ID)
		assert.Contains(t, s, "Slipway is a governance CLI for AI-assisted software delivery.", "%s: missing framework intro", cfg.ID)
		assert.Contains(t, s, "`done-ready` means", "%s: missing done-ready semantics", cfg.ID)
		assert.Contains(t, s, "explicit `slipway done`", "%s: missing done finalization semantics", cfg.ID)
		assert.Contains(t, s, "`slipway new --json`", "%s: missing governed entry guidance", cfg.ID)
		assert.Contains(t, s, "JSON stdin fields for `slipway new --json`, not command-line flags", "%s: missing JSON stdin classification transport contract", cfg.ID)
		assert.Contains(t, s, `echo '{"description":"fix typo","guardrail_domain":"","needs_discovery":false,"complexity":"simple"}' | slipway new --json`, "%s: missing minimal JSON stdin example", cfg.ID)
		assert.NotContains(t, s, "--guardrail-domain", "%s: unsupported guardrail flag leaked", cfg.ID)
		assert.NotContains(t, s, "--needs-discovery", "%s: unsupported discovery flag leaked", cfg.ID)
		assert.NotContains(t, s, "--complexity", "%s: unsupported complexity flag leaked", cfg.ID)
		assert.Contains(t, s, "`next_skill.name`", "%s: missing governed host handoff", cfg.ID)
		assert.Contains(t, s, "slipway-{name}", "%s: missing caller-owned skill path contract", cfg.ID)
		assert.NotContains(t, s, "`next_skill.agent_hint`", "%s: stale agent hint contract leaked", cfg.ID)
		assert.NotContains(t, s, "`next_skill.prompt_path`", "%s: stale prompt_path contract leaked", cfg.ID)
		assert.NotContains(t, s, "`next_skill.resolved_tool_id`", "%s: stale resolved_tool_id contract leaked", cfg.ID)
		assert.Contains(t, s, "`references/command-reference.md`", "%s: missing workflow reference handoff", cfg.ID)
		assert.Contains(t, s, filepath.ToSlash(SkillIndexPath(cfg)), "%s: missing workflow skill index path", cfg.ID)
		assert.Contains(t, s, "informational only", "%s: missing skill index authority boundary", cfg.ID)
		assert.NotContains(t, s, "follow the listed catalog artifact path", "%s: stale catalog artifact triage guidance leaked", cfg.ID)
		assert.NotContains(t, s, "using-slipway-catalog.md", "%s: stale top-level catalog manifest leaked", cfg.ID)

		refPath := filepath.Join(root, cfg.SkillsDir, "slipway", "references", "command-reference.md")
		refContent, err := os.ReadFile(refPath)
		require.NoError(t, err, "%s: missing workflow command reference", cfg.ID)
		ref := string(refContent)
		assert.Contains(t, ref, "## Lifecycle Core", "%s: missing lifecycle section", cfg.ID)
		assert.Contains(t, ref, "## Supporting Commands", "%s: missing supporting section", cfg.ID)
		assert.Contains(t, ref, "## Diagnostics", "%s: missing diagnostics section", cfg.ID)
		assert.Contains(t, ref, "### `slipway new`", "%s: missing new command entry", cfg.ID)
		assert.Contains(t, ref, "JSON stdin fields for `slipway new --json`, not command-line flags", "%s: missing new stdin contract notes", cfg.ID)
		assert.Contains(t, ref, "`guardrail_domain`, `needs_discovery`, and `complexity`", "%s: missing new stdin classification shape", cfg.ID)
		assert.Contains(t, ref, "### `slipway run`", "%s: missing run command entry", cfg.ID)
		assert.Contains(t, ref, "### `slipway repair`", "%s: missing repair command entry", cfg.ID)
		assert.Contains(t, ref, "### `slipway codebase-map`", "%s: missing diagnostics command entry", cfg.ID)
		assert.Contains(t, ref, "Can be used with or without an active change.", "%s: missing explicit status prerequisite", cfg.ID)
		assert.Contains(t, ref, "an active change must exist, or pass `--change <slug>` when supported.", "%s: missing helper-default prerequisite", cfg.ID)

		// instructions reads embedded templates only; its reference section must
		// declare it prereq-free and must NOT leak the catch-all default
		// prerequisites (run `slipway init` / an active change) (issue #91).
		instrSection := referenceSectionFor(ref, "### `slipway instructions`")
		assert.NotEmpty(t, instrSection, "%s: missing instructions command entry", cfg.ID)
		assert.Contains(t, instrSection, "serves a static template and guidance", "%s: instructions reference missing prereq-free declaration", cfg.ID)
		assert.NotContains(t, instrSection, "an active change must exist", "%s: instructions reference leaked false active-change prerequisite", cfg.ID)
		assert.NotContains(t, instrSection, "run `slipway init`", "%s: instructions reference leaked false init prerequisite", cfg.ID)
		for _, focus := range capability.ExplicitFocusSurfaces() {
			selector := "`slipway " + focus.Command + " --focus " + focus.PublicName + "`"
			assert.Contains(t, ref, selector, "%s: missing focus selector %s", cfg.ID, selector)
			assert.Contains(t, ref, "`"+focus.BackingID+"`", "%s: missing focus backing %s", cfg.ID, focus.BackingID)
		}

		_, err = os.Stat(filepath.Join(root, cfg.SkillsDir, "slipway", "references", "command-reference.md.tmpl"))
		assert.True(t, os.IsNotExist(err), "%s: raw workflow template leaked into generated tree", cfg.ID)

		// Adapters that expose commands as project-local prompts must not also
		// emit a per-command standalone skill. Codex (CommandSkillSurface) is the
		// exception: it deliberately renders one command skill per command.
		_, err = os.Stat(filepath.Join(root, cfg.SkillsDir, "slipway-new", "SKILL.md"))
		if cfg.CommandSkillSurface {
			assert.NoError(t, err, "%s: missing expected per-command skill", cfg.ID)
		} else {
			assert.True(t, os.IsNotExist(err), "%s: unexpected per-command standalone skill generated", cfg.ID)
		}
	}

	_, err := os.Stat(filepath.Join(codexHome, "prompts", "slipway.md"))
	assert.True(t, os.IsNotExist(err), "codex should not emit a workflow global prompt")
}

func TestGeneratedNewCommandSurfacesDocumentJSONStdinClassification(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	require.NoError(t, Generate(root, []string{"claude", "codex"}, true))

	surfaces := map[string]string{
		"claude": filepath.Join(root, ".claude", "commands", "slipway", "new.md"),
		"codex":  filepath.Join(root, ".codex", "skills", "slipway-new", "SKILL.md"),
	}
	for id, path := range surfaces {
		content, err := os.ReadFile(path)
		require.NoError(t, err, "%s: missing generated new command surface", id)
		s := string(content)

		assert.Contains(t, s, "If the AI caller already knows the classification, provide it as JSON stdin with `--json`.", "%s: missing explicit JSON stdin path", id)
		assert.Contains(t, s, "`guardrail_domain`, `needs_discovery`, and `complexity` are JSON stdin fields, not command-line flags.", "%s: missing classification field transport", id)
		assert.Contains(t, s, `echo '{"description":"fix typo","guardrail_domain":"","needs_discovery":false,"complexity":"simple"}' | slipway new --json`, "%s: missing minimal JSON stdin example", id)
		assert.NotContains(t, s, "--guardrail-domain", "%s: unsupported guardrail flag leaked", id)
		assert.NotContains(t, s, "--needs-discovery", "%s: unsupported discovery flag leaked", id)
		assert.NotContains(t, s, "--complexity", "%s: unsupported complexity flag leaked", id)
	}
}

func TestRenderCatalogSkillUsesFixedTypedTemplateOrder(t *testing.T) {
	t.Parallel()

	skill := capability.Skill{
		ID: "_test-catalog-assembler",
		Bindings: []capability.Binding{
			{Attachment: capability.AttachmentProcedure},
			{Attachment: capability.AttachmentChecklist},
			{Attachment: capability.AttachmentReportSchema},
		},
	}

	content, err := renderCatalogSkill(skill)
	require.NoError(t, err)

	bodyIdx := regexp.MustCompile("BODY_SECTION").FindStringIndex(content)
	proseIdx := regexp.MustCompile("PROSE_SECTION").FindStringIndex(content)
	checklistIdx := regexp.MustCompile("CHECKLIST_SECTION").FindStringIndex(content)
	verdictIdx := regexp.MustCompile("VERDICT_SECTION").FindStringIndex(content)
	require.NotNil(t, bodyIdx)
	require.NotNil(t, proseIdx)
	require.NotNil(t, checklistIdx)
	require.NotNil(t, verdictIdx)
	assert.Less(t, bodyIdx[0], proseIdx[0])
	assert.Less(t, proseIdx[0], checklistIdx[0])
	assert.Less(t, checklistIdx[0], verdictIdx[0])
}

func TestRenderCatalogSkillPreservesSingleFileWhenNoTypedTemplates(t *testing.T) {
	t.Parallel()

	reg := capability.DefaultRegistry()
	skill, ok := reg.Lookup("context-assembly")
	require.True(t, ok)

	content, err := renderCatalogSkill(skill)
	require.NoError(t, err)

	raw, err := tmpl.Content(filepath.ToSlash(filepath.Join("skills", "context-assembly", "SKILL.md")))
	require.NoError(t, err)

	// Output equals the source body with the adapter-frontmatter header
	// prepended; no typed-template sections are appended.
	injected, err := injectAdapterFrontmatter(raw, "slipway-"+skill.ID, skill.Summary)
	require.NoError(t, err)
	assert.Equal(t, injected, content)
}

func TestInjectAdapterFrontmatterPrependsNameAndDescription(t *testing.T) {
	t.Parallel()

	src := "---\nskill_id: demo\nsummary: \"Use when X. Triggers on Y.\"\n---\n\n# Body\n"

	out, err := injectAdapterFrontmatter(src, "slipway-demo", "Use when X. Triggers on Y.")
	require.NoError(t, err)

	// Header lines appear before existing fields.
	nameAt := strings.Index(out, "name: slipway-demo")
	descAt := strings.Index(out, "description: \"Use when X. Triggers on Y.\"")
	skillIDAt := strings.Index(out, "skill_id: demo")
	bodyAt := strings.Index(out, "# Body")
	require.GreaterOrEqual(t, nameAt, 0)
	require.GreaterOrEqual(t, descAt, 0)
	require.GreaterOrEqual(t, skillIDAt, 0)
	require.GreaterOrEqual(t, bodyAt, 0)
	assert.Less(t, nameAt, descAt)
	assert.Less(t, descAt, skillIDAt)
	assert.Less(t, skillIDAt, bodyAt)
}

func TestInjectAdapterFrontmatterNormalizesCRLFDelimiters(t *testing.T) {
	t.Parallel()

	src := "---\r\nskill_id: demo\r\nsummary: \"Use when X. Triggers on Y.\"\r\n---\r\n\r\n# Body\r\n"

	out, err := injectAdapterFrontmatter(src, "slipway-demo", "Use when X. Triggers on Y.")
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(out, "---\nname: slipway-demo\n"))
	assert.Contains(t, out, "\n---\n\n# Body\n")
	assert.NotContains(t, out, "\r")
}

func TestInjectAdapterFrontmatterEscapesDoubleQuotes(t *testing.T) {
	t.Parallel()

	src := "---\nskill_id: demo\n---\n\nbody\n"

	out, err := injectAdapterFrontmatter(src, "slipway-demo", `needs "quote" and \backslash`)
	require.NoError(t, err)
	assert.Contains(t, out, `description: "needs \"quote\" and \\backslash"`)
}

func TestInjectAdapterFrontmatterRejectsMissingDelimiter(t *testing.T) {
	t.Parallel()

	_, err := injectAdapterFrontmatter("no frontmatter here", "slipway-x", "desc")
	assert.Error(t, err)
}

func TestExtractAndStripAdapterFieldsStripsCanonicalFields(t *testing.T) {
	t.Parallel()

	src := "---\nskill_id: demo\nname: slipway-demo\ndescription: \"Use when X. Triggers on Y.\"\nsummary: \"keep me\"\n---\n\n# Body\n"

	description, stripped, err := extractAndStripAdapterFields(src, "demo")
	require.NoError(t, err)
	assert.Equal(t, "Use when X. Triggers on Y.", description)
	assert.Contains(t, stripped, "skill_id: demo")
	assert.Contains(t, stripped, "summary: \"keep me\"")
	assert.NotContains(t, stripped, "\nname:")
	assert.NotContains(t, stripped, "\ndescription:")
}

func TestExtractAndStripAdapterFieldsRejectsPublicNameDrift(t *testing.T) {
	t.Parallel()

	src := "---\nskill_id: demo\nname: demo\ndescription: \"Use when X. Triggers on Y.\"\n---\n"

	_, _, err := extractAndStripAdapterFields(src, "demo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slipway-demo")
}

func TestAdapterSkillNameUsesBareEntryNameOnlyForWorkflow(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "slipway", adapterSkillName("workflow"))
	assert.Equal(t, "slipway-demo", adapterSkillName("demo"))
}

func TestExtractAndStripAdapterFieldsRejectsMissingDescription(t *testing.T) {
	t.Parallel()

	src := "---\nskill_id: demo\nname: slipway-demo\n---\n"

	_, _, err := extractAndStripAdapterFields(src, "demo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing description")
}

func TestRenderCatalogSkillUsesTypedTemplatesForProductionSkill(t *testing.T) {
	t.Parallel()

	reg := capability.DefaultRegistry()
	skill, ok := reg.Lookup("independent-review")
	require.True(t, ok)

	content, err := renderCatalogSkill(skill)
	require.NoError(t, err)

	raw, err := tmpl.Content(filepath.ToSlash(filepath.Join("skills", "independent-review", "SKILL.md")))
	require.NoError(t, err)

	// This asserts production catalog assembly actually uses optional typed
	// templates, not only the dedicated test fixture.
	assert.NotEqual(t, raw, content)
	assert.Contains(t, content, "## Procedure")
	assert.Contains(t, content, "## Checklist")
	assert.Contains(t, content, "## Report schema")
}

func TestGeneratedAdapterSurfacesStayInSyncWithRegistry(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	toolIDs, err := ResolveTools("all")
	require.NoError(t, err)
	require.NoError(t, Generate(root, toolIDs, true))

	for _, cfg := range Registry() {
		for _, id := range commandIDs() {
			_, absPath := commandSurfacePath(root, codexHome, cfg, id)
			content, err := os.ReadFile(absPath)
			require.NoError(t, err, "missing generated command surface for %s/%s", cfg.ID, id)

			s := string(content)
			assert.Contains(t, s, commandDescriptions[id], "%s/%s missing registry description", cfg.ID, id)
			if cfg.CommandSkillSurface {
				assert.Contains(t, s, "name: "+adapterSkillName(id), "%s/%s missing command skill name frontmatter", cfg.ID, id)
				assert.Contains(t, s, commandTrigger(cfg, id), "%s/%s missing registry trigger", cfg.ID, id)
			} else {
				assert.Contains(t, s, commandTrigger(cfg, id), "%s/%s missing registry trigger", cfg.ID, id)
			}
		}

		if cfg.SettingsPath != "" && cfg.SessionHook != "" {
			assertHookCommandRegistered(
				t,
				filepath.Join(root, cfg.SettingsPath),
				cfg.SessionEvent,
				sessionStartHookCommand,
			)
		}
	}
}

// extractAdapterFrontmatter reads a minimal adapter contract from a
// generated SKILL.md: the set of `key: value` pairs inside the leading
// `---` / `---` frontmatter block. It tolerates double-quoted values and
// ignores nested YAML (lists, maps), which are not part of the adapter
// contract.
func extractAdapterFrontmatter(t *testing.T, raw, toolID, skillID string) map[string]string {
	t.Helper()
	require.Truef(t, len(raw) >= 4 && raw[:4] == "---\n",
		"%s: catalog skill %s: missing frontmatter opener", toolID, skillID)
	rest := raw[4:]
	end := indexOf(rest, "\n---")
	require.GreaterOrEqualf(t, end, 0,
		"%s: catalog skill %s: missing frontmatter closer", toolID, skillID)
	out := map[string]string{}
	for _, line := range splitLines(rest[:end]) {
		// Only scan flat top-level `key: value` pairs. Lines that start
		// with whitespace or `-` belong to nested structures.
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '-' {
			continue
		}
		colon := indexOf(line, ":")
		if colon <= 0 {
			continue
		}
		key := line[:colon]
		value := ""
		if colon+1 < len(line) {
			value = strings.TrimSpace(line[colon+1:])
		}
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		out[key] = value
	}
	return out
}

func indexOf(s, sub string) int { return strings.Index(s, sub) }
func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

func writeGeneratedAdapterMarker(t *testing.T, root string, cfg ToolConfig) {
	t.Helper()

	markerPath := filepath.Join(root, GeneratedAdapterMarkerPath(cfg))
	require.NoError(t, os.MkdirAll(filepath.Dir(markerPath), 0o755))
	require.NoError(t, os.WriteFile(markerPath, []byte("marker"), 0o644))
}

func TestGeneratedSkillsReferenceValidCommands(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"claude"}, true))

	// Build set of valid commands.
	validCmds := map[string]bool{
		"new": true, "next": true, "run": true, "status": true, "done": true,
		"abort": true, "cancel": true, "review": true, "validate": true,
		"pivot": true, "preset": true, "repair": true, "init": true, "checkpoint": true,
		"instructions": true,
	}

	// Pattern to find `slipway <cmd>` references in generated files.
	re := regexp.MustCompile("`slipway ([a-z][a-z-]*)`")

	cfg := toolRegistry["claude"]
	allSkillIDs := append([]string(nil), GovernanceSkillNames...)
	allSkillIDs = append(allSkillIDs, standaloneGovernanceNames...)
	allSkillIDs = append(allSkillIDs, TemplatedGovernanceSkillNames...)
	allSkillIDs = append(allSkillIDs, techniqueNames...)

	for _, id := range allSkillIDs {
		path := filepath.Join(root, cfg.SkillsDir, "slipway-"+id, "SKILL.md")
		content, err := os.ReadFile(path)
		if err != nil {
			continue // skip if file doesn't exist
		}

		matches := re.FindAllStringSubmatch(string(content), -1)
		for _, m := range matches {
			cmd := m[1]
			// Allow `slipway next --json` style references (extract base command).
			assert.True(t, validCmds[cmd],
				"skill %q references unknown command `slipway %s`", id, cmd)
		}

		// Verify no `slipway advance` references remain (D4 validation).
		assert.NotContains(t, string(content), "slipway advance",
			"skill %q still references retired `slipway advance`", id)
	}
}

func TestGenerateDeterministicAndRefresh(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"claude"}, false))

	commandPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")
	firstCommand, err := os.ReadFile(commandPath)
	require.NoError(t, err)

	// Non-refresh generation should keep existing files unchanged.
	require.NoError(t, os.WriteFile(commandPath, []byte("custom"), 0o644))
	require.NoError(t, Generate(root, []string{"claude"}, false))
	secondCommand, err := os.ReadFile(commandPath)
	require.NoError(t, err)
	assert.Equal(t, "custom", string(secondCommand))

	// Refresh should deterministically regenerate content.
	require.NoError(t, Generate(root, []string{"claude"}, true))
	refreshedCommand, err := os.ReadFile(commandPath)
	require.NoError(t, err)
	assert.Equal(t, string(firstCommand), string(refreshedCommand))
}

func TestGenerateRefreshPrunesOnlyGeneratedTopLevelSkillEntries(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())

	require.NoError(t, Generate(root, []string{"claude"}, true))

	staleSkillDir := filepath.Join(root, ".claude", "skills", "slipway-tdd")
	prefixedUserOwnedDir := filepath.Join(root, ".claude", "skills", "slipway-user-owned")
	prefixedUserOwnedFile := filepath.Join(prefixedUserOwnedDir, "SKILL.md")
	unrelatedDir := filepath.Join(root, ".claude", "skills", "user-owned")
	unrelatedFile := filepath.Join(unrelatedDir, "SKILL.md")

	require.NoError(t, os.MkdirAll(staleSkillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staleSkillDir, "SKILL.md"), []byte("stale generated skill"), 0o644))
	require.NoError(t, os.MkdirAll(prefixedUserOwnedDir, 0o755))
	require.NoError(t, os.WriteFile(prefixedUserOwnedFile, []byte("keep prefixed user skill"), 0o644))
	require.NoError(t, os.MkdirAll(unrelatedDir, 0o755))
	require.NoError(t, os.WriteFile(unrelatedFile, []byte("keep me"), 0o644))
	oldCatalogRoutePath := filepath.Join(root, ".claude", "skills", "slipway", "references", "catalog", "sast-orchestration.md")
	oldCatalogSupportPath := filepath.Join(root, ".claude", "skills", "slipway", "references", "catalog", "sast-orchestration", "scripts", "merge-sarif.sh")
	require.NoError(t, os.MkdirAll(filepath.Dir(oldCatalogRoutePath), 0o755))
	require.NoError(t, os.WriteFile(oldCatalogRoutePath, []byte("old catalog route"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Dir(oldCatalogSupportPath), 0o755))
	require.NoError(t, os.WriteFile(oldCatalogSupportPath, []byte("old catalog support"), 0o644))

	require.NoError(t, Generate(root, []string{"claude"}, true))

	_, err := os.Stat(staleSkillDir)
	assert.True(t, os.IsNotExist(err), "refresh should remove managed generated skill dirs that are no longer exported")
	_, err = os.Stat(prefixedUserOwnedFile)
	assert.NoError(t, err, "refresh must not delete unknown user-managed slipway-* skill dirs")
	_, err = os.Stat(unrelatedFile)
	assert.NoError(t, err, "refresh must not delete unrelated user-managed entries under skills dir")
	_, err = os.Stat(oldCatalogRoutePath)
	assert.True(t, os.IsNotExist(err), "refresh should remove stale workflow support files")
	_, err = os.Stat(oldCatalogSupportPath)
	assert.True(t, os.IsNotExist(err), "refresh should remove stale workflow support files")
	_, err = os.Stat(filepath.Join(root, ".claude", "skills", "slipway", "references", skillIndexFileName))
	assert.NoError(t, err, "skill index should live under workflow references")
}

func TestGenerateRefreshDoesNotPruneSkillDirsWithoutGeneratedAdapterMarker(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())

	userOwnedSlipwayDir := filepath.Join(root, ".claude", "skills", "slipway-user-owned")
	require.NoError(t, os.MkdirAll(userOwnedSlipwayDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(userOwnedSlipwayDir, "SKILL.md"), []byte("keep me"), 0o644))

	require.NoError(t, Generate(root, []string{"claude"}, true))

	_, err := os.Stat(filepath.Join(userOwnedSlipwayDir, "SKILL.md"))
	assert.NoError(t, err, "refresh without a generated adapter marker must not prune user-owned slipway-* skill dirs")
}

func TestCodexGenerationOmitsProjectAgentSurfaces(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())
	require.NoError(t, Generate(root, []string{"codex"}, true))

	_, err := os.Stat(filepath.Join(root, ".codex", "agents"))
	assert.True(t, os.IsNotExist(err), "codex should not generate exported agents")
	_, err = os.Stat(filepath.Join(root, ".codex", "config.toml"))
	assert.True(t, os.IsNotExist(err), "codex should not create project config on fresh init")
}

func TestGeminiTOMLCommandFormat(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"gemini"}, true))

	// Gemini commands should use .toml extension.
	path := filepath.Join(root, ".gemini", "commands", "slipway", "new.toml")
	content, err := os.ReadFile(path)
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, `description = "`)
	assert.Contains(t, s, `prompt = """`)

	// No .md command files should exist.
	mdPath := filepath.Join(root, ".gemini", "commands", "slipway", "new.md")
	_, err = os.Stat(mdPath)
	assert.True(t, os.IsNotExist(err), "gemini should not have .md command files")
}

func TestHookSettingsRegistrationForClaudeAndGemini(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())
	require.NoError(t, Generate(root, []string{"claude", "gemini"}, true))

	// Claude registers BOTH the inline session-start and context-pressure
	// commands directly in settings.json, with no launcher path, no
	// `.claude/hooks/` reference, no `--tool` flag, and no shell operator.
	claudeSettings, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	require.NoError(t, err)
	claude := string(claudeSettings)
	assert.Contains(t, claude, "SessionStart")
	assert.Contains(t, claude, sessionStartHookCommand)
	assert.Contains(t, claude, "PostToolUse")
	assert.Contains(t, claude, contextPressureHookCommand)
	assert.NotContains(t, claude, ".claude/hooks/", "claude settings must not reference a launcher path")
	assert.NotContains(t, claude, "slipway-session-start", "claude settings must not name a launcher file")
	assert.NotContains(t, claude, "slipway-context-pressure-post-tool-use", "claude settings must not name a launcher file")
	assert.NotContains(t, claude, "--tool", "claude settings must use the bare inline command")
	assert.NotContains(t, claude, "bash", "claude settings must not require bash")
	assert.NotContains(t, claude, "||", "claude settings must not require shell fallback operators")
	assert.NotContains(t, claude, "&&", "claude settings must not require shell fallback operators")

	// Gemini registers ONLY the inline session-start command (no PostToolUse).
	geminiSettings, err := os.ReadFile(filepath.Join(root, ".gemini", "settings.json"))
	require.NoError(t, err)
	gemini := string(geminiSettings)
	assert.Contains(t, gemini, "SessionStart")
	assert.Contains(t, gemini, sessionStartHookCommand)
	assert.NotContains(t, gemini, "PostToolUse")
	assert.NotContains(t, gemini, contextPressureHookCommand)
	assert.NotContains(t, gemini, ".gemini/hooks/", "gemini settings must not reference a launcher path")
	assert.NotContains(t, gemini, "slipway-session-start", "gemini settings must not name a launcher file")
	assert.NotContains(t, gemini, "--tool", "gemini settings must use the bare inline command")
	assert.NotContains(t, gemini, "bash", "gemini settings must not require bash")
	assert.NotContains(t, gemini, "||", "gemini settings must not require shell fallback operators")

	// Neither settings-capable host emits any launcher file (extensionless +
	// .ps1/.cmd/.sh) for either hook event.
	for _, base := range []string{
		filepath.Join(".claude", "hooks", "slipway-session-start"),
		filepath.Join(".claude", "hooks", "slipway-context-pressure-post-tool-use"),
		filepath.Join(".gemini", "hooks", "slipway-session-start"),
	} {
		for _, suffix := range []string{"", ".ps1", ".cmd", ".sh"} {
			p := filepath.Join(root, base+suffix)
			_, err := os.Stat(p)
			assert.True(t, os.IsNotExist(err),
				"settings-capable host must not emit launcher file %s", base+suffix)
		}
	}
}

func TestGenerateRefreshPrunesOrphanedHookLaunchersForSettingsCapableHosts(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())

	// Seed the full orphaned launcher family (extensionless + .ps1 + .cmd + .sh)
	// for both settings-capable hosts (claude, gemini), plus the same for the
	// file-by-path hosts (cursor, opencode) which legitimately re-emit launchers.
	settingsCapableOrphans := []string{
		filepath.Join(root, ".claude", "hooks", "slipway-session-start"),
		filepath.Join(root, ".claude", "hooks", "slipway-session-start.ps1"),
		filepath.Join(root, ".claude", "hooks", "slipway-session-start.cmd"),
		filepath.Join(root, ".claude", "hooks", "slipway-session-start.sh"),
		filepath.Join(root, ".claude", "hooks", "slipway-context-pressure-post-tool-use"),
		filepath.Join(root, ".claude", "hooks", "slipway-context-pressure-post-tool-use.ps1"),
		filepath.Join(root, ".claude", "hooks", "slipway-context-pressure-post-tool-use.cmd"),
		filepath.Join(root, ".claude", "hooks", "slipway-context-pressure-post-tool-use.sh"),
		filepath.Join(root, ".gemini", "hooks", "slipway-session-start"),
		filepath.Join(root, ".gemini", "hooks", "slipway-session-start.ps1"),
		filepath.Join(root, ".gemini", "hooks", "slipway-session-start.cmd"),
		filepath.Join(root, ".gemini", "hooks", "slipway-session-start.sh"),
	}
	fileByPathSeeds := []string{
		filepath.Join(root, ".cursor", "hooks", "slipway-session-start.sh"),
		filepath.Join(root, ".opencode", "hooks", "slipway-session-start.sh"),
	}
	for _, p := range append(append([]string{}, settingsCapableOrphans...), fileByPathSeeds...) {
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755))
	}

	require.NoError(t, Generate(root, []string{"claude", "gemini", "cursor", "opencode"}, true))

	// Settings-capable hosts no longer emit launchers; every orphaned launcher
	// file is pruned on refresh.
	for _, p := range settingsCapableOrphans {
		_, err := os.Stat(p)
		assert.True(t, os.IsNotExist(err),
			"refresh must prune orphaned launcher %s for settings-capable host", p)
	}

	// File-by-path hosts (cursor, opencode) still emit their session-start
	// launcher family, so the extensionless entry remains present.
	for _, p := range []string{
		filepath.Join(root, ".cursor", "hooks", "slipway-session-start"),
		filepath.Join(root, ".opencode", "hooks", "slipway-session-start"),
	} {
		_, err := os.Stat(p)
		assert.NoError(t, err, "file-by-path host must retain its session-start launcher %s", p)
	}
}

func TestCommandEntryPrerequisitesAreCommandSpecific(t *testing.T) {
	t.Parallel()

	newEntry, err := renderCommandEntry(toolRegistry["claude"], "new")
	require.NoError(t, err)
	assert.Contains(t, newEntry, "Create a governed change with intake-first workflow")
	assert.Contains(t, newEntry, ".slipway.yaml` must exist")
	assert.NotContains(t, newEntry, "an active change must exist")
	assert.NotContains(t, newEntry, "change intake")

	statusEntry, err := renderCommandEntry(toolRegistry["claude"], "status")
	require.NoError(t, err)
	assert.Contains(t, statusEntry, ".slipway.yaml` must exist")
	assert.NotContains(t, statusEntry, "an active change must exist")

	nextEntry, err := renderCommandEntry(toolRegistry["claude"], "next")
	require.NoError(t, err)
	assert.Contains(t, nextEntry, ".slipway.yaml` must exist")
	assert.Contains(t, nextEntry, "an active change must exist")

	initEntry, err := renderCommandEntry(toolRegistry["gemini"], "init")
	require.NoError(t, err)
	assert.NotContains(t, initEntry, ".slipway.yaml` must exist")
	assert.NotContains(t, initEntry, "an active change must exist")
}

func TestCommandEntriesLockNextAndRunExecutionContracts(t *testing.T) {
	t.Parallel()

	nextEntry, err := renderCommandEntry(toolRegistry["claude"], "next")
	require.NoError(t, err)
	normalizedNextEntry := strings.Join(strings.Fields(nextEntry), " ")
	assert.Contains(t, normalizedNextEntry, "without advancing lifecycle state")
	assert.Contains(t, nextEntry, "`next_skill.name` is the authoritative governed-host handoff")
	assert.Contains(t, nextEntry, "Run `slipway run --json` when evidence is ready")

	runEntry, err := renderCommandEntry(toolRegistry["claude"], "run")
	require.NoError(t, err)
	assert.Contains(t, runEntry, "Advance governed execution until it surfaces a next skill, blocker, checkpoint, or done-ready outcome")
	assert.Contains(t, runEntry, "`run` owns continuous governed execution")
	assert.Contains(t, runEntry, "`run` reuses the same `next --json` contract")
}

func TestReadmeAndCommandDescriptionsReflectCurrentEntrySurface(t *testing.T) {
	t.Parallel()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)

	readmePath := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "README.md"))
	content, err := os.ReadFile(readmePath)
	require.NoError(t, err)
	readme := string(content)

	assert.Contains(t, readme, "`slipway new`")
	assert.Contains(t, readme, "`slipway run`")
	assert.Contains(t, readme, "`artifacts/changes/`")
	assert.Contains(t, readme, "`artifacts/codebase/`")
	assert.NotContains(t, readme, "request intake")
	assert.NotContains(t, readme, "active request resolution")
	assert.NotContains(t, readme, "`openspec/`")
	assert.NotContains(t, readme, "`openspec/`: change and spec artifacts used by governed workflows")

	assert.Equal(t, "Create a governed change with intake-first workflow", commandDescriptions["new"])
	assert.Equal(t, "Advance governed execution until a skill, blocker, checkpoint, or done-ready outcome is surfaced", commandDescriptions["run"])
	assert.Equal(t, "Finalize a done-ready change and archive it", commandDescriptions["done"])
}

func TestWaveOrchestrationSkillForcesParallelByDefault(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"claude"}, true))

	cfg := toolRegistry["claude"]
	body, err := os.ReadFile(filepath.Join(root, cfg.SkillsDir, "slipway-wave-orchestration", "SKILL.md"))
	require.NoError(t, err)
	skill := strings.Join(strings.Fields(string(body)), " ")

	assert.Contains(t, skill, "concurrently by default",
		"wave-orchestration must instruct parallel-by-default dispatch")
	assert.Contains(t, skill, "parallel: true",
		"wave-orchestration must reference the per-wave parallel signal from next --json")
	assert.Contains(t, skill, "degradation",
		"wave-orchestration must require noting a degraded sequential fallback")
	assert.Contains(t, skill, "dispatch_mode:wave=<wave_index>:degraded_sequential",
		"wave-orchestration must require structured degraded dispatch evidence")
	assert.Contains(t, skill, "parallelization: off",
		"wave-orchestration must describe the parallelization off-switch")
	assert.Contains(t, skill, "post-wave integration gate",
		"wave-orchestration must require a post-wave integration gate before the next wave")
	assert.Contains(t, skill, "Do not run shared-worktree-wide integration commands",
		"wave-orchestration must keep shared-worktree build/test commands out of task executors")
	assert.Contains(t, skill, "Notes/prose alone",
		"wave-orchestration must make the structured dispatch reference contract explicit")
	assert.Contains(t, skill, "do not start the next wave",
		"wave-orchestration must block subsequent waves when integration fails")
	assert.Contains(t, skill, "real executor subagent fan-out",
		"wave-orchestration must require real fan-out for parallel waves")
	assert.Contains(t, skill, "same-context inline execution",
		"wave-orchestration must reject silent inline execution for capable runtimes")
	assert.Contains(t, skill, "task_id",
		"wave-orchestration must name the stable executor result contract")
	assert.Contains(t, skill, "single worktree",
		"wave-orchestration must describe the single-worktree execution boundary")
	assert.Contains(t, skill, "target-overlap preflight",
		"wave-orchestration must require a target-overlap preflight before spawning")
	assert.Contains(t, skill, "post-result changed-file conflict",
		"wave-orchestration must require post-result conflict detection before integration")
	assert.Contains(t, skill, "executor_agent:wave=<wave_index>:task=<task_id>:<handle>",
		"wave-orchestration must require per-task executor handle references")
}

func TestGeneratedWaveOrchestrationCodexDispatchUsesSpawnAgent(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"codex"}, true))

	cfg := toolRegistry["codex"]
	refPath := filepath.Join(
		root,
		cfg.SkillsDir,
		"slipway-wave-orchestration",
		"references",
		"executor-dispatch-reference.md",
	)
	raw, err := os.ReadFile(refPath)
	require.NoError(t, err)
	ref := strings.Join(strings.Fields(string(raw)), " ")

	assert.Contains(t, ref, "spawn_agent",
		"generated Codex dispatch reference must use spawn_agent")
	assert.Contains(t, ref, "tool_search",
		"generated Codex dispatch reference must discover deferred tools")
	assert.Contains(t, ref, "fork_context: false",
		"generated Codex dispatch reference must request fresh-context execution")
	assert.Contains(t, ref, "collect agent IDs",
		"generated Codex dispatch reference must collect spawned agent handles")
	assert.Contains(t, ref, "wait for all",
		"generated Codex dispatch reference must wait for all executors")
	assert.Contains(t, ref, "close each agent",
		"generated Codex dispatch reference must close executors when supported")
	assert.NotContains(t, ref, "codex -q --task",
		"generated Codex dispatch reference must not include the old shell fan-out path")
	assert.Contains(t, ref, "explicit user authorization",
		"generated Codex dispatch reference must stop for authorization when required")
	assert.Contains(t, ref, "executor_dispatch_stalled",
		"generated Codex dispatch reference must define stalled-executor recovery")
	assert.Contains(t, ref, "executor_result_missing",
		"generated Codex dispatch reference must define missing-result recovery")
	assert.Contains(t, ref, ".git/config.lock",
		"generated dispatch reference must preserve safe background fan-out guidance")
	assert.Contains(t, ref, "Do not wrap a spawner workflow inside another subagent",
		"generated dispatch reference must preserve nested-spawner guidance")
}

func codexCommandSkillPath(root, id string) string {
	return filepath.Join(root, ".codex", "skills", "slipway-"+id, "SKILL.md")
}

func TestCodexCommandSkillsUseCommandSpecificPrerequisites(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())

	require.NoError(t, Generate(root, []string{"codex"}, true))

	newSkill, err := os.ReadFile(codexCommandSkillPath(root, "new"))
	require.NoError(t, err)
	assert.NotContains(t, string(newSkill), "an active change must exist")

	statusSkill, err := os.ReadFile(codexCommandSkillPath(root, "status"))
	require.NoError(t, err)
	assert.NotContains(t, string(statusSkill), "an active change must exist")

	nextSkill, err := os.ReadFile(codexCommandSkillPath(root, "next"))
	require.NoError(t, err)
	assert.Contains(t, string(nextSkill), "an active change must exist")

	initSkill, err := os.ReadFile(codexCommandSkillPath(root, "init"))
	require.NoError(t, err)
	assert.NotContains(t, string(initSkill), ".slipway.yaml` must exist")
}

func TestCodexCommandSkillsUseCommandRegistryArguments(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())

	require.NoError(t, Generate(root, []string{"codex"}, true))

	for _, id := range commandIDs() {
		body, err := os.ReadFile(codexCommandSkillPath(root, id))
		require.NoError(t, err, "missing codex command skill for %s", id)

		args := CommandArguments(id)
		require.NotEmpty(t, args, "registry Arguments missing for command %s", id)
		assert.Contains(t, string(body), args,
			"codex command skill %s must render registry Arguments", id)
	}
}

func TestCodexCommandSkillsIncludeTierAndSurface(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())

	require.NoError(t, Generate(root, []string{"codex"}, true))

	// Core command should have tier: "core".
	newSkill, err := os.ReadFile(codexCommandSkillPath(root, "new"))
	require.NoError(t, err)
	assert.Contains(t, string(newSkill), `class: "mutation"`, "core command skill missing class metadata")
	assert.Contains(t, string(newSkill), `tier: "core"`, "core command skill missing tier metadata")
	assert.Contains(t, string(newSkill), `surface: "skill"`, "command skill missing surface metadata")

	// Query command should preserve class alongside tier and surface metadata.
	statusSkill, err := os.ReadFile(codexCommandSkillPath(root, "status"))
	require.NoError(t, err)
	assert.Contains(t, string(statusSkill), `class: "query"`, "query command skill missing class metadata")
	assert.Contains(t, string(statusSkill), `tier: "core"`, "query command skill missing tier metadata")
	assert.Contains(t, string(statusSkill), `surface: "skill"`, "query command skill missing surface metadata")
}

func TestGeneratedCommandEntriesIncludeClassMetadata(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())

	require.NoError(t, Generate(root, []string{"claude", "gemini", "codex"}, true))

	claudeStatus, err := os.ReadFile(filepath.Join(root, ".claude", "commands", "slipway", "status.md"))
	require.NoError(t, err)
	assert.Contains(t, string(claudeStatus), `class: "query"`)

	geminiRun, err := os.ReadFile(filepath.Join(root, ".gemini", "commands", "slipway", "run.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(geminiRun), `class = "mutation"`)

	codexAbort, err := os.ReadFile(codexCommandSkillPath(root, "abort"))
	require.NoError(t, err)
	assert.Contains(t, string(codexAbort), `class: "mutation"`)
}

func TestGeneratedWaveOrchestrationSkillUsesWavePlanSummaryGuidance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"claude"}, true))

	path := filepath.Join(root, ".claude", "skills", "slipway-wave-orchestration", "SKILL.md")
	content, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Contains(t, string(content), "description, total tasks, wave count")
	assert.NotContains(t, string(content), "{request_description}")
}

func TestCodexRefreshDoesNotCreateManagedConfigTOML(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())

	require.NoError(t, Generate(root, []string{"codex"}, true))
	_, err := os.Stat(filepath.Join(root, ".codex", "config.toml"))
	assert.True(t, os.IsNotExist(err), "fresh generation should not create config.toml")

	require.NoError(t, Generate(root, []string{"codex"}, true))
	_, err = os.Stat(filepath.Join(root, ".codex", "config.toml"))
	assert.True(t, os.IsNotExist(err), "refresh should not create project config")
}

func TestCursorNoAgents(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"cursor"}, true))

	// No agent directory should be created for Cursor.
	agentsDir := filepath.Join(root, ".cursor", "agents")
	_, err := os.Stat(agentsDir)
	assert.True(t, os.IsNotExist(err), "cursor should not have agents directory")
}

func TestCodexNoProjectLocalCommands(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())
	require.NoError(t, Generate(root, []string{"codex"}, true))

	// No commands directory should be created.
	commandsDir := filepath.Join(root, ".codex", "commands")
	_, err := os.Stat(commandsDir)
	assert.True(t, os.IsNotExist(err), "codex should not have project-local commands")
}

func TestOpenCodeFlatCommands(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"opencode"}, true))

	// OpenCode command names come directly from file names, so use flat paths
	// to keep slash-hyphen invocation stable.
	path := filepath.Join(root, ".opencode", "commands", "slipway-new.md")
	_, err := os.Stat(path)
	assert.NoError(t, err, "opencode should have flat command paths")
}

func TestOpenCodeRefreshPrunesLegacyNestedCommands(t *testing.T) {
	root := t.TempDir()

	sentinelPath := filepath.Join(root, ".opencode", "slipway", ".adapter-generated")
	require.NoError(t, os.MkdirAll(filepath.Dir(sentinelPath), 0o755))
	require.NoError(t, os.WriteFile(sentinelPath, []byte("generated\n"), 0o644))

	legacyDir := filepath.Join(root, ".opencode", "commands", "slipway")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "new.md"), []byte("legacy generated command"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "custom.md"), []byte("keep user command"), 0o644))

	require.NoError(t, Generate(root, []string{"opencode"}, true))

	_, err := os.Stat(filepath.Join(root, ".opencode", "commands", "slipway-new.md"))
	assert.NoError(t, err, "refresh should write flat opencode command paths")
	_, err = os.Stat(filepath.Join(legacyDir, "new.md"))
	assert.True(t, os.IsNotExist(err), "refresh should prune legacy generated nested commands")
	_, err = os.Stat(filepath.Join(legacyDir, "custom.md"))
	assert.NoError(t, err, "refresh must not delete unknown nested user commands")
}

func TestOpenCodeRefreshWithoutGeneratedMarkerDoesNotPruneNestedCommands(t *testing.T) {
	root := t.TempDir()

	legacyDir := filepath.Join(root, ".opencode", "commands", "slipway")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "new.md"), []byte("user command"), 0o644))

	require.NoError(t, Generate(root, []string{"opencode"}, true))

	_, err := os.Stat(filepath.Join(root, ".opencode", "commands", "slipway-new.md"))
	assert.NoError(t, err, "refresh should write flat opencode command paths")
	_, err = os.Stat(filepath.Join(legacyDir, "new.md"))
	assert.NoError(t, err, "refresh without a generated adapter marker must not prune nested user commands")
}

func TestCodexCommandSkills(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	require.NoError(t, Generate(root, []string{"codex"}, true))

	// Command surfaces should be generated as discoverable per-command skills
	// under .codex/skills/slipway-<command>/SKILL.md.
	skillPath := codexCommandSkillPath(root, "new")
	content, err := os.ReadFile(skillPath)
	require.NoError(t, err)

	s := string(content)
	// Check injected frontmatter.
	assert.Contains(t, s, "name: slipway-new")
	assert.Contains(t, s, "description:")
	assert.Contains(t, s, `surface: "skill"`)
	// The skill surface must not carry the retired prompt frontmatter.
	assert.NotContains(t, s, "argument-hint:")
	assert.NotContains(t, s, "$ARGUMENTS")
	// Check it contains the body partial content.
	assert.Contains(t, s, "slipway new")

	// Verify all command IDs have command skills, each carrying its name and
	// description frontmatter.
	for _, id := range commandIDs() {
		body, err := os.ReadFile(codexCommandSkillPath(root, id))
		require.NoError(t, err, "missing codex command skill for %s", id)
		assert.Contains(t, string(body), "name: slipway-"+id, "codex command skill %s missing name frontmatter", id)
		assert.Contains(t, string(body), "description:", "codex command skill %s missing description frontmatter", id)
	}

	// No generated legacy command prompt files must be written under
	// $CODEX_HOME/prompts.
	assertNoCodexLegacyCommandPrompts(t, codexHome)

	// Verify refresh protection.
	require.NoError(t, os.WriteFile(skillPath, []byte("custom"), 0o644))
	require.NoError(t, Generate(root, []string{"codex"}, false))
	customContent, _ := os.ReadFile(skillPath)
	assert.Equal(t, "custom", string(customContent), "command skill should not be overwritten without refresh")

	// Verify refresh overwrites.
	require.NoError(t, Generate(root, []string{"codex"}, true))
	refreshedContent, _ := os.ReadFile(skillPath)
	assert.NotEqual(t, "custom", string(refreshedContent), "command skill should be overwritten with refresh")
}

func TestCodexRefreshPrunesOnlyLegacyGeneratedCommandPrompts(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	promptsDir := filepath.Join(codexHome, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "slipway-new.md"), []byte("legacy generated command"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "slipway-run.md"), []byte("legacy generated command"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "slipway-personal.md"), []byte("user prompt"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "not-slipway.md"), []byte("user prompt"), 0o644))

	require.NoError(t, Generate(root, []string{"codex"}, true))

	assertNoCodexLegacyCommandPrompts(t, codexHome)

	personal, err := os.ReadFile(filepath.Join(promptsDir, "slipway-personal.md"))
	require.NoError(t, err, "refresh must not delete user-owned slipway-* prompts outside the command registry")
	assert.Equal(t, "user prompt", string(personal))
	other, err := os.ReadFile(filepath.Join(promptsDir, "not-slipway.md"))
	require.NoError(t, err, "refresh must not delete unrelated prompts")
	assert.Equal(t, "user prompt", string(other))
}

// assertNoCodexLegacyCommandPrompts asserts that no retired generated Codex
// command prompt files exist under $CODEX_HOME/prompts. It checks the command
// registry filenames exactly instead of the whole slipway-* namespace because
// that directory is user-owned host state.
func assertNoCodexLegacyCommandPrompts(t *testing.T, codexHome string) {
	t.Helper()
	promptsDir := filepath.Join(codexHome, "prompts")
	for _, id := range commandIDs() {
		_, err := os.Stat(filepath.Join(promptsDir, "slipway-"+id+".md"))
		assert.True(t, os.IsNotExist(err), "codex must not write retired generated command prompt for %s", id)
	}
}

func TestByteStabilityAllTools(t *testing.T) {
	for _, toolID := range []string{"claude", "codex", "cursor", "gemini", "opencode"} {
		t.Run(toolID, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("CODEX_HOME", t.TempDir())
			require.NoError(t, Generate(root, []string{toolID}, true))

			// Collect all generated files.
			files := map[string][]byte{}
			err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(root, path)
				content, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				files[rel] = content
				return nil
			})
			require.NoError(t, err)

			// Regenerate and compare.
			require.NoError(t, Generate(root, []string{toolID}, true))
			err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(root, path)
				content, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				original, exists := files[rel]
				assert.True(t, exists, "new file appeared on second generation: %s", rel)
				if exists {
					assert.Equal(t, string(original), string(content), "file %s not byte-stable", rel)
				}
				return nil
			})
			require.NoError(t, err)
		})
	}
}

func TestGeneratedGovernanceSkillsHaveMinimalFrontmatter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"claude"}, true))

	routingFields := []string{
		"required_levels:", "state:", "type:", "skill_name:",
		"guardrail_required:", "closeout_conditional:",
		"reviewer_independent:", "run_summary_bound:", "mitigation_target:",
	}

	allStaticGov := append([]string{}, GovernanceSkillNames...)
	allStaticGov = append(allStaticGov, standaloneGovernanceNames...)
	for _, name := range allStaticGov {
		path := filepath.Join(root, ".claude", "skills", "slipway-"+name, "SKILL.md")
		content, err := os.ReadFile(path)
		require.NoError(t, err, "failed to read %s", name)
		s := string(content)

		// Extract frontmatter between --- markers.
		parts := splitFrontmatter(s)
		require.Len(t, parts, 3, "%s missing frontmatter", name)
		fm := parts[1]
		assert.Contains(t, fm, "name:", "%s missing name", name)
		assert.Contains(t, fm, "skill_id:", "%s missing skill_id", name)
		assert.Contains(t, fm, "description:", "%s missing description", name)
		assert.Contains(t, fm, "Use when ", "%s description must use trigger-oriented wording", name)
		assert.Contains(t, fm, "Triggers on", "%s description must describe trigger contract", name)
		for _, field := range routingFields {
			assert.NotContains(t, fm, field, "%s frontmatter has routing field %s", name, field)
		}
	}

	for _, name := range TemplatedGovernanceSkillNames {
		path := filepath.Join(root, ".claude", "skills", "slipway-"+name, "SKILL.md")
		content, err := os.ReadFile(path)
		require.NoError(t, err, "failed to read %s", name)
		s := string(content)
		parts := splitFrontmatter(s)
		require.Len(t, parts, 3, "%s missing frontmatter", name)
		fm := parts[1]
		assert.Contains(t, fm, "name:", "%s missing name", name)
		assert.Contains(t, fm, "skill_id:", "%s missing skill_id", name)
		assert.Contains(t, fm, "description:", "%s missing description", name)
		assert.Contains(t, fm, "Use when ", "%s description must use trigger-oriented wording", name)
		assert.Contains(t, fm, "Triggers on", "%s description must describe trigger contract", name)
		// wave-orchestration is tool-aware; the 4 converted governance skills
		// use Render for partial support but are tool-independent.
		if name == "wave-orchestration" {
			assert.Contains(t, fm, "tool:", "%s missing tool", name)
		}
		for _, field := range []string{"required_levels:", "state:", "type:", "mitigation_target:", "run_summary_bound:"} {
			assert.NotContains(t, fm, field, "%s frontmatter has routing field %s", name, field)
		}
	}
}

func TestGeneratedAdapterAndStandaloneSkillsHaveFrontmatterDescriptions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"claude"}, true))

	for _, name := range standaloneNames {
		path := filepath.Join(root, ".claude", "skills", adapterSkillName(name), "SKILL.md")
		content, err := os.ReadFile(path)
		require.NoError(t, err, "failed to read %s", name)
		parts := splitFrontmatter(string(content))
		require.Len(t, parts, 3, "%s missing frontmatter", name)
		fm := parts[1]
		assert.Contains(t, fm, "name:", "%s missing name", name)
		assert.Contains(t, fm, "skill_id:", "%s missing skill_id", name)
		assert.Contains(t, fm, "description:", "%s missing description", name)
		assert.Contains(t, fm, "Use when ", "%s description must use trigger-oriented wording", name)
		assert.Contains(t, fm, "Triggers on", "%s description must describe trigger contract", name)
		for _, field := range []string{"state:", "type:", "required_levels:", "mitigation_target:", "run_summary_bound:"} {
			assert.NotContains(t, fm, field, "%s frontmatter has retired field %s", name, field)
		}
	}
}

func TestDoneSkillDocumentsAllReadyAcrossActiveExecutions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"claude"}, true))

	content, err := os.ReadFile(filepath.Join(root, ".claude", "commands", "slipway", "done.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "`--all-ready` archives every active change that is currently done-ready.")
}

func splitFrontmatter(content string) []string {
	// Split on "---" preserving the text between first two markers.
	i := 0
	for i < len(content) && content[i] == '\n' {
		i++
	}
	if i < len(content)-3 && content[i:i+3] == "---" {
		rest := content[i+3:]
		end := -1
		for j := 0; j < len(rest); j++ {
			if j == 0 || rest[j-1] == '\n' {
				if len(rest[j:]) >= 3 && rest[j:j+3] == "---" {
					end = j
					break
				}
			}
		}
		if end >= 0 {
			return []string{content[:i], rest[:end], rest[end+3:]}
		}
	}
	return []string{content}
}

func commandSurfacePath(root, codexHome string, cfg ToolConfig, id string) (string, string) {
	_ = codexHome
	if cfg.CommandSkillSurface {
		rel := filepath.ToSlash(SkillPath(cfg, id))
		return rel, filepath.Join(root, filepath.FromSlash(rel))
	}

	ext := ".md"
	if cfg.CommandFormat == "toml" {
		ext = ".toml"
	}
	var rel string
	switch cfg.CommandStyle {
	case "flat":
		rel = filepath.ToSlash(filepath.Join(cfg.CommandsDir, "slipway-"+id+ext))
	default:
		rel = filepath.ToSlash(filepath.Join(cfg.CommandsDir, "slipway", id+ext))
	}
	return rel, filepath.Join(root, filepath.FromSlash(rel))
}

func assertHookCommandRegistered(t *testing.T, settingsPath, eventName, command string) {
	t.Helper()

	commands := hookCommandsForEvent(t, settingsPath, eventName)
	assert.Contains(t, commands, command, "expected %s to register hook command %q", settingsPath, command)
}

func hookCommandsForEvent(t *testing.T, settingsPath, eventName string) []string {
	t.Helper()

	content, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	settings := map[string]any{}
	require.NoError(t, json.Unmarshal(content, &settings))

	hooks, ok := settings["hooks"].(map[string]any)
	require.True(t, ok, "settings missing hooks object")

	entries, ok := hooks[eventName].([]any)
	require.True(t, ok, "settings missing hook event %s", eventName)

	commands := []string{}
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
			if !ok {
				continue
			}
			command, ok := hookMap["command"].(string)
			if ok {
				commands = append(commands, command)
			}
		}
	}
	return commands
}

// hydratedSkillIDs lists the PR-1 slice of skills required to ship a
// non-empty references/ tree. Kept local so drift in the registry does not
// silently expand the set without updating the plan.
var hydratedSkillIDs = []string{
	"gha-security-review",
	"incident-response",
	"multi-reviewer-calibration",
	"mutation-testing",
	"property-testing",
	"root-cause-tracing",
	"sast-orchestration",
	"supply-chain-audit",
	"variant-analysis",
}

// referencesBudgetPerFile caps any single reference at 24 KB so hydrate
// payloads never saturate the per-skill ceiling with one oversized file.
const referencesBudgetPerFile = 24 * 1024

// referencesBudgetPerSkill caps total reference bytes per skill at 64 KB so
// reference shelves stay reviewable and explicit `--hydrate-ref` selection
// remains practical even when operators choose to print the full set.
const referencesBudgetPerSkill = 64 * 1024

const longReferenceNavigationLineThreshold = 240

// TestCatalogSkillHasReferences asserts the PR-1 five have a non-empty
// references/ directory with at least one .md file, and every .md starts
// with an H1 so hydrate output renders a readable heading.
func TestCatalogSkillHasReferences(t *testing.T) {
	t.Parallel()
	templateFS := tmpl.TemplateFS()
	for _, id := range hydratedSkillIDs {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			refsDir := path.Join("skills", id, "references")
			info, err := fs.Stat(templateFS, refsDir)
			require.NoErrorf(t, err, "missing references/ dir for %s", id)
			require.Truef(t, info.IsDir(), "%s: references path is not a directory", id)

			entries, err := fs.ReadDir(templateFS, refsDir)
			require.NoError(t, err)
			mdCount := 0
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				mdCount++
				raw, err := fs.ReadFile(templateFS, path.Join(refsDir, e.Name()))
				require.NoError(t, err)
				head := strings.TrimLeft(string(raw), "\ufeff \t\n")
				assert.Truef(t, strings.HasPrefix(head, "# "),
					"%s/%s must open with an H1 heading (# ...) for hydrate readability", id, e.Name())
			}
			assert.NotZerof(t, mdCount, "%s: references/ must contain at least one .md file", id)
		})
	}
}

// TestHydrateReferencesResolveToFiles is the PR-1 temporary frontmatter-only
// gate: every SKILL.md that declares hydrate_references[] must use typed
// records whose name is a bare .md basename resolving to a file under that
// skill's references/ directory. Names must be unique within the skill and
// must not contain path separators or ...
func TestHydrateReferencesResolveToFiles(t *testing.T) {
	t.Parallel()
	templateFS := tmpl.TemplateFS()
	entries, err := fs.ReadDir(templateFS, "skills")
	require.NoError(t, err)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		skillPath := path.Join("skills", id, "SKILL.md")
		if _, err := fs.Stat(templateFS, skillPath); err != nil {
			continue
		}
		refs := loadHydrateReferencesFromFS(t, templateFS, skillPath)
		if len(refs) == 0 {
			continue
		}
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			seen := make(map[string]struct{}, len(refs))
			for _, r := range refs {
				assert.NotEmptyf(t, r.Name, "%s: hydrate_references entry missing name", id)
				assert.NotEmptyf(t, r.Reason, "%s: hydrate_references entry %q missing reason", id, r.Name)
				assert.Falsef(t,
					strings.ContainsAny(r.Name, "/\\") || strings.Contains(r.Name, ".."),
					"%s: hydrate_references name %q must be a basename with no path separators or ..", id, r.Name)
				assert.Truef(t, strings.HasSuffix(r.Name, ".md"),
					"%s: hydrate_references name %q must end with .md", id, r.Name)

				if _, dup := seen[r.Name]; dup {
					t.Errorf("%s: duplicate hydrate_references name %q", id, r.Name)
				}
				seen[r.Name] = struct{}{}

				refPath := path.Join("skills", id, "references", r.Name)
				_, err := fs.Stat(templateFS, refPath)
				assert.NoErrorf(t, err,
					"%s: hydrate_references %q does not resolve to a file under references/", id, r.Name)
			}
		})
	}
}

// TestReferenceFileSizeBudget enforces per-file and per-skill size caps on
// reference material so hydrate payloads stay bounded.
func TestReferenceFileSizeBudget(t *testing.T) {
	t.Parallel()
	templateFS := tmpl.TemplateFS()
	entries, err := fs.ReadDir(templateFS, "skills")
	require.NoError(t, err)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		refsDir := path.Join("skills", id, "references")
		info, err := fs.Stat(templateFS, refsDir)
		if err != nil || !info.IsDir() {
			continue
		}
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			files, err := fs.ReadDir(templateFS, refsDir)
			require.NoError(t, err)
			total := 0
			for _, f := range files {
				if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
					continue
				}
				p := path.Join(refsDir, f.Name())
				st, err := fs.Stat(templateFS, p)
				require.NoError(t, err)
				size := int(st.Size())
				assert.LessOrEqualf(t, size, referencesBudgetPerFile,
					"%s/%s size %d bytes exceeds per-file cap %d", id, f.Name(), size, referencesBudgetPerFile)
				total += size
			}
			assert.LessOrEqualf(t, total, referencesBudgetPerSkill,
				"%s: references total %d bytes exceeds per-skill cap %d", id, total, referencesBudgetPerSkill)
		})
	}
}

func TestLongReferenceFilesHaveQuickNavigation(t *testing.T) {
	t.Parallel()
	templateFS := tmpl.TemplateFS()
	checked := 0

	err := fs.WalkDir(templateFS, "skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.Contains(p, "/references/") || !strings.HasSuffix(p, ".md") {
			return nil
		}
		raw, err := fs.ReadFile(templateFS, p)
		if err != nil {
			return err
		}
		content := string(raw)
		lineCount := strings.Count(content, "\n")
		if !strings.HasSuffix(content, "\n") {
			lineCount++
		}
		if lineCount <= longReferenceNavigationLineThreshold {
			return nil
		}
		checked++
		lines := strings.Split(content, "\n")
		limit := 40
		if len(lines) < limit {
			limit = len(lines)
		}
		quickNavigationLine := 0
		for i := 0; i < limit; i++ {
			if strings.TrimSpace(lines[i]) == "## Quick Navigation" {
				quickNavigationLine = i + 1
				break
			}
		}
		assert.NotZerof(t, quickNavigationLine,
			"%s has %d lines and needs a top-level quick navigation section within the first %d lines",
			p, lineCount, limit)
		return nil
	})
	require.NoError(t, err)
	assert.NotZero(t, checked, "expected at least one long reference file to be checked")
}

// TestTypedPartsRendered asserts that the PR-3 typed partials land in the
// assembled SKILL.md for the four skills the plan extends (spec-trace,
// threat-modeling, coverage-analysis, security-review). Each expected
// section heading must appear exactly once so source-body sections and
// partial-rendered sections never co-exist.
func TestTypedPartsRendered(t *testing.T) {
	root := generatedSkillsRoot(t)
	cases := []struct {
		skill    string
		headings []string
	}{
		{"spec-trace", []string{"## Checklist"}},
		{"threat-modeling", []string{"## Procedure", "## Checklist"}},
		{"coverage-analysis", []string{"## Report schema"}},
		{"security-review", []string{"## Checklist"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.skill, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(root, "slipway-"+tc.skill, "SKILL.md")
			var body string
			if !shouldExportAsHostSkill(tc.skill) {
				skill, ok := capability.DefaultRegistry().Lookup(tc.skill)
				require.True(t, ok, "missing catalog skill %s", tc.skill)
				rendered, err := renderCatalogSkill(skill)
				require.NoError(t, err)
				body = rendered
			} else {
				raw, err := os.ReadFile(path)
				require.NoErrorf(t, err, "missing rendered SKILL.md for %s", tc.skill)
				body = string(raw)
			}
			for _, h := range tc.headings {
				count := strings.Count(body, "\n"+h+"\n")
				assert.Equalf(t, 1, count,
					"%s: heading %q must appear exactly once in assembled SKILL.md (found %d)",
					tc.skill, h, count)
			}
		})
	}
}

func TestRenderedTDDGovernanceExpandsFreshEvidencePartials(t *testing.T) {
	root := generatedSkillsRoot(t)
	path := filepath.Join(root, "slipway-tdd-governance", "SKILL.md")

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	body := string(raw)

	assert.Contains(t, body, "Current `run_version` matches the latest execution run for this change.")
	assert.Contains(t, body, "reproducible command or transcript reference")
	assert.Contains(t, body, "with a valid task `--verdict`")
	assert.Contains(t, body, "not through a separate evidence-note command")
	assert.NotContains(t, body, "recorded not-applicable via a `slipway evidence task` note")
	assert.NotContains(t, body, "rather than a TDD verdict")
	assert.NotContains(t, body, "{{template", "rendered tdd-governance must not leak raw template directives")
}

// TestSecurityReviewReferenceOverlaysPresent asserts that the rendered
// security-review/references/ directory contains the six topic /
// infrastructure overlays authored after removing language-specific overlays.
func TestSecurityReviewReferenceOverlaysPresent(t *testing.T) {
	root := generatedSkillsRoot(t)
	refsDir := filepath.Join(root, "slipway-security-review", "references")
	expected := []string{
		"authentication.md",
		"authorization.md",
		"injection.md",
		"xss.md",
		"ssrf.md",
		"infrastructure-docker.md",
	}
	for _, name := range expected {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := filepath.Join(refsDir, name)
			info, err := os.Stat(p)
			require.NoErrorf(t, err, "security-review overlay %q missing from rendered tree", name)
			assert.Falsef(t, info.IsDir(), "%s: expected file, got directory", name)
			assert.NotZerof(t, info.Size(), "%s: overlay is empty", name)
		})
	}
}

// TestRenderedSkillProseUsesCanonicalPublicNames verifies that exported-skill
// prose references stay on the canonical adapter-visible public-name form once
// the generated tree has been canonicalized. Runtime-owned identifiers such as
// `skill_id`, bindings, and verification filenames are covered elsewhere.
func TestRenderedSkillProseUsesCanonicalPublicNames(t *testing.T) {
	root := generatedSkillsRoot(t)
	cases := []struct {
		skill       string
		mustContain []string
		mustOmit    []string
	}{
		{
			skill: "research-orchestration",
			mustContain: []string{
				"stage (`slipway-plan-audit`) can build on",
				"[Questions that slipway-plan-audit must address]",
			},
			mustOmit: []string{
				"(`plan-audit`) can build on",
				"[Questions that plan-audit must address]",
				"`plan-audit` to validate",
			},
		},
		{
			skill: "codebase-mapping",
			mustContain: []string{
				"`slipway-research-orchestration`, `slipway-plan-audit`, and `slipway-wave-orchestration` SHOULD consume",
			},
			mustOmit: []string{
				"research-orchestration, plan-audit, and wave-orchestration SHOULD consume",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.skill, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(root, "slipway-"+tc.skill, "SKILL.md")
			raw, err := os.ReadFile(path)
			require.NoErrorf(t, err, "missing rendered SKILL.md for %s", tc.skill)
			body := string(raw)
			for _, want := range tc.mustContain {
				assert.Containsf(t, body, want, "%s: expected canonical public-name prose", tc.skill)
			}
			for _, unwanted := range tc.mustOmit {
				assert.NotContainsf(t, body, unwanted, "%s: found stale bare public-name prose", tc.skill)
			}
		})
	}
}

// TestSharedChecklistReferenceIsEmittedToConsumingSkills pins that the shared
// requirements-quality checklist actually ships into the generated references/
// dir of every skill that points at it, so the "Apply references/checklist-quality.md"
// pointer is reachable (previously it named a top-level sibling no generation
// path emitted).
func TestSharedChecklistReferenceIsEmittedToConsumingSkills(t *testing.T) {
	skillsRoot := generatedSkillsRoot(t)
	for _, skill := range []string{"slipway-plan-audit", "slipway-spec-compliance-review"} {
		ref := filepath.Join(skillsRoot, skill, "references", "checklist-quality.md")
		body, err := os.ReadFile(ref)
		require.NoErrorf(t, err, "shared checklist not emitted for %s", skill)
		assert.Contains(t, string(body), "Requirement-to-intent traceability")
	}
}

// generatedSkillsRoot generates a codex tree and returns the path to
// `<root>/.codex/skills/` for script-contract tests.
func generatedSkillsRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("CODEX_HOME", t.TempDir())
	require.NoError(t, Generate(root, []string{"codex"}, true))
	cfg := toolRegistry["codex"]
	return filepath.Join(root, cfg.SkillsDir)
}

func TestGeneratedSkillsDoNotShipScriptDirectories(t *testing.T) {
	root := generatedSkillsRoot(t)

	var scriptPaths []string
	require.NoError(t, filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && filepath.Base(p) == "scripts" {
			rel, relErr := filepath.Rel(root, p)
			if relErr != nil {
				return relErr
			}
			scriptPaths = append(scriptPaths, filepath.ToSlash(rel))
		}
		return nil
	}))
	assert.Empty(t, scriptPaths, "generated skills must route helper behavior through slipway tool, not scripts/")
}

func TestSkillHelperDocsUseSlipwayTool(t *testing.T) {
	cases := []struct {
		path     string
		contains string
	}{
		{path: "skills/ci-triage/SKILL.md", contains: "slipway tool fetch-pr-checks"},
		{path: "skills/review-comment-triage/SKILL.md", contains: "slipway tool fetch-pr-feedback"},
		{path: "skills/review-comment-triage/SKILL.md", contains: "slipway tool fetch-review-requests"},
		{path: "skills/review-comment-triage/SKILL.md", contains: "slipway tool reply-to-thread"},
		{path: "skills/root-cause-tracing/references/root-cause-tracing.md", contains: "slipway tool find-polluter-go"},
		{path: "skills/sast-orchestration/references/sarif-merge.md", contains: "slipway tool merge-sarif"},
		{path: "skills/variant-analysis/SKILL.md", contains: "slipway tool find-variant"},
	}

	for _, tc := range cases {
		content, err := fs.ReadFile(tmpl.TemplateFS(), tc.path)
		require.NoError(t, err, "missing generated helper doc %s", tc.path)
		assert.Contains(t, string(content), tc.contains, "%s must point at compiled helper command", tc.path)
		assert.NotContains(t, string(content), "scripts/", "%s must not point at generated scripts", tc.path)
	}
}

// TestToolConfigInvocationSummary locks the per-adapter invocation surface string
// that `slipway init` prints at setup time (issue #210), so the discoverable
// surface can never silently regress to the retired prompt-based description.
func TestToolConfigInvocationSummary(t *testing.T) {
	t.Run("command skill surface", func(t *testing.T) {
		cfg := ToolConfig{CommandSkillSurface: true}
		assert.Equal(t,
			"invoke skills: $slipway (entry), $slipway-<command> per command, or /skills",
			cfg.InvocationSummary())
	})
	t.Run("slash-colon commands", func(t *testing.T) {
		cfg := ToolConfig{TriggerPrefix: "/slipway", TriggerStyle: "slash-colon"}
		assert.Equal(t, "invoke commands as /slipway:<command>", cfg.InvocationSummary())
	})
	t.Run("mention commands", func(t *testing.T) {
		cfg := ToolConfig{TriggerPrefix: "/slipway-", TriggerStyle: "slash-hyphen"}
		assert.Equal(t, "invoke commands as /slipway-<command>", cfg.InvocationSummary())
	})
	t.Run("codex adapter routes to skills", func(t *testing.T) {
		cfg, ok := LookupTool("codex")
		require.True(t, ok)
		summary := cfg.InvocationSummary()
		assert.Contains(t, summary, "invoke skills:")
		assert.Contains(t, summary, "$slipway-<command>")
		assert.NotContains(t, summary, "prompts")
	})
}

func TestMergeHookSettingsPrunesStaleHookFormsAndPreservesUserHooks(t *testing.T) {
	root := t.TempDir()
	cfg := ToolConfig{
		ID:           "claude",
		SettingsPath: filepath.Join(".claude", "settings.json"),
		SessionEvent: "SessionStart",
		SessionHook:  filepath.Join(".claude", "hooks", "slipway-session-start"),
	}

	// Seed settings with every retired hook shape that must be replaced on refresh:
	//   - the legacy `bash "...sh"` (and sh / direct exec / abs-path) launcher forms,
	//   - the OS-pinned launcher-path forms (bare POSIX launcher and Windows ".cmd")
	//     that earlier versions wrote and that break for teammates on another OS,
	//   - the shell-chained direct `slipway hook ... || exit 0` form, which is
	//     not parseable by Windows PowerShell 5.1,
	// plus an unrelated user-authored hook (with a matcher) that must survive.
	seed := `{
	  "hooks": {
	    "SessionStart": [
	      {"hooks":[{"type":"command","command":"bash \"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.sh\""}]},
	      {"hooks":[{"type":"command","command":"sh \"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.sh\""}]},
	      {"hooks":[{"type":"command","command":"\"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.sh\""}]},
	      {"hooks":[{"type":"command","command":"/bin/bash \"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.sh\""}]},
	      {"hooks":[{"type":"command","command":"\".claude/hooks/slipway-session-start\""}]},
	      {"hooks":[{"type":"command","command":"\".claude/hooks/slipway-session-start.cmd\""}]},
	      {"hooks":[{"type":"command","command":"slipway hook session-start --tool \"claude\" || exit 0"}]},
	      {"hooks":[{"type":"command","command":"\"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start\""}]},
	      {"hooks":[{"type":"command","command":"\"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.ps1\""}]},
	      {"hooks":[{"type":"command","command":"\"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.cmd\""}]},
	      {"hooks":[{"type":"command","command":"bash \"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-startup.sh\""}]},
	      {"hooks":[{"type":"command","command":"bash -lc \"echo .claude/hooks/slipway-session-start\""}]},
	      {"hooks":[{"type":"command","command":"\"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-startup\""}]},
	      {"matcher":"*","hooks":[{"type":"command","command":"echo user-owned-hook"}]}
	    ]
	  }
	}`
	settingsPath := filepath.Join(root, cfg.SettingsPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0o755))
	require.NoError(t, os.WriteFile(settingsPath, []byte(seed), 0o644))

	require.NoError(t, mergeHookSettingsJSON(root, cfg, true))

	first, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	got := string(first)
	commands := hookCommandsForEvent(t, settingsPath, cfg.SessionEvent)

	// Every retired launcher-path command (extensionless, .ps1, .cmd, .sh;
	// direct exec or via bash/sh), plus the short-lived `--tool ... || exit 0`
	// direct command, is migrated away.
	assert.NotContains(t, commands, `bash "$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.sh"`)
	assert.NotContains(t, commands, `sh "$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.sh"`)
	assert.NotContains(t, commands, `"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.sh"`)
	assert.NotContains(t, commands, `/bin/bash "$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.sh"`)
	assert.NotContains(t, commands, `".claude/hooks/slipway-session-start"`)
	assert.NotContains(t, commands, `".claude/hooks/slipway-session-start.cmd"`)
	assert.NotContains(t, commands, `"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start"`)
	assert.NotContains(t, commands, `"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.ps1"`)
	assert.NotContains(t, commands, `"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-start.cmd"`)
	assert.NotContains(t, commands, `slipway hook session-start --tool "claude" || exit 0`)
	// Unrelated user hooks, including similarly prefixed paths and `bash -lc`
	// command strings, are preserved verbatim.
	assert.Contains(t, got, "echo user-owned-hook")
	assert.Contains(t, got, `"matcher": "*"`)
	assert.Contains(t, commands, `bash "$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-startup.sh"`)
	assert.Contains(t, commands, `bash -lc "echo .claude/hooks/slipway-session-start"`)
	assert.Contains(t, commands, `"$CLAUDE_PROJECT_DIR/.claude/hooks/slipway-session-startup"`)
	// The bare inline session-start command is registered in its place.
	assert.Contains(t, got, sessionStartHookCommand)
	assert.NotContains(t, got, "||")
	assert.NotContains(t, got, "&&")
	assert.NotContains(t, got, " exit ")
	assertHookCommandRegistered(
		t,
		settingsPath,
		cfg.SessionEvent,
		sessionStartHookCommand,
	)

	// Refresh is idempotent: a second merge does not duplicate or mutate.
	require.NoError(t, mergeHookSettingsJSON(root, cfg, true))
	second, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second))
}
