package toolgen

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/engine/capability"
	"github.com/signalridge/slipway/internal/fsutil"
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

func TestRegistryHasTenTools(t *testing.T) {
	t.Parallel()
	registry := Registry()
	require.Len(t, registry, 10)

	ids := make([]string, len(registry))
	for i, cfg := range registry {
		ids[i] = cfg.ID
	}
	assert.Equal(t, []string{
		"claude",
		"codex",
		"copilot",
		"cursor",
		"kilo",
		"kiro",
		"opencode",
		"pi",
		"qwen",
		"windsurf",
	}, ids)
}

func TestResolveTools(t *testing.T) {
	t.Parallel()
	all, err := ResolveTools("all")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"claude",
		"codex",
		"copilot",
		"cursor",
		"kilo",
		"kiro",
		"opencode",
		"pi",
		"qwen",
		"windsurf",
	}, all)

	none, err := ResolveTools("none")
	require.NoError(t, err)
	assert.Nil(t, none)

	selected, err := ResolveTools("cursor,claude,cursor")
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "cursor"}, selected)

	_, err = ResolveTools("unknown")
	require.Error(t, err)

	removedTool := "ge" + "mini"
	_, err = ResolveTools(removedTool)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported tool "+strconv.Quote(removedTool))
}

func TestCommandRegistryContainsAllAdapterSkillIDs(t *testing.T) {
	t.Parallel()
	// Verify registry has 25 commands across the same public groups as root help.
	assert.Len(t, commandRegistry, 25)

	// Verify all registry entries have the required fields.
	validTiers := []string{"core", "discovery", "situational", "helpers", "diagnostics", "setup"}
	for _, def := range commandRegistry {
		assert.NotEmpty(t, def.ID, "registry entry missing ID")
		assert.Contains(t, []CommandClass{CommandClassQuery, CommandClassMutation}, def.Class,
			"registry entry %s has invalid Class %q", def.ID, def.Class)
		assert.NotEmpty(t, def.Description, "registry entry %s missing Description", def.ID)
		assert.NotEmpty(t, def.Tier, "registry entry %s missing Tier", def.ID)
		assert.Contains(t, validTiers, def.Tier, "registry entry %s has invalid Tier %q", def.ID, def.Tier)
	}

	// Count tiers.
	core, discovery, sit, helpers, diag, setup := 0, 0, 0, 0, 0, 0
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
		case "discovery":
			discovery++
		case "situational":
			sit++
		case "helpers":
			helpers++
		case "diagnostics":
			diag++
		case "setup":
			setup++
		}
	}
	assert.Equal(t, 10, core, "expected 10 core commands")
	assert.Equal(t, 1, discovery, "expected 1 discovery command")
	assert.Equal(t, 8, sit, "expected 8 situational commands")
	assert.Equal(t, 2, helpers, "expected 2 helper commands")
	assert.Equal(t, 2, diag, "expected 2 diagnostics commands")
	assert.Equal(t, 2, setup, "expected 2 setup commands")
	assert.Equal(t, 5, query, "expected 5 query commands")
	assert.Equal(t, 20, mutation, "expected 20 mutation commands")

	var fixDef CommandDef
	var foundFix bool
	for _, def := range commandRegistry {
		if def.ID == "fix" {
			fixDef = def
			foundFix = true
			break
		}
	}
	require.True(t, foundFix)
	assert.Contains(t, fixDef.Arguments, "--start-reexecution",
		"fix command registry should expose the explicit reexecution mode")
	require.NotEmpty(t, fixDef.Notes)
	assert.Contains(t, fixDef.Notes[0], "Ordinary `slipway fix` discovery does not run local-integrity repair and does not advance lifecycle state")
	assert.Contains(t, fixDef.Notes[0], "`slipway fix --start-reexecution` explicitly reopens S2")
	assert.NotContains(t, CommandArguments("done"), "--json",
		"done emits JSON unconditionally and should not publish a no-op --json flag")
	assert.NotContains(t, CommandArguments("validate"), "--json",
		"validate emits JSON unconditionally and should not publish a no-op --json flag")

	// Verify commandIDs() returns a sorted list covering every command that
	// ships a prompt surface. CLI-only helpers such as `tool` remain registered
	// but intentionally do not generate host prompt wrappers.
	ids := commandIDs()
	assert.Len(t, ids, 22)
	for i := 1; i < len(ids); i++ {
		assert.True(t, ids[i-1] < ids[i], "commandIDs not sorted: %s >= %s", ids[i-1], ids[i])
	}
	assert.NotContains(t, ids, "tool")
	assert.NotContains(t, ids, "config")
	assert.NotContains(t, ids, "hook")
}

func TestSurfaceManifestRegistersHookAsCLIOnlyImplementationSurface(t *testing.T) {
	t.Parallel()

	manifest := BuildSurfaceManifest()
	var hookRow *SurfaceManifestRow
	var hookImplementationRow *SurfaceManifestRow
	for i := range manifest.Rows {
		row := manifest.Rows[i]
		if row.Kind == "command" && row.Name == "hook" {
			hookRow = &row
		}
		if row.Kind == "implementation" && row.Name == "hook" {
			hookImplementationRow = &row
		}
		assert.False(t, row.Kind == "skill" && row.Name == "slipway-hook",
			"hook must not export a generated host prompt surface")
		assert.NotEqual(t, "$slipway-hook", row.Token,
			"hook must not export a generated host prompt token")
	}

	require.NotNil(t, hookRow, "surface manifest must include the public hook command")
	assert.Equal(t, "internal/toolgen/toolgen.go:commandRegistry", hookRow.Source)
	assert.Equal(t, "docs/reference/commands.md", hookRow.Docs)
	assert.Equal(t, "slipway hook", hookRow.Token)

	require.NotNil(t, hookImplementationRow, "surface manifest must include the hook implementation source")
	assert.Equal(t, "cmd/session_start_hook.go", hookImplementationRow.Source)
	assert.Equal(t, "docs/reference/commands.md", hookImplementationRow.Docs)
	assert.Equal(t, "slipway hook", hookImplementationRow.Token)
}

func TestEvidenceCommandArgumentsUseResultFileOnlyTaskSurface(t *testing.T) {
	t.Parallel()

	arguments := CommandArguments("evidence")
	require.NotEmpty(t, arguments)
	assert.Contains(t, arguments, "task --result-file <path> [--result-file <path> ...]",
		"evidence command registry should teach compact task result import")
	assert.NotContains(t, arguments, "| task --task-id <id>",
		"evidence command registry should keep manual task mode out of agent-facing Arguments")
	assert.NotContains(t, arguments, "--run-summary-version <n>",
		"evidence command registry should keep manual task ledger fields out of agent-facing Arguments")
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

func TestPromotedReviewHostsAreWorkflowOwnedTemplatedSurfaces(t *testing.T) {
	t.Parallel()

	descriptors := map[string]governanceSurfaceDescriptor{}
	for _, desc := range governanceSurfaceDescriptors {
		descriptors[desc.ID] = desc
	}

	for _, id := range []string{"independent-review", "security-review"} {
		desc, ok := descriptors[id]
		require.Truef(t, ok, "%s must be promoted into the workflow-owned governance descriptor table", id)
		assert.Equal(t, governanceRenderTemplated, desc.RenderMode, "%s must render from HOST_SKILL.md.tmpl", id)
		assert.Truef(t, desc.WorkflowOwned, "%s must be owned by workflow governance", id)
		assert.Falsef(t, desc.ExportOnlyExtra, "%s must not be an export-only catalog helper", id)
		assert.Truef(t, shouldExportAsHostSkill(id), "%s must export as a host skill", id)
		assert.Contains(t, TemplatedGovernanceSkillNames, id)
	}
}

func TestPromotedReviewCatalogBindingsPreserveCommandAutoOnly(t *testing.T) {
	t.Parallel()

	reg := capability.DefaultRegistry()
	cases := []struct {
		id         string
		attachment capability.AttachmentMode
	}{
		{id: "independent-review", attachment: capability.AttachmentReportSchema},
		{id: "security-review", attachment: capability.AttachmentChecklist},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			t.Parallel()

			skill, ok := reg.Lookup(tc.id)
			require.True(t, ok)

			foundCommandAuto := false
			for _, binding := range skill.Bindings {
				assert.NotEqual(t, capability.BindingHostEmbedded, binding.Type, "%s must not be embedded into review hosts", tc.id)
				if binding.Type == capability.BindingCommandAuto &&
					binding.Target == "review" &&
					binding.Attachment == tc.attachment {
					foundCommandAuto = true
				}
			}
			assert.Truef(t, foundCommandAuto, "%s must preserve its review command-auto binding", tc.id)
		})
	}
}

func TestGeneratedHostSkillSetEqualsAllowlist(t *testing.T) {
	root := t.TempDir()
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
	assert.Len(t, got, 22, "host skill count should stay within the slim exported surface target")

	// Every Slipway command must have its own discoverable Codex command skill.
	assert.ElementsMatch(t, commandSkillDirNames(), gotCommandSkills,
		"codex command skill dirs must cover exactly the command set")
}

func TestGeneratedPromotedReviewHostsUseWorkflowTemplates(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"codex"}, true))

	cfg := toolRegistry["codex"]
	for _, id := range []string{"independent-review", "security-review"} {
		id := id
		t.Run(id, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, SkillPath(cfg, id)))
			require.NoError(t, err)
			body := string(raw)

			assert.Contains(t, body, "S3_REVIEW")
			assert.Contains(t, body, "native subagent")
			assert.Contains(t, body, "SHARED change worktree")
			assert.Contains(t, body, "context_origin:stage=review=<handle>")
			assert.Contains(t, body, "MUST be DISTINCT")
			assert.NotContains(t, body, "host-embedded")
			assert.NotContains(t, body, "base reader that both review hosts")
			assert.NotContains(t, body, "review_origin:skill=")
			assert.NotContains(t, body, "review_context:skill=")
		})
	}
}

func TestNonExportedRegistrySkillsDoNotEmitAgentFacingCatalogArtifacts(t *testing.T) {
	root := t.TempDir()
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

	// ResolveNextSkill returns a skill set per state. S3_REVIEW emits the
	// selected review set as concurrent peers, so every member of every state's
	// output set must map to an exported host skill.
	for _, id := range []string{
		"intake-clarification",
		"research-orchestration",
		"worktree-preflight",
		"plan-audit",
		"wave-orchestration",
		"spec-compliance-review",
		"code-quality-review",
		"independent-review",
		"security-review",
		"ship-verification",
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

func TestDetectExistingToolsIgnoresBareUnmanagedHostDirectories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	for _, dir := range []string{
		".pi",
		".qwen",
		".kiro",
		".github",
		".github/copilot",
		".windsurf",
		".kilocode",
	} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755))
	}

	assert.Empty(t, DetectExistingTools(root),
		"bare unmanaged host directories must not trigger refresh auto-detection without Slipway sentinels")
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
	toolIDs, err := ResolveTools("all")
	require.NoError(t, err)
	require.NoError(t, Generate(root, toolIDs, true))

	for _, toolID := range toolIDs {
		cfg := toolRegistry[toolID]

		// Sentinel marker
		sentinelPath := filepath.Join(root, GeneratedAdapterMarkerPath(cfg))
		_, err = os.Stat(sentinelPath)
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
			for _, id := range commandIDs() {
				path := filepath.Join(root, filepath.FromSlash(commandEntryRelPath(cfg, id)))
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

		// Hook emission is keyed on whether the host owns hook settings.
		// Settings-capable hook hosts (claude, qwen) register a bare
		// inline command and emit NO launcher files. Pi registers skills/prompts
		// in settings and uses a managed extension bridge instead of launcher
		// files. File-by-path hosts (cursor, opencode) still emit the
		// session-start launcher family. Skill-only/no-hook hosts emit none.
		switch {
		case cfg.SettingsPath != "" && cfg.SettingsKind != settingsKindPiRegistration:
			// Settings-capable hosts must not write any launcher file for the
			// session-start hook (the extensionless POSIX entry or its
			// .ps1/.cmd/.sh variants).
			for _, base := range []string{cfg.SessionHook} {
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
		case cfg.SettingsKind == settingsKindPiRegistration:
			assertPiRegistrationSettings(t, filepath.Join(root, cfg.SettingsPath))
			require.NotEmpty(t, cfg.HookExtensionPath, "%s: pi registration must name a hook extension bridge", toolID)
			assertPiHooksExtension(t, filepath.Join(root, filepath.FromSlash(cfg.HookExtensionPath)))
		case cfg.SettingsKind == settingsKindCodexHooks:
			settingsPath := filepath.Join(root, cfg.SettingsPath)
			content, err := os.ReadFile(settingsPath)
			assert.NoError(t, err, "%s: missing Codex hook config", toolID)
			settings := string(content)
			assert.Contains(t, settings, "[[hooks.SessionStart]]", "%s: missing Codex SessionStart hook", toolID)
			assert.Contains(t, settings, `slipway hook session-start --tool codex`, "%s: missing Codex session-start command", toolID)
			assert.NotContains(t, settings, "[[hooks.UserPromptSubmit]]", "%s: retired context-pressure hook must not register UserPromptSubmit", toolID)
			assert.NotContains(t, settings, "context-pressure", "%s: retired context-pressure command must not appear", toolID)
			assert.Contains(t, settings, "inert until Codex trusts this repo and each hook", "%s: missing trust caveat", toolID)
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
			// The context-pressure PostToolUse hook was retired: no settings-capable
			// host registers a PostToolUse event or the context-pressure command.
			assert.NotContains(t, settings, "PostToolUse", "%s: unexpected post-tool registration", toolID)
			assert.NotContains(t, settings, retiredContextPressureInlineCommand,
				"%s: unexpected context-pressure registration", toolID)
		default:
			settingsPath := filepath.Join(root, "."+toolID, "settings.json")
			_, err := os.Stat(settingsPath)
			assert.True(t, os.IsNotExist(err), "%s: unexpected settings file generated", toolID)
		}
	}

	copilotCfg := toolRegistry["copilot"]
	assert.Equal(t,
		".github/prompts/slipway-new.prompt.md",
		commandEntryRelPath(copilotCfg, "new"),
		"copilot command prompts must use GitHub Copilot's .prompt.md extension",
	)
	assert.FileExists(t,
		filepath.Join(root, ".github", "prompts", "slipway-new.prompt.md"),
		"copilot command prompt must be written under the shared .github prompts root",
	)
	assert.FileExists(t,
		filepath.Join(root, ".github", "copilot", "slipway", ".adapter-generated"),
		"copilot ownership sentinel must stay under .github/copilot, not shared .github",
	)
	_, err = os.Stat(filepath.Join(root, ".github", "slipway", ".adapter-generated"))
	assert.True(t, os.IsNotExist(err), "copilot must not claim the shared .github root")

	assert.FileExists(t, filepath.Join(root, ".kilocode", "workflows", "slipway-new.md"))
	assert.FileExists(t, filepath.Join(root, ".windsurf", "workflows", "slipway-new.md"))

	qwenNew, err := os.ReadFile(filepath.Join(root, ".qwen", "skills", "slipway-new", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(qwenNew), "/slipway-new")
	kiroNew, err := os.ReadFile(filepath.Join(root, ".kiro", "skills", "slipway-new", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(kiroNew), "@slipway:new")
}

// referenceSectionFor returns a command-reference section or command body.
func referenceSectionFor(ref, header string) string {
	idx := strings.Index(ref, header)
	if idx < 0 {
		return ""
	}
	rest := ref[idx+len(header):]
	cut := len(rest)
	if strings.HasPrefix(header, "### ") {
		if next := strings.Index(rest, "\n### "); next >= 0 && next < cut {
			cut = next
		}
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

	toolIDs, err := ResolveTools("all")
	require.NoError(t, err)
	require.NoError(t, Generate(root, toolIDs, true))

	for _, cfg := range Registry() {
		skillPath := filepath.Join(root, cfg.SkillsDir, "slipway", "SKILL.md")
		content, err := os.ReadFile(skillPath)
		require.NoError(t, err, "%s: missing workflow skill", cfg.ID)

		s := string(content)
		sText := strings.Join(strings.Fields(s), " ")
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
		assert.Contains(t, s, "## Session Continuity", "%s: missing session-continuity contract", cfg.ID)
		assert.Contains(t, sText, "Governed continuity comes ONLY from `slipway status --json` and `slipway next --json`", "%s: missing status/next continuity authority", cfg.ID)
		assert.Contains(t, sText, "are informal context only and are NEVER governance authority", "%s: missing host-summary non-authority boundary", cfg.ID)
		assert.Contains(t, sText, "MUST run `slipway status` / `slipway next` and MUST NOT infer the governed position from a host summary", "%s: missing resume-must-run requirement", cfg.ID)
		// Slipway no longer measures host context; the handoff mechanism is present
		// and host-invoked, but stays advisory (never lifecycle authority).
		assert.Contains(t, sText, "Slipway does not measure your context window", "%s: missing host-owns-context principle", cfg.ID)
		assert.Contains(t, s, "slipway handoff write", "%s: missing handoff write mechanism", cfg.ID)
		assert.Contains(t, s, "slipway handoff show", "%s: missing handoff show mechanism", cfg.ID)
		assert.Contains(t, sText, "it is NOT lifecycle authority, governed evidence, freshness input, or a gate", "%s: missing handoff advisory boundary", cfg.ID)
		assert.NotContains(t, s, "handoff.md is lifecycle authority", "%s: stale handoff authority wording leaked", cfg.ID)
		assert.NotContains(t, s, "handoff.md is governed evidence", "%s: stale handoff evidence wording leaked", cfg.ID)
		assert.NotContains(t, s, "handoff.md is freshness input", "%s: stale handoff freshness wording leaked", cfg.ID)
		assert.NotContains(t, s, "handoff.md replaces", "%s: stale handoff replacement wording leaked", cfg.ID)
		assert.NotContains(t, s, "skip `slipway status --json`", "%s: stale status bypass wording leaked", cfg.ID)
		assert.NotContains(t, s, "skip `slipway next --json`", "%s: stale next bypass wording leaked", cfg.ID)
		assert.NotContains(t, s, "skip lifecycle gates", "%s: stale lifecycle bypass wording leaked", cfg.ID)
		assert.NotContains(t, s, "skip evidence checks", "%s: stale evidence bypass wording leaked", cfg.ID)
		// REQ-002/REQ-003: the generated workflow skill must ship the fresh-session
		// resume protocol that drives status -> --change -> handoff show -> next.
		assert.Contains(t, s, "## Continuing A Change In A Fresh Session", "%s: missing fresh-session resume protocol section", cfg.ID)
		assert.Contains(t, sText, "Run `slipway status --json` to discover the active change", "%s: resume protocol missing status --json step", cfg.ID)
		assert.Contains(t, sText, "select the one you are resuming and pass `--change <slug>`", "%s: resume protocol missing multi-change --change selection step", cfg.ID)
		assert.Contains(t, sText, "Run `slipway handoff show --change <slug>` to read the prior session's advisory", "%s: resume protocol missing handoff show step", cfg.ID)
		assert.Contains(t, sText, "Then run `slipway next`", "%s: resume protocol missing next step", cfg.ID)
		assert.Contains(t, s, "`references/command-reference.md`", "%s: missing workflow reference handoff", cfg.ID)
		assert.Contains(t, s, filepath.ToSlash(SkillIndexPath(cfg)), "%s: missing workflow skill index path", cfg.ID)
		assert.Contains(t, s, "informational only", "%s: missing skill index authority boundary", cfg.ID)
		assert.NotContains(t, s, "follow the listed catalog artifact path", "%s: stale catalog artifact triage guidance leaked", cfg.ID)
		assert.NotContains(t, s, "using-slipway-catalog.md", "%s: stale top-level catalog manifest leaked", cfg.ID)
		assert.Contains(t, s, "Every workflow command listed in `references/command-reference.md` ships a", "%s: missing prompt-surface command boundary", cfg.ID)
		assert.Contains(t, sText, "CLI-only helper namespaces such as `slipway tool` are intentionally", "%s: missing CLI-only helper command boundary", cfg.ID)
		assert.NotContains(t, s, "Every CLI command ships a command surface", "%s: stale all-commands surface claim leaked", cfg.ID)

		refPath := filepath.Join(root, cfg.SkillsDir, "slipway", "references", "command-reference.md")
		refContent, err := os.ReadFile(refPath)
		require.NoError(t, err, "%s: missing workflow command reference", cfg.ID)
		ref := string(refContent)
		refText := strings.Join(strings.Fields(ref), " ")
		assert.Contains(t, ref, "## Lifecycle Core", "%s: missing lifecycle section", cfg.ID)
		assert.Contains(t, ref, "## Discovery", "%s: missing discovery section", cfg.ID)
		assert.Contains(t, ref, "## Situational Commands", "%s: missing situational section", cfg.ID)
		assert.Contains(t, ref, "## Diagnostics", "%s: missing diagnostics section", cfg.ID)
		assert.Contains(t, ref, "## Setup", "%s: missing setup section", cfg.ID)
		assert.NotContains(t, ref, "## Supporting Commands", "%s: stale supporting section leaked", cfg.ID)
		assert.Contains(t, ref, "### `slipway new`", "%s: missing new command entry", cfg.ID)
		assert.Contains(t, ref, "JSON stdin fields for `slipway new --json`, not command-line flags", "%s: missing new stdin contract notes", cfg.ID)
		assert.Contains(t, ref, "`guardrail_domain`, `needs_discovery`, and `complexity`", "%s: missing new stdin classification shape", cfg.ID)
		assert.Contains(t, ref, "### `slipway run`", "%s: missing run command entry", cfg.ID)
		assert.Contains(t, ref, "### `slipway repair`", "%s: missing repair command entry", cfg.ID)
		assert.Contains(t, ref, "### `slipway codebase-map`", "%s: missing discovery command entry", cfg.ID)
		assert.Contains(t, ref, "### `slipway init`", "%s: missing setup command entry", cfg.ID)
		discoverySection := referenceSectionFor(ref, "## Discovery")
		assert.Contains(t, discoverySection, "### `slipway codebase-map`", "%s: codebase-map must be in discovery", cfg.ID)
		assert.NotContains(t, discoverySection, "### `slipway init`", "%s: init must not be in discovery", cfg.ID)
		situationalSection := referenceSectionFor(ref, "## Situational Commands")
		assert.Contains(t, situationalSection, "### `slipway repair`", "%s: repair must be situational", cfg.ID)
		assert.NotContains(t, situationalSection, "### `slipway init`", "%s: init must not be situational", cfg.ID)
		assert.Contains(t, ref, "### `slipway handoff`", "%s: missing handoff command entry in reference", cfg.ID)
		diagnosticsSection := referenceSectionFor(ref, "## Diagnostics")
		assert.Contains(t, diagnosticsSection, "### `slipway instructions`", "%s: instructions must remain diagnostic", cfg.ID)
		assert.NotContains(t, diagnosticsSection, "### `slipway codebase-map`", "%s: codebase-map must not be diagnostic", cfg.ID)
		setupSection := referenceSectionFor(ref, "## Setup")
		assert.Contains(t, setupSection, "### `slipway init`", "%s: init must be in setup", cfg.ID)
		assert.NotContains(t, setupSection, "### `slipway config`", "%s: config must remain CLI-only, not a generated prompt command", cfg.ID)
		assert.Contains(t, ref, "Can be used with or without an active change.", "%s: missing explicit status prerequisite", cfg.ID)
		assert.Contains(t, ref, "an active change must exist, or pass `--change <slug>` when supported.", "%s: missing helper-default prerequisite", cfg.ID)
		assert.Contains(t, refText, "every non-hidden, non-help Cobra flag appears here unless it belongs to the narrow manual `evidence task` ledger surface", "%s: missing flag completeness wording", cfg.ID)
		assert.NotContains(t, ref, "`review --artifact`", "%s: stale review artifact exemption leaked", cfg.ID)
		assert.NotContains(t, refText, "every non-hidden Cobra flag appears here", "%s: stale all-visible-flags completeness wording leaked", cfg.ID)

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

}

func TestGeneratedNewCommandSurfacesDocumentJSONStdinClassification(t *testing.T) {
	root := t.TempDir()

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

	raw, err := tmpl.Content(filepath.ToSlash(filepath.Join("skills", "context-assembly", catalogSkillSourceFile)))
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

	raw, err := tmpl.Content(filepath.ToSlash(filepath.Join("skills", "independent-review", catalogSkillSourceFile)))
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

	toolIDs, err := ResolveTools("all")
	require.NoError(t, err)
	require.NoError(t, Generate(root, toolIDs, true))

	for _, cfg := range Registry() {
		for _, id := range commandIDs() {
			_, absPath := commandSurfacePath(root, cfg, id)
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
			if id == "run" {
				sText := strings.Join(strings.Fields(s), " ")
				assert.Contains(t, s, "`next_skill.selected_review_skills`", "%s/%s must route reviewer set authority to current output", cfg.ID, id)
				assert.Contains(t, sText, "code-quality-review` when selected by the workflow profile", "%s/%s must qualify code-quality-review by workflow profile", cfg.ID, id)
				assert.Contains(t, sText, "security-review` when selected by policy", "%s/%s must qualify security-review by policy", cfg.ID, id)
				assert.NotContains(t, sText, "`spec-compliance-review`, `code-quality-review`, `independent-review`", "%s/%s must not hard-code code-quality-review as a baseline peer", cfg.ID, id)
			}
		}

		if cfg.SettingsPath != "" && cfg.SettingsKind != settingsKindCodexHooks && cfg.SessionHook != "" {
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
	require.NoError(t, os.WriteFile(markerPath, []byte("generated\n"), 0o644))
}

func addOwnershipManifestFiles(t *testing.T, root string, cfg ToolConfig, paths ...string) {
	t.Helper()

	manifest, found, err := loadOwnershipManifest(root, cfg)
	require.NoError(t, err)
	if !found {
		manifest = ownershipManifest{
			Version: ownershipManifestVersion,
			ToolID:  cfg.ID,
		}
	}
	byPath := manifest.index()
	for _, p := range paths {
		rel, err := filepath.Rel(root, p)
		require.NoError(t, err)
		rel, err = normalizeOwnershipPath(filepath.ToSlash(rel))
		require.NoError(t, err)
		raw, err := os.ReadFile(p)
		require.NoError(t, err)
		byPath[rel] = ownershipManifestFile{
			Path:   rel,
			SHA256: hashBytes(raw),
		}
	}
	manifest.Files = manifest.Files[:0]
	for _, file := range byPath {
		manifest.Files = append(manifest.Files, file)
	}
	raw, err := encodeOwnershipManifest(manifest)
	require.NoError(t, err)
	manifestPath := filepath.Join(root, generatedOwnershipManifestPath(cfg))
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0o755))
	require.NoError(t, os.WriteFile(manifestPath, raw, 0o644))
}

func TestGeneratedSkillsReferenceValidCommands(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"claude"}, true))

	// Build set of valid commands.
	validCmds := map[string]bool{
		"new": true, "intake": true, "plan": true, "implement": true,
		"next": true, "run": true, "status": true, "done": true, "fix": true,
		"abort": true, "cancel": true, "review": true, "validate": true,
		"preset": true, "repair": true, "init": true,
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

	// Refresh should deterministically regenerate pristine managed content.
	require.NoError(t, Generate(root, []string{"claude"}, true))
	refreshedCommand, err := os.ReadFile(commandPath)
	require.NoError(t, err)
	assert.Equal(t, string(firstCommand), string(refreshedCommand))

	// Non-refresh generation should keep existing files unchanged.
	require.NoError(t, os.WriteFile(commandPath, []byte("custom"), 0o644))
	require.NoError(t, Generate(root, []string{"claude"}, false))
	secondCommand, err := os.ReadFile(commandPath)
	require.NoError(t, err)
	assert.Equal(t, "custom", string(secondCommand))

	// Refresh refuses to overwrite managed content that was modified outside
	// toolgen.
	err = Generate(root, []string{"claude"}, true)
	require.Error(t, err)
	assert.ErrorContains(t, err, "managed-modified")
	keptCommand, err := os.ReadFile(commandPath)
	require.NoError(t, err)
	assert.Equal(t, "custom", string(keptCommand))
}

func TestGenerateRefreshInvalidatesTrustedSurfacesAfterTransactionFailure(t *testing.T) {
	root := t.TempDir()

	require.NoError(t, Generate(root, []string{"claude"}, true))
	commandPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")
	manifestPath := filepath.Join(root, generatedOwnershipManifestPath(toolRegistry["claude"]))
	sentinelPath := filepath.Join(root, GeneratedAdapterMarkerPath(toolRegistry["claude"]))
	beforeManifest, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	failErr := errors.New("simulated transaction write failure")
	previousApply := toolgenApplyFileTransaction
	t.Cleanup(func() {
		toolgenApplyFileTransaction = previousApply
	})
	toolgenApplyFileTransaction = func(ops []fsutil.FileTransactionOp) error {
		writeCount := 0
		return fsutil.ApplyFileTransactionWithHooks(ops, fsutil.FileTransactionHooks{
			WriteFile: func(path string, data []byte, perm os.FileMode) error {
				writeCount++
				if writeCount == 3 {
					return failErr
				}
				return fsutil.WriteFileAtomic(path, data, perm)
			},
			RemoveFile: func(path string) error {
				if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return err
				}
				return nil
			},
			RemoveAll: func(path string) error {
				return os.RemoveAll(path)
			},
		})
	}

	err = Generate(root, []string{"claude"}, true)
	require.Error(t, err)
	assert.ErrorIs(t, err, failErr)

	afterManifest, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.Equal(t, string(beforeManifest), string(afterManifest))

	_, statErr := os.Stat(sentinelPath)
	assert.True(t, os.IsNotExist(statErr), "failed refresh must leave the sentinel absent")
	_, statErr = os.Stat(commandPath)
	assert.True(t, os.IsNotExist(statErr), "failed refresh must not leave previously trusted command prompts in place")
}

func TestGenerateRefreshPrunesOnlyGeneratedTopLevelSkillEntries(t *testing.T) {
	root := t.TempDir()

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
	addOwnershipManifestFiles(
		t,
		root,
		toolRegistry["claude"],
		filepath.Join(staleSkillDir, "SKILL.md"),
		oldCatalogRoutePath,
		oldCatalogSupportPath,
	)

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

func TestGenerateRefreshPrunesRetiredCommandSkillDirsByContent(t *testing.T) {
	retiredSkills := []struct {
		id       string
		sourceID string
	}{
		{id: "synthetic-query-retired-command", sourceID: "next"},
		{id: "synthetic-mutation-retired-command", sourceID: "new"},
		{id: "synthetic-setup-retired-command", sourceID: "init"},
		{id: "synthetic-never-enumerated-retired-command", sourceID: "preset"},
	}

	t.Run("manifest absent residue from a real generated command body is pruned", func(t *testing.T) {
		root := t.TempDir()
		cfg := toolRegistry["codex"]

		require.NoError(t, Generate(root, []string{cfg.ID}, true))

		id := "synthetic-realistic-retired"
		sourceID := "next"
		sourceSkill, err := os.ReadFile(commandSkillPath(root, cfg, sourceID))
		require.NoError(t, err)
		skillPath := commandSkillPath(root, cfg, id)
		require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
		require.NoError(t, os.WriteFile(
			skillPath,
			[]byte(rewriteGeneratedCommandSkillIdentityForTest(t, string(sourceSkill), cfg, sourceID, id)),
			0o644,
		))

		require.NoError(t, Generate(root, []string{cfg.ID}, true))

		_, err = os.Stat(filepath.Dir(skillPath))
		assert.True(t, os.IsNotExist(err), "refresh should prune realistic manifestless generated command skill residue")
		assertNoOrphanCommandSkills(t, root, cfg)
	})

	t.Run("manifest absent realistic legacy stats sample is pruned", func(t *testing.T) {
		for _, cfg := range commandSkillHostConfigs(t) {
			t.Run(cfg.ID, func(t *testing.T) {
				root := t.TempDir()

				require.NoError(t, Generate(root, []string{cfg.ID}, true))

				skillPath := commandSkillPath(root, cfg, "stats")
				require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
				require.NoError(t, os.WriteFile(
					skillPath,
					[]byte(realisticLegacyRetiredCommandSkillForTest(t, cfg, "stats")),
					0o644,
				))

				require.NoError(t, Generate(root, []string{cfg.ID}, true))

				_, err := os.Stat(filepath.Dir(skillPath))
				assert.True(t, os.IsNotExist(err), "%s refresh should prune realistic legacy stats residue", cfg.ID)
				assertNoOrphanCommandSkills(t, root, cfg)
			})
		}
	})

	t.Run("manifest absent legacy generated command samples are pruned", func(t *testing.T) {
		legacyIDs := []string{"stats", "learn", "checkpoint", "pivot"}
		for _, cfg := range commandSkillHostConfigs(t) {
			t.Run(cfg.ID, func(t *testing.T) {
				root := t.TempDir()

				require.NoError(t, Generate(root, []string{cfg.ID}, true))

				for _, id := range legacyIDs {
					skillPath := commandSkillPath(root, cfg, id)
					require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
					require.NoError(t, os.WriteFile(
						skillPath,
						[]byte(realisticLegacyRetiredCommandSkillForTest(t, cfg, id)),
						0o644,
					))
				}

				require.NoError(t, Generate(root, []string{cfg.ID}, true))

				for _, id := range legacyIDs {
					_, err := os.Stat(filepath.Dir(commandSkillPath(root, cfg, id)))
					assert.True(t, os.IsNotExist(err), "%s refresh should prune realistic legacy command skill %s", cfg.ID, id)
				}
				assertNoOrphanCommandSkills(t, root, cfg)
			})
		}
	})

	t.Run("manifest absent residue for every command skill host", func(t *testing.T) {
		for _, cfg := range commandSkillHostConfigs(t) {
			t.Run(cfg.ID, func(t *testing.T) {
				root := t.TempDir()

				require.NoError(t, Generate(root, []string{cfg.ID}, true))

				for _, retired := range retiredSkills {
					skillPath := commandSkillPath(root, cfg, retired.id)
					require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
					require.NoError(t, os.WriteFile(
						skillPath,
						[]byte(generatedRetiredCommandSkill(t, cfg, retired.sourceID, retired.id)),
						0o644,
					))
				}
				userOwnedSkill := commandSkillPath(root, cfg, "user-owned")
				require.NoError(t, os.MkdirAll(filepath.Dir(userOwnedSkill), 0o755))
				require.NoError(t, os.WriteFile(userOwnedSkill, []byte("keep user-owned skill"), 0o644))

				require.NoError(t, Generate(root, []string{cfg.ID}, true))

				for _, retired := range retiredSkills {
					_, err := os.Stat(filepath.Dir(commandSkillPath(root, cfg, retired.id)))
					assert.True(t, os.IsNotExist(err), "%s refresh should remove retired generated command skill %s", cfg.ID, retired.id)
				}
				_, err := os.Stat(userOwnedSkill)
				assert.NoError(t, err, "%s refresh must preserve adjacent user-owned skill dirs", cfg.ID)
				assertNoOrphanCommandSkills(t, root, cfg)
			})
		}
	})

	t.Run("manifest tracked residue is pruned", func(t *testing.T) {
		for _, cfg := range commandSkillHostConfigs(t) {
			t.Run(cfg.ID, func(t *testing.T) {
				root := t.TempDir()

				require.NoError(t, Generate(root, []string{cfg.ID}, true))

				id := "synthetic-manifest-tracked-retired-command"
				skillPath := commandSkillPath(root, cfg, id)
				require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
				require.NoError(t, os.WriteFile(skillPath, []byte(generatedRetiredCommandSkill(t, cfg, "new", id)), 0o644))
				addOwnershipManifestFiles(t, root, cfg, skillPath)

				require.NoError(t, Generate(root, []string{cfg.ID}, true))

				_, err := os.Stat(filepath.Dir(skillPath))
				assert.True(t, os.IsNotExist(err), "%s refresh should prune manifest-tracked retired generated command skill", cfg.ID)
				assertNoOrphanCommandSkills(t, root, cfg)
			})
		}
	})

	t.Run("manifest absent user modified generated shape is preserved", func(t *testing.T) {
		root := t.TempDir()
		cfg := toolRegistry["codex"]

		require.NoError(t, Generate(root, []string{cfg.ID}, true))

		afterFooterID := "synthetic-user-modified-after-footer-command"
		afterFooterSkill := commandSkillPath(root, cfg, afterFooterID)
		require.NoError(t, os.MkdirAll(filepath.Dir(afterFooterSkill), 0o755))
		require.NoError(t, os.WriteFile(afterFooterSkill, []byte(userModifiedRetiredCommandSkill(t, cfg, "next", afterFooterID)), 0o644))
		beforeFooterID := "synthetic-user-modified-before-footer-command"
		beforeFooterSkill := commandSkillPath(root, cfg, beforeFooterID)
		require.NoError(t, os.MkdirAll(filepath.Dir(beforeFooterSkill), 0o755))
		require.NoError(t, os.WriteFile(
			beforeFooterSkill,
			[]byte(userModifiedRetiredCommandSkillBeforeFooter(t, cfg, "new", beforeFooterID)),
			0o644,
		))
		shapePreservingID := "synthetic-user-modified-shape-command"
		shapePreservingSkill := commandSkillPath(root, cfg, shapePreservingID)
		require.NoError(t, os.MkdirAll(filepath.Dir(shapePreservingSkill), 0o755))
		require.NoError(t, os.WriteFile(
			shapePreservingSkill,
			[]byte(userModifiedRetiredCommandSkillBullet(t, cfg, "next", shapePreservingID)),
			0o644,
		))

		require.NoError(t, Generate(root, []string{cfg.ID}, true))

		got, err := os.ReadFile(afterFooterSkill)
		require.NoError(t, err, "refresh must preserve user-modified generated-shape content under retired command names")
		assert.Contains(t, string(got), "local operator notes")
		got, err = os.ReadFile(beforeFooterSkill)
		require.NoError(t, err, "refresh must preserve user-modified generated-shape content before the footer")
		assert.Contains(t, string(got), "local operator notes")
		got, err = os.ReadFile(shapePreservingSkill)
		require.NoError(t, err, "refresh must preserve shape-preserving user-modified generated content")
		assert.Contains(t, string(got), "local operator override")
	})

	t.Run("manifest absent user modified realistic legacy stats sample is preserved", func(t *testing.T) {
		for _, cfg := range commandSkillHostConfigs(t) {
			t.Run(cfg.ID, func(t *testing.T) {
				root := t.TempDir()

				require.NoError(t, Generate(root, []string{cfg.ID}, true))

				id := "stats"
				skillPath := commandSkillPath(root, cfg, id)
				require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
				sample := realisticLegacyRetiredCommandSkillForTest(t, cfg, id)
				footer := commandSkillFooter(id)
				require.Contains(t, sample, footer)
				modified := replaceOnceForTest(
					t,
					sample,
					"Show repo-wide governance freshness and workflow statistics across every change.",
					"Show repo-wide governance freshness and local operator notes across every change.",
				)
				require.Contains(t, modified, footer)
				require.NoError(t, os.WriteFile(skillPath, []byte(modified), 0o644))

				require.NoError(t, Generate(root, []string{cfg.ID}, true))

				got, err := os.ReadFile(skillPath)
				require.NoError(t, err, "%s refresh must preserve user-modified realistic legacy stats content", cfg.ID)
				assert.Equal(t, modified, string(got))
			})
		}
	})

	t.Run("manifest tracked user modified generated shape refuses deletion", func(t *testing.T) {
		root := t.TempDir()
		cfg := toolRegistry["codex"]

		require.NoError(t, Generate(root, []string{cfg.ID}, true))

		id := "synthetic-manifest-tracked-user-modified-command"
		skillPath := commandSkillPath(root, cfg, id)
		require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
		require.NoError(t, os.WriteFile(skillPath, []byte(generatedRetiredCommandSkill(t, cfg, "next", id)), 0o644))
		addOwnershipManifestFiles(t, root, cfg, skillPath)
		require.NoError(t, os.WriteFile(skillPath, []byte(userModifiedRetiredCommandSkill(t, cfg, "next", id)), 0o644))

		err := Generate(root, []string{cfg.ID}, true)
		require.Error(t, err)
		assert.ErrorContains(t, err, "managed-modified")

		got, readErr := os.ReadFile(skillPath)
		require.NoError(t, readErr)
		assert.Contains(t, string(got), "local operator notes")
	})
}

func commandSkillHostConfigs(t *testing.T) []ToolConfig {
	t.Helper()

	var out []ToolConfig
	for _, cfg := range Registry() {
		if cfg.CommandSkillSurface {
			out = append(out, cfg)
		}
	}
	require.NotEmpty(t, out)
	return out
}

func commandSkillPath(root string, cfg ToolConfig, id string) string {
	return filepath.Join(root, SkillPath(cfg, id))
}

func assertNoOrphanCommandSkills(t *testing.T, root string, cfg ToolConfig) {
	t.Helper()

	skillsRoot := filepath.Join(root, cfg.SkillsDir)
	err := filepath.WalkDir(skillsRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return walkErr
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fm, _, err := splitSkillFrontmatter(string(raw))
		if err != nil {
			return nil
		}
		var meta struct {
			CommandID string `yaml:"command_id"`
			Surface   string `yaml:"surface"`
		}
		if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
			return nil
		}
		id := strings.TrimSpace(meta.CommandID)
		if id == "" || strings.TrimSpace(meta.Surface) != "skill" {
			return nil
		}
		if _, ok := commandRegistryMap[id]; !ok {
			t.Fatalf("%s refresh left orphan command skill %s at %s", cfg.ID, id, path)
		}
		return nil
	})
	require.NoError(t, err)
}

func generatedRetiredCommandSkill(t *testing.T, cfg ToolConfig, sourceID, id string) string {
	t.Helper()

	content, err := renderCommandSkill(cfg, sourceID)
	require.NoError(t, err)
	return rewriteGeneratedCommandSkillIdentityForTest(t, content, cfg, sourceID, id)
}

func realisticLegacyRetiredCommandSkillForTest(t *testing.T, cfg ToolConfig, id string) string {
	t.Helper()

	sample := realisticLegacyCodexRetiredCommandSkillForTest(t, id)
	return replaceOnceForTest(
		t,
		sample,
		fmt.Sprintf("trigger: \"$slipway-%s\"", id),
		"trigger: \""+commandTrigger(cfg, id)+"\"",
	)
}

func realisticLegacyCodexRetiredCommandSkillForTest(t *testing.T, id string) string {
	t.Helper()

	switch id {
	case "stats":
		return legacyRetiredCommandSkillSampleForTest(
			"---",
			"name: slipway-stats",
			"description: \"Show repo-wide governance freshness and workflow statistics\"",
			"install_profiles:",
			"  - full",
			"requires: []",
			"command_id: \"stats\"",
			"trigger: \"$slipway-stats\"",
			"class: \"query\"",
			"tier: \"diagnostics\"",
			"surface: \"skill\"",
			"---",
			"# Stats",
			"",
			"Show repo-wide governance freshness and workflow statistics across every change.",
			"",
			"## Invocation",
			"```bash",
			"slipway stats --json",
			"```",
			"",
			"## Contract",
			"- Read-only repo-wide observability: active/archived counts, freshness summaries,",
			"  and workflow statistics. It is not scoped to a single change — use",
			"  `slipway status` for the active change.",
			"",
			"## Flags",
			"- `--json`: JSON output",
			"",
			"## Arguments",
			"```text",
			"[--json]",
			"```",
			"",
			"## Prerequisites",
			"- `.slipway.yaml` must exist (run `slipway init` first)",
			"",
			"Invoke the authoritative `slipway stats` CLI surface directly; do not reimplement Slipway lifecycle semantics.",
		)
	case "learn":
		return legacyRetiredCommandSkillSampleForTest(
			"---",
			"name: slipway-learn",
			"description: \"Preview governance learning proposals from lifecycle evidence\"",
			"install_profiles:",
			"  - full",
			"requires: []",
			"command_id: \"learn\"",
			"trigger: \"$slipway-learn\"",
			"class: \"query\"",
			"tier: \"diagnostics\"",
			"surface: \"skill\"",
			"---",
			"# Learn",
			"",
			"Preview read-only governance learning proposals derived from accumulated",
			"lifecycle evidence.",
			"",
			"## Invocation",
			"```bash",
			"slipway learn --preview --json",
			"```",
			"",
			"## Contract",
			"- Read-only and non-mutating: it surfaces *proposed* governance adjustments for",
			"  human review; it never applies them. Proposals are advisory only.",
			"- `--preview` is the default. This is a maintainer/observability surface, not a",
			"  step in driving a single change.",
			"",
			"## Flags",
			"- `--preview`: generate read-only governance learning proposals (default true)",
			"- `--json`: JSON output",
			"",
			"## Arguments",
			"```text",
			"[--preview] [--json]",
			"```",
			"",
			"## Prerequisites",
			"- `.slipway.yaml` must exist (run `slipway init` first)",
			"",
			"Invoke the authoritative `slipway learn` CLI surface directly; do not reimplement Slipway lifecycle semantics.",
		)
	case "checkpoint":
		return legacyRetiredCommandSkillSampleForTest(
			"---",
			"name: slipway-checkpoint",
			"description: \"Set an active checkpoint to pause wave execution and request user input\"",
			"install_profiles:",
			"  - full",
			"requires: []",
			"command_id: \"checkpoint\"",
			"trigger: \"$slipway-checkpoint\"",
			"class: \"mutation\"",
			"tier: \"situational\"",
			"surface: \"skill\"",
			"---",
			"# Checkpoint",
			"",
			"Pause wave execution and request user input for a specific task.",
			"",
			"## Invocation",
			"```bash",
			"slipway checkpoint --task-id <task_id> [--type <type>] [--allowed-responses <responses>] --json",
			"```",
			"",
			"## Contract",
			"- Sets an active checkpoint on the current governed change.",
			"- Only valid during `S2_IMPLEMENT` state.",
			"- Only one checkpoint can be active at a time.",
			"- Resume with `slipway run --resume-response \"<response>\"`.",
			"- After checkpoint resume, a **fresh subagent MUST be spawned** — do NOT continue in the same context.",
			"",
			"## When to Use",
			"- Task encounters a blocker requiring human judgment (architectural decision, ambiguous requirement).",
			"- Task encounters deviation that requires user decision.",
			"- Retry budget exhausted — surface failure to user for decision.",
			"",
			"## Flags",
			"- `--task-id <id>`: ID of the paused task (required)",
			"- `--type <type>`: Checkpoint type — `human_verify` (default), `decision`, `human_action`",
			"- `--allowed-responses <responses>`: Comma-separated allowed response values (required for `type=decision`)",
			"- `--json`: JSON output",
			"- `--change <slug>`: target a specific active change",
			"",
			"## Arguments",
			"```text",
			"--task-id <id> [--type human_verify|decision|human_action] [--allowed-responses <value> ...] [--json] [--change <slug>]",
			"```",
			"",
			"## Prerequisites",
			"- `.slipway.yaml` must exist (run `slipway init` first)",
			"- An active governed change must be in S2_IMPLEMENT with a materialized wave plan (run `slipway repair` if `wave-plan.yaml` is missing).",
			"",
			"Invoke the authoritative `slipway checkpoint` CLI surface directly; do not reimplement Slipway lifecycle semantics.",
		)
	case "pivot":
		return legacyRetiredCommandSkillSampleForTest(
			"---",
			"name: slipway-pivot",
			"description: \"Reroute or rescope an active change\"",
			"command_id: \"pivot\"",
			"trigger: \"$slipway-pivot\"",
			"class: \"mutation\"",
			"tier: \"situational\"",
			"surface: \"skill\"",
			"---",
			"# Pivot",
			"",
			"Reroute (re-evaluate the routing/discovery decision) or rescope (reopen intake to",
			"amend scope) an active change. Both set `needs_discovery=true` and clear",
			"execution residue.",
			"",
			"## Invocation",
			"```bash",
			"slipway pivot --reroute",
			"slipway pivot --rescope",
			"```",
			"",
			"## Contract",
			"- Show pivot summary with before/after state.",
			"- Confirm pivot action with user before executing.",
			"- `--reroute` (the default when no flag is given) is valid in `S1_PLAN`,",
			"  `S2_IMPLEMENT`, or `S3_REVIEW`; it returns the change to `S1_PLAN`",
			"  with discovery forced on. An invalid state is blocked (`pivot_state_invalid`).",
			"- `--rescope` is valid in `S2_IMPLEMENT` or `S3_REVIEW`; it returns",
			"  the change to `S0_INTAKE` (intake/clarify) and clears the intent",
			"  `## Approved Summary` so it must be re-confirmed. Before execution",
			"  (`S0_INTAKE`/`S1_PLAN`) and terminal states are blocked",
			"  (`rescope_state_invalid`).",
			"",
			"## Flags",
			"- `--reroute`: Re-evaluate routing/discovery and re-enter `S1_PLAN` (valid in S1_PLAN/S2_IMPLEMENT/S3_REVIEW).",
			"- `--rescope`: Reopen intake — return to `S0_INTAKE` to amend scope, clearing the Approved Summary (valid in S2_IMPLEMENT/S3_REVIEW).",
			"- `--json`: JSON output",
			"- `--change <slug>`: target a specific active change",
			"",
			"## Arguments",
			"```text",
			"[--reroute|--rescope] [--json] [--change <slug>]",
			"```",
			"",
			"## Prerequisites",
			"- `.slipway.yaml` must exist (run `slipway init` first)",
			"- an active change must exist, or pass `--change <slug>` when supported.",
			"",
			"Invoke the authoritative `slipway pivot` CLI surface directly; do not reimplement Slipway lifecycle semantics.",
		)
	default:
		t.Fatalf("unknown realistic legacy command skill sample %q", id)
		return ""
	}
}

func legacyRetiredCommandSkillSampleForTest(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

func rewriteGeneratedCommandSkillIdentityForTest(t *testing.T, content string, cfg ToolConfig, sourceID, id string) string {
	t.Helper()

	content = normalizeTemplateLineEndings(content)
	for _, replacement := range []struct {
		old string
		new string
	}{
		{
			old: "name: " + adapterSkillName(sourceID) + "\n",
			new: "name: " + adapterSkillName(id) + "\n",
		},
		{
			old: "command_id: \"" + sourceID + "\"\n",
			new: "command_id: \"" + id + "\"\n",
		},
		{
			old: "trigger: \"" + commandTrigger(cfg, sourceID) + "\"\n",
			new: "trigger: \"" + commandTrigger(cfg, id) + "\"\n",
		},
		{
			old: commandSkillFooter(sourceID),
			new: commandSkillFooter(id),
		},
	} {
		require.Equal(t, 1, strings.Count(content, replacement.old), "test fixture should contain one generated identity marker %q", replacement.old)
		content = strings.Replace(content, replacement.old, replacement.new, 1)
	}
	return content
}

func userModifiedRetiredCommandSkill(t *testing.T, cfg ToolConfig, sourceID, id string) string {
	t.Helper()

	return generatedRetiredCommandSkill(t, cfg, sourceID, id) + "\nlocal operator notes\n"
}

func userModifiedRetiredCommandSkillBeforeFooter(t *testing.T, cfg ToolConfig, sourceID, id string) string {
	t.Helper()

	footer := commandSkillFooter(id)
	return replaceOnceForTest(t, generatedRetiredCommandSkill(t, cfg, sourceID, id), "\n"+footer, "\nlocal operator notes\n\n"+footer)
}

func userModifiedRetiredCommandSkillBullet(t *testing.T, cfg ToolConfig, sourceID, id string) string {
	t.Helper()

	return replaceOnceForTest(
		t,
		generatedRetiredCommandSkill(t, cfg, sourceID, id),
		"- `next_skill.name` is the authoritative governed-host handoff.",
		"- `next_skill.name` is a local operator override.",
	)
}

func replaceOnceForTest(t *testing.T, content, old, new string) string {
	t.Helper()

	require.Equal(t, 1, strings.Count(content, old), "test fixture should contain one copy of %q", old)
	return strings.Replace(content, old, new, 1)
}

func TestGenerateRefreshDoesNotPruneSkillDirsWithoutGeneratedAdapterMarker(t *testing.T) {
	root := t.TempDir()

	userOwnedSlipwayDir := filepath.Join(root, ".claude", "skills", "slipway-user-owned")
	require.NoError(t, os.MkdirAll(userOwnedSlipwayDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(userOwnedSlipwayDir, "SKILL.md"), []byte("keep me"), 0o644))

	require.NoError(t, Generate(root, []string{"claude"}, true))

	_, err := os.Stat(filepath.Join(userOwnedSlipwayDir, "SKILL.md"))
	assert.NoError(t, err, "refresh without a generated adapter marker must not prune user-owned slipway-* skill dirs")
}

func TestGenerateRefreshPreservesUserOwnedCopilotGitHubFiles(t *testing.T) {
	root := t.TempDir()

	userFiles := map[string]string{
		".github/workflows/ci.yml":           "name: ci\n",
		".github/prompts/user.prompt.md":     "# user prompt\n",
		".github/skills/user-skill/SKILL.md": "# user skill\n",
		".github/copilot/user-note.md":       "keep user note\n",
	}
	for rel, content := range userFiles {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}

	require.NoError(t, Generate(root, []string{"copilot"}, true))
	require.NoError(t, Generate(root, []string{"copilot"}, true))

	assert.FileExists(t, filepath.Join(root, ".github", "copilot", "slipway", ".adapter-generated"))
	assert.FileExists(t, filepath.Join(root, ".github", "prompts", "slipway-new.prompt.md"))
	assert.FileExists(t, filepath.Join(root, ".github", "skills", "slipway", "SKILL.md"))
	_, err := os.Stat(filepath.Join(root, ".github", "slipway", ".adapter-generated"))
	assert.True(t, os.IsNotExist(err), "copilot refresh must not claim shared .github ownership")

	manifest, found, err := loadOwnershipManifest(root, toolRegistry["copilot"])
	require.NoError(t, err)
	require.True(t, found, "copilot refresh must write an ownership manifest")
	owned := manifest.index()
	assert.Contains(t, owned, ".github/copilot/slipway/.adapter-generated")
	assert.Contains(t, owned, ".github/prompts/slipway-new.prompt.md")
	assert.Contains(t, owned, ".github/skills/slipway/SKILL.md")

	for rel, want := range userFiles {
		got, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		require.NoError(t, readErr, "refresh must preserve user-owned %s", rel)
		assert.Equal(t, want, string(got), "refresh must not rewrite user-owned %s", rel)
		assert.NotContains(t, owned, rel, "refresh must not mark user-owned %s as generated", rel)
	}
	for rel := range owned {
		assert.Truef(t,
			strings.HasPrefix(rel, ".github/copilot/slipway/") ||
				strings.HasPrefix(rel, ".github/prompts/slipway-") ||
				strings.HasPrefix(rel, ".github/skills/slipway"),
			"copilot ownership manifest must not claim non-generated shared .github path %s", rel)
	}
}

func TestGenerateRefreshPreservesUnknownCleanupTargetsAndRefusesManagedModified(t *testing.T) {
	root := t.TempDir()
	cfg := toolRegistry["claude"]

	require.NoError(t, Generate(root, []string{"claude"}, true))

	staleSkillPath := filepath.Join(root, ".claude", "skills", "slipway-tdd", "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(staleSkillPath), 0o755))
	require.NoError(t, os.WriteFile(staleSkillPath, []byte("unknown stale skill"), 0o644))

	require.NoError(t, Generate(root, []string{"claude"}, true))
	got, err := os.ReadFile(staleSkillPath)
	require.NoError(t, err, "unknown stale cleanup target must be preserved")
	assert.Equal(t, "unknown stale skill", string(got))

	addOwnershipManifestFiles(t, root, cfg, staleSkillPath)
	require.NoError(t, os.WriteFile(staleSkillPath, []byte("edited managed stale skill"), 0o644))

	err = Generate(root, []string{"claude"}, true)
	require.Error(t, err)
	assert.ErrorContains(t, err, "managed-modified")
	got, readErr := os.ReadFile(staleSkillPath)
	require.NoError(t, readErr)
	assert.Equal(t, "edited managed stale skill", string(got))
}

func TestGenerateRefreshWithoutOwnershipManifestBootstrapsIntoTrackedState(t *testing.T) {
	// When an adapter sentinel exists but no ownership manifest is present
	// (pre-manifest legacy state), --refresh bootstraps by regenerating all
	// files and writing the manifest. User modifications that predate
	// manifest tracking cannot be distinguished from Slipway-generated
	// content and will be overwritten.
	t.Run("command prompt", func(t *testing.T) {
		root := t.TempDir()
		cfg := toolRegistry["claude"]
		writeGeneratedAdapterMarker(t, root, cfg)

		commandPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(commandPath), 0o755))
		require.NoError(t, os.WriteFile(commandPath, []byte("user modified command"), 0o644))

		err := Generate(root, []string{"claude"}, true)
		require.NoError(t, err)

		// Bootstrap overwrites pre-existing file and creates the manifest.
		got, readErr := os.ReadFile(commandPath)
		require.NoError(t, readErr)
		assert.NotEqual(t, "user modified command", string(got))

		manifestPath := filepath.Join(root, generatedOwnershipManifestPath(cfg))
		_, statErr := os.Stat(manifestPath)
		assert.NoError(t, statErr, "bootstrap refresh must create ownership manifest")
	})

	t.Run("sentinel", func(t *testing.T) {
		root := t.TempDir()
		cfg := toolRegistry["claude"]
		sentinelPath := filepath.Join(root, GeneratedAdapterMarkerPath(cfg))
		require.NoError(t, os.MkdirAll(filepath.Dir(sentinelPath), 0o755))
		require.NoError(t, os.WriteFile(sentinelPath, []byte("user modified marker"), 0o644))

		err := Generate(root, []string{"claude"}, true)
		require.NoError(t, err)

		// Bootstrap overwrites sentinel with canonical content.
		got, readErr := os.ReadFile(sentinelPath)
		require.NoError(t, readErr)
		assert.Equal(t, "generated\n", string(got))

		// Manifest is created and tracks the sentinel.
		manifestPath := filepath.Join(root, generatedOwnershipManifestPath(cfg))
		_, statErr := os.Stat(manifestPath)
		assert.NoError(t, statErr, "bootstrap refresh must create ownership manifest")
	})

	t.Run("skill file", func(t *testing.T) {
		root := t.TempDir()
		cfg := toolRegistry["claude"]
		writeGeneratedAdapterMarker(t, root, cfg)

		// Sentinel-only legacy bootstrap overwrites pre-existing generated-path
		// content and brings the adapter into manifest tracking.
		skillPath := filepath.Join(root, ".claude", "skills", "slipway", "SKILL.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
		require.NoError(t, os.WriteFile(skillPath, []byte("will be overwritten"), 0o644))

		err := Generate(root, []string{"claude"}, true)
		require.NoError(t, err)

		got, readErr := os.ReadFile(skillPath)
		require.NoError(t, readErr)
		assert.NotEqual(t, "will be overwritten", string(got))

		manifestPath := filepath.Join(root, generatedOwnershipManifestPath(cfg))
		_, statErr := os.Stat(manifestPath)
		assert.NoError(t, statErr, "bootstrap refresh must create ownership manifest")
	})

	t.Run("stale generated cleanup targets", func(t *testing.T) {
		root := t.TempDir()
		cfg := toolRegistry["claude"]
		writeGeneratedAdapterMarker(t, root, cfg)

		staleCommand := filepath.Join(root, ".claude", "commands", "slipway", "advance.md")
		staleSkill := filepath.Join(root, ".claude", "skills", "slipway-tdd", "SKILL.md")
		staleHook := filepath.Join(root, ".claude", "hooks", "slipway-session-start")
		for _, p := range []string{staleCommand, staleSkill, staleHook} {
			require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
			require.NoError(t, os.WriteFile(p, []byte("legacy generated content"), 0o644))
		}

		require.NoError(t, Generate(root, []string{"claude"}, true))

		for _, p := range []string{staleCommand, staleSkill, staleHook} {
			_, err := os.Stat(p)
			assert.True(t, os.IsNotExist(err), "sentinel-only bootstrap must prune stale generated path %s", p)
		}

		manifestPath := filepath.Join(root, generatedOwnershipManifestPath(cfg))
		_, statErr := os.Stat(manifestPath)
		assert.NoError(t, statErr, "bootstrap refresh must create ownership manifest")
	})

	t.Run("flat command stale entry", func(t *testing.T) {
		root := t.TempDir()
		cfg := toolRegistry["cursor"]
		writeGeneratedAdapterMarker(t, root, cfg)

		staleCommand := filepath.Join(root, ".cursor", "commands", "slipway-advance.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(staleCommand), 0o755))
		require.NoError(t, os.WriteFile(staleCommand, []byte("legacy generated command"), 0o644))

		require.NoError(t, Generate(root, []string{"cursor"}, true))

		_, err := os.Stat(staleCommand)
		assert.True(t, os.IsNotExist(err), "sentinel-only bootstrap must prune stale flat command path")

		manifestPath := filepath.Join(root, generatedOwnershipManifestPath(cfg))
		_, statErr := os.Stat(manifestPath)
		assert.NoError(t, statErr, "bootstrap refresh must create ownership manifest")
	})
}

func TestGenerateRefreshWithoutOwnershipManifestRefusesUserOwnedPiHookExtension(t *testing.T) {
	root := t.TempDir()
	cfg := toolRegistry["pi"]
	writeGeneratedAdapterMarker(t, root, cfg)

	extensionPath := filepath.Join(root, ".pi", "extensions", "slipway-hooks.ts")
	require.NoError(t, os.MkdirAll(filepath.Dir(extensionPath), 0o755))
	userContent := "export default function userOwnedPiExtension() { return \"keep me\"; }\n"
	require.NoError(t, os.WriteFile(extensionPath, []byte(userContent), 0o644))

	err := Generate(root, []string{"pi"}, true)
	require.Error(t, err)
	assert.ErrorContains(t, err, "refusing to overwrite unknown file .pi/extensions/slipway-hooks.ts")

	got, readErr := os.ReadFile(extensionPath)
	require.NoError(t, readErr)
	assert.Equal(t, userContent, string(got))

	manifestPath := filepath.Join(root, generatedOwnershipManifestPath(cfg))
	_, statErr := os.Stat(manifestPath)
	assert.True(t, os.IsNotExist(statErr), "failed refresh must not create an ownership manifest")
}

func TestGenerateRefreshWithoutOwnershipManifestRequiresBootstrapAuthority(t *testing.T) {
	t.Run("missing sentinel refuses unknown generated path collision", func(t *testing.T) {
		root := t.TempDir()
		cfg := toolRegistry["claude"]

		commandPath := filepath.Join(root, ".claude", "commands", "slipway", "new.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(commandPath), 0o755))
		require.NoError(t, os.WriteFile(commandPath, []byte("user command"), 0o644))

		err := Generate(root, []string{"claude"}, true)
		require.Error(t, err)
		assert.ErrorContains(t, err, "refusing to overwrite unknown file")

		got, readErr := os.ReadFile(commandPath)
		require.NoError(t, readErr)
		assert.Equal(t, "user command", string(got))

		manifestPath := filepath.Join(root, generatedOwnershipManifestPath(cfg))
		_, statErr := os.Stat(manifestPath)
		assert.True(t, os.IsNotExist(statErr), "failed refresh must not create an ownership manifest")
	})

	t.Run("missing sentinel allows same content generated path collision", func(t *testing.T) {
		cfg := toolRegistry["claude"]

		canonicalRoot := t.TempDir()
		require.NoError(t, Generate(canonicalRoot, []string{"claude"}, true))
		relSkillPath := filepath.Join(".claude", "skills", "slipway", "SKILL.md")
		canonicalSkill, err := os.ReadFile(filepath.Join(canonicalRoot, relSkillPath))
		require.NoError(t, err)

		root := t.TempDir()
		skillPath := filepath.Join(root, relSkillPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
		require.NoError(t, os.WriteFile(skillPath, canonicalSkill, 0o644))

		require.NoError(t, Generate(root, []string{"claude"}, true))

		got, readErr := os.ReadFile(skillPath)
		require.NoError(t, readErr)
		assert.Equal(t, string(canonicalSkill), string(got))

		manifestPath := filepath.Join(root, generatedOwnershipManifestPath(cfg))
		_, statErr := os.Stat(manifestPath)
		assert.NoError(t, statErr, "same-content refresh should create ownership manifest")
	})
}

func TestCodexGenerationCreatesHandoffHooksConfigWithoutAgents(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"codex"}, true))

	_, err := os.Stat(filepath.Join(root, ".codex", "agents"))
	assert.True(t, os.IsNotExist(err), "codex should not generate exported agents")
	config, err := os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	require.NoError(t, err, "codex should generate project-local hook config")
	s := string(config)
	assert.Contains(t, s, "[[hooks.SessionStart]]")
	assert.Contains(t, s, `slipway hook session-start --tool codex`)
	assert.NotContains(t, s, "[[hooks.UserPromptSubmit]]")
	assert.NotContains(t, s, "context-pressure")
	assert.Contains(t, s, "inert until Codex trusts this repo and each hook")
	assert.Contains(t, s, "never edits global Codex trust")
}

func TestCodexGenerationFromLinkedWorktreeWritesRootCheckoutConfig(t *testing.T) {
	root := t.TempDir()
	runToolgenGit(t, root, "init")
	runToolgenGit(t, root, "config", "user.email", "slipway@example.invalid")
	runToolgenGit(t, root, "config", "user.name", "Slipway Test")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# repo\n"), 0o644))
	runToolgenGit(t, root, "add", "README.md")
	runToolgenGit(t, root, "commit", "-m", "initial")

	worktreeRoot := filepath.Join(t.TempDir(), "linked-worktree")
	runToolgenGit(t, root, "worktree", "add", worktreeRoot, "-b", "feature/codex-hooks")

	require.NoError(t, Generate(worktreeRoot, []string{"codex"}, true))

	rootConfig := filepath.Join(root, ".codex", "config.toml")
	config, err := os.ReadFile(rootConfig)
	require.NoError(t, err, "linked worktree generation should write Codex config in the root checkout")
	assert.Contains(t, string(config), "[[hooks.SessionStart]]")
	assert.Contains(t, string(config), `slipway hook session-start --tool codex`)

	_, err = os.Stat(filepath.Join(worktreeRoot, ".codex", "config.toml"))
	assert.True(t, os.IsNotExist(err), "linked worktree must not be the only Codex hook config location")
	assert.FileExists(t, filepath.Join(worktreeRoot, ".codex", "skills", "slipway-next", "SKILL.md"))
	assert.FileExists(t, filepath.Join(worktreeRoot, ".codex", "skills", "slipway-handoff", "SKILL.md"))
}

func runToolgenGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %s failed: %s", strings.Join(args, " "), string(out))
}

// TestHandoffCommandSurfaceIsGenerated locks that the restored handoff command
// ships a generated surface for both a skill-surface adapter (codex) and the
// command-file adapters: `slipway handoff` is a first-class command, not a
// retired one. Only the outside-the-loop context-pressure detection was removed
// on this branch; the handoff write/show mechanism is retained.
func TestHandoffCommandSurfaceIsGenerated(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"codex"}, true))
	assert.FileExists(t, filepath.Join(root, ".codex", "skills", "slipway-handoff", "SKILL.md"))

	for _, cfg := range commandSkillHostConfigs(t) {
		t.Run(cfg.ID, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, Generate(root, []string{cfg.ID}, true))
			handoffSkill := commandSkillPath(root, cfg, "handoff")
			assert.FileExists(t, handoffSkill, "%s must generate the handoff command surface", cfg.ID)
		})
	}
}

func TestHookSettingsRegistrationForSettingsCapableHosts(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"claude", "qwen"}, true))

	// Claude registers the inline session-start command directly in
	// settings.json, with no launcher path, no `.claude/hooks/` reference, no
	// `--tool` flag, and no shell operator. The context-pressure PostToolUse hook
	// was retired and must not appear.
	claudeSettings, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	require.NoError(t, err)
	claude := string(claudeSettings)
	assert.Contains(t, claude, "SessionStart")
	assert.Contains(t, claude, sessionStartHookCommand)
	assert.NotContains(t, claude, "PostToolUse", "claude must not register the retired context-pressure PostToolUse hook")
	assert.NotContains(t, claude, retiredContextPressureInlineCommand, "claude must not register the retired context-pressure command")
	assert.NotContains(t, claude, ".claude/hooks/", "claude settings must not reference a launcher path")
	assert.NotContains(t, claude, "slipway-session-start", "claude settings must not name a launcher file")
	assert.NotContains(t, claude, "slipway-context-pressure-post-tool-use", "claude settings must not name a launcher file")
	assert.NotContains(t, claude, "--tool", "claude settings must use the bare inline command")
	assert.NotContains(t, claude, "bash", "claude settings must not require bash")
	assert.NotContains(t, claude, "||", "claude settings must not require shell fallback operators")
	assert.NotContains(t, claude, "&&", "claude settings must not require shell fallback operators")

	// Qwen registers ONLY the inline session-start command (no PostToolUse).
	qwenSettings, err := os.ReadFile(filepath.Join(root, ".qwen", "settings.json"))
	require.NoError(t, err)
	qwen := string(qwenSettings)
	assert.Contains(t, qwen, "SessionStart")
	assert.Contains(t, qwen, sessionStartHookCommand)
	assert.NotContains(t, qwen, "PostToolUse")
	assert.NotContains(t, qwen, retiredContextPressureInlineCommand)
	assert.NotContains(t, qwen, ".qwen/hooks/", "qwen settings must not reference a launcher path")
	assert.NotContains(t, qwen, "slipway-session-start", "qwen settings must not name a launcher file")
	assert.NotContains(t, qwen, "--tool", "qwen settings must use the bare inline command")
	assert.NotContains(t, qwen, "bash", "qwen settings must not require bash")
	assert.NotContains(t, qwen, "||", "qwen settings must not require shell fallback operators")

	// Neither settings-capable host emits any launcher file (extensionless +
	// .ps1/.cmd/.sh) for the session-start hook, nor for the retired
	// context-pressure hook.
	for _, base := range []string{
		filepath.Join(".claude", "hooks", "slipway-session-start"),
		filepath.Join(".claude", "hooks", "slipway-context-pressure-post-tool-use"),
		filepath.Join(".qwen", "hooks", "slipway-session-start"),
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

	// Seed the full orphaned launcher family (extensionless + .ps1 + .cmd + .sh)
	// for both settings-capable hosts (claude, qwen), plus the same for the
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
		filepath.Join(root, ".qwen", "hooks", "slipway-session-start"),
		filepath.Join(root, ".qwen", "hooks", "slipway-session-start.ps1"),
		filepath.Join(root, ".qwen", "hooks", "slipway-session-start.cmd"),
		filepath.Join(root, ".qwen", "hooks", "slipway-session-start.sh"),
	}
	fileByPathSeeds := []string{
		filepath.Join(root, ".cursor", "hooks", "slipway-session-start.sh"),
		filepath.Join(root, ".opencode", "hooks", "slipway-session-start.sh"),
	}
	for _, p := range append(append([]string{}, settingsCapableOrphans...), fileByPathSeeds...) {
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755))
	}
	addOwnershipManifestFiles(t, root, toolRegistry["claude"], settingsCapableOrphans[:8]...)
	addOwnershipManifestFiles(t, root, toolRegistry["qwen"], settingsCapableOrphans[8:]...)

	require.NoError(t, Generate(root, []string{"claude", "qwen", "cursor", "opencode"}, true))

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

	initEntry, err := renderCommandEntry(toolRegistry["claude"], "init")
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
	assert.Contains(t, nextEntry, "has no `--auto`/`--no-auto` flags")
	assert.Contains(t, nextEntry, "never mutates pending preset confirmations")
	assert.Contains(t, normalizedNextEntry, "`evidence_continuation` with prior authorization sufficient")
	assert.Contains(t, normalizedNextEntry, "run/stage loops still stop for the host to run the skill or review and record evidence")
	for _, phrase := range []string{
		"`security-review` boundaries",
		"sensitive/guardrail confirmations",
		"the intake Approved Summary",
		"done finalization",
		"evidence gates",
	} {
		assert.Contains(t, normalizedNextEntry, phrase,
			"next command entry missing auto-mode redline phrase")
	}

	runEntry, err := renderCommandEntry(toolRegistry["claude"], "run")
	require.NoError(t, err)
	normalizedRunEntry := strings.Join(strings.Fields(runEntry), " ")
	runArguments := CommandArguments("run")
	assert.Contains(t, runArguments, "[--auto|--no-auto]",
		"run registry arguments must expose per-run auto overrides")
	assert.Contains(t, runEntry, "Shortcut driver for the current lifecycle stage")
	assert.Contains(t, runEntry, "`run` is an auto-driver shortcut")
	assert.Contains(t, runEntry, "JSON output includes `delegated_to`")
	assert.Contains(t, runEntry, "`run` reuses the same `next --json` contract")
	assert.Contains(t, normalizedRunEntry, "`--auto`/`--no-auto`: override `execution.auto` for this run",
		"run command entry must document the override behavior")
	assert.Contains(t, normalizedRunEntry, "Skill handoffs and review batches still stop the run loop for host work")
	assert.Contains(t, normalizedRunEntry, "`evidence_continuation` instead of `hard_stop`")
	assert.NotContains(t, runArguments, "--resume-response",
		"run registry arguments must not advertise the deleted checkpoint resume surface")
	for _, phrase := range []string{
		"`security-review` boundaries",
		"sensitive/guardrail confirmations",
		"the intake Approved Summary",
		"evidence gates",
	} {
		assert.Contains(t, normalizedRunEntry, phrase,
			"run command entry missing auto-mode redline phrase")
	}

	for _, commandID := range []string{"intake", "plan", "implement"} {
		commandID := commandID
		t.Run(commandID+"_auto_mode_note", func(t *testing.T) {
			t.Parallel()
			entry, renderErr := renderCommandEntry(toolRegistry["claude"], commandID)
			require.NoError(t, renderErr)
			assert.Contains(t, entry, "Config-level `execution.auto` applies to this stage command")
			assert.Contains(t, entry, "there are no per-stage")
			assert.Contains(t, entry, "`evidence_continuation`")
			assert.Contains(t, entry, "skill/review handoffs still stop the loop for host work")
			assert.Contains(t, entry, "sensitive/guardrail confirmations")
		})
	}
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
	normalizedReadme := strings.Join(strings.Fields(readme), " ")
	for _, phrase := range []string{
		"overridden per run with `slipway run --auto`",
		"`slipway run --no-auto` forces a single run back to manual pacing",
		"the per-run `--auto` / `--no-auto` overrides live only on `slipway run`",
		"Auto mode never relaxes governance.",
		"`security-review` boundaries",
		"sensitive/guardrail confirmations",
		"the intake Approved Summary",
		"every evidence gate",
	} {
		assert.Contains(t, normalizedReadme, phrase,
			"README auto-mode safety phrase missing")
	}
	assert.NotContains(t, readme, "request intake")
	assert.NotContains(t, readme, "active request resolution")
	assert.NotContains(t, readme, "`openspec/`")
	assert.NotContains(t, readme, "`openspec/`: change and spec artifacts used by governed workflows")

	assert.Equal(t, "Create a governed change with intake-first workflow", commandDescriptions["new"])
	assert.Equal(t, "Shortcut driver for the current lifecycle stage", commandDescriptions["run"])
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

	initSkill, err := os.ReadFile(codexCommandSkillPath(root, "init"))
	require.NoError(t, err)
	assert.Contains(t, string(initSkill), `tier: "setup"`, "setup command skill missing setup tier")

	codebaseMapSkill, err := os.ReadFile(codexCommandSkillPath(root, "codebase-map"))
	require.NoError(t, err)
	assert.Contains(t, string(codebaseMapSkill), `tier: "discovery"`, "discovery command skill missing discovery tier")
}

func TestGeneratedCommandEntriesIncludeClassMetadata(t *testing.T) {
	root := t.TempDir()

	require.NoError(t, Generate(root, []string{"claude", "cursor", "codex"}, true))

	claudeStatus, err := os.ReadFile(filepath.Join(root, ".claude", "commands", "slipway", "status.md"))
	require.NoError(t, err)
	assert.Contains(t, string(claudeStatus), `class: "query"`)

	cursorRun, err := os.ReadFile(filepath.Join(root, ".cursor", "commands", "slipway-run.md"))
	require.NoError(t, err)
	assert.Contains(t, string(cursorRun), `class: "mutation"`)

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

func TestCodexRefreshMaintainsManagedConfigTOML(t *testing.T) {
	root := t.TempDir()

	require.NoError(t, Generate(root, []string{"codex"}, true))
	configPath := filepath.Join(root, ".codex", "config.toml")
	content, err := os.ReadFile(configPath)
	require.NoError(t, err, "fresh generation should create Codex hook config")
	assert.Contains(t, string(content), "[[hooks.SessionStart]]")

	require.NoError(t, Generate(root, []string{"codex"}, true))
	content, err = os.ReadFile(configPath)
	require.NoError(t, err, "refresh should keep Codex hook config")
	assert.Equal(t, 1, strings.Count(string(content), codexHooksBlockStart), "refresh should replace, not duplicate, managed block")
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
	addOwnershipManifestFiles(t, root, toolRegistry["opencode"], filepath.Join(legacyDir, "new.md"))

	require.NoError(t, Generate(root, []string{"opencode"}, true))

	_, err := os.Stat(filepath.Join(root, ".opencode", "commands", "slipway-new.md"))
	assert.NoError(t, err, "refresh should write flat opencode command paths")
	_, err = os.Stat(filepath.Join(legacyDir, "new.md"))
	assert.True(t, os.IsNotExist(err), "refresh should prune legacy generated nested commands")
	_, err = os.Stat(filepath.Join(legacyDir, "custom.md"))
	assert.NoError(t, err, "refresh must not delete unknown nested user commands")
}

func TestOpenCodeRefreshWithoutOwnershipManifestPreservesLegacyNestedCommands(t *testing.T) {
	root := t.TempDir()
	writeGeneratedAdapterMarker(t, root, toolRegistry["opencode"])

	legacyDir := filepath.Join(root, ".opencode", "commands", "slipway")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "new.md"), []byte("legacy command without manifest proof"), 0o644))

	require.NoError(t, Generate(root, []string{"opencode"}, true))

	_, err := os.Stat(filepath.Join(root, ".opencode", "commands", "slipway-new.md"))
	assert.NoError(t, err, "refresh should write flat opencode command paths")
	got, err := os.ReadFile(filepath.Join(legacyDir, "new.md"))
	require.NoError(t, err, "marker-only refresh must preserve legacy nested commands without ownership proof")
	assert.Equal(t, "legacy command without manifest proof", string(got))
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

	// Verify refresh protection.
	require.NoError(t, os.WriteFile(skillPath, []byte("custom"), 0o644))
	require.NoError(t, Generate(root, []string{"codex"}, false))
	customContent, _ := os.ReadFile(skillPath)
	assert.Equal(t, "custom", string(customContent), "command skill should not be overwritten without refresh")

	// Verify refresh refuses managed-modified generated files.
	err = Generate(root, []string{"codex"}, true)
	require.Error(t, err)
	assert.ErrorContains(t, err, "managed-modified")
	refreshedContent, _ := os.ReadFile(skillPath)
	assert.Equal(t, "custom", string(refreshedContent), "command skill should not be overwritten with refresh")
}

func TestByteStabilityAllTools(t *testing.T) {
	toolIDs, err := ResolveTools("all")
	require.NoError(t, err)
	for _, toolID := range toolIDs {
		t.Run(toolID, func(t *testing.T) {
			root := t.TempDir()
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

func TestGeneratedSkillDescriptionsMatchCurrentRouting(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"claude"}, true))

	readSkill := func(id string) string {
		path := filepath.Join(root, ".claude", "skills", "slipway-"+id, "SKILL.md")
		content, err := os.ReadFile(path)
		require.NoError(t, err, "failed to read %s", id)
		return string(content)
	}
	frontmatterDescription := func(id string, content string) string {
		fm := extractAdapterFrontmatter(t, content, "claude", id)
		return fm["description"]
	}

	ship := readSkill("ship-verification")
	shipDescription := frontmatterDescription("ship-verification", ship)
	assert.Contains(t, shipDescription, "single terminal pre-ship verification gate",
		"ship-verification must be described as the single terminal pre-ship gate")
	assert.Contains(t, shipDescription, "Runs LAST in S3",
		"ship-verification must declare it runs last in S3 after the review peers")
	assert.NotContains(t, shipDescription, "selected S3 review peer",
		"ship-verification is the terminal gate, not a selected review peer")

	preflight := readSkill("worktree-preflight")
	assert.Contains(t, frontmatterDescription("worktree-preflight", preflight), "missing or unavailable early worktree binding",
		"worktree-preflight must describe the resolver-supported binding gap")
	assert.NotContains(t, frontmatterDescription("worktree-preflight", preflight), "invalid",
		"worktree-preflight description must not claim invalid-binding routing")

	security := readSkill("security-review")
	securityText := strings.Join(strings.Fields(security), " ")
	assert.Contains(t, frontmatterDescription("security-review", security), "blast-radius policy",
		"security-review must name the current selected-control route")
	assert.Contains(t, securityText, "review scope hints after this peer is selected",
		"security-review must distinguish path/surface hints from resolver triggers")
	assert.NotContains(t, security, "changes to auth/crypto/session paths",
		"security-review must not claim path-glob resolver triggers")

	gitRecovery := readSkill("git-recovery")
	assert.Contains(t, gitRecovery, "host-embedded worktree-preflight guidance",
		"git-recovery must keep the embedded preflight guidance route")
	assert.NotContains(t, gitRecovery, "git-state blockers",
		"git-recovery must not claim blocker-trigger routing")
	assert.NotContains(t, gitRecovery, "blocker_reason",
		"git-recovery must not expose unsupported blocker trigger metadata")

	wave := readSkill("wave-orchestration")
	assert.Contains(t, frontmatterDescription("wave-orchestration", wave), "parallel-wave fan-out",
		"wave-orchestration must describe parallel fan-out")
	assert.NotContains(t, frontmatterDescription("wave-orchestration", wave), "must be executed in sequence",
		"wave-orchestration description must not serialize all task execution")
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

func TestPiHooksExtensionRuntimeBehaviorWithNode(t *testing.T) {
	t.Parallel()

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable; skipping Pi TypeScript extension runtime check")
	}

	root := t.TempDir()
	probePath := filepath.Join(root, "probe.ts")
	require.NoError(t, os.WriteFile(
		probePath,
		[]byte(`export default function (value: { message?: string }) { return value.message ?? ""; }`+"\n"),
		0o644,
	))
	probe := exec.Command(node, "--check", probePath)
	if out, err := probe.CombinedOutput(); err != nil {
		t.Skipf("node --check cannot parse TypeScript in this environment: %s", out)
	}

	require.NoError(t, Generate(root, []string{"pi"}, true))
	extensionPath := filepath.Join(root, ".pi", "extensions", "slipway-hooks.ts")
	check := exec.Command(node, "--check", extensionPath)
	out, err := check.CombinedOutput()
	require.NoErrorf(t, err, "node --check %s failed:\n%s", extensionPath, out)

	harnessPath := filepath.Join(root, "pi-extension-harness.mjs")
	require.NoError(t, os.WriteFile(harnessPath, []byte(`
import { pathToFileURL } from "node:url";

const extensionPath = process.argv[2];
const payload = '<slipway-session-start tool="pi">ready</slipway-session-start>';
const handlers = new Map();
let execCalls = 0;
const pi = {
  on(name, handler) {
    handlers.set(name, handler);
  },
  async exec(_command, _args, _options) {
    execCalls += 1;
    return { stdout: execCalls === 1 ? "" : payload, stderr: "", code: 0, killed: false };
  },
};

const mod = await import(pathToFileURL(extensionPath).href);
mod.default(pi);

const sessionStart = handlers.get("session_start");
const beforeAgentStart = handlers.get("before_agent_start");
if (!sessionStart || !beforeAgentStart) {
  throw new Error("extension did not register required handlers");
}

sessionStart();
const failedFirstTurn = await beforeAgentStart({ systemPrompt: "base" }, { cwd: process.cwd() });
if (failedFirstTurn !== undefined) {
  throw new Error("empty first hook output must not inject a prompt");
}
if (execCalls !== 1) {
  throw new Error("expected one failed hook attempt, got " + execCalls);
}

const retriedSecondTurn = await beforeAgentStart({ systemPrompt: "base" }, { cwd: process.cwd() });
if (retriedSecondTurn?.systemPrompt !== "base\n\n" + payload) {
  throw new Error("second turn did not inject cached payload: " + retriedSecondTurn?.systemPrompt);
}
if (execCalls !== 2) {
  throw new Error("expected retry after empty hook output, got " + execCalls + " exec calls");
}

const idempotentThirdTurn = await beforeAgentStart(
  { systemPrompt: retriedSecondTurn.systemPrompt },
  { cwd: process.cwd() },
);
if (idempotentThirdTurn?.systemPrompt !== retriedSecondTurn.systemPrompt) {
  throw new Error("already-injected prompt must stay unchanged");
}
const count = idempotentThirdTurn.systemPrompt.split(payload).length - 1;
if (count !== 1) {
  throw new Error("expected one injected payload, got " + count);
}
if (execCalls !== 2) {
  throw new Error("cached successful payload should not rerun hook, got " + execCalls + " exec calls");
}

const objectSystemPromptTurn = await beforeAgentStart(
  { systemPrompt: { segments: ["base"] } },
  { cwd: process.cwd() },
);
if (objectSystemPromptTurn?.systemPrompt !== payload) {
  throw new Error("non-string systemPrompt must not be stringified: " + objectSystemPromptTurn?.systemPrompt);
}
if (execCalls !== 2) {
  throw new Error("non-string systemPrompt should use cached payload, got " + execCalls + " exec calls");
}
`), 0o644))
	harness := exec.Command(node, harnessPath, extensionPath)
	out, err = harness.CombinedOutput()
	require.NoErrorf(t, err, "Pi extension runtime harness failed:\n%s", out)
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

func commandSurfacePath(root string, cfg ToolConfig, id string) (string, string) {
	if cfg.CommandSkillSurface {
		rel := filepath.ToSlash(SkillPath(cfg, id))
		return rel, filepath.Join(root, filepath.FromSlash(rel))
	}

	rel := commandEntryRelPath(cfg, id)
	return rel, filepath.Join(root, filepath.FromSlash(rel))
}

func assertPiRegistrationSettings(t *testing.T, settingsPath string) {
	t.Helper()

	content, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "missing Pi settings file")

	settings := map[string]any{}
	require.NoError(t, json.Unmarshal(content, &settings))
	assert.Equal(t, true, settings["enableSkillCommands"])
	assertJSONSettingArrayContains(t, settings, "skills", "./skills")
	assertJSONSettingArrayContains(t, settings, "prompts", "./prompts")
	assert.NotContains(t, settings, "hooks", "Pi settings register skills/prompts, not hooks")
	assert.NotContains(t, string(content), "slipway hook", "Pi settings must not register hook commands")
}

func assertPiHooksExtension(t *testing.T, extensionPath string) {
	t.Helper()

	require.True(t, strings.HasSuffix(extensionPath, ".ts"),
		"Pi hooks extension must be a TypeScript module (pi loads .pi/extensions via jiti), got %s", extensionPath)

	content, err := os.ReadFile(extensionPath)
	require.NoError(t, err, "missing Pi hooks extension bridge file")

	ts := string(content)
	assert.Contains(t, ts, "Generated by Slipway", "Pi hooks extension must carry the generated-file header")
	assert.Contains(t, ts, "export default function", "Pi hooks extension must have a default export factory")
	assert.Contains(t, ts, `pi.on("session_start"`, "Pi hooks extension must register session_start")
	assert.Contains(t, ts, `pi.on("before_agent_start"`, "Pi hooks extension must register before_agent_start")

	// The session-start argv must be the exact hook invocation for this tool,
	// JSON-encoded (json.Marshal emits no spaces between elements).
	assert.Contains(t, ts, `"hook","session-start","--tool","pi"`,
		"Pi hooks extension must bridge the session-start hook for the pi tool")
	assert.Contains(t, ts, "pi.exec", "Pi hooks extension must shell out via pi.exec")

	// A hung or slow hook must not block the turn: exec is bounded by a timeout,
	// follows the before_agent_start abort signal, and killed/timed-out/non-zero
	// execs collapse to empty.
	assert.Contains(t, ts, "timeout: HOOK_TIMEOUT_MS", "Pi hooks extension exec must pass a bounded timeout")
	assert.Contains(t, ts, "signal: ctx.signal", "Pi hooks extension exec must follow the turn abort signal")
	assert.Contains(t, ts, "result.killed", "Pi hooks extension must treat a killed/timed-out exec as empty output")
	assert.Contains(t, ts, `typeof result.code === "number"`,
		"Pi hooks extension must only treat documented numeric exit codes as authoritative")
	assert.Contains(t, ts, "hasNumericExitCode && result.code !== 0",
		"Pi hooks extension must treat non-zero numeric exec exits as empty output")

	// Injection contract: session_start marks the session hook cache stale, then
	// before_agent_start retries until it gets a non-empty payload, caches that
	// content, and appends it to every per-turn system prompt. This keeps the
	// governance bridge visible after the first user prompt without storing
	// persistent messages that would duplicate on reload/resume.
	assert.Contains(t, ts, "sessionHookPending", "Pi hooks extension must track whether the session cache is stale")
	assert.Contains(t, ts, "sessionHookContent", "Pi hooks extension must cache the session-start hook output")
	assert.Contains(t, ts, "const content = await runHook(SESSION_ARGV, ctx);",
		"Pi hooks extension must retry from the session-start hook while the cache is stale")
	assert.Regexp(t, regexp.MustCompile(`if \(content\) \{\s+sessionHookContent = content;\s+sessionHookPending = false;\s+\}`), ts,
		"Pi hooks extension must clear the stale marker only after a non-empty hook output")
	assert.Contains(t, ts, "systemPrompt:", "Pi hooks extension must inject through the per-turn system prompt")
	assert.Contains(t, ts, "event.systemPrompt", "Pi hooks extension must preserve Pi's current system prompt")
	assert.Contains(t, ts, `typeof event.systemPrompt === "string"`,
		"Pi hooks extension must not stringify non-string system prompt shapes")
	assert.Contains(t, ts, "currentSystemPrompt.includes(sessionHookContent)",
		"Pi hooks extension must avoid appending duplicate governance blocks")
	assert.Contains(t, ts, "[currentSystemPrompt, sessionHookContent]",
		"Pi hooks extension must append cached session hook content on every agent start")
	assert.NotContains(t, ts, "event.systemPrompt?.includes(sessionHookContent)",
		"Pi hooks extension must guard the system prompt shape before duplicate checks")
	assert.NotContains(t, ts, "shouldInjectSessionHook",
		"Pi hooks extension must not gate system prompt injection to the first prompt only")
	assert.NotContains(t, ts, `customType: "slipway-hook"`, "Pi hooks extension must not store persistent custom messages")
	assert.NotContains(t, ts, "display: false", "Pi hooks extension must not rely on hidden persistent messages")

	// The context-pressure hook was retired entirely; the pi extension must carry
	// no context-pressure argv or stashed constant. Target the JSON argv fragment
	// and stashed constant, not prose.
	assert.NotContains(t, ts, `"hook","context-pressure"`,
		"Pi hooks extension must not carry a context-pressure argv")
	assert.NotContains(t, ts, "PRESSURE_ARGV",
		"Pi hooks extension must not stash a context-pressure argv constant")
}

func assertJSONSettingArrayContains(t *testing.T, settings map[string]any, name, want string) {
	t.Helper()

	raw, ok := settings[name].([]any)
	require.True(t, ok, "settings missing %s array", name)
	assert.Contains(t, raw, want, "settings %s must contain %q", name, want)
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
	assert.Contains(t, body, "compact executor result JSON")
	assert.Contains(t, body, "slipway evidence task --result-file")
	assert.NotContains(t, body, "with a valid task `--verdict`")
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
		{path: "skills/ci-triage/CATALOG_SKILL.md", contains: "slipway tool fetch-pr-checks"},
		{path: "skills/gha-security-review/CATALOG_SKILL.md", contains: "slipway tool pin-actions"},
		{path: "skills/review-comment-triage/CATALOG_SKILL.md", contains: "slipway tool fetch-pr-feedback"},
		{path: "skills/review-comment-triage/CATALOG_SKILL.md", contains: "slipway tool fetch-review-requests"},
		{path: "skills/review-comment-triage/CATALOG_SKILL.md", contains: "slipway tool reply-to-thread"},
		{path: "skills/root-cause-tracing/references/root-cause-tracing.md", contains: "slipway tool find-polluter-go"},
		{path: "skills/sast-orchestration/references/sarif-merge.md", contains: "slipway tool merge-sarif"},
		{path: "skills/variant-analysis/CATALOG_SKILL.md", contains: "slipway tool find-variant"},
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
	t.Run("kiro command skills use at-colon triggers", func(t *testing.T) {
		cfg, ok := LookupTool("kiro")
		require.True(t, ok)
		assert.Equal(t, "invoke command skills as @slipway:<command> or via host skill picker", cfg.InvocationSummary())
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

	require.NoError(t, mergeHookSettingsJSONWithPlan(root, cfg, true, nil))

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
	require.NoError(t, mergeHookSettingsJSONWithPlan(root, cfg, true, nil))
	second, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second))
}

// --- In-repo (dogfooding) go-run hook rendering ---

// writeSlipwayGoMod writes a go.mod into dir declaring the Slipway module path so
// hookLaunch detects dir as the in-repo source tree and renders go-run hooks.
func writeSlipwayGoMod(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module github.com/signalridge/slipway\n\ngo 1.26\n"), 0o644))
}

func TestModulePathFromGoMod(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"plain", "module github.com/signalridge/slipway\n\ngo 1.26\n", "github.com/signalridge/slipway"},
		{"leading tab", "\tmodule example.com/x\n", "example.com/x"},
		{"trailing comment", "module example.com/x // legacy\n", "example.com/x"},
		{"quoted", "module \"example.com/x\"\n", "example.com/x"},
		{"not first line", "// header\n\nmodule example.com/x\n", "example.com/x"},
		{"module-prefixed key ignored", "modulewide = 1\nmodule example.com/x\n", "example.com/x"},
		{"missing", "go 1.26\n", ""},
		{"empty", "", ""},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, modulePathFromGoMod([]byte(tc.in)))
		})
	}
}

func TestIsShellSafePath(t *testing.T) {
	t.Parallel()
	for _, p := range []string{
		"/Users/dev/ghq/github.com/signalridge/slipway",
		"/var/folders/qx/T/TestX_001",
		"/repo-1.2_x+y@z",
		"/home/RUNNER~1/repo", // 8.3-style short name (~) must stay launchable
	} {
		assert.Truef(t, isShellSafePath(p), "%q should be shell-safe", p)
	}
	for _, p := range []string{
		"",
		"/has space/repo",
		"/has;semicolon",
		"/has$var",
		"/has\"quote",
		"/has'quote",
		"/has(paren)",
		"/tab\there",
		"/has%var",   // cmd.exe %VAR% expansion
		"/has,comma", // PowerShell array operator
		"/has`tick",  // command substitution
		"/has^caret", // cmd.exe escape
	} {
		assert.Falsef(t, isShellSafePath(p), "%q should be shell-unsafe", p)
	}

	// The OS path separator must be accepted so in-repo go-run hooks render with
	// a native absolute path. On Windows that separator is the backslash; on a
	// POSIX host a backslash is the shell escape and must be rejected instead.
	if runtime.GOOS == "windows" {
		assert.True(t, isShellSafePath(`C:\Users\RUNNER~1\AppData\Local\Temp\T_001`),
			"a Windows absolute path must be shell-safe")
	} else {
		assert.False(t, isShellSafePath(`/has\backslash`),
			"a backslash is the shell escape on POSIX and must be rejected")
	}
}

func TestHookLaunchInRepoVsRelease(t *testing.T) {
	t.Parallel()

	t.Run("in-repo shell-safe path uses go run", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeSlipwayGoMod(t, root)
		abs, err := filepath.Abs(root)
		require.NoError(t, err)
		prefix, probe := hookLaunch(root)
		assert.Equal(t, "go -C "+abs+" run .", prefix)
		assert.Equal(t, "go", probe)
	})

	t.Run("foreign module falls back to bare slipway", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"),
			[]byte("module example.com/other\n"), 0o644))
		prefix, probe := hookLaunch(root)
		assert.Equal(t, "slipway", prefix)
		assert.Equal(t, "slipway", probe)
	})

	t.Run("no go.mod falls back to bare slipway", func(t *testing.T) {
		t.Parallel()
		prefix, probe := hookLaunch(t.TempDir())
		assert.Equal(t, "slipway", prefix)
		assert.Equal(t, "slipway", probe)
	})

	t.Run("unsafe module path falls back to bare slipway", func(t *testing.T) {
		t.Parallel()
		unsafe := filepath.Join(t.TempDir(), "has space")
		require.NoError(t, os.MkdirAll(unsafe, 0o755))
		writeSlipwayGoMod(t, unsafe)
		prefix, probe := hookLaunch(unsafe)
		assert.Equal(t, "slipway", prefix, "a shell-unsafe module root must not be embedded in a go-run command")
		assert.Equal(t, "slipway", probe)
	})
}

func TestStripSlipwayInvocation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		in     string
		want   string
		wantOK bool
	}{
		{"bare", "slipway hook session-start", "hook session-start", true},
		{"bare with flags", "slipway hook session-start --tool codex", "hook session-start --tool codex", true},
		{"absolute binary", "/usr/local/bin/slipway hook context-pressure", "hook context-pressure", true},
		{"go run", "go -C /repo run . hook session-start --tool codex", "hook session-start --tool codex", true},
		{"go run no flags", "go -C /repo run . hook context-pressure", "hook context-pressure", true},
		{"bare go run is user-owned, not a slipway launcher", "go run . hook session-start --local-mode", "", false},
		{"go run without -C is user-owned", "go run ./cmd/slipway hook session-start", "", false},
		{"go build is not a launcher", "go build ./...", "", false},
		{"bash -lc is not slipway", `bash -lc "echo .claude"`, "", false},
		{"empty", "", "", false},
		{"unrelated", "echo hello", "", false},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := stripSlipwayInvocation(tc.in)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIsStaleDirectHookCommandAcrossLaunchers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		command string
		current string
		want    bool
	}{
		{"release pruned when switching to go-run", "slipway hook session-start", "go -C /repo run . hook session-start", true},
		{"go-run pruned when switching to release", "go -C /old run . hook session-start", "slipway hook session-start", true},
		{"tool-flag variant pruned", `slipway hook session-start --tool "claude"`, "slipway hook session-start", true},
		{"shell-chained variant pruned", "slipway hook session-start || exit 0", "slipway hook session-start", true},
		{"different event kept", "slipway hook context-pressure", "slipway hook session-start", false},
		{"identical kept", "slipway hook session-start", "slipway hook session-start", false},
		{"non-slipway kept", "echo user-owned", "slipway hook session-start", false},
		{"user-authored go run kept", "go run . hook session-start --local-mode", "go -C /repo run . hook session-start", false},
		{"empty current kept", "slipway hook session-start", "", false},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isStaleDirectHookCommand(tc.command, tc.current))
		})
	}
}

func TestInRepoGenerationRendersGoRunHookCommands(t *testing.T) {
	root := t.TempDir()
	writeSlipwayGoMod(t, root)
	abs, err := filepath.Abs(root)
	require.NoError(t, err)
	require.NoError(t, Generate(root, []string{"codex", "claude", "cursor", "pi"}, true))

	// Codex inline TOML hooks launch the worktree source via go run. The command
	// is stored as a TOML basic string, so backslashes in a Windows path are
	// escaped (C:\dir -> C:\\dir); the assertion mirrors that encoding. On POSIX
	// abs has no backslashes, so this is a no-op.
	codexConfig, err := os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	require.NoError(t, err)
	codex := string(codexConfig)
	codexAbs := strings.ReplaceAll(abs, `\`, `\\`)
	assert.Contains(t, codex, "go -C "+codexAbs+" run . hook session-start --tool codex")
	assert.NotContains(t, codex, "context-pressure", "codex hooks must not carry the retired context-pressure command")
	assert.NotContains(t, codex, `"slipway hook`, "in-repo codex hooks must not embed the bare release command")

	// Claude inline settings.json hooks launch the worktree source via go run.
	claudeSettings := filepath.Join(root, ".claude", "settings.json")
	sessionCommands := hookCommandsForEvent(t, claudeSettings, "SessionStart")
	assert.Contains(t, sessionCommands, "go -C "+abs+" run . hook session-start")
	claudeRaw, err := os.ReadFile(claudeSettings)
	require.NoError(t, err)
	assert.NotContains(t, string(claudeRaw), "PostToolUse", "claude must not register the retired context-pressure PostToolUse hook")
	assert.NotContains(t, string(claudeRaw), "context-pressure", "claude must not register the retired context-pressure command")

	// Cursor launcher scripts probe `go` and dispatch through go run.
	launcher, err := os.ReadFile(filepath.Join(root, ".cursor", "hooks", "slipway-session-start"))
	require.NoError(t, err)
	l := string(launcher)
	assert.Contains(t, l, "command -v go >/dev/null 2>&1 || exit 0")
	assert.Contains(t, l, "go -C "+abs+` run . hook session-start --tool "cursor"`)

	// Pi uses a TypeScript extension instead of host hook settings or launcher
	// files. In a source checkout, its JSON argv must still dispatch through the
	// checked-out source tree and be tracked as managed generated content.
	piExtension, err := os.ReadFile(filepath.Join(root, ".pi", "extensions", "slipway-hooks.ts"))
	require.NoError(t, err)
	piArgv, err := json.Marshal([]string{"go", "-C", abs, "run", ".", "hook", "session-start", "--tool", "pi"})
	require.NoError(t, err)
	assert.Contains(t, string(piExtension), "const SESSION_ARGV = "+string(piArgv)+";")
	piManifest, found, err := loadOwnershipManifest(root, toolRegistry["pi"])
	require.NoError(t, err)
	require.True(t, found, "pi generation must write an ownership manifest")
	assert.Contains(t, piManifest.index(), ".pi/extensions/slipway-hooks.ts",
		"pi hook extension must be tracked as a managed generated file")
}

func TestNonRepoGenerationKeepsBareSlipwayHookCommands(t *testing.T) {
	root := t.TempDir() // no slipway go.mod -> release path
	require.NoError(t, Generate(root, []string{"codex", "claude"}, true))

	codexConfig, err := os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(codexConfig), "slipway hook session-start --tool codex")
	assert.NotContains(t, string(codexConfig), "go -C ")

	sessionCommands := hookCommandsForEvent(t, filepath.Join(root, ".claude", "settings.json"), "SessionStart")
	assert.Contains(t, sessionCommands, sessionStartHookCommand)
	assert.NotContains(t, strings.Join(sessionCommands, "\n"), "go -C ")
}

func TestInRepoRefreshDoesNotDuplicateAcrossDevReleaseSwitch(t *testing.T) {
	root := t.TempDir()
	writeSlipwayGoMod(t, root)
	abs, err := filepath.Abs(root)
	require.NoError(t, err)
	cfg := ToolConfig{
		ID:           "claude",
		SettingsPath: filepath.Join(".claude", "settings.json"),
		SessionEvent: "SessionStart",
		SessionHook:  filepath.Join(".claude", "hooks", "slipway-session-start"),
	}

	// A prior release install left the bare command; an in-repo refresh must
	// replace it with the go-run form and leave exactly one Slipway entry.
	seed := `{
	  "hooks": {
	    "SessionStart": [
	      {"hooks":[{"type":"command","command":"slipway hook session-start"}]},
	      {"matcher":"*","hooks":[{"type":"command","command":"echo user-owned-hook"}]}
	    ]
	  }
	}`
	settingsPath := filepath.Join(root, cfg.SettingsPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0o755))
	require.NoError(t, os.WriteFile(settingsPath, []byte(seed), 0o644))

	require.NoError(t, mergeHookSettingsJSONWithPlan(root, cfg, true, nil))

	commands := hookCommandsForEvent(t, settingsPath, "SessionStart")
	goRun := "go -C " + abs + " run . hook session-start"
	assert.Contains(t, commands, goRun)
	assert.NotContains(t, commands, "slipway hook session-start", "stale bare release command must be pruned")
	assert.Equal(t, 1, countCommandOccurrences(commands, goRun), "exactly one go-run session-start entry")
	assert.Contains(t, commands, "echo user-owned-hook", "user-authored hooks survive")

	// Switching back to a release install (no slipway go.mod) prunes the go-run
	// command and restores the bare form without leaving a duplicate.
	releaseRoot := t.TempDir()
	releaseSettings := filepath.Join(releaseRoot, cfg.SettingsPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(releaseSettings), 0o755))
	devOnRelease := fmt.Sprintf(`{
	  "hooks": {
	    "SessionStart": [
	      {"hooks":[{"type":"command","command":%q}]}
	    ]
	  }
	}`, goRun)
	require.NoError(t, os.WriteFile(releaseSettings, []byte(devOnRelease), 0o644))

	require.NoError(t, mergeHookSettingsJSONWithPlan(releaseRoot, cfg, true, nil))
	releaseCommands := hookCommandsForEvent(t, releaseSettings, "SessionStart")
	assert.Contains(t, releaseCommands, sessionStartHookCommand)
	assert.NotContains(t, strings.Join(releaseCommands, "\n"), "go -C ", "stale go-run command must be pruned on release")
	assert.Equal(t, 1, countCommandOccurrences(releaseCommands, sessionStartHookCommand))
}

// A user who hand-writes a `go run . hook ...` entry (a form Slipway never
// generates — it always emits `go -C <root> run .`) must keep it across an
// in-repo refresh: stale-hook pruning is scoped to Slipway-owned launcher forms.
func TestInRepoRefreshKeepsUserAuthoredBareGoRunHook(t *testing.T) {
	root := t.TempDir()
	writeSlipwayGoMod(t, root)
	abs, err := filepath.Abs(root)
	require.NoError(t, err)
	cfg := ToolConfig{
		ID:           "claude",
		SettingsPath: filepath.Join(".claude", "settings.json"),
		SessionEvent: "SessionStart",
		SessionHook:  filepath.Join(".claude", "hooks", "slipway-session-start"),
	}

	userHook := "go run . hook session-start --local-mode"
	seed := fmt.Sprintf(`{
	  "hooks": {
	    "SessionStart": [
	      {"hooks":[{"type":"command","command":"slipway hook session-start"}]},
	      {"matcher":"*","hooks":[{"type":"command","command":%q}]}
	    ]
	  }
	}`, userHook)
	settingsPath := filepath.Join(root, cfg.SettingsPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0o755))
	require.NoError(t, os.WriteFile(settingsPath, []byte(seed), 0o644))

	require.NoError(t, mergeHookSettingsJSONWithPlan(root, cfg, true, nil))

	commands := hookCommandsForEvent(t, settingsPath, "SessionStart")
	assert.Contains(t, commands, "go -C "+abs+" run . hook session-start", "the managed go-run entry is merged in")
	assert.NotContains(t, commands, "slipway hook session-start", "the stale bare release entry is pruned")
	assert.Contains(t, commands, userHook, "a user-authored bare `go run .` hook must survive refresh")
}
func countCommandOccurrences(items []string, target string) int {
	n := 0
	for _, item := range items {
		if item == target {
			n++
		}
	}
	return n
}
