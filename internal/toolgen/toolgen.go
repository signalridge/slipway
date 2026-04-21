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
	"gopkg.in/yaml.v3"
)

// catalogManifestFileName is the outbound `using-slipway-catalog.md`
// document that describes the Go-owned capability registry to external
// agents. It sits at the top level of `<SkillsDir>/`.
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
	// Situational (9)
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

type governanceRenderMode string

const (
	governanceRenderStatic     governanceRenderMode = "static"
	governanceRenderTemplated  governanceRenderMode = "templated"
	governanceRenderStandalone governanceRenderMode = "standalone"
)

type governanceSurfaceDescriptor struct {
	ID              string
	RenderMode      governanceRenderMode
	WorkflowOwned   bool
	ExportOnlyExtra bool
}

// governanceSurfaceDescriptors is the single toolgen-owned source of truth
// for exported governance surfaces. It keeps workflow-governance ownership
// distinct from extra exported helpers while preserving render-mode details.
var governanceSurfaceDescriptors = []governanceSurfaceDescriptor{
	{ID: "intake-clarification", RenderMode: governanceRenderStatic, WorkflowOwned: true},
	{ID: "research-orchestration", RenderMode: governanceRenderStatic, WorkflowOwned: true},
	{ID: "plan-audit", RenderMode: governanceRenderStatic, WorkflowOwned: true},
	{ID: "tdd-governance", RenderMode: governanceRenderStatic, ExportOnlyExtra: true},
	{ID: "worktree-preflight", RenderMode: governanceRenderStandalone, ExportOnlyExtra: true},
	{ID: "wave-orchestration", RenderMode: governanceRenderTemplated, WorkflowOwned: true},
	{ID: "spec-compliance-review", RenderMode: governanceRenderTemplated, WorkflowOwned: true},
	{ID: "code-quality-review", RenderMode: governanceRenderTemplated, WorkflowOwned: true},
	{ID: "goal-verification", RenderMode: governanceRenderTemplated, WorkflowOwned: true},
	{ID: "final-closeout", RenderMode: governanceRenderTemplated, WorkflowOwned: true},
}

func governanceSurfaceIDs(filter func(governanceSurfaceDescriptor) bool) []string {
	out := make([]string, 0, len(governanceSurfaceDescriptors))
	for _, desc := range governanceSurfaceDescriptors {
		if filter(desc) {
			out = append(out, desc.ID)
		}
	}
	return out
}

func governanceSurfaceIDsByRenderMode(mode governanceRenderMode) []string {
	return governanceSurfaceIDs(func(desc governanceSurfaceDescriptor) bool {
		return desc.RenderMode == mode
	})
}

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

const workflowSkillID = "workflow"

const workflowEntryPublicName = "slipway"

// standaloneNames lists standalone skills (not governance, not technique) to generate.
var standaloneNames = []string{workflowSkillID}

var workflowLifecycleCommandIDs = []string{"new", "status", "next", "run", "done"}

var workflowSupportingCommandIDs = []string{
	"review",
	"validate",
	"repair",
	"init",
	"cancel",
	"checkpoint",
	"preset",
	"pivot",
	"abort",
}

var workflowDiagnosticCommandIDs = []string{"stats", "health", "codebase-map"}

type workflowSkillData struct {
	ToolID              string
	PublicName          string
	CatalogManifestPath string
	CommandsDir         string
	LifecycleCommands   []commandEntry
	SupportingCommands  []commandEntry
	DiagnosticCommands  []commandEntry
}

type commandEntry struct {
	Name          string
	Description   string
	Arguments     string
	Prerequisites []string
}

// techniqueNames lists the technique skills to generate.
var techniqueNames = []string{
	"tdd",
	"code-review-protocol",
	"codebase-mapping",
	"coding-discipline",
}

// GovernanceSkillNames lists the static exported governance surfaces (.md).
var GovernanceSkillNames = governanceSurfaceIDsByRenderMode(governanceRenderStatic)

// workflowGovernanceNames lists the workflow-state-owned governance hosts.
var workflowGovernanceNames = governanceSurfaceIDs(func(desc governanceSurfaceDescriptor) bool {
	return desc.WorkflowOwned
})

// extraExportedGovernanceNames lists exported helpers that are intentionally
// outside the workflow-governance registry.
var extraExportedGovernanceNames = governanceSurfaceIDs(func(desc governanceSurfaceDescriptor) bool {
	return desc.ExportOnlyExtra
})

// standaloneGovernanceNames lists standalone exported governance helpers.
var standaloneGovernanceNames = governanceSurfaceIDsByRenderMode(governanceRenderStandalone)

// TemplatedGovernanceSkillNames lists governance surfaces rendered from .md.tmpl.
var TemplatedGovernanceSkillNames = governanceSurfaceIDsByRenderMode(governanceRenderTemplated)

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

type commandRenderData struct {
	ID            string
	Class         CommandClass
	Tier          string
	Description   string
	Arguments     string
	Prerequisites []string
}

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

func buildCommandRenderData(id string) (commandRenderData, error) {
	def, ok := commandRegistryMap[id]
	if !ok {
		return commandRenderData{}, fmt.Errorf("command %q missing from command registry", id)
	}
	return commandRenderData{
		ID:            id,
		Class:         def.Class,
		Tier:          def.Tier,
		Description:   commandDescriptions[id],
		Arguments:     commandArguments(id),
		Prerequisites: commandPrerequisites(id),
	}, nil
}

func buildWorkflowCommandEntries(ids []string, expectedTier string) ([]commandEntry, error) {
	out := make([]commandEntry, 0, len(ids))
	for _, id := range ids {
		meta, err := buildCommandRenderData(id)
		if err != nil {
			return nil, err
		}
		if meta.Tier != expectedTier {
			return nil, fmt.Errorf("workflow command %q expected tier %q, got %q", id, expectedTier, meta.Tier)
		}
		out = append(out, commandEntry{
			Name:          meta.ID,
			Description:   meta.Description,
			Arguments:     meta.Arguments,
			Prerequisites: meta.Prerequisites,
		})
	}
	return out, nil
}

func validateWorkflowCommandCoverage(groups ...[]string) error {
	grouped := make(map[string]struct{}, len(commandRegistry))
	for _, ids := range groups {
		for _, id := range ids {
			if _, exists := grouped[id]; exists {
				return fmt.Errorf("workflow command %q declared more than once", id)
			}
			grouped[id] = struct{}{}
		}
	}

	for _, def := range commandRegistry {
		if _, ok := grouped[def.ID]; !ok {
			return fmt.Errorf("workflow command groups missing registry command %q", def.ID)
		}
	}
	if len(grouped) != len(commandRegistry) {
		return fmt.Errorf("workflow command groups cover %d commands, want %d", len(grouped), len(commandRegistry))
	}
	return nil
}

func buildWorkflowSkillData(cfg ToolConfig) (workflowSkillData, error) {
	if err := validateWorkflowCommandCoverage(
		workflowLifecycleCommandIDs,
		workflowSupportingCommandIDs,
		workflowDiagnosticCommandIDs,
	); err != nil {
		return workflowSkillData{}, err
	}

	lifecycle, err := buildWorkflowCommandEntries(workflowLifecycleCommandIDs, "core")
	if err != nil {
		return workflowSkillData{}, err
	}
	supporting, err := buildWorkflowCommandEntries(workflowSupportingCommandIDs, "situational")
	if err != nil {
		return workflowSkillData{}, err
	}
	diagnostics, err := buildWorkflowCommandEntries(workflowDiagnosticCommandIDs, "diagnostics")
	if err != nil {
		return workflowSkillData{}, err
	}

	return workflowSkillData{
		ToolID:              cfg.ID,
		PublicName:          adapterSkillName("workflow"),
		CatalogManifestPath: filepath.ToSlash(CatalogManifestPath(cfg)),
		CommandsDir:         filepath.ToSlash(cfg.CommandsDir),
		LifecycleCommands:   lifecycle,
		SupportingCommands:  supporting,
		DiagnosticCommands:  diagnostics,
	}, nil
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
	skillsRoot := filepath.Join(root, cfg.SkillsDir)
	if entries, err := os.ReadDir(skillsRoot); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, "slipway-") || name == catalogManifestFileName {
				return true
			}
		}
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

func adapterSkillName(id string) string {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == workflowSkillID {
		return workflowEntryPublicName
	}
	return "slipway-" + trimmedID
}

func AdapterSkillName(id string) string {
	return adapterSkillName(id)
}

func exportedSkillDirName(id string) string {
	return adapterSkillName(id)
}

func sourceSkillTemplatePath(id, leaf string) string {
	return path.Join("skills", id, leaf)
}

// SkillPath returns the relative path to a skill's SKILL.md for the given tool config.
func SkillPath(cfg ToolConfig, skillName string) string {
	return filepath.Join(cfg.SkillsDir, exportedSkillDirName(skillName), "SKILL.md")
}

// CatalogManifestPath returns the relative path to the generated
// `using-slipway-catalog.md` outbound manifest for the given tool config.
// External agents read this file to triage catalog skills by description.
func CatalogManifestPath(cfg ToolConfig) string {
	return filepath.Join(cfg.SkillsDir, catalogManifestFileName)
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
		content, err := tmpl.Content(sourceSkillTemplatePath(name, "SKILL.md"))
		if err != nil {
			return fmt.Errorf("load governance skill %q: %w", name, err)
		}
		content, err = renderSourceManagedSkill(content, name)
		if err != nil {
			return fmt.Errorf("canonicalize governance skill %q: %w", name, err)
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
		if name == "workflow" {
			content, err := renderStandaloneWorkflowSkill(cfg)
			if err != nil {
				return fmt.Errorf("render standalone %q for %s: %w", name, cfg.ID, err)
			}
			skillPath := filepath.Join(root, SkillPath(cfg, name))
			if err := writeDeterministic(skillPath, content, refresh); err != nil {
				return err
			}
			if err := emitSkillSupportFiles(root, cfg, name, refresh); err != nil {
				return fmt.Errorf("emit support files for standalone skill %q (%s): %w", name, cfg.ID, err)
			}
			refContent, err := renderStandaloneWorkflowCommandReference(cfg)
			if err != nil {
				return fmt.Errorf("render standalone workflow reference for %s: %w", cfg.ID, err)
			}
			refPath := filepath.Join(
				root,
				filepath.Dir(SkillPath(cfg, name)),
				"references",
				"command-reference.md",
			)
			if err := writeDeterministic(refPath, refContent, refresh); err != nil {
				return err
			}
			continue
		}

		content, err := tmpl.Content(sourceSkillTemplatePath(name, "SKILL.md"))
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
		content, err := tmpl.Content(sourceSkillTemplatePath(name, "SKILL.md"))
		if err != nil {
			return fmt.Errorf("load technique %q: %w", name, err)
		}
		content, err = renderSourceManagedSkill(content, name)
		if err != nil {
			return fmt.Errorf("canonicalize technique %q: %w", name, err)
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
	if cfg.ID == "codex" {
		if err := cleanupManagedCodexAgentBlock(root); err != nil {
			return err
		}
	}
	return nil
}

func cleanupStaleSkillDirs(root string, cfg ToolConfig) error {
	skillsRoot := filepath.Join(root, cfg.SkillsDir)
	if err := removePathIfExists(filepath.Join(skillsRoot, "slipway")); err != nil {
		return err
	}

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
			expected[exportedSkillDirName(name)] = struct{}{}
		}
	}
	expected[catalogManifestFileName] = struct{}{}

	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := expected[name]; ok {
			continue
		}
		if name == catalogManifestFileName || strings.HasPrefix(name, "slipway-") {
			if err := os.RemoveAll(filepath.Join(skillsRoot, name)); err != nil {
				return err
			}
		}
	}
	return nil
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
	if cfg.AgentsDir == "" {
		return nil
	}
	return cleanupPrefixedEntries(filepath.Join(root, cfg.AgentsDir), "slipway-", nil)
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
		if expected != nil {
			if _, ok := expected[name]; ok {
				continue
			}
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
	base, err := tmpl.Content(sourceSkillTemplatePath(sk.ID, "SKILL.md"))
	if err != nil {
		return "", fmt.Errorf("load catalog base body: %w", err)
	}
	base, err = injectAdapterFrontmatter(base, adapterSkillName(sk.ID), sk.Summary)
	if err != nil {
		return "", fmt.Errorf("rewrite frontmatter for %q: %w", sk.ID, err)
	}

	// Typed partials are assembled whenever authored on disk; attachment-mode
	// gating lives in the capability layer, not in the assembler.
	prose, err := loadOptionalTemplate(sourceSkillTemplatePath(sk.ID, "PROSE.tmpl"), true)
	if err != nil {
		return "", err
	}
	checklist, err := loadOptionalTemplate(sourceSkillTemplatePath(sk.ID, "CHECKLIST.tmpl"), true)
	if err != nil {
		return "", err
	}
	verdict, err := loadOptionalTemplate(sourceSkillTemplatePath(sk.ID, "VERDICT.tmpl"), true)
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

func renderSourceManagedSkill(raw, id string) (string, error) {
	description, stripped, err := extractAndStripAdapterFields(raw, id)
	if err != nil {
		return "", err
	}
	return injectAdapterFrontmatter(stripped, adapterSkillName(id), description)
}

// injectAdapterFrontmatter prepends `name` and `description` to the source
// frontmatter so adapter loaders (Codex/Claude) accept the output. The
// existing authoring fields are preserved verbatim below them.
//
// The function is string-based on purpose: it keeps the body byte-for-byte
// identical to the source (important for tier-size and schema-lint gates
// that measure post-frontmatter bytes).
func injectAdapterFrontmatter(raw, publicName, description string) (string, error) {
	fm, tail, err := splitSkillFrontmatter(raw)
	if err != nil {
		return "", err
	}

	header := "name: " + publicName + "\n" +
		"description: " + yamlDoubleQuoted(description) + "\n"
	return "---\n" + header + fm + tail, nil
}

func extractAndStripAdapterFields(raw, id string) (description string, stripped string, err error) {
	fm, tail, err := splitSkillFrontmatter(raw)
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(fm, "\n")
	kept := make([]string, 0, len(lines))
	var skillID string
	var publicName string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "skill_id:"):
			skillID, err = parseSingleLineFrontmatterScalar(strings.TrimPrefix(line, "skill_id:"), "skill_id", id)
			if err != nil {
				return "", "", err
			}
			kept = append(kept, line)
		case strings.HasPrefix(line, "name:"):
			publicName, err = parseSingleLineFrontmatterScalar(strings.TrimPrefix(line, "name:"), "name", id)
			if err != nil {
				return "", "", err
			}
		case strings.HasPrefix(line, "description:"):
			description, err = parseSingleLineFrontmatterScalar(strings.TrimPrefix(line, "description:"), "description", id)
			if err != nil {
				return "", "", err
			}
		default:
			kept = append(kept, line)
		}
	}

	if skillID == "" {
		return "", "", fmt.Errorf("skill %q missing skill_id", id)
	}
	if skillID != id {
		return "", "", fmt.Errorf("skill %q has unexpected skill_id %q", id, skillID)
	}
	if description == "" {
		return "", "", fmt.Errorf("skill %q missing description", id)
	}
	if publicName != "" && publicName != adapterSkillName(id) {
		return "", "", fmt.Errorf("skill %q name must equal %q", id, adapterSkillName(id))
	}

	return description, "---\n" + strings.Join(kept, "\n") + tail, nil
}

func splitSkillFrontmatter(raw string) (fm string, tail string, err error) {
	const open = "---\n"
	if !strings.HasPrefix(raw, open) {
		return "", "", fmt.Errorf("SKILL.md missing opening `---` delimiter")
	}
	rest := raw[len(open):]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", "", fmt.Errorf("SKILL.md missing closing `---` delimiter")
	}
	return rest[:idx], rest[idx:], nil
}

func parseSingleLineFrontmatterScalar(raw, field, id string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "|") || strings.HasPrefix(raw, ">") {
		return "", fmt.Errorf("skill %q %s must use a single-line YAML scalar", id, field)
	}

	var decoded struct {
		Value string `yaml:"value"`
	}
	if err := yaml.Unmarshal([]byte("value: "+raw+"\n"), &decoded); err != nil {
		return "", fmt.Errorf("parse %s for %q: %w", field, id, err)
	}
	return strings.TrimSpace(decoded.Value), nil
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
		if err := emitSharedSkillSupportFromFS(srcFS, skillID, sub, dstDir, refresh); err != nil {
			return fmt.Errorf("copy shared %s for %q: %w", sub, skillID, err)
		}
		if err := copyTemplateSubtreeFromFS(srcFS, sourceSkillTemplatePath(skillID, sub), dstDir, refresh); err != nil {
			return fmt.Errorf("copy %s for %q: %w", sub, skillID, err)
		}
	}
	return nil
}

func emitSharedSkillSupportFromFS(srcFS fs.FS, skillID, sub, dstDir string, refresh bool) error {
	if sub != "scripts" {
		return nil
	}
	usesSharedHelper, err := skillUsesSharedScriptHelper(srcFS, skillID)
	if err != nil {
		return err
	}
	if !usesSharedHelper {
		return nil
	}
	return copyTemplateSubtreeFromFS(srcFS, path.Join("skills", "_shared", sub), dstDir, refresh)
}

func skillUsesSharedScriptHelper(srcFS fs.FS, skillID string) (bool, error) {
	scriptsDir := sourceSkillTemplatePath(skillID, "scripts")
	info, err := fs.Stat(srcFS, scriptsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("expected directory at %q", scriptsDir)
	}

	sharedHelperReferenced := errors.New("shared helper referenced")
	err = fs.WalkDir(srcFS, scriptsDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if shouldSkipSupportArtifact(path.Base(p), true) {
				return fs.SkipDir
			}
			return nil
		}
		if shouldSkipSupportArtifact(path.Base(p), false) || path.Ext(p) != ".sh" {
			return nil
		}
		content, err := fs.ReadFile(srcFS, p)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), "gh-common.sh") {
			return sharedHelperReferenced
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sharedHelperReferenced) {
			return true, nil
		}
		return false, err
	}
	return false, nil
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
	raw, err := tmpl.Render(sourceSkillTemplatePath(id, "SKILL.md.tmpl"), data)
	if err != nil {
		return "", err
	}
	return renderSourceManagedSkill(raw, id)
}

func renderStandaloneWorkflowSkill(cfg ToolConfig) (string, error) {
	data, err := buildWorkflowSkillData(cfg)
	if err != nil {
		return "", err
	}
	raw, err := tmpl.Render(sourceSkillTemplatePath("workflow", "SKILL.md.tmpl"), data)
	if err != nil {
		return "", err
	}
	return renderSourceManagedSkill(raw, "workflow")
}

func renderStandaloneWorkflowCommandReference(cfg ToolConfig) (string, error) {
	data, err := buildWorkflowSkillData(cfg)
	if err != nil {
		return "", err
	}
	return tmpl.Render(sourceSkillTemplatePath("workflow", "command-reference.md.tmpl"), data)
}

// renderCommandEntry renders an inline command prompt from the appropriate template.
func renderCommandEntry(cfg ToolConfig, id string) (string, error) {
	meta, err := buildCommandRenderData(id)
	if err != nil {
		return "", err
	}
	data := map[string]any{
		"CommandID":     meta.ID,
		"ToolID":        cfg.ID,
		"Trigger":       commandTrigger(cfg, meta.ID),
		"Class":         meta.Class,
		"Description":   meta.Description,
		"BodyTemplate":  "command-" + id + "-body",
		"Arguments":     meta.Arguments,
		"Prerequisites": meta.Prerequisites,
		"Tier":          meta.Tier,
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

const (
	codexMarkerBegin = "# BEGIN slipway agents"
	codexMarkerEnd   = "# END slipway agents"
)

func cleanupManagedCodexAgentBlock(root string) error {
	configPath := filepath.Join(root, ".codex", "config.toml")
	existing, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	content := string(existing)
	beginIdx := strings.Index(content, codexMarkerBegin)
	endIdx := strings.Index(content, codexMarkerEnd)

	if beginIdx < 0 && endIdx < 0 {
		return nil
	}
	if beginIdx < 0 || endIdx < 0 {
		return fmt.Errorf("config.toml has incomplete slipway markers (BEGIN=%v, END=%v); fix or remove the markers manually", beginIdx >= 0, endIdx >= 0)
	}

	endIdx += len(codexMarkerEnd)
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}
	newContent := content[:beginIdx] + content[endIdx:]
	newContent = strings.TrimLeft(newContent, "\n")
	if strings.TrimSpace(newContent) == "" {
		return os.WriteFile(configPath, []byte(""), 0o644)
	}
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	return os.WriteFile(configPath, []byte(newContent), 0o644)
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
		meta, err := buildCommandRenderData(id)
		if err != nil {
			return err
		}
		data := map[string]any{
			"CommandID":     meta.ID,
			"ToolID":        cfg.ID,
			"Trigger":       commandTrigger(cfg, meta.ID),
			"Class":         meta.Class,
			"Description":   meta.Description,
			"BodyTemplate":  "command-" + id + "-body",
			"Arguments":     meta.Arguments,
			"Prerequisites": meta.Prerequisites,
			"Tier":          meta.Tier,
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
