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

// skillIndexFileName is the workflow-owned informational index that describes
// exported host skills to external agents.
const skillIndexFileName = "skill-index.md"

// ToolConfig describes a tool adapter target (Claude, Cursor, Codex, OpenCode, Gemini).
type ToolConfig struct {
	ID            string
	SkillsDir     string
	CommandsDir   string // "" = no project-local commands (Codex)
	CommandStyle  string // "nested", "flat", "" = no project-local commands
	CommandFormat string // "md" (default), "toml" (Gemini)
	// CommandSkillSurface, when true, generates one host skill per Slipway command
	// under SkillsDir (slipway-<command>/SKILL.md) instead of project-local command
	// prompt files. Codex uses this: its current CLI discovers skills, not the
	// deprecated custom-prompt surface.
	CommandSkillSurface bool
	SettingsPath        string
	SessionEvent        string
	SessionHook         string
	PostToolEvent       string
	PostToolHook        string
	TriggerPrefix       string
	TriggerStyle        string
	AutoDetectPath      []string
}

var toolRegistry = map[string]ToolConfig{
	"claude": {
		ID:            "claude",
		SkillsDir:     ".claude/skills",
		CommandsDir:   ".claude/commands",
		CommandStyle:  "nested",
		CommandFormat: "md",
		SettingsPath:  ".claude/settings.json",
		SessionEvent:  "SessionStart",
		SessionHook:   ".claude/hooks/slipway-session-start",
		PostToolEvent: "PostToolUse",
		PostToolHook:  ".claude/hooks/slipway-context-pressure-post-tool-use",
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
		SettingsPath:  "",
		SessionEvent:  "",
		SessionHook:   ".cursor/hooks/slipway-session-start",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		AutoDetectPath: []string{
			".cursor",
		},
	},
	"codex": {
		ID:                  "codex",
		SkillsDir:           ".codex/skills",
		CommandsDir:         "",
		CommandStyle:        "",
		CommandFormat:       "",
		CommandSkillSurface: true,
		SettingsPath:        "",
		SessionEvent:        "",
		SessionHook:         "",
		TriggerPrefix:       "$slipway-",
		TriggerStyle:        "dollar-mention",
		AutoDetectPath: []string{
			".codex",
		},
	},
	"opencode": {
		ID:            "opencode",
		SkillsDir:     ".opencode/skills",
		CommandsDir:   ".opencode/commands",
		CommandStyle:  "flat",
		CommandFormat: "md",
		SettingsPath:  "",
		SessionEvent:  "",
		SessionHook:   ".opencode/hooks/slipway-session-start",
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
		SettingsPath:  ".gemini/settings.json",
		SessionEvent:  "SessionStart",
		SessionHook:   ".gemini/hooks/slipway-session-start",
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
	Notes            []string
	Tier             string // "core" | "situational" | "diagnostics"
	HasPromptSurface bool   // true = generates inline command prompt surface; false for CLI-only commands
}

// commandRegistry is the single source of truth for adapter command metadata.
// Entry order is for readability; commandIDs() returns IDs sorted alphabetically.
var commandRegistry = []CommandDef{
	// Core (5)
	{ID: "new", Class: CommandClassMutation, Description: "Create a governed change with intake-first workflow", Tier: "core", HasPromptSurface: true,
		Arguments:     `"<description>" [--preset light|standard|strict] [--profile code|docs|research|config|meta] [--discuss] [--full] [--trivial] [--from-doc <path>] [--json]`,
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "No conflicting active change should already exist in the workspace."},
		Notes: []string{
			"JSON stdin fields for `slipway new --json`, not command-line flags: `guardrail_domain`, `needs_discovery`, and `complexity`.",
			"Minimal explicit-classification example: `echo '{\"description\":\"fix typo\",\"guardrail_domain\":\"\",\"needs_discovery\":false,\"complexity\":\"simple\"}' | slipway new --json`.",
		}},
	{ID: "next", Class: CommandClassQuery, Description: "Query next actionable skill (read-only, does not advance state)", Tier: "core", HasPromptSurface: true,
		Arguments: "[--json] [--diagnostics] [--context-guard] [--no-auto-pass] [--change <slug>]"},
	{ID: "run", Class: CommandClassMutation, Description: "Advance governed execution until a skill, blocker, checkpoint, or done-ready outcome is surfaced", Tier: "core", HasPromptSurface: true,
		Arguments: "[--json] [--diagnostics] [--resume] [--resume-response \"<text>\"] [--change <slug>]"},
	{ID: "status", Class: CommandClassQuery, Description: "Show lifecycle status, blockers, and next actions", Tier: "core", HasPromptSurface: true,
		Arguments:     "[--json] [--format text|yaml|json] [--focus <alias>] [--list-focuses] [--hydrate] [--hydrate-ref <skill-id>/<name>] [--root] [--stats] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "Can be used with or without an active change."}},
	{ID: "done", Class: CommandClassMutation, Description: "Finalize a done-ready change and archive it", Tier: "core", HasPromptSurface: true,
		Arguments: "[--json] [--all-ready] [--change <slug>]"},
	// Situational (12)
	{ID: "init", Class: CommandClassMutation, Description: "Initialize runtime layout and optional tool artifacts", Tier: "situational", HasPromptSurface: true,
		Arguments:     "[--tools all|none|claude,cursor,...] [--refresh]",
		Prerequisites: []string{"Run from the target project root or any child directory inside it.", "The workspace must be inside a git working tree."}},
	{ID: "cancel", Class: CommandClassMutation, Description: "Cancel an active change and archive terminal state", Tier: "situational", HasPromptSurface: true,
		Arguments: "[--json] [--change <slug>]"},
	{ID: "delete", Class: CommandClassMutation, Description: "Discard an abandoned governed change: its bundle, runtime binding, optional worktree, or an archived record", Tier: "situational", HasPromptSurface: true,
		Arguments:     "[--change <slug>] [--worktree] [--archived] [--yes] [--force] [--json]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "Operates on an abandoned/accidental or partially-deleted change; distinct from `cancel`, which archives a terminal record."},
		Notes: []string{
			"Default (no `--yes`) prints a dry-run plan and deletes nothing; `--yes` executes.",
			"`--worktree` also removes the bound git worktree (refused on dirty or unsafe-untracked changes unless `--force`); `--archived` purges an archived terminal record. The implementation/PR branch is never deleted.",
			"`cancel` archives a terminal record; `delete` discards local governed state. `status`/`next` route here when a change is abandoned, broken, or bound elsewhere.",
		}},
	{ID: "review", Class: CommandClassMutation, Description: "Bidirectional artifact-code alignment review", Tier: "situational", HasPromptSurface: true,
		Arguments:     "[--json] [--all|--changed-only] [--focus <alias>] [--list-focuses] [--format text|json] [--hydrate] [--hydrate-ref <skill-id>/<name>] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "An active governed change must be in S2_EXECUTE, S3_REVIEW, or S4_VERIFY with execution-summary evidence (run wave-orchestration first)."}},
	{ID: "validate", Class: CommandClassQuery, Description: "Read-only evidence and gate check", Tier: "situational", HasPromptSurface: true,
		Arguments:     "[--json] [--focus <alias>] [--list-focuses] [--format text|json] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "Can be used with or without an active change."}},
	{ID: "checkpoint", Class: CommandClassMutation, Description: "Set an active checkpoint to pause wave execution and request user input", Tier: "situational", HasPromptSurface: true,
		Arguments:     "--task-id <id> [--type human_verify|decision|human_action] [--allowed-responses <value> ...] [--json] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "An active governed change must be in S2_EXECUTE with a materialized wave plan (run `slipway repair` if `wave-plan.yaml` is missing)."}},
	{ID: "preset", Class: CommandClassMutation, Description: "Confirm or override the active change workflow preset", Tier: "situational", HasPromptSurface: true,
		Arguments:     "<light|standard|strict> [--json] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "An active governed change should already exist, or pass `--change <slug>`."}},
	{ID: "pivot", Class: CommandClassMutation, Description: "Reroute or rescope an active change", Tier: "situational", HasPromptSurface: true,
		Arguments: "[--reroute|--rescope] [--json] [--change <slug>]"},
	{ID: "abort", Class: CommandClassMutation, Description: "Abort the active execution session without archiving the change", Tier: "situational", HasPromptSurface: true,
		Arguments:     "[--json] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "An active governed change must be in S2_EXECUTE; outside S2_EXECUTE use `slipway cancel` instead."}},
	{ID: "repair", Class: CommandClassMutation, Description: "Run safe local integrity and layout repairs", Tier: "situational", HasPromptSurface: true,
		Arguments:     "[--json] [--focus <alias>] [--list-focuses] [--format text|json]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)"}},
	{ID: "evidence", Class: CommandClassMutation, Description: "Record supported runtime and skill verification evidence", Tier: "situational", HasPromptSurface: true,
		Arguments:     "task --task-id <id> --run-summary-version <n> --task-kind <kind> --verdict <verdict> --evidence-ref <ref> [--changed-file <path> ...] [--target-file <path> ...] [--blocker <code[:detail]> ...] [--captured-at <RFC3339Nano>] [--session-id <id>] [--json] [--change <slug>]; skill --skill <name> --verdict <pass|fail> [--reference <ref> ...] [--blocker <code[:detail]> ...] [--notes <text>|--notes-file <path>] [--json] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "`task` requires an active governed change in S2_EXECUTE with a materialized wave plan.", "`skill` requires an active governed change at the lifecycle state owned by the named governance skill; run-summary-bound skills also require current execution evidence."}},
	{ID: "tool", Class: CommandClassMutation, Description: "Run Slipway helper tools", Tier: "situational", HasPromptSurface: false,
		Arguments:     "<helper> [helper flags]",
		Prerequisites: []string{"None — public CLI-only helper namespace used by generated skills. Individual helpers may require GitHub tokens, local files, or explicit confirmation."},
		Notes: []string{
			"`tool` is intentionally CLI-only: generated skills call `slipway tool ...` directly, but Slipway does not export `$slipway-tool` or host command prompt wrappers.",
		}},
	// Diagnostics (5)
	{ID: "learn", Class: CommandClassQuery, Description: "Preview governance learning proposals from lifecycle evidence", Tier: "diagnostics", HasPromptSurface: true,
		Arguments:     "[--preview] [--json]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)"}},
	{ID: "stats", Class: CommandClassQuery, Description: "Show repo-wide governance freshness and workflow statistics", Tier: "diagnostics", HasPromptSurface: true,
		Arguments:     "[--json]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)"}},
	{ID: "health", Class: CommandClassQuery, Description: "Show repo-local integrity and repairability findings", Tier: "diagnostics", HasPromptSurface: true,
		Arguments:     "[--json] [--governance] [--all] [--observations] [--doctor] [--focus <alias>] [--list-focuses] [--format text|json] [--hydrate] [--hydrate-ref <skill-id>/<name>] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)"}},
	{ID: "codebase-map", Class: CommandClassMutation, Description: "Create or refresh the durable repo-scoped codebase map", Tier: "diagnostics", HasPromptSurface: true,
		Arguments:     "[--json]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)"}},
	{ID: "instructions", Class: CommandClassQuery, Description: "Show the authoring contract (template, quality bar, and inside a change the resolved output path + dependency graph) for a governed artifact or codebase-map doc", Tier: "diagnostics", HasPromptSurface: true,
		Arguments: "<artifact> [--change <slug>] [--json]",
		// instructions serves a static template/guidance exemplar with no
		// `.slipway.yaml` and no active change, for governed bundle artifacts and
		// codebase-map docs alike. Inside a change it additionally resolves output
		// path/dependencies, but never requires one. Declare prereq-free explicitly
		// so the generated command-reference does not leak the catch-all default
		// prerequisites (run `slipway init` / an active change), which would be
		// false for this prereq-free surface (issues #91, #119).
		Prerequisites: []string{"None — serves a static template and guidance with no `slipway init` or active change required."}},
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
	{ID: "tdd-governance", RenderMode: governanceRenderTemplated, ExportOnlyExtra: true},
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
	"evidence",
	"init",
	"cancel",
	"delete",
	"checkpoint",
	"preset",
	"pivot",
	"abort",
}

var workflowDiagnosticCommandIDs = []string{"learn", "stats", "health", "codebase-map", "instructions"}

type workflowSkillData struct {
	ToolID             string
	PublicName         string
	SkillIndexPath     string
	CommandsDir        string
	LifecycleCommands  []commandEntry
	SupportingCommands []commandEntry
	DiagnosticCommands []commandEntry
}

type commandEntry struct {
	Name          string
	Description   string
	Arguments     string
	Prerequisites []string
	Notes         []string
	Focuses       []commandFocusEntry
}

type commandFocusEntry struct {
	PublicName string
	BackingID  string
	Summary    string
}

// techniqueNames lists the technique skills to generate.
var techniqueNames = []string{
	"tdd",
	"codebase-mapping",
	"coding-discipline",
}

var hostSkillExportAllowlist = map[string]struct{}{
	workflowSkillID: {},
	"slipway":       {},

	"intake-clarification":   {},
	"research-orchestration": {},
	"plan-audit":             {},
	"worktree-preflight":     {},
	"wave-orchestration":     {},
	"tdd-governance":         {},
	"spec-compliance-review": {},
	"code-quality-review":    {},
	"goal-verification":      {},
	"final-closeout":         {},

	"independent-review": {},
	"context-assembly":   {},
	"root-cause-tracing": {},
	"security-review":    {},
	"spec-trace":         {},
	"coverage-analysis":  {},
	"test-design":        {},
	"coding-discipline":  {},
	"git-recovery":       {},
	"codebase-mapping":   {},

	"ci-triage":         {},
	"incident-response": {},
}

func shouldExportAsHostSkill(id string) bool {
	_, ok := hostSkillExportAllowlist[strings.TrimSpace(id)]
	return ok
}

func ShouldExportAsHostSkill(id string) bool {
	return shouldExportAsHostSkill(id)
}

// GovernanceSkillNames lists the static exported governance surfaces (.md).
var GovernanceSkillNames = governanceSurfaceIDsByRenderMode(governanceRenderStatic)

// standaloneGovernanceNames lists standalone exported governance helpers.
var standaloneGovernanceNames = governanceSurfaceIDsByRenderMode(governanceRenderStandalone)

// TemplatedGovernanceSkillNames lists governance surfaces rendered from .md.tmpl.
var TemplatedGovernanceSkillNames = governanceSurfaceIDsByRenderMode(governanceRenderTemplated)

// catalogSkillIDs returns Go-registry skill IDs sorted for deterministic
// generation and cleanup.
var catalogSkillIDs = func() []string {
	return capability.DefaultRegistry().IDs()
}()

func exportedCapabilityRegistry(reg *capability.Registry) (*capability.Registry, error) {
	if reg == nil {
		return capability.NewRegistry()
	}
	skills := make([]capability.Skill, 0, reg.Len())
	for _, sk := range reg.All() {
		if shouldExportAsHostSkill(sk.ID) {
			skills = append(skills, sk)
		}
	}
	return capability.NewRegistry(skills...)
}

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
	Notes         []string
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
		Arguments:     CommandArguments(id),
		Prerequisites: commandPrerequisites(id),
		Notes:         append([]string(nil), def.Notes...),
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
			Notes:         append([]string(nil), meta.Notes...),
			Focuses:       buildCommandFocusEntries(meta.ID),
		})
	}
	return out, nil
}

func buildCommandFocusEntries(command string) []commandFocusEntry {
	focuses := capability.ExplicitFocusesForCommand(command)
	out := make([]commandFocusEntry, 0, len(focuses))
	for _, focus := range focuses {
		out = append(out, commandFocusEntry{
			PublicName: focus.PublicName,
			BackingID:  focus.BackingID,
			Summary:    focus.Summary,
		})
	}
	return out
}

func validateWorkflowCommandCoverage(groups ...[]string) error {
	grouped := make(map[string]struct{}, len(commandRegistry))
	for _, ids := range groups {
		for _, id := range ids {
			if _, exists := grouped[id]; exists {
				return fmt.Errorf("workflow command %q declared more than once", id)
			}
			def, ok := commandRegistryMap[id]
			if !ok {
				return fmt.Errorf("workflow command group references unknown registry command %q", id)
			}
			if !def.HasPromptSurface {
				return fmt.Errorf("workflow command group includes CLI-only command %q", id)
			}
			grouped[id] = struct{}{}
		}
	}

	expected := 0
	for _, def := range commandRegistry {
		if !def.HasPromptSurface {
			continue
		}
		expected++
		if _, ok := grouped[def.ID]; !ok {
			return fmt.Errorf("workflow command groups missing registry command %q", def.ID)
		}
	}
	if len(grouped) != expected {
		return fmt.Errorf("workflow command groups cover %d commands, want %d", len(grouped), expected)
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
		ToolID:             cfg.ID,
		PublicName:         adapterSkillName("workflow"),
		SkillIndexPath:     filepath.ToSlash(SkillIndexPath(cfg)),
		CommandsDir:        filepath.ToSlash(cfg.CommandsDir),
		LifecycleCommands:  lifecycle,
		SupportingCommands: supporting,
		DiagnosticCommands: diagnostics,
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
			if strings.HasPrefix(entry.Name(), "slipway-") {
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

// Generate creates tool skills and command surfaces for the given tools.
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

func sourceSkillTemplatePath(id, leaf string) string {
	return path.Join("skills", id, leaf)
}

// SkillPath returns the relative path to a skill's SKILL.md for the given tool config.
func SkillPath(cfg ToolConfig, skillName string) string {
	return filepath.Join(cfg.SkillsDir, adapterSkillName(skillName), "SKILL.md")
}

// SkillIndexPath returns the relative path to the generated workflow-owned
// informational skill index for the given tool config.
func SkillIndexPath(cfg ToolConfig) string {
	return filepath.Join(
		cfg.SkillsDir,
		adapterSkillName(workflowSkillID),
		"references",
		skillIndexFileName,
	)
}

func generateForTool(root string, cfg ToolConfig, refresh bool) error {
	sentinelPath := filepath.Join(root, GeneratedAdapterMarkerPath(cfg))
	_, sentinelErr := os.Stat(sentinelPath)
	hadGeneratedAdapter := sentinelErr == nil

	if refresh {
		// Remove existing sentinel before the mutating pass.
		_ = os.Remove(sentinelPath)

		// Purge direct command prompt surfaces before rewrite (project-local only).
		if cfg.CommandsDir != "" {
			if err := purgeCommandPromptSurfaces(root, cfg); err != nil {
				return err
			}
		}

		if err := cleanupStaleGeneratedArtifacts(root, cfg, hadGeneratedAdapter); err != nil {
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
		if !shouldExportAsHostSkill(id) {
			continue
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
		if !shouldExportAsHostSkill(name) {
			continue
		}
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

	// Command prompt surfaces (inline command prompts for prompt-backed adapters).
	// Skip when CommandsDir is empty (Codex uses command skills instead).
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

	// Command skill surfaces (Codex): one discoverable skill per command under
	// SkillsDir.
	if cfg.CommandSkillSurface {
		for _, id := range commandIDs() {
			content, err := renderCommandSkill(cfg, id)
			if err != nil {
				return fmt.Errorf("render command skill %q for %s: %w", id, cfg.ID, err)
			}
			if err := writeDeterministic(filepath.Join(root, SkillPath(cfg, id)), content, refresh); err != nil {
				return err
			}
		}
		// Legacy migration: prior versions wrote host-global Codex prompt files.
		// Remove them on refresh so the retired surface does not linger.
		if refresh {
			if err := cleanupLegacyCodexPrompts(cfg); err != nil {
				return err
			}
		}
	}

	// Hook registration is keyed on whether the host owns a settings.json.
	//
	//   - Settings-capable hosts (claude, gemini): register each hook event as a
	//     bare inline `slipway hook <subcommand>` command directly in
	//     settings.json. No launcher script files are written, and on refresh any
	//     previously generated launcher files are pruned.
	//   - Settings-less hosts (cursor, opencode): keep emitting the advisory
	//     session-start launcher files (extensionless + .ps1 + .cmd), since they
	//     have no settings.json to register an inline command in.
	if strings.TrimSpace(cfg.SettingsPath) != "" {
		if refresh {
			if err := pruneHookLauncherFiles(root, cfg.SessionHook); err != nil {
				return err
			}
			if err := pruneHookLauncherFiles(root, cfg.PostToolHook); err != nil {
				return err
			}
		}
		if err := mergeHookSettingsJSON(root, cfg, refresh); err != nil {
			return err
		}
	} else if strings.TrimSpace(cfg.SessionHook) != "" {
		for _, launcher := range hookLauncherOutputs(cfg.SessionHook, "session-start") {
			content, err := renderHookLauncher(cfg, launcher.template)
			if err != nil {
				return fmt.Errorf("render session hook launcher for %s: %w", cfg.ID, err)
			}
			p := filepath.Join(root, launcher.path)
			if err := writeDeterministicMode(p, content, refresh, launcher.mode); err != nil {
				return err
			}
		}
		if refresh {
			if err := cleanupLegacyHookLauncher(root, cfg.SessionHook); err != nil {
				return err
			}
		}
	}

	// Workflow skill index (read by external agents; not consumed by the
	// Slipway kernel). Regenerated deterministically from the Go-owned
	// capability registry so every adapter sees direct host skill paths.
	exportedReg, err := exportedCapabilityRegistry(reg)
	if err != nil {
		return err
	}
	index := capability.BuildSkillIndexWithPaths(exportedReg, func(id string) string {
		return filepath.ToSlash(SkillPath(cfg, id))
	})
	indexPath := filepath.Join(root, SkillIndexPath(cfg))
	if err := writeDeterministic(indexPath, index, refresh); err != nil {
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

func cleanupStaleGeneratedArtifacts(root string, cfg ToolConfig, hadGeneratedAdapter bool) error {
	if err := cleanupStaleSkillDirs(root, cfg, hadGeneratedAdapter); err != nil {
		return err
	}
	if err := cleanupStaleCommandEntries(root, cfg, hadGeneratedAdapter); err != nil {
		return err
	}
	return nil
}

func cleanupStaleSkillDirs(root string, cfg ToolConfig, hadGeneratedAdapter bool) error {
	if !hadGeneratedAdapter {
		return nil
	}
	skillsRoot := filepath.Join(root, cfg.SkillsDir)
	if err := removePathIfExists(filepath.Join(skillsRoot, "slipway")); err != nil {
		return err
	}

	expected := map[string]struct{}{}
	for _, names := range [][]string{
		GovernanceSkillNames,
		standaloneGovernanceNames,
		TemplatedGovernanceSkillNames,
		standaloneNames,
		techniqueNames,
	} {
		for _, name := range names {
			if !shouldExportAsHostSkill(name) {
				continue
			}
			expected[adapterSkillName(name)] = struct{}{}
		}
	}
	for _, name := range catalogSkillIDs {
		if shouldExportAsHostSkill(name) {
			expected[adapterSkillName(name)] = struct{}{}
		}
	}
	if cfg.CommandSkillSurface {
		for _, name := range commandSkillDirNames() {
			expected[name] = struct{}{}
		}
	}
	managed := generatedSkillDirNameSet(cfg)

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
		if _, ok := managed[name]; ok {
			if err := os.RemoveAll(filepath.Join(skillsRoot, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func generatedSkillDirNameSet(cfg ToolConfig) map[string]struct{} {
	managed := map[string]struct{}{}
	for _, names := range [][]string{
		GovernanceSkillNames,
		standaloneGovernanceNames,
		TemplatedGovernanceSkillNames,
		standaloneNames,
		techniqueNames,
		catalogSkillIDs,
	} {
		for _, name := range names {
			managed[adapterSkillName(name)] = struct{}{}
		}
	}
	if cfg.CommandSkillSurface {
		for _, name := range commandSkillDirNames() {
			managed[name] = struct{}{}
		}
	}
	return managed
}

func cleanupStaleCommandEntries(root string, cfg ToolConfig, hadGeneratedAdapter bool) error {
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
		if err := cleanupPrefixedEntries(filepath.Join(root, cfg.CommandsDir), "slipway-", expected); err != nil {
			return err
		}
		if hadGeneratedAdapter {
			return cleanupLegacyNestedCommandEntries(root, cfg, ext)
		}
		return nil
	default:
		return cleanupUnexpectedEntries(filepath.Join(root, cfg.CommandsDir, "slipway"), expected)
	}
}

func cleanupLegacyNestedCommandEntries(root string, cfg ToolConfig, ext string) error {
	dir := filepath.Join(root, cfg.CommandsDir, "slipway")
	for _, id := range commandIDs() {
		if err := removePathIfExists(filepath.Join(dir, id+ext)); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	return os.Remove(dir)
}

// cleanupLegacyCodexPrompts removes the retired pre-skill Codex command prompt
// files that Slipway used to generate. Keep cleanup scoped to the command
// registry: $CODEX_HOME/prompts is shared user state, so a user-owned
// slipway-*.md prompt that was never generated by Slipway must survive refresh.
func cleanupLegacyCodexPrompts(cfg ToolConfig) error {
	if !cfg.CommandSkillSurface {
		return nil
	}
	promptsDir, err := codexPromptsDir()
	if err != nil {
		return err
	}
	for _, id := range commandIDs() {
		if err := removePathIfExists(filepath.Join(promptsDir, "slipway-"+id+".md")); err != nil {
			return err
		}
	}
	return nil
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
// identical to the source after line-ending normalization (important for
// tier-size and schema-lint gates that measure post-frontmatter bytes).
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
	raw = normalizeTemplateLineEndings(raw)
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

func normalizeTemplateLineEndings(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	return strings.ReplaceAll(raw, "\r", "\n")
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

// emitSkillSupportFiles copies optional support artifacts next to a generated
// skill. Skill scripts are intentionally not exported anymore; the `scripts`
// entry remains only as a refresh-time stale cleanup target.
func emitSkillSupportFiles(root string, cfg ToolConfig, skillID string, refresh bool) error {
	skillDirRel := filepath.Dir(SkillPath(cfg, skillID))
	dstBase := filepath.Join(root, skillDirRel)
	return emitSkillSupportFilesFromFS(tmpl.TemplateFS(), skillID, dstBase, refresh)
}

// emitSkillSupportFilesFromFS is the testable core: it sources support files
// from an arbitrary fs.FS rooted like tmpl.TemplateFS() (so paths begin with
// "skills/<id>/...") and writes them under dstBase on the local filesystem.
func emitSkillSupportFilesFromFS(srcFS fs.FS, skillID, dstBase string, refresh bool) error {
	for _, sub := range optionalSkillSupportDirs {
		dstDir := filepath.Join(dstBase, sub)
		if refresh {
			if err := removePathIfExists(dstDir); err != nil {
				return err
			}
		}
		if sub == "scripts" {
			continue
		}
		if sub == "references" {
			if err := emitSharedReferenceSupportFromFS(srcFS, skillID, dstDir, refresh); err != nil {
				return fmt.Errorf("copy shared references for %q: %w", skillID, err)
			}
		}
		if err := copyTemplateSubtreeFromFS(srcFS, sourceSkillTemplatePath(skillID, sub), dstDir, refresh); err != nil {
			return fmt.Errorf("copy %s for %q: %w", sub, skillID, err)
		}
	}
	return nil
}

// sharedReferenceDocs maps a shared reference filename to the detection that
// decides whether a skill should receive it. A skill gets the shared doc copied
// into its references/ dir only when its SKILL.md actually points at it, so the
// "Apply `references/<doc>`" pointer the skill prints is always reachable in the
// generated tree (it previously named a top-level sibling that no generation
// path emitted).
var sharedReferenceDocs = []string{"checklist-quality.md"}

func emitSharedReferenceSupportFromFS(srcFS fs.FS, skillID, dstDir string, refresh bool) error {
	for _, doc := range sharedReferenceDocs {
		referenced, err := skillReferencesSharedDoc(srcFS, skillID, doc)
		if err != nil {
			return err
		}
		if !referenced {
			continue
		}
		content, err := fs.ReadFile(srcFS, path.Join("skills", "_shared", "references", doc))
		if err != nil {
			return err
		}
		if err := writeDeterministic(filepath.Join(dstDir, doc), string(content), refresh); err != nil {
			return err
		}
	}
	return nil
}

// skillReferencesSharedDoc reports whether a skill's authored SKILL.md (or
// SKILL.md.tmpl) names the shared reference doc, so generation only ships it to
// the skills that consume it.
func skillReferencesSharedDoc(srcFS fs.FS, skillID, doc string) (bool, error) {
	for _, leaf := range []string{"SKILL.md", "SKILL.md.tmpl"} {
		content, err := fs.ReadFile(srcFS, sourceSkillTemplatePath(skillID, leaf))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return false, err
		}
		if strings.Contains(string(content), doc) {
			return true, nil
		}
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

// renderCommandSkill renders one discoverable host skill per Slipway command for
// adapters that expose commands as skills (Codex). The injected frontmatter
// `name`/`description` use the same YAML-safe escaping as every other generated
// skill so adapter skill loaders accept the output.
func renderCommandSkill(cfg ToolConfig, id string) (string, error) {
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
	}
	raw, err := tmpl.Render(path.Join("commands", "command-skill.md.tmpl"), data)
	if err != nil {
		return "", err
	}
	return injectAdapterFrontmatter(raw, adapterSkillName(id), meta.Description)
}

// commandSkillDirNames returns the generated skill directory names for each
// adapter command (slipway-<command>).
func commandSkillDirNames() []string {
	out := make([]string, 0, len(commandIDs()))
	for _, id := range commandIDs() {
		out = append(out, adapterSkillName(id))
	}
	return out
}

type hookLauncherOutput struct {
	path     string
	template string
	mode     os.FileMode
}

func hookLauncherOutputs(basePath, hookTemplateBase string) []hookLauncherOutput {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return nil
	}
	return []hookLauncherOutput{
		{path: basePath, template: path.Join("hooks", hookTemplateBase+".sh.tmpl"), mode: 0o755},
		{path: basePath + ".ps1", template: path.Join("hooks", hookTemplateBase+".ps1.tmpl"), mode: 0o644},
		{path: basePath + ".cmd", template: path.Join("hooks", hookTemplateBase+".cmd.tmpl"), mode: 0o644},
	}
}

// Bare inline hook commands registered in a settings-capable host's
// settings.json. They are intentionally launcher-free and shell-operator-free so
// the same string is portable across `/bin/sh -c`, `cmd /c`, and PowerShell.
const (
	sessionStartHookCommand    = "slipway hook session-start"
	contextPressureHookCommand = "slipway hook context-pressure"
)

func renderHookLauncher(cfg ToolConfig, templatePath string) (string, error) {
	data := map[string]string{
		"ToolID":     cfg.ID,
		"EntrySkill": workflowEntryPublicName,
	}
	return tmpl.Render(templatePath, data)
}

func writeDeterministic(path, content string, refresh bool) error {
	return writeDeterministicMode(path, content, refresh, defaultFileModeForPath(path))
}

func writeDeterministicMode(path, content string, refresh bool, mode os.FileMode) error {
	if !refresh {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return err
	}
	return os.WriteFile(path, []byte(content), mode)
}

func defaultFileModeForPath(path string) os.FileMode {
	if strings.HasSuffix(path, ".sh") {
		return 0o755
	}
	return 0o644
}

func mergeHookSettingsJSON(root string, cfg ToolConfig, refresh bool) error {
	settingsPath := filepath.Join(root, cfg.SettingsPath)
	if !refresh {
		if _, err := os.Stat(settingsPath); err == nil {
			return nil
		}
	}

	settings := map[string]any{}
	existing, err := os.ReadFile(settingsPath) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
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
		pruneStaleSlipwayHookCommands(hooks, cfg.SessionEvent, cfg.SessionHook, sessionStartHookCommand)
		mergeHookEventCommand(hooks, cfg.SessionEvent, sessionStartHookCommand)
	}
	if strings.TrimSpace(cfg.PostToolEvent) != "" && strings.TrimSpace(cfg.PostToolHook) != "" {
		pruneStaleSlipwayHookCommands(hooks, cfg.PostToolEvent, cfg.PostToolHook, contextPressureHookCommand)
		mergeHookEventCommand(hooks, cfg.PostToolEvent, contextPressureHookCommand)
	}
	settings["hooks"] = hooks

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return err
	}
	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(settingsPath, content, 0o644) // #nosec G306 -- file is a user-facing project or governance artifact where operator-readable mode is intentional.
}

func legacyShellHookPath(basePath string) string {
	return filepath.ToSlash(strings.TrimSpace(basePath) + ".sh")
}

func cleanupLegacyHookLauncher(root, basePath string) error {
	if strings.TrimSpace(basePath) == "" {
		return nil
	}
	return removePathIfExists(filepath.Join(root, filepath.FromSlash(legacyShellHookPath(basePath))))
}

// hookLauncherFileSuffixes lists the extensions Slipway has ever emitted for a
// hook launcher beside the extensionless POSIX entry. Used to prune the
// now-retired launcher family for settings-capable hosts.
var hookLauncherFileSuffixes = []string{"", ".ps1", ".cmd", ".sh"}

// pruneHookLauncherFiles removes every Slipway-generated launcher file derived
// from basePath (the extensionless POSIX entry plus its .ps1/.cmd/.sh variants).
// It is the refresh-time orphan cleanup for settings-capable hosts that no
// longer emit launcher scripts.
func pruneHookLauncherFiles(root, basePath string) error {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return nil
	}
	for _, suffix := range hookLauncherFileSuffixes {
		p := filepath.Join(root, filepath.FromSlash(basePath+suffix))
		if err := removePathIfExists(p); err != nil {
			return err
		}
	}
	return nil
}

// pruneStaleSlipwayHookCommands removes every previously generated Slipway hook
// command for an event so refresh can re-register the current bare inline form.
// It prunes retired launcher-path commands and the short-lived direct forms
// (`slipway hook ... --tool <id>` / `slipway hook ... || exit 0`) while
// preserving unrelated user-authored hook entries.
func pruneStaleSlipwayHookCommands(hooks map[string]any, eventName, basePath, currentCommand string) {
	rawEntries, ok := hooks[eventName].([]any)
	if !ok {
		return
	}
	var keptEntries []any
	for _, entry := range rawEntries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			keptEntries = append(keptEntries, entry)
			continue
		}
		hookList, ok := entryMap["hooks"].([]any)
		if !ok {
			keptEntries = append(keptEntries, entry)
			continue
		}
		var keptHooks []any
		for _, hook := range hookList {
			if isStaleSlipwayHookCommand(hook, basePath, currentCommand) {
				continue
			}
			keptHooks = append(keptHooks, hook)
		}
		if len(keptHooks) == 0 {
			continue
		}
		entryCopy := map[string]any{}
		for key, value := range entryMap {
			entryCopy[key] = value
		}
		entryCopy["hooks"] = keptHooks
		keptEntries = append(keptEntries, entryCopy)
	}
	hooks[eventName] = keptEntries
}

// isLegacySlipwayHookCommand reports whether a settings.json hook command points
// at a retired Slipway launcher derived from basePath (the extensionless POSIX
// entry or its .sh/.ps1/.cmd variants), either executed directly or as the script
// file passed to a bash/sh interpreter. User-authored commands that merely
// mention an unrelated or similarly prefixed path are left untouched.
func isStaleSlipwayHookCommand(hook any, basePath, currentCommand string) bool {
	hookMap, ok := hook.(map[string]any)
	if !ok {
		return false
	}
	command, _ := hookMap["command"].(string)
	command = filepath.ToSlash(strings.TrimSpace(command))
	basePath = filepath.ToSlash(strings.TrimSpace(basePath))
	if basePath != "" {
		variants := legacyHookLauncherVariants(basePath)
		fields := strings.Fields(command)
		if len(fields) > 0 {
			first := strings.Trim(fields[0], `"'`)
			// Match the interpreter by basename so an absolute path (e.g. /bin/bash) is
			// pruned too, not just the bare "bash"/"sh" Slipway historically generated.
			// A direct exec of any launcher variant (first token is the launcher base
			// path or one of its .sh/.ps1/.cmd siblings) also qualifies.
			firstBase := path.Base(first)
			if firstBase == "bash" || firstBase == "sh" {
				if scriptArg, ok := firstShellScriptArg(fields[1:]); ok && matchesLegacyHookLauncher(scriptArg, variants) {
					return true
				}
			} else if matchesLegacyHookLauncher(first, variants) {
				return true
			}
		}
	}
	return isStaleDirectHookCommand(command, currentCommand)
}

func isStaleDirectHookCommand(command, currentCommand string) bool {
	currentCommand = strings.TrimSpace(currentCommand)
	if currentCommand == "" || command == currentCommand {
		return false
	}
	if !strings.HasPrefix(command, currentCommand) {
		return false
	}
	suffix := strings.TrimSpace(strings.TrimPrefix(command, currentCommand))
	return suffix == "|| exit 0" ||
		strings.HasPrefix(suffix, "--tool ") ||
		strings.HasPrefix(suffix, "--tool=")
}

func legacyHookLauncherVariants(basePath string) []string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return nil
	}
	variants := make([]string, 0, len(hookLauncherFileSuffixes))
	for _, suffix := range hookLauncherFileSuffixes {
		variants = append(variants, filepath.ToSlash(basePath+suffix))
	}
	return variants
}

func matchesLegacyHookLauncher(token string, variants []string) bool {
	token = filepath.ToSlash(strings.Trim(strings.TrimSpace(token), `"'`))
	if token == "" {
		return false
	}
	for _, variant := range variants {
		if token == variant || strings.HasSuffix(token, "/"+variant) {
			return true
		}
	}
	return false
}

func firstShellScriptArg(fields []string) (string, bool) {
	for _, field := range fields {
		arg := strings.Trim(strings.TrimSpace(field), `"'`)
		if arg == "" {
			continue
		}
		if arg == "--" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			// `bash -c` / `bash -lc` receives a command string, not a script
			// filename. Leave such user-authored hooks untouched.
			if strings.Contains(arg, "c") {
				return "", false
			}
			continue
		}
		return arg, true
	}
	return "", false
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

func commandTrigger(cfg ToolConfig, commandID string) string {
	if cfg.TriggerStyle == "slash-colon" {
		return fmt.Sprintf("%s:%s", cfg.TriggerPrefix, commandID)
	}
	return fmt.Sprintf("%s%s", cfg.TriggerPrefix, commandID)
}

// CommandArguments returns the registry argument summary for a command ID.
// It is the single string the generated command reference renders from, so the
// reverse flag-contract guard asserts every registered Cobra flag appears here.
func CommandArguments(id string) string {
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

// InvocationSummary describes, for humans, how generated command surfaces are
// invoked for this tool. `slipway init` prints it per selected tool so the
// invocation surface is explicit at setup time.
func (c ToolConfig) InvocationSummary() string {
	if c.CommandSkillSurface {
		return "invoke skills: $slipway (entry), $slipway-<command> per command, or /skills"
	}
	if c.TriggerStyle == "slash-colon" {
		return fmt.Sprintf("invoke commands as %s:<command>", c.TriggerPrefix)
	}
	return fmt.Sprintf("invoke commands as %s<command>", c.TriggerPrefix)
}
