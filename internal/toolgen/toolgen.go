package toolgen

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/capability"
	"github.com/signalridge/slipway/internal/tmpl"
)

// catalogManifestFileName is the outbound `using-slipway-catalog.md`
// document that describes the Go-owned capability registry to external
// agents. It sits alongside the generated SKILL.md directories under
// `<SkillsDir>/slipway/`.
const catalogManifestFileName = "using-slipway-catalog.md"

// ToolConfig describes a tool adapter target (Claude, Cursor, Codex, OpenCode, Gemini).
type ToolConfig struct {
	ID             string
	SkillsDir      string
	CommandsDir    string // "" = no project-local commands (Codex)
	CommandStyle   string // "nested", "flat", "" = no project-local commands
	CommandFormat  string // "md" (default), "toml" (Gemini)
	AgentStyle     string // "md" (default), "toml" (Codex), "" = no agents (Cursor)
	AgentsDir      string // "" = no agents (Cursor)
	PromptsStyle   string // "global" (Codex), "" = no global prompts
	SettingsPath   string
	SessionEvent   string
	SessionHook    string
	TriggerPrefix  string
	TriggerStyle   string
	AutoDetectPath []string
}

var toolRegistry = map[string]ToolConfig{
	"claude": {
		ID:            "claude",
		SkillsDir:     ".claude/skills",
		CommandsDir:   ".claude/commands",
		CommandStyle:  "nested",
		CommandFormat: "md",
		AgentStyle:    "md",
		AgentsDir:     ".claude/agents",
		PromptsStyle:  "",
		SettingsPath:  ".claude/settings.json",
		SessionEvent:  "SessionStart",
		SessionHook:   ".claude/hooks/slipway-session-start.sh",
		TriggerPrefix: "/slipway",
		TriggerStyle:  "slash-colon",
		AutoDetectPath: []string{
			".claude",
		},
	},
	"cursor": {
		ID:            "cursor",
		SkillsDir:     ".cursor/skills",
		CommandsDir:   ".cursor/commands",
		CommandStyle:  "flat",
		CommandFormat: "md",
		AgentStyle:    "",
		AgentsDir:     "",
		PromptsStyle:  "",
		SettingsPath:  "",
		SessionEvent:  "",
		SessionHook:   ".cursor/hooks/slipway-session-start.sh",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		AutoDetectPath: []string{
			".cursor",
		},
	},
	"codex": {
		ID:            "codex",
		SkillsDir:     ".codex/skills",
		CommandsDir:   "",
		CommandStyle:  "",
		CommandFormat: "",
		AgentStyle:    "toml",
		AgentsDir:     ".codex/agents",
		PromptsStyle:  "global",
		SettingsPath:  "",
		SessionEvent:  "",
		SessionHook:   "",
		TriggerPrefix: "$slipway-",
		TriggerStyle:  "dollar-mention",
		AutoDetectPath: []string{
			".codex",
		},
	},
	"opencode": {
		ID:            "opencode",
		SkillsDir:     ".opencode/skills",
		CommandsDir:   ".opencode/commands",
		CommandStyle:  "nested",
		CommandFormat: "md",
		AgentStyle:    "md",
		AgentsDir:     ".opencode/agents",
		PromptsStyle:  "",
		SettingsPath:  "",
		SessionEvent:  "",
		SessionHook:   ".opencode/hooks/slipway-session-start.sh",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		AutoDetectPath: []string{
			".opencode",
		},
	},
	"gemini": {
		ID:            "gemini",
		SkillsDir:     ".gemini/skills",
		CommandsDir:   ".gemini/commands",
		CommandStyle:  "nested",
		CommandFormat: "toml",
		AgentStyle:    "md",
		AgentsDir:     ".gemini/agents",
		PromptsStyle:  "",
		SettingsPath:  ".gemini/settings.json",
		SessionEvent:  "SessionStart",
		SessionHook:   ".gemini/hooks/slipway-session-start.sh",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		AutoDetectPath: []string{
			".gemini",
		},
	},
}

// CommandDef describes a single adapter command with all metadata consolidated.
type CommandClass string

const (
	CommandClassQuery    CommandClass = "query"
	CommandClassMutation CommandClass = "mutation"
)

type CommandDef struct {
	ID               string
	Class            CommandClass
	Description      string
	Arguments        string
	Prerequisites    []string
	Tier             string // "core" | "situational" | "diagnostics"
	HasPromptSurface bool   // true = generates inline command prompt surface; false for CLI-only commands
}

// commandRegistry is the single source of truth for adapter command metadata.
// Entry order is for readability; commandIDs() returns IDs sorted alphabetically.
var commandRegistry = []CommandDef{
	// Core (5)
	{ID: "new", Class: CommandClassMutation, Description: "Create a governed change with intake-first workflow", Tier: "core", HasPromptSurface: true,
		Arguments:     `"<description>" [--preset light|standard|strict] [--discuss] [--full] [--trivial] [--from-doc <path>] [--json]`,
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "No conflicting active change should already exist in the workspace."}},
	{ID: "next", Class: CommandClassQuery, Description: "Query next actionable skill (read-only, does not advance state)", Tier: "core", HasPromptSurface: true,
		Arguments: "[--json] [--context-guard] [--change <slug>]"},
	{ID: "run", Class: CommandClassMutation, Description: "Advance governed execution until a skill, blocker, checkpoint, or done-ready outcome is surfaced", Tier: "core", HasPromptSurface: true,
		Arguments: "[--json] [--resume] [--resume-response \"<text>\"] [--change <slug>]"},
	{ID: "status", Class: CommandClassQuery, Description: "Show lifecycle status, blockers, and next actions", Tier: "core", HasPromptSurface: true,
		Arguments:     "[--json] [--focus <alias>] [--list-focuses] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "Can be used with or without an active change."}},
	{ID: "done", Class: CommandClassMutation, Description: "Finalize a done-ready change and archive it", Tier: "core", HasPromptSurface: true,
		Arguments: "[--json] [--all-ready] [--change <slug>]"},
	// Situational (8)
	{ID: "init", Class: CommandClassMutation, Description: "Initialize runtime layout and optional tool artifacts", Tier: "situational", HasPromptSurface: true,
		Arguments:     "[--tools all|none|claude,cursor,...] [--refresh]",
		Prerequisites: []string{"Run from the target project root or any child directory inside it.", "The workspace must be inside a git working tree."}},
	{ID: "cancel", Class: CommandClassMutation, Description: "Cancel an active change and archive terminal state", Tier: "situational", HasPromptSurface: true,
		Arguments: "[--json] [--change <slug>]"},
	{ID: "review", Class: CommandClassMutation, Description: "Bidirectional artifact-code alignment review", Tier: "situational", HasPromptSurface: true,
		Arguments: "[--json] [--all|--changed-only] [--focus <alias>] [--list-focuses] [--change <slug>]"},
	{ID: "validate", Class: CommandClassQuery, Description: "Read-only evidence and gate check", Tier: "situational", HasPromptSurface: true,
		Arguments:     "[--json] [--focus <alias>] [--list-focuses] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "Can be used with or without an active change."}},
	{ID: "checkpoint", Class: CommandClassMutation, Description: "Set an active checkpoint to pause wave execution and request user input", Tier: "situational", HasPromptSurface: true,
		Arguments: "--task-id <id> [--type human_verify|decision|human_action] [--allowed-responses <value> ...] [--json] [--change <slug>]"},
	{ID: "preset", Class: CommandClassMutation, Description: "Confirm or override the active change workflow preset", Tier: "situational", HasPromptSurface: true,
		Arguments:     "<light|standard|strict> [--json] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "An active governed change should already exist, or pass `--change <slug>`."}},
	{ID: "pivot", Class: CommandClassMutation, Description: "Reroute or rescope an active change", Tier: "situational", HasPromptSurface: true,
		Arguments: "[--reroute|--rescope] [--json] [--change <slug>]"},
	{ID: "abort", Class: CommandClassMutation, Description: "Abort the active execution session without archiving the change", Tier: "situational", HasPromptSurface: true,
		Arguments: "[--json] [--change <slug>]"},
	{ID: "repair", Class: CommandClassMutation, Description: "Run safe local integrity and layout repairs", Tier: "situational", HasPromptSurface: true,
		Arguments:     "[--json] [--focus <alias>] [--list-focuses]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)"}},
	// Diagnostics (3) — CLI-only, no generated prompt surfaces
	{ID: "stats", Class: CommandClassQuery, Description: "Show repo-wide governance freshness and workflow statistics", Tier: "diagnostics",
		Arguments:     "[--json]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)"}},
	{ID: "health", Class: CommandClassQuery, Description: "Show repo-local integrity and repairability findings", Tier: "diagnostics",
		Arguments:     "[--json] [--governance] [--all] [--observations] [--doctor] [--focus <alias>] [--list-focuses] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)"}},
	{ID: "codebase-map", Class: CommandClassMutation, Description: "Create or refresh the durable repo-scoped codebase map", Tier: "diagnostics",
		Arguments:     "[--json]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)"}},
}

// commandRegistryMap provides O(1) lookup by command ID.
var commandRegistryMap = func() map[string]CommandDef {
	m := make(map[string]CommandDef, len(commandRegistry))
	for _, def := range commandRegistry {
		m[def.ID] = def
	}
	return m
}()

// CommandDescription returns the registry description for a command ID.
// Returns empty string if the command is not registered.
func CommandDescription(id string) string {
	if def, ok := commandRegistryMap[id]; ok {
		return def.Description
	}
	return ""
}

// CommandClassification returns the registry classification for a command ID.
// Returns empty string if the command is not registered.
func CommandClassification(id string) string {
	if def, ok := commandRegistryMap[id]; ok {
		return string(def.Class)
	}
	return ""
}

// promptSurfaceIDs returns IDs of commands that have generated prompt surfaces.
// It is derived from commandRegistry to keep the generated surfaces in sync.
var promptSurfaceIDs = func() []string {
	out := make([]string, 0, len(commandRegistry))
	for _, def := range commandRegistry {
		if def.HasPromptSurface {
			out = append(out, def.ID)
		}
	}
	return out
}()

// commandIDs returns the prompt surface IDs that have user-facing command entries (sorted).
func commandIDs() []string {
	out := make([]string, len(promptSurfaceIDs))
	copy(out, promptSurfaceIDs)
	slices.Sort(out)
	return out
}

// standaloneNames lists standalone skills (not governance, not technique) to generate.
var standaloneNames = []string{}

// techniqueNames lists the technique skills to generate.
var techniqueNames = []string{
	"tdd",
	"systematic-debugging",
	"code-review-protocol",
	"codebase-mapping",
}

// GovernanceSkillNames lists the governance skills generated for each tool (static .md).
var GovernanceSkillNames = []string{
	"intake-clarification",
	"research-orchestration",
	"plan-audit",
	"tdd-governance",
}

// standaloneGovernanceNames lists non-registry standalone skills still generated
// for adapter guidance. These are not governance-registry skills but provide
// useful procedural guidance for agents (e.g. worktree setup).
var standaloneGovernanceNames = []string{
	"worktree-preflight",
}

// TemplatedGovernanceSkillNames lists governance skills rendered from .md.tmpl.
// These support template partials via {{ template "partial-name" . }}.
var TemplatedGovernanceSkillNames = []string{
	"wave-orchestration",
	"spec-compliance-review",
	"code-quality-review",
	"goal-verification",
	"final-closeout",
}

// catalogSkillIDs returns Go-registry skill IDs sorted for deterministic
// generation and cleanup.
var catalogSkillIDs = func() []string {
	return capability.DefaultRegistry().IDs()
}()

// commandDescriptions returns the description for a command from the registry.
var commandDescriptions = func() map[string]string {
	m := make(map[string]string, len(commandRegistry))
	for _, def := range commandRegistry {
		m[def.ID] = def.Description
	}
	return m
}()

func commandPrerequisites(id string) []string {
	if def, ok := commandRegistryMap[id]; ok && len(def.Prerequisites) > 0 {
		return def.Prerequisites
	}
	// Default prerequisites for commands not explicitly configured.
	return []string{
		"`.slipway.yaml` must exist (run `slipway init` first)",
		"an active change must exist, or pass `--change <slug>` when supported.",
	}
}

// Registry returns all tool configs sorted by ID.
func Registry() []ToolConfig {
	out := make([]ToolConfig, 0, len(toolRegistry))
	for _, cfg := range toolRegistry {
		out = append(out, cfg)
	}
	slices.SortFunc(out, func(a, b ToolConfig) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return out
}

// ToolAmbiguityError is returned when multiple generated adapters exist and
// no SLIPWAY_TOOL override disambiguates the selection.
type ToolAmbiguityError struct {
	DetectedAdapters []string
}

func (e *ToolAmbiguityError) Error() string {
	return fmt.Sprintf("tool_ambiguity: multiple adapters detected [%s]; set SLIPWAY_TOOL=<tool> to disambiguate (e.g. SLIPWAY_TOOL=codex slipway next --json)",
		strings.Join(e.DetectedAdapters, ", "))
}

// ResolveWorkspaceTool selects the matching generated tool adapter for the
// current workspace. Selection order:
// 1. SLIPWAY_TOOL env override, when that adapter exists in the workspace
// 2. exactly one generated adapter in the workspace
// 3. fail closed with ToolAmbiguityError when multiple adapters exist
// 4. fail closed when no generated adapter exists
func ResolveWorkspaceTool(root string) (ToolConfig, error) {
	if override := strings.ToLower(strings.TrimSpace(os.Getenv("SLIPWAY_TOOL"))); override != "" {
		cfg, ok := toolRegistry[override]
		if !ok {
			return ToolConfig{}, fmt.Errorf("SLIPWAY_TOOL=%q: unknown tool adapter", override)
		}
		if !hasGeneratedAdapter(root, cfg) {
			return ToolConfig{}, fmt.Errorf("SLIPWAY_TOOL=%q: adapter not generated in workspace; %s", override, workspaceAdapterRemediation(root, cfg))
		}
		return cfg, nil
	}

	generated := make([]ToolConfig, 0, len(toolRegistry))
	for _, cfg := range Registry() {
		if hasGeneratedAdapter(root, cfg) {
			generated = append(generated, cfg)
		}
	}
	if len(generated) == 1 {
		return generated[0], nil
	}
	if len(generated) > 1 {
		ids := make([]string, 0, len(generated))
		for _, cfg := range generated {
			ids = append(ids, cfg.ID)
		}
		return ToolConfig{}, &ToolAmbiguityError{DetectedAdapters: ids}
	}
	return ToolConfig{}, missingWorkspaceAdapterError(root)
}

// LookupTool returns the ToolConfig for a given tool ID.
func LookupTool(id string) (ToolConfig, bool) {
	cfg, ok := toolRegistry[id]
	return cfg, ok
}

// HasSentinel returns true if the dedicated adapter sentinel exists for the tool.
func HasSentinel(root string, cfg ToolConfig) bool {
	return hasGeneratedAdapter(root, cfg)
}

// HasWorkspaceLocalSurfaces returns true if workspace-local Slipway-generated
// surfaces exist for the given tool (skill dirs under the tool root).
func HasWorkspaceLocalSurfaces(root string, cfg ToolConfig) bool {
	skillsRoot := filepath.Join(root, cfg.SkillsDir, "slipway")
	if _, err := os.Stat(skillsRoot); err == nil {
		return true
	}
	if cfg.CommandsDir != "" {
		var cmdDir string
		switch cfg.CommandStyle {
		case "flat":
			cmdDir = filepath.Join(root, cfg.CommandsDir)
		default:
			cmdDir = filepath.Join(root, cfg.CommandsDir, "slipway")
		}
		if entries, err := os.ReadDir(cmdDir); err == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), "slipway") || cfg.CommandStyle != "flat" {
					return true
				}
			}
		}
	}
	return false
}

// ResolveTools parses tool selection string into a list of tool IDs.
func ResolveTools(selection string) ([]string, error) {
	selection = strings.TrimSpace(selection)
	if selection == "" {
		return nil, nil
	}
	if strings.EqualFold(selection, "all") {
		return []string{"claude", "codex", "cursor", "gemini", "opencode"}, nil
	}
	if strings.EqualFold(selection, "none") {
		return nil, nil
	}

	parts := strings.Split(selection, ",")
	seen := map[string]struct{}{}
	tools := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		if _, ok := toolRegistry[name]; !ok {
			return nil, fmt.Errorf("unsupported tool %q", name)
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		tools = append(tools, name)
	}
	slices.Sort(tools)
	return tools, nil
}

// DetectExistingTools scans the workspace for previously generated slipway
// adapter surfaces and returns the list of tool IDs found. Used by init --refresh
// when --tools is not explicitly provided.
//
// Detection checks for the dedicated .adapter-generated sentinel rather than
// skill marker paths, so pre-sentinel workspaces are not auto-detected.
func DetectExistingTools(root string) []string {
	var found []string
	for _, cfg := range Registry() {
		marker := filepath.Join(root, GeneratedAdapterMarkerPath(cfg))
		if _, err := os.Stat(marker); err == nil {
			found = append(found, cfg.ID)
		}
	}
	slices.Sort(found)
	return found
}

// Generate creates tool prompt surfaces and commands for the given tools.
func Generate(root string, tools []string, refresh bool) error {
	for _, tool := range tools {
		cfg, ok := toolRegistry[tool]
		if !ok {
			return fmt.Errorf("unsupported tool %q", tool)
		}
		if err := generateForTool(root, cfg, refresh); err != nil {
			return err
		}
	}
	return nil
}

// ToolRootPath returns the single builtin adapter root from cfg.AutoDetectPath.
func ToolRootPath(cfg ToolConfig) string {
	if len(cfg.AutoDetectPath) == 0 {
		return ""
	}
	return cfg.AutoDetectPath[0]
}

// GeneratedAdapterMarkerPath returns the workspace-local sentinel path for a tool.
func GeneratedAdapterMarkerPath(cfg ToolConfig) string {
	return filepath.Join(ToolRootPath(cfg), "slipway", ".adapter-generated")
}

// SkillPath returns the relative path to a skill's SKILL.md for the given tool config.
func SkillPath(cfg ToolConfig, skillName string) string {
	return filepath.Join(cfg.SkillsDir, "slipway", skillName, "SKILL.md")
}

// CatalogManifestPath returns the relative path to the generated
// `using-slipway-catalog.md` outbound manifest for the given tool config.
// External agents read this file to triage catalog skills by description.
func CatalogManifestPath(cfg ToolConfig) string {
	return filepath.Join(cfg.SkillsDir, "slipway", catalogManifestFileName)
}

// AgentPath returns the relative path to an agent definition for the given tool config.
// Returns empty string for tools with no agent support (AgentStyle == "").
func AgentPath(cfg ToolConfig, agentName string) string {
	if cfg.AgentStyle == "" {
		return ""
	}
	ext := ".md"
	if cfg.AgentStyle == "toml" {
		ext = ".toml"
	}
	return filepath.Join(cfg.AgentsDir, agentName+ext)
}

func generateForTool(root string, cfg ToolConfig, refresh bool) error {
	sentinelPath := filepath.Join(root, GeneratedAdapterMarkerPath(cfg))

	if refresh {
		// Remove existing sentinel before the mutating pass.
		_ = os.Remove(sentinelPath)

		// Purge direct command prompt surfaces before rewrite (project-local only).
		if cfg.CommandsDir != "" {
			if err := purgeCommandPromptSurfaces(root, cfg); err != nil {
				return err
			}
		}

		if err := cleanupStaleGeneratedArtifacts(root, cfg); err != nil {
			return err
		}
	}

	// Governance skills (static content)
	// Includes both registry governance skills and standalone governance guidance skills.
	allStaticGovernance := append([]string{}, GovernanceSkillNames...)
	allStaticGovernance = append(allStaticGovernance, standaloneGovernanceNames...)
	for _, name := range allStaticGovernance {
		content, err := tmpl.Content(path.Join("skills", name, "SKILL.md"))
		if err != nil {
			return fmt.Errorf("load governance skill %q: %w", name, err)
		}
		skillPath := filepath.Join(root, SkillPath(cfg, name))
		if err := writeDeterministic(skillPath, content, refresh); err != nil {
			return err
		}
		if err := emitSkillSupportFiles(root, cfg, name, refresh); err != nil {
			return fmt.Errorf("emit support files for governance skill %q (%s): %w", name, cfg.ID, err)
		}
	}

	// Templated governance skills (tool-aware .md.tmpl)
	for _, name := range TemplatedGovernanceSkillNames {
		content, err := renderTemplatedGovernanceSkill(cfg, name)
		if err != nil {
			return fmt.Errorf("render templated governance skill %q for %s: %w", name, cfg.ID, err)
		}
		skillPath := filepath.Join(root, SkillPath(cfg, name))
		if err := writeDeterministic(skillPath, content, refresh); err != nil {
			return err
		}
		if err := emitSkillSupportFiles(root, cfg, name, refresh); err != nil {
			return fmt.Errorf("emit support files for templated governance skill %q (%s): %w", name, cfg.ID, err)
		}
	}

	// Catalog skills (registry-owned, assembled from SKILL.md plus optional
	// typed templates in fixed order).
	reg := capability.DefaultRegistry()
	for _, id := range catalogSkillIDs {
		sk, ok := reg.Lookup(id)
		if !ok {
			return fmt.Errorf("catalog skill %q missing from registry lookup", id)
		}
		content, err := renderCatalogSkill(sk)
		if err != nil {
			return fmt.Errorf("render catalog skill %q for %s: %w", id, cfg.ID, err)
		}
		skillPath := filepath.Join(root, SkillPath(cfg, id))
		if err := writeDeterministic(skillPath, content, refresh); err != nil {
			return err
		}
		if err := emitSkillSupportFiles(root, cfg, id, refresh); err != nil {
			return fmt.Errorf("emit support files for catalog skill %q (%s): %w", id, cfg.ID, err)
		}
	}

	// Standalone skills (static content, not governance, not technique)
	for _, name := range standaloneNames {
		content, err := tmpl.Content(path.Join("skills", name, "SKILL.md"))
		if err != nil {
			return fmt.Errorf("load standalone %q: %w", name, err)
		}
		p := filepath.Join(root, SkillPath(cfg, name))
		if err := writeDeterministic(p, content, refresh); err != nil {
			return err
		}
		if err := emitSkillSupportFiles(root, cfg, name, refresh); err != nil {
			return fmt.Errorf("emit support files for standalone skill %q (%s): %w", name, cfg.ID, err)
		}
	}

	// Technique skills (static content)
	for _, name := range techniqueNames {
		content, err := tmpl.Content(path.Join("skills", name, "SKILL.md"))
		if err != nil {
			return fmt.Errorf("load technique %q: %w", name, err)
		}
		p := filepath.Join(root, SkillPath(cfg, name))
		if err := writeDeterministic(p, content, refresh); err != nil {
			return err
		}
		if err := emitSkillSupportFiles(root, cfg, name, refresh); err != nil {
			return fmt.Errorf("emit support files for technique skill %q (%s): %w", name, cfg.ID, err)
		}
	}

	// Command prompt surfaces (inline command prompts for all adapter commands).
	// Skip when CommandsDir is empty (Codex uses global prompts instead).
	if cfg.CommandsDir != "" {
		ext := ".md"
		if cfg.CommandFormat == "toml" {
			ext = ".toml"
		}
		for _, id := range commandIDs() {
			content, err := renderCommandEntry(cfg, id)
			if err != nil {
				return fmt.Errorf("render command prompt %q for %s: %w", id, cfg.ID, err)
			}
			var p string
			switch cfg.CommandStyle {
			case "flat":
				p = filepath.Join(root, cfg.CommandsDir, "slipway-"+id+ext)
			default: // "nested"
				p = filepath.Join(root, cfg.CommandsDir, "slipway", id+ext)
			}
			if err := writeDeterministic(p, content, refresh); err != nil {
				return err
			}
		}
	}

	// Agent definitions — skip when AgentStyle is empty (Cursor).
	switch cfg.AgentStyle {
	case "md":
		for _, name := range tmpl.AgentNames() {
			content, err := tmpl.Content(path.Join("agents", name+".md"))
			if err != nil {
				return fmt.Errorf("load agent %q: %w", name, err)
			}
			p := filepath.Join(root, AgentPath(cfg, name))
			if err := writeDeterministic(p, content, refresh); err != nil {
				return err
			}
		}
	case "toml":
		if err := generateCodexAgents(root, cfg, refresh); err != nil {
			return err
		}
	default:
		// AgentStyle == "": skip agent generation entirely (Cursor).
	}

	// Global prompts (Codex: ~/.codex/prompts/) — writes outside project root.
	if cfg.PromptsStyle == "global" {
		if err := generateCodexPrompts(cfg, refresh); err != nil {
			return err
		}
		if refresh {
			// Codex prompts are host-global, so stale slipway-* entries are pruned
			// only after the current expected prompt set has been written
			// successfully.
			if err := cleanupStaleGlobalPrompts(cfg); err != nil {
				return err
			}
		}
	}

	// Session-start hook helper (hook-capable runtimes).
	if strings.TrimSpace(cfg.SessionHook) != "" {
		content, err := renderSessionHook(cfg)
		if err != nil {
			return fmt.Errorf("render session hook for %s: %w", cfg.ID, err)
		}
		p := filepath.Join(root, cfg.SessionHook)
		if err := writeDeterministic(p, content, refresh); err != nil {
			return err
		}
	}
	if strings.TrimSpace(cfg.SettingsPath) != "" {
		if err := mergeHookSettingsJSON(root, cfg, refresh); err != nil {
			return err
		}
	}

	// Outbound catalog manifest (read by external agents; not consumed by
	// the Slipway kernel). Regenerated deterministically from the Go-owned
	// capability registry so every adapter sees the same triage index.
	manifest := capability.BuildCatalogManifest(capability.DefaultRegistry())
	manifestPath := filepath.Join(root, CatalogManifestPath(cfg))
	if err := writeDeterministic(manifestPath, manifest, refresh); err != nil {
		return err
	}

	// Write sentinel last — a missing sentinel means the tree is invalid.
	if err := writeDeterministic(sentinelPath, "generated\n", true); err != nil {
		return err
	}
	return nil
}

// purgeCommandPromptSurfaces removes all expected command prompt files for
// project-local adapters before rewrite. This ensures a failed refresh
// cannot leave previously trusted prompt surfaces in place.
func purgeCommandPromptSurfaces(root string, cfg ToolConfig) error {
	if cfg.CommandsDir == "" {
		return nil
	}
	ext := ".md"
	if cfg.CommandFormat == "toml" {
		ext = ".toml"
	}
	for _, id := range commandIDs() {
		var p string
		switch cfg.CommandStyle {
		case "flat":
			p = filepath.Join(root, cfg.CommandsDir, "slipway-"+id+ext)
		default:
			p = filepath.Join(root, cfg.CommandsDir, "slipway", id+ext)
		}
		if err := removePathIfExists(p); err != nil {
			return err
		}
	}
	return nil
}

func cleanupStaleGeneratedArtifacts(root string, cfg ToolConfig) error {
	if err := cleanupStaleSkillDirs(root, cfg); err != nil {
		return err
	}
	if err := cleanupStaleCommandEntries(root, cfg); err != nil {
		return err
	}
	if err := cleanupStaleAgentFiles(root, cfg); err != nil {
		return err
	}
	return nil
}

func cleanupStaleSkillDirs(root string, cfg ToolConfig) error {
	skillsRoot := filepath.Join(root, cfg.SkillsDir, "slipway")
	expected := map[string]struct{}{}
	for _, names := range [][]string{
		GovernanceSkillNames,
		standaloneGovernanceNames,
		TemplatedGovernanceSkillNames,
		catalogSkillIDs,
		standaloneNames,
		techniqueNames,
	} {
		for _, name := range names {
			expected[name] = struct{}{}
		}
	}
	expected[catalogManifestFileName] = struct{}{}
	return cleanupUnexpectedEntries(skillsRoot, expected)
}

func cleanupStaleCommandEntries(root string, cfg ToolConfig) error {
	if cfg.CommandsDir == "" {
		return nil
	}

	ext := ".md"
	if cfg.CommandFormat == "toml" {
		ext = ".toml"
	}
	expected := map[string]struct{}{}
	for _, id := range commandIDs() {
		name := id + ext
		if cfg.CommandStyle == "flat" {
			name = "slipway-" + id + ext
		}
		expected[name] = struct{}{}
	}

	switch cfg.CommandStyle {
	case "flat":
		return cleanupPrefixedEntries(filepath.Join(root, cfg.CommandsDir), "slipway-", expected)
	default:
		return cleanupUnexpectedEntries(filepath.Join(root, cfg.CommandsDir, "slipway"), expected)
	}
}

func cleanupStaleAgentFiles(root string, cfg ToolConfig) error {
	if cfg.AgentStyle == "" || cfg.AgentsDir == "" {
		return nil
	}

	ext := ".md"
	if cfg.AgentStyle == "toml" {
		ext = ".toml"
	}
	expected := map[string]struct{}{}
	for _, name := range tmpl.AgentNames() {
		expected[name+ext] = struct{}{}
	}
	return cleanupPrefixedEntries(filepath.Join(root, cfg.AgentsDir), "slipway-", expected)
}

func cleanupStaleGlobalPrompts(cfg ToolConfig) error {
	if cfg.PromptsStyle != "global" {
		return nil
	}

	promptsDir, err := codexPromptsDir()
	if err != nil {
		return err
	}
	expected := map[string]struct{}{}
	for _, id := range commandIDs() {
		expected["slipway-"+id+".md"] = struct{}{}
	}
	return cleanupPrefixedEntries(promptsDir, "slipway-", expected)
}

func cleanupUnexpectedEntries(dir string, expected map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if _, ok := expected[entry.Name()]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func cleanupPrefixedEntries(dir, prefix string, expected map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if _, ok := expected[name]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// renderCatalogSkill assembles a registry-owned catalog SKILL.md from the
// source body and optional typed templates in fixed order:
// SKILL.md body -> PROSE.tmpl -> CHECKLIST.tmpl -> VERDICT.tmpl.
//
// The assembled output rewrites the authoring-side frontmatter so adapter
// skill loaders (Codex, Claude) see the required `name` and `description`
// fields. Internal fields (skill_id, summary, bindings, hydrate_references,
// ...) are preserved below them so audit and binding-compare gates still
// work.
func renderCatalogSkill(sk capability.Skill) (string, error) {
	base, err := tmpl.Content(path.Join("skills", sk.ID, "SKILL.md"))
	if err != nil {
		return "", fmt.Errorf("load catalog base body: %w", err)
	}
	base, err = injectAdapterFrontmatter(base, sk)
	if err != nil {
		return "", fmt.Errorf("rewrite frontmatter for %q: %w", sk.ID, err)
	}

	// Typed partials are assembled whenever authored on disk; attachment-mode
	// gating lives in the capability layer, not in the assembler.
	prose, err := loadOptionalTemplate(path.Join("skills", sk.ID, "PROSE.tmpl"), true)
	if err != nil {
		return "", err
	}
	checklist, err := loadOptionalTemplate(path.Join("skills", sk.ID, "CHECKLIST.tmpl"), true)
	if err != nil {
		return "", err
	}
	verdict, err := loadOptionalTemplate(path.Join("skills", sk.ID, "VERDICT.tmpl"), true)
	if err != nil {
		return "", err
	}

	sections := []string{
		trimCatalogSection(base),
		trimCatalogSection(prose),
		trimCatalogSection(checklist),
		trimCatalogSection(verdict),
	}
	out := make([]string, 0, len(sections))
	for _, section := range sections {
		if section == "" {
			continue
		}
		out = append(out, section)
	}
	if len(out) == 1 {
		// Preserve exact SKILL.md content when no typed-template section exists.
		return base, nil
	}
	return strings.Join(out, "\n\n") + "\n", nil
}

// injectAdapterFrontmatter prepends `name` and `description` to the source
// frontmatter so adapter loaders (Codex/Claude) accept the output. The
// existing authoring fields are preserved verbatim below them.
//
// The function is string-based on purpose: it keeps the body byte-for-byte
// identical to the source (important for tier-size and schema-lint gates
// that measure post-frontmatter bytes).
func injectAdapterFrontmatter(raw string, sk capability.Skill) (string, error) {
	const open = "---\n"
	if !strings.HasPrefix(raw, open) {
		return "", fmt.Errorf("SKILL.md missing opening `---` delimiter")
	}
	rest := raw[len(open):]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", fmt.Errorf("SKILL.md missing closing `---` delimiter")
	}
	fm := rest[:idx]
	tail := rest[idx:] // starts with "\n---"

	header := "name: slipway-" + sk.ID + "\n" +
		"description: " + yamlDoubleQuoted(sk.Summary) + "\n"
	return open + header + fm + tail, nil
}

// yamlDoubleQuoted renders s as a YAML double-quoted scalar. It escapes `\`
// and `"`; other printable ASCII, including backticks, are safe inside
// double-quoted YAML strings.
func yamlDoubleQuoted(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

var optionalSkillSupportDirs = []string{"references", "scripts"}

// emitSkillSupportFiles copies optional support artifacts (`references/`,
// `scripts/`) next to a generated skill. Skills with no support payload are a
// silent no-op. Refresh mode also sweeps stale copies so catalog drops clean
// up their previous output.
func emitSkillSupportFiles(root string, cfg ToolConfig, skillID string, refresh bool) error {
	skillDirRel := filepath.Dir(SkillPath(cfg, skillID))
	dstBase := filepath.Join(root, skillDirRel)
	return emitSkillSupportFilesFromFS(tmpl.TemplateFS(), skillID, dstBase, refresh)
}

// emitSkillSupportFilesFromFS is the testable core: it sources support files
// from an arbitrary fs.FS rooted like tmpl.TemplateFS() (so paths begin with
// "skills/<id>/...") and writes them under dstBase on the local filesystem.
func emitSkillSupportFilesFromFS(srcFS fs.FS, skillID, dstBase string, refresh bool) error {
	if refresh {
		// Sweep legacy provenance.yaml files so an older generated tree
		// cleans up after the knowledge-only cleanup.
		if err := removePathIfExists(filepath.Join(dstBase, "provenance.yaml")); err != nil {
			return err
		}
	}

	for _, sub := range optionalSkillSupportDirs {
		dstDir := filepath.Join(dstBase, sub)
		if refresh {
			if err := removePathIfExists(dstDir); err != nil {
				return err
			}
		}
		if err := copyTemplateSubtreeFromFS(srcFS, path.Join("skills", skillID, sub), dstDir, refresh); err != nil {
			return fmt.Errorf("copy %s for %q: %w", sub, skillID, err)
		}
	}
	return nil
}

func removePathIfExists(name string) error {
	err := os.RemoveAll(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// copyTemplateSubtreeFromFS walks an embedded template directory and writes each
// file to dstDir preserving relative paths. Missing source directories are
// a no-op.
func copyTemplateSubtreeFromFS(tfs fs.FS, srcPrefix, dstDir string, refresh bool) error {
	info, err := fs.Stat(tfs, srcPrefix)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("expected directory at %q", srcPrefix)
	}
	return fs.WalkDir(tfs, srcPrefix, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if shouldSkipSupportArtifact(path.Base(p), true) {
				return fs.SkipDir
			}
			return nil
		}
		if shouldSkipSupportArtifact(path.Base(p), false) {
			return nil
		}
		rel, err := filepath.Rel(srcPrefix, p)
		if err != nil {
			return err
		}
		content, err := fs.ReadFile(tfs, p)
		if err != nil {
			return err
		}
		return writeDeterministic(filepath.Join(dstDir, filepath.FromSlash(rel)), string(content), refresh)
	})
}

func shouldSkipSupportArtifact(name string, isDir bool) bool {
	if isDir {
		return name == "__pycache__"
	}
	return strings.HasSuffix(name, ".pyc") || strings.HasSuffix(name, ".pyo")
}

func loadOptionalTemplate(name string, needed bool) (string, error) {
	if !needed {
		return "", nil
	}
	content, exists, err := tmpl.ContentIfExists(name)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", nil
	}
	return content, nil
}

func trimCatalogSection(section string) string {
	return strings.Trim(section, "\n")
}

// renderTemplatedGovernanceSkill renders a templated governance SKILL.md from a .tmpl template.
func renderTemplatedGovernanceSkill(cfg ToolConfig, id string) (string, error) {
	data := map[string]string{
		"ToolID":      cfg.ID,
		"Trigger":     commandTrigger(cfg, id),
		"Description": commandDescriptions[id],
	}
	return tmpl.Render(path.Join("skills", id, "SKILL.md.tmpl"), data)
}

// renderCommandEntry renders an inline command prompt from the appropriate template.
func renderCommandEntry(cfg ToolConfig, id string) (string, error) {
	tier := "situational"
	if def, ok := commandRegistryMap[id]; ok {
		tier = def.Tier
	}
	data := map[string]any{
		"CommandID":     id,
		"ToolID":        cfg.ID,
		"Trigger":       commandTrigger(cfg, id),
		"Class":         CommandClassification(id),
		"Description":   commandDescriptions[id],
		"BodyTemplate":  "command-" + id + "-body",
		"Arguments":     commandArguments(id),
		"Prerequisites": commandPrerequisites(id),
		"Tier":          tier,
		"Surface":       "adapter",
	}
	tmplName := "command-entry.md.tmpl"
	if cfg.CommandFormat == "toml" {
		tmplName = "command-entry.toml.tmpl"
	}
	return tmpl.Render(path.Join("commands", tmplName), data)
}

func renderSessionHook(cfg ToolConfig) (string, error) {
	data := map[string]string{
		"ToolID": cfg.ID,
	}
	return tmpl.Render(path.Join("hooks", "session-start.sh.tmpl"), data)
}

func writeDeterministic(path, content string, refresh bool) error {
	if !refresh {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if strings.HasSuffix(path, ".sh") {
		mode = 0o755
	}
	return os.WriteFile(path, []byte(content), mode)
}

func mergeHookSettingsJSON(root string, cfg ToolConfig, refresh bool) error {
	settingsPath := filepath.Join(root, cfg.SettingsPath)
	if !refresh {
		if _, err := os.Stat(settingsPath); err == nil {
			return nil
		}
	}

	settings := map[string]any{}
	existing, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(existing, &settings); err != nil {
			return fmt.Errorf("parse %s: %w", cfg.SettingsPath, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	var hooks map[string]any
	switch existingHooks := settings["hooks"].(type) {
	case nil:
		hooks = map[string]any{}
	case map[string]any:
		hooks = existingHooks
	default:
		return fmt.Errorf("%s contains a non-object hooks field", cfg.SettingsPath)
	}

	if strings.TrimSpace(cfg.SessionEvent) != "" && strings.TrimSpace(cfg.SessionHook) != "" {
		mergeHookEventCommand(hooks, cfg.SessionEvent, fmt.Sprintf(`bash "%s"`, filepath.ToSlash(cfg.SessionHook)))
	}
	removeLegacyHookCommands(hooks, []string{"PostToolUse", "AfterTool"}, "slipway-context-monitor")
	settings["hooks"] = hooks

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(settingsPath, content, 0o644)
}

func mergeHookEventCommand(hooks map[string]any, eventName, command string) {
	rawEntries, ok := hooks[eventName]
	if !ok {
		hooks[eventName] = []any{
			map[string]any{
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": command,
					},
				},
			},
		}
		return
	}

	entries, ok := rawEntries.([]any)
	if !ok {
		hooks[eventName] = []any{
			map[string]any{
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": command,
					},
				},
			},
		}
		return
	}

	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hookList, ok := entryMap["hooks"].([]any)
		if !ok {
			continue
		}
		for _, hook := range hookList {
			hookMap, ok := hook.(map[string]any)
			if !ok {
				continue
			}
			if hookMap["command"] == command {
				return
			}
		}
	}

	hooks[eventName] = append(entries, map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
			},
		},
	})
}

func removeLegacyHookCommands(hooks map[string]any, eventNames []string, substring string) {
	for _, eventName := range eventNames {
		rawEntries, ok := hooks[eventName]
		if !ok {
			continue
		}
		entries, ok := rawEntries.([]any)
		if !ok {
			continue
		}
		var kept []any
		for _, entry := range entries {
			entryMap, ok := entry.(map[string]any)
			if !ok {
				kept = append(kept, entry)
				continue
			}
			hookList, ok := entryMap["hooks"].([]any)
			if !ok {
				kept = append(kept, entry)
				continue
			}
			var filteredHooks []any
			for _, hook := range hookList {
				hookMap, ok := hook.(map[string]any)
				if !ok {
					filteredHooks = append(filteredHooks, hook)
					continue
				}
				cmd, _ := hookMap["command"].(string)
				if !strings.Contains(cmd, substring) {
					filteredHooks = append(filteredHooks, hook)
				}
			}
			if len(filteredHooks) > 0 {
				entryMap["hooks"] = filteredHooks
				kept = append(kept, entry)
			}
		}
		if len(kept) == 0 {
			delete(hooks, eventName)
		} else {
			hooks[eventName] = kept
		}
	}
}

func commandTrigger(cfg ToolConfig, commandID string) string {
	if cfg.TriggerStyle == "slash-colon" {
		return fmt.Sprintf("%s:%s", cfg.TriggerPrefix, commandID)
	}
	return fmt.Sprintf("%s%s", cfg.TriggerPrefix, commandID)
}

func commandArguments(id string) string {
	if def, ok := commandRegistryMap[id]; ok && def.Arguments != "" {
		return def.Arguments
	}
	return "[--json]"
}

func hasGeneratedAdapter(root string, cfg ToolConfig) bool {
	p := filepath.Join(root, GeneratedAdapterMarkerPath(cfg))
	if _, err := os.Stat(p); err == nil {
		return true
	}
	return false
}

func workspaceAdapterRemediation(root string, cfg ToolConfig) string {
	if HasWorkspaceLocalSurfaces(root, cfg) && !hasGeneratedAdapter(root, cfg) {
		return fmt.Sprintf("run `slipway init --tools %s --refresh`", cfg.ID)
	}
	return fmt.Sprintf("run `slipway init --tools %s`", cfg.ID)
}

func missingWorkspaceAdapterError(root string) error {
	dirtyTools := make([]string, 0, len(toolRegistry))
	for _, cfg := range Registry() {
		if HasWorkspaceLocalSurfaces(root, cfg) && !hasGeneratedAdapter(root, cfg) {
			dirtyTools = append(dirtyTools, cfg.ID)
		}
	}
	slices.Sort(dirtyTools)
	switch len(dirtyTools) {
	case 0:
		return fmt.Errorf("no generated tool adapter found in workspace; run `slipway init --tools <tool>`")
	case 1:
		toolID := dirtyTools[0]
		return fmt.Errorf("no generated tool adapter found in workspace; workspace has existing Slipway surfaces for %s without a sentinel; run `slipway init --tools %s --refresh`", toolID, toolID)
	default:
		return fmt.Errorf("no generated tool adapter found in workspace; workspace has existing Slipway surfaces without sentinels for [%s]; rerun with `slipway init --tools <tool> --refresh`", strings.Join(dirtyTools, ", "))
	}
}

// codexAgentSandboxMode returns the sandbox_mode for a Codex agent TOML file.
func codexAgentSandboxMode(name string) string {
	switch name {
	case "slipway-reviewer", "slipway-auditor", "slipway-verifier":
		return "read-only"
	default:
		return "workspace-write"
	}
}

// parseAgentFrontmatter extracts description from agent .md template frontmatter.
// Supports two closing forms:
//   - "\n---\n" (standard: closing delimiter followed by body content)
//   - "\n---" at EOF (frontmatter-only file with no trailing newline)
//
// The "\n---\n" form is checked first via strings.Index, so a file ending with
// "\n---\n" (trailing newline after delimiter) is handled by the standard path,
// not the EOF branch.
func parseAgentFrontmatter(content string) (description string, status string, body string) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", "", content
	}
	rest := content[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		// Closing --- at EOF with no trailing newline — frontmatter-only file.
		if strings.HasSuffix(rest, "\n---") {
			end = len(rest) - 4 // position before "\n---"
			fm := rest[:end]
			return extractDescription(fm), extractAgentStatus(fm), ""
		}
		return "", "", content
	}
	fm := rest[:end]
	body = rest[end+5:] // skip past closing "\n---\n"

	return extractDescription(fm), extractAgentStatus(fm), body
}

func extractDescription(fm string) string {
	return extractFrontmatterValue(fm, "description")
}

func extractAgentStatus(fm string) string {
	return extractFrontmatterValue(fm, "agent_status")
}

func extractFrontmatterValue(fm, key string) string {
	for _, line := range strings.Split(fm, "\n") {
		prefix := key + ":"
		if strings.HasPrefix(line, prefix) {
			desc := strings.TrimPrefix(line, prefix)
			desc = strings.TrimSpace(desc)
			desc = strings.Trim(desc, `"`)
			return desc
		}
	}
	return ""
}

func agentStatusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case "manual_only":
		return "manual-only helper"
	case "governance_mapped":
		return "governance-mapped"
	default:
		return ""
	}
}

func codexAgentDescription(description, status string) string {
	label := agentStatusLabel(status)
	if label == "" {
		return description
	}
	if strings.TrimSpace(description) == "" {
		return label
	}
	return fmt.Sprintf("%s (%s)", description, label)
}

// generateCodexAgents generates .codex/agents/<name>.toml files and registers
// them in .codex/config.toml.
func generateCodexAgents(root string, cfg ToolConfig, refresh bool) error {
	agentsDir := filepath.Join(root, cfg.AgentsDir)

	var entries []codexAgentEntry

	for _, name := range tmpl.AgentNames() {
		content, err := tmpl.Content(path.Join("agents", name+".md"))
		if err != nil {
			return fmt.Errorf("load agent %q: %w", name, err)
		}

		description, status, body := parseAgentFrontmatter(content)
		sandbox := codexAgentSandboxMode(name)

		body = strings.TrimSpace(body)
		if label := agentStatusLabel(status); label != "" {
			body = strings.TrimSpace("Agent status: " + label + ".\n\n" + body)
		}
		// TOML multi-line basic strings: escape backslashes and break any
		// triple-quote sequences that would prematurely close the string.
		body = strings.ReplaceAll(body, `\`, `\\`)
		body = strings.ReplaceAll(body, `"""`, `""\"`)

		tomlContent := fmt.Sprintf("sandbox_mode = %q\ndeveloper_instructions = \"\"\"\n%s\n\"\"\"\n", sandbox, body)

		path := filepath.Join(agentsDir, name+".toml")
		if err := writeDeterministic(path, tomlContent, refresh); err != nil {
			return err
		}

		entries = append(entries, codexAgentEntry{Name: name, Description: codexAgentDescription(description, status)})
	}

	return mergeCodexConfigTOML(root, entries, refresh)
}

const (
	codexMarkerBegin = "# BEGIN slipway agents"
	codexMarkerEnd   = "# END slipway agents"
)

type codexAgentEntry struct {
	Name        string
	Description string
}

// mergeCodexConfigTOML writes/updates [agents.slipway-*] sections in .codex/config.toml
// using marker comments for idempotency.
func mergeCodexConfigTOML(root string, entries []codexAgentEntry, refresh bool) error {
	configPath := filepath.Join(root, ".codex", "config.toml")

	// Build the managed section.
	var managed strings.Builder
	managed.WriteString(codexMarkerBegin + "\n")
	for _, e := range entries {
		_, _ = fmt.Fprintf(&managed, "\n[agents.%s]\n", e.Name)
		_, _ = fmt.Fprintf(&managed, "description = %q\n", e.Description)
		_, _ = fmt.Fprintf(&managed, "config_file = %q\n", "agents/"+e.Name+".toml")
	}
	managed.WriteString("\n" + codexMarkerEnd + "\n")

	existing, err := os.ReadFile(configPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		// File doesn't exist — create with managed section (even without refresh).
		if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(configPath, []byte(managed.String()), 0o644)
	}

	content := string(existing)
	beginIdx := strings.Index(content, codexMarkerBegin)
	endIdx := strings.Index(content, codexMarkerEnd)

	if beginIdx >= 0 && endIdx >= 0 {
		if !refresh {
			// Managed section already exists and refresh not requested — skip.
			return nil
		}
		// Replace existing managed section.
		endIdx += len(codexMarkerEnd)
		// Include trailing newline if present.
		if endIdx < len(content) && content[endIdx] == '\n' {
			endIdx++
		}
		newContent := content[:beginIdx] + managed.String() + content[endIdx:]
		return os.WriteFile(configPath, []byte(newContent), 0o644)
	}

	// Partial marker — one present without the other. Warn and bail out to
	// avoid corrupting the file with a duplicate marker block.
	if beginIdx >= 0 || endIdx >= 0 {
		return fmt.Errorf("config.toml has incomplete slipway markers (BEGIN=%v, END=%v); fix or remove the markers manually", beginIdx >= 0, endIdx >= 0)
	}

	// No markers found — append managed section (even without refresh,
	// since this is first-time registration into an existing file).
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n" + managed.String()
	return os.WriteFile(configPath, []byte(content), 0o644)
}

// codexPromptsDir resolves the Codex global prompts directory.
// Uses $CODEX_HOME/prompts/ if set, otherwise ~/.codex/prompts/.
func codexPromptsDir() (string, error) {
	if home := os.Getenv("CODEX_HOME"); home != "" {
		return filepath.Join(home, "prompts"), nil
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(userHome, ".codex", "prompts"), nil
}

// generateCodexPrompts renders and writes global prompt files to ~/.codex/prompts/
// (or $CODEX_HOME/prompts/ when set). Note: this writes outside the project root
// to the user's home directory.
func generateCodexPrompts(cfg ToolConfig, refresh bool) error {
	promptsDir, err := codexPromptsDir()
	if err != nil {
		return err
	}

	for _, id := range commandIDs() {
		tier := "situational"
		if def, ok := commandRegistryMap[id]; ok {
			tier = def.Tier
		}
		data := map[string]any{
			"CommandID":     id,
			"ToolID":        cfg.ID,
			"Trigger":       commandTrigger(cfg, id),
			"Class":         CommandClassification(id),
			"Description":   commandDescriptions[id],
			"BodyTemplate":  "command-" + id + "-body",
			"Arguments":     commandArguments(id),
			"Prerequisites": commandPrerequisites(id),
			"Tier":          tier,
			"Surface":       "adapter",
		}
		content, err := tmpl.Render(path.Join("commands", "command-entry.codex-prompt.md.tmpl"), data)
		if err != nil {
			return fmt.Errorf("render codex prompt %q: %w", id, err)
		}
		path := filepath.Join(promptsDir, "slipway-"+id+".md")
		if err := writeDeterministic(path, content, refresh); err != nil {
			return err
		}
	}
	return nil
}
