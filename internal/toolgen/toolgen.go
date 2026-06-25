package toolgen

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/capability"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/tmpl"
	"gopkg.in/yaml.v3"
)

// skillIndexFileName is the workflow-owned informational index that describes
// exported host skills to external agents.
const skillIndexFileName = "skill-index.md"

// ToolConfig describes a tool adapter target (Claude, Cursor, Codex, OpenCode,
// and other generated AI-tool hosts).
type ToolConfig struct {
	ID               string
	SkillsDir        string
	CommandsDir      string // "" = no project-local commands (Codex/Qwen/Kiro)
	CommandStyle     string // "nested", "flat", "" = no project-local commands
	CommandFormat    string // "md" (default), "toml"
	CommandExtension string // optional full extension override, for example ".prompt.md"
	// CommandSkillSurface, when true, generates one host skill per Slipway command
	// under SkillsDir (slipway-<command>/SKILL.md) instead of project-local command
	// prompt files. Skill-first hosts use this when their CLI discovers skills
	// instead of prompt/workflow command files.
	CommandSkillSurface bool
	SettingsPath        string
	SettingsKind        string
	SessionEvent        string
	SessionHook         string
	PostToolEvent       string
	PostToolHook        string
	TriggerPrefix       string
	TriggerStyle        string
	AutoDetectPath      []string
}

const (
	settingsKindPiRegistration = "pi-registration"
	settingsKindCodexHooks     = "codex-hooks"
	stageAutoModeNote          = "Config-level `execution.auto` applies to this stage command; there are no per-stage `--auto`/`--no-auto` flags. Under auto, only pure-pacing boundaries may continue on prior authorization; sensitive/guardrail confirmations, the intake Approved Summary, and evidence gates still stop."
	nextAutoModeNote           = "`slipway next` is query-only: it reflects config-level `execution.auto` in displayed confirmation requirements, has no `--auto`/`--no-auto` flags, and never mutates pending preset confirmations. Per-run `slipway run --auto` behavior is visible on `run`."
)

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
		SettingsPath:        ".codex/config.toml",
		SettingsKind:        settingsKindCodexHooks,
		SessionEvent:        "SessionStart",
		SessionHook:         "slipway hook session-start --tool codex",
		PostToolEvent:       "UserPromptSubmit",
		PostToolHook:        "slipway hook context-pressure --tool codex",
		TriggerPrefix:       "$slipway-",
		TriggerStyle:        "dollar-mention",
		AutoDetectPath: []string{
			".codex",
		},
	},
	"copilot": {
		ID:               "copilot",
		SkillsDir:        ".github/skills",
		CommandsDir:      ".github/prompts",
		CommandStyle:     "flat",
		CommandFormat:    "md",
		CommandExtension: ".prompt.md",
		SettingsPath:     "",
		SessionEvent:     "",
		SessionHook:      "",
		TriggerPrefix:    "/slipway-",
		TriggerStyle:     "slash-hyphen",
		AutoDetectPath: []string{
			".github/copilot",
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
	"kilo": {
		ID:            "kilo",
		SkillsDir:     ".kilocode/skills",
		CommandsDir:   ".kilocode/workflows",
		CommandStyle:  "flat",
		CommandFormat: "md",
		SettingsPath:  "",
		SessionEvent:  "",
		SessionHook:   "",
		TriggerPrefix: "/slipway",
		TriggerStyle:  "slash-colon",
		AutoDetectPath: []string{
			".kilocode",
		},
	},
	"kiro": {
		ID:                  "kiro",
		SkillsDir:           ".kiro/skills",
		CommandsDir:         "",
		CommandStyle:        "",
		CommandFormat:       "",
		CommandSkillSurface: true,
		SettingsPath:        "",
		SessionEvent:        "",
		SessionHook:         "",
		TriggerPrefix:       "@slipway",
		TriggerStyle:        "at-colon",
		AutoDetectPath: []string{
			".kiro",
		},
	},
	"pi": {
		ID:            "pi",
		SkillsDir:     ".pi/skills",
		CommandsDir:   ".pi/prompts",
		CommandStyle:  "flat",
		CommandFormat: "md",
		SettingsPath:  ".pi/settings.json",
		SettingsKind:  settingsKindPiRegistration,
		SessionEvent:  "",
		SessionHook:   "",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		AutoDetectPath: []string{
			".pi",
		},
	},
	"qwen": {
		ID:                  "qwen",
		SkillsDir:           ".qwen/skills",
		CommandsDir:         "",
		CommandStyle:        "",
		CommandFormat:       "",
		CommandSkillSurface: true,
		SettingsPath:        ".qwen/settings.json",
		SessionEvent:        "SessionStart",
		SessionHook:         ".qwen/hooks/slipway-session-start",
		TriggerPrefix:       "/slipway-",
		TriggerStyle:        "slash-hyphen",
		AutoDetectPath: []string{
			".qwen",
		},
	},
	"windsurf": {
		ID:            "windsurf",
		SkillsDir:     ".windsurf/skills",
		CommandsDir:   ".windsurf/workflows",
		CommandStyle:  "flat",
		CommandFormat: "md",
		SettingsPath:  "",
		SessionEvent:  "",
		SessionHook:   "",
		TriggerPrefix: "/slipway-",
		TriggerStyle:  "slash-hyphen",
		AutoDetectPath: []string{
			".windsurf",
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
	Tier             string // "core" | "discovery" | "situational" | "helpers" | "diagnostics" | "setup"
	HasPromptSurface bool   // true = generates inline command prompt surface; false for CLI-only commands
}

// commandRegistry is the single source of truth for adapter command metadata.
// Entry order is for readability; commandIDs() returns IDs sorted alphabetically.
var commandRegistry = []CommandDef{
	// Core (10)
	{ID: "new", Class: CommandClassMutation, Description: "Create a governed change with intake-first workflow", Tier: "core", HasPromptSurface: true,
		Arguments:     `"<description>" [--preset light|standard|strict] [--profile code|docs|research|config|meta] [--discuss] [--full] [--trivial] [--from-doc <path>] [--json]`,
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "No conflicting active change should already exist in the workspace."},
		Notes: []string{
			"JSON stdin fields for `slipway new --json`, not command-line flags: `guardrail_domain`, `needs_discovery`, and `complexity`.",
			"Minimal explicit-classification example: `echo '{\"description\":\"fix typo\",\"guardrail_domain\":\"\",\"needs_discovery\":false,\"complexity\":\"simple\"}' | slipway new --json`.",
		}},
	{ID: "intake", Class: CommandClassMutation, Description: "Complete intake clarification and authorization for the active change", Tier: "core", HasPromptSurface: true,
		Arguments:     "[--json] [--diagnostics] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "An active governed change must be in S0_INTAKE."},
		Notes: []string{
			"`slipway intake` is the explicit S0 stage command. `slipway run` delegates here when the current state is S0_INTAKE.",
			stageAutoModeNote,
		}},
	{ID: "plan", Class: CommandClassMutation, Description: "Author or amend the governed plan artifacts for the active change", Tier: "core", HasPromptSurface: true,
		Arguments:     "[--json] [--diagnostics] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "An active governed change must be in S1_PLAN."},
		Notes: []string{
			"`slipway plan` is the explicit S1 stage command. Same-intent change amendments update the current bundle without a separate recovery command.",
			stageAutoModeNote,
		}},
	{ID: "implement", Class: CommandClassMutation, Description: "Execute governed implementation waves for the active change", Tier: "core", HasPromptSurface: true,
		Arguments:     "[--json] [--diagnostics] [--resume] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "An active governed change must be in S2_IMPLEMENT with a materialized wave plan."},
		Notes: []string{
			"`slipway implement` is the explicit S2 stage command. `slipway run` delegates here when the current state is S2_IMPLEMENT.",
			stageAutoModeNote,
		}},
	{ID: "review", Class: CommandClassMutation, Description: "Run review convergence for artifact-code alignment and feedback repairs", Tier: "core", HasPromptSurface: true,
		Arguments:     "[--json] [--diagnostics] [--all|--changed-only] [--focus <alias>] [--list-focuses] [--format text|json] [--hydrate] [--hydrate-ref <skill-id>/<name>] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "An active governed change must be in S3_REVIEW with execution-summary evidence (run wave-orchestration first)."}},
	{ID: "fix", Class: CommandClassMutation, Description: "Dispatch fresh-context fixes for S3 review findings", Tier: "core", HasPromptSurface: true,
		Arguments:     "[--json] [--reviewer <skill>] [--start-reexecution] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "An active governed change must be in S3_REVIEW with review findings to repair."},
		Notes: []string{
			"Ordinary `slipway fix` discovery does not run local-integrity repair and does not advance lifecycle state; `slipway fix --start-reexecution` explicitly reopens S2 and materializes a fresh execution run for review-driven implementation repairs.",
			"After the repair subagent edits code/artifacts, rerun and record the affected reviewer evidence, then run `slipway review`.",
		}},
	{ID: "next", Class: CommandClassQuery, Description: "Query next actionable skill (read-only, does not advance state)", Tier: "core", HasPromptSurface: true,
		Arguments: "[--json] [--diagnostics] [--context-guard] [--no-auto-pass] [--change <slug>]",
		Notes: []string{
			nextAutoModeNote,
		}},
	{ID: "run", Class: CommandClassMutation, Description: "Shortcut driver for the current lifecycle stage", Tier: "core", HasPromptSurface: true,
		Arguments: "[--json] [--diagnostics] [--resume] [--auto|--no-auto] [--change <slug>]",
		Notes: []string{
			"`slipway run` is an auto-driver shortcut. JSON output includes `delegated_to` so hosts can see the primary stage command it invoked.",
			"`--auto`/`--no-auto` override `execution.auto` for one run: auto-advances pure-pacing pauses (review batches and non-sensitive skill handoffs) on prior authorization and auto-confirms a pending workflow-preset upgrade-only (never downgraded), while sensitive/guardrail confirmations, the intake Approved Summary, and every evidence gate still hard-stop.",
		}},
	{ID: "status", Class: CommandClassQuery, Description: "Show lifecycle status, blockers, and next actions", Tier: "core", HasPromptSurface: true,
		Arguments:     "[--json] [--format text|yaml|json] [--focus <alias>] [--list-focuses] [--hydrate] [--hydrate-ref <skill-id>/<name>] [--root] [--stats] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "Can be used with or without an active change."}},
	{ID: "handoff", Class: CommandClassMutation, Description: "Write or show per-change advisory session handoff notes", Tier: "situational", HasPromptSurface: true,
		Arguments: "[write [--change <slug>] [--section <name>] | show [--change <slug>] [--json] [--brief]]",
		Notes: []string{
			"`slipway handoff` without a subcommand behaves as `slipway handoff write`.",
			"Write at meaningful moments: task completion, before stopping, before a review split, or when a context-pressure nudge asks for continuity.",
			"Read on resume with `slipway handoff show`; use `show --brief` for a bounded descriptor.",
			"This generated surface is hook-agnostic: run write/show yourself and never assume SessionStart, PreCompact, or any host hook fired.",
			"The handoff is advisory only; `slipway status` and `slipway next` remain lifecycle authority.",
		}},
	{ID: "done", Class: CommandClassMutation, Description: "Finalize a done-ready change and archive it", Tier: "core", HasPromptSurface: true,
		Arguments: "[--json] [--all-ready] [--change <slug>]"},
	// Setup (1)
	{ID: "init", Class: CommandClassMutation, Description: "Initialize runtime layout and optional tool artifacts", Tier: "setup", HasPromptSurface: true,
		Arguments:     "[--tools all|none|claude,cursor,...] [--refresh]",
		Prerequisites: []string{"Run from the target project root or any child directory inside it.", "The workspace must be inside a git working tree."}},
	// Situational (7)
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
	{ID: "validate", Class: CommandClassQuery, Description: "Read-only evidence and gate check", Tier: "situational", HasPromptSurface: true,
		Arguments:     "[--json] [--focus <alias>] [--list-focuses] [--format text|json] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "Can be used with or without an active change."}},
	{ID: "preset", Class: CommandClassMutation, Description: "Confirm or override the active change workflow preset", Tier: "situational", HasPromptSurface: true,
		Arguments:     "<light|standard|strict> [--json] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "An active governed change should already exist, or pass `--change <slug>`."}},
	{ID: "abort", Class: CommandClassMutation, Description: "Abort the active execution session without archiving the change", Tier: "situational", HasPromptSurface: true,
		Arguments:     "[--json] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "An active governed change must be in S2_IMPLEMENT; outside S2_IMPLEMENT use `slipway cancel` instead."}},
	{ID: "repair", Class: CommandClassMutation, Description: "Run safe local integrity and layout repairs", Tier: "situational", HasPromptSurface: true,
		Arguments:     "[--json] [--focus <alias>] [--list-focuses] [--format text|json]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)"}},
	{ID: "evidence", Class: CommandClassMutation, Description: "Record supported runtime and skill verification evidence", Tier: "situational", HasPromptSurface: true,
		Arguments:     "task --result-file <path> [--result-file <path> ...] [--json] [--change <slug>]; skill --skill <name> --verdict <pass|fail> [--reference <ref> ...] [--blocker <code[:detail]> ...] [--notes <text>|--notes-file <path>] [--json] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)", "`task` requires an active governed change in S2_IMPLEMENT with a materialized wave plan.", "`skill` requires an active governed change at the lifecycle state owned by the named governance skill; run-summary-bound skills also require current execution evidence."}},
	// Helpers (1)
	{ID: "tool", Class: CommandClassMutation, Description: "Run Slipway helper tools", Tier: "helpers", HasPromptSurface: false,
		Arguments:     "<helper> [helper flags]",
		Prerequisites: []string{"None — public CLI-only helper namespace used by generated skills. Individual helpers may require GitHub tokens, local files, or explicit confirmation."},
		Notes: []string{
			"`tool` is intentionally CLI-only: generated skills call `slipway tool ...` directly, but Slipway does not export `$slipway-tool` or host command prompt wrappers.",
		}},
	// Diagnostics (2)
	{ID: "health", Class: CommandClassQuery, Description: "Show repo-local integrity and repairability findings", Tier: "diagnostics", HasPromptSurface: true,
		Arguments:     "[--json] [--governance] [--all] [--observations] [--doctor] [--focus <alias>] [--list-focuses] [--format text|json] [--hydrate] [--hydrate-ref <skill-id>/<name>] [--change <slug>]",
		Prerequisites: []string{"`.slipway.yaml` must exist (run `slipway init` first)"}},
	// Discovery (1)
	{ID: "codebase-map", Class: CommandClassMutation, Description: "Create or refresh the durable repo-scoped codebase map", Tier: "discovery", HasPromptSurface: true,
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
	{ID: "independent-review", RenderMode: governanceRenderTemplated, WorkflowOwned: true},
	{ID: "security-review", RenderMode: governanceRenderTemplated, WorkflowOwned: true},
	{ID: "ship-verification", RenderMode: governanceRenderTemplated, WorkflowOwned: true},
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

var governanceSurfaceIDSet = func() map[string]struct{} {
	out := make(map[string]struct{}, len(governanceSurfaceDescriptors))
	for _, desc := range governanceSurfaceDescriptors {
		out[desc.ID] = struct{}{}
	}
	return out
}()

func isGovernanceSurfaceID(id string) bool {
	_, ok := governanceSurfaceIDSet[strings.TrimSpace(id)]
	return ok
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

var workflowLifecycleCommandIDs = []string{"new", "intake", "plan", "implement", "review", "fix", "done", "next", "run", "status"}

var workflowDiscoveryCommandIDs = []string{"codebase-map"}

var workflowSituationalCommandIDs = []string{
	"validate",
	"repair",
	"evidence",
	"handoff",
	"cancel",
	"delete",
	"preset",
	"abort",
}

var workflowDiagnosticCommandIDs = []string{"health", "instructions"}

var workflowSetupCommandIDs = []string{"init"}

type workflowSkillData struct {
	ToolID              string
	PublicName          string
	SkillIndexPath      string
	CommandsDir         string
	LifecycleCommands   []commandEntry
	DiscoveryCommands   []commandEntry
	SituationalCommands []commandEntry
	DiagnosticCommands  []commandEntry
	SetupCommands       []commandEntry
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
	"ship-verification":      {},

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

func exportedCapabilityRegistryForInstallClosure(
	reg *capability.Registry,
	closure skillInstallClosure,
) (*capability.Registry, error) {
	if reg == nil {
		return capability.NewRegistry()
	}
	skills := make([]capability.Skill, 0, reg.Len())
	for _, sk := range reg.All() {
		if shouldExportAsHostSkill(sk.ID) && closure.includesHostSkill(sk.ID) {
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
		workflowDiscoveryCommandIDs,
		workflowSituationalCommandIDs,
		workflowDiagnosticCommandIDs,
		workflowSetupCommandIDs,
	); err != nil {
		return workflowSkillData{}, err
	}

	lifecycle, err := buildWorkflowCommandEntries(workflowLifecycleCommandIDs, "core")
	if err != nil {
		return workflowSkillData{}, err
	}
	discovery, err := buildWorkflowCommandEntries(workflowDiscoveryCommandIDs, "discovery")
	if err != nil {
		return workflowSkillData{}, err
	}
	situational, err := buildWorkflowCommandEntries(workflowSituationalCommandIDs, "situational")
	if err != nil {
		return workflowSkillData{}, err
	}
	diagnostics, err := buildWorkflowCommandEntries(workflowDiagnosticCommandIDs, "diagnostics")
	if err != nil {
		return workflowSkillData{}, err
	}
	setup, err := buildWorkflowCommandEntries(workflowSetupCommandIDs, "setup")
	if err != nil {
		return workflowSkillData{}, err
	}

	return workflowSkillData{
		ToolID:              cfg.ID,
		PublicName:          adapterSkillName("workflow"),
		SkillIndexPath:      filepath.ToSlash(SkillIndexPath(cfg)),
		CommandsDir:         filepath.ToSlash(cfg.CommandsDir),
		LifecycleCommands:   lifecycle,
		DiscoveryCommands:   discovery,
		SituationalCommands: situational,
		DiagnosticCommands:  diagnostics,
		SetupCommands:       setup,
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
		tools := make([]string, 0, len(toolRegistry))
		for _, cfg := range Registry() {
			tools = append(tools, cfg.ID)
		}
		return tools, nil
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
	return GenerateWithInstallProfile(root, tools, refresh, SkillInstallProfileFull)
}

// GenerateWithInstallProfile creates tool skills and command surfaces for the
// given tools using an explicit generated-skill install profile. Command prompt
// files for hosts that expose commands outside skills remain complete; the
// profile applies only to generated SKILL.md surfaces.
func GenerateWithInstallProfile(root string, tools []string, refresh bool, profile SkillInstallProfile) error {
	closure, err := installProfileClosure(profile)
	if err != nil {
		return err
	}
	for _, tool := range tools {
		cfg, ok := toolRegistry[tool]
		if !ok {
			return fmt.Errorf("unsupported tool %q", tool)
		}
		if err := generateForTool(root, cfg, refresh, closure); err != nil {
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

func commandFileExtension(cfg ToolConfig) string {
	if ext := strings.TrimSpace(cfg.CommandExtension); ext != "" {
		if strings.HasPrefix(ext, ".") {
			return ext
		}
		return "." + ext
	}
	if cfg.CommandFormat == "toml" {
		return ".toml"
	}
	return ".md"
}

func commandEntryRelPath(cfg ToolConfig, id string) string {
	ext := commandFileExtension(cfg)
	switch cfg.CommandStyle {
	case "flat":
		return filepath.ToSlash(filepath.Join(cfg.CommandsDir, "slipway-"+id+ext))
	default:
		return filepath.ToSlash(filepath.Join(cfg.CommandsDir, "slipway", id+ext))
	}
}

func commandEntryPath(root string, cfg ToolConfig, id string) string {
	return filepath.Join(root, filepath.FromSlash(commandEntryRelPath(cfg, id)))
}

func generateForTool(root string, cfg ToolConfig, refresh bool, closure skillInstallClosure) (err error) {
	sentinelPath := filepath.Join(root, GeneratedAdapterMarkerPath(cfg))
	_, sentinelErr := os.Stat(sentinelPath)
	sentinelFound := sentinelErr == nil
	manifestFound := false
	if refresh || !sentinelFound {
		_, found, manifestErr := loadOwnershipManifest(root, cfg)
		if manifestErr != nil {
			return manifestErr
		}
		manifestFound = found
	}
	hadGeneratedAdapter := sentinelFound || manifestFound
	plan, err := newToolRefreshPlan(root, cfg, refresh, sentinelFound && !manifestFound)
	if err != nil {
		return err
	}
	if refresh && hadGeneratedAdapter && plan != nil {
		defer func() {
			if err == nil {
				return
			}
			if invalidateErr := invalidateFailedRefreshTrustSurfaces(root, cfg, sentinelPath, plan); invalidateErr != nil {
				err = errors.Join(err, invalidateErr)
			}
		}()
	}

	if refresh {
		// Purge direct command prompt surfaces before rewrite (project-local only).
		if cfg.CommandsDir != "" {
			if err := purgeCommandPromptSurfaces(root, cfg, plan); err != nil {
				return err
			}
		}

		if err := cleanupStaleGeneratedArtifacts(root, cfg, hadGeneratedAdapter, plan, closure); err != nil {
			return err
		}
	}

	// Governance skills (static content)
	// Includes both registry governance skills and standalone governance guidance skills.
	allStaticGovernance := append([]string{}, GovernanceSkillNames...)
	allStaticGovernance = append(allStaticGovernance, standaloneGovernanceNames...)
	for _, name := range allStaticGovernance {
		if !closure.includesHostSkill(name) {
			continue
		}
		content, err := tmpl.Content(sourceSkillTemplatePath(name, "SKILL.md"))
		if err != nil {
			return fmt.Errorf("load governance skill %q: %w", name, err)
		}
		content, err = renderSourceManagedSkill(content, name)
		if err != nil {
			return fmt.Errorf("canonicalize governance skill %q: %w", name, err)
		}
		skillPath := filepath.Join(root, SkillPath(cfg, name))
		if err := writeDeterministicWithPlan(plan, skillPath, content, refresh); err != nil {
			return err
		}
		if err := emitSkillSupportFiles(root, cfg, name, refresh, plan); err != nil {
			return fmt.Errorf("emit support files for governance skill %q (%s): %w", name, cfg.ID, err)
		}
	}

	// Templated governance skills (tool-aware .md.tmpl)
	for _, name := range TemplatedGovernanceSkillNames {
		if !closure.includesHostSkill(name) {
			continue
		}
		content, err := renderTemplatedGovernanceSkill(cfg, name)
		if err != nil {
			return fmt.Errorf("render templated governance skill %q for %s: %w", name, cfg.ID, err)
		}
		skillPath := filepath.Join(root, SkillPath(cfg, name))
		if err := writeDeterministicWithPlan(plan, skillPath, content, refresh); err != nil {
			return err
		}
		if err := emitSkillSupportFiles(root, cfg, name, refresh, plan); err != nil {
			return fmt.Errorf("emit support files for templated governance skill %q (%s): %w", name, cfg.ID, err)
		}
	}

	// Catalog skills (registry-owned, assembled from SKILL.md plus optional
	// typed templates in fixed order).
	reg := capability.DefaultRegistry()
	for _, id := range catalogSkillIDs {
		if !closure.includesHostSkill(id) {
			continue
		}
		if isGovernanceSurfaceID(id) {
			continue
		}
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
		if err := writeDeterministicWithPlan(plan, skillPath, content, refresh); err != nil {
			return err
		}
		if err := emitSkillSupportFiles(root, cfg, id, refresh, plan); err != nil {
			return fmt.Errorf("emit support files for catalog skill %q (%s): %w", id, cfg.ID, err)
		}
	}

	// Standalone skills (static content, not governance, not technique)
	for _, name := range standaloneNames {
		if !closure.includesHostSkill(name) {
			continue
		}
		if name == "workflow" {
			content, err := renderStandaloneWorkflowSkill(cfg)
			if err != nil {
				return fmt.Errorf("render standalone %q for %s: %w", name, cfg.ID, err)
			}
			skillPath := filepath.Join(root, SkillPath(cfg, name))
			if err := writeDeterministicWithPlan(plan, skillPath, content, refresh); err != nil {
				return err
			}
			if err := emitSkillSupportFiles(root, cfg, name, refresh, plan); err != nil {
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
			if err := writeDeterministicWithPlan(plan, refPath, refContent, refresh); err != nil {
				return err
			}
			continue
		}

		content, err := tmpl.Content(sourceSkillTemplatePath(name, "SKILL.md"))
		if err != nil {
			return fmt.Errorf("load standalone %q: %w", name, err)
		}
		p := filepath.Join(root, SkillPath(cfg, name))
		if err := writeDeterministicWithPlan(plan, p, content, refresh); err != nil {
			return err
		}
		if err := emitSkillSupportFiles(root, cfg, name, refresh, plan); err != nil {
			return fmt.Errorf("emit support files for standalone skill %q (%s): %w", name, cfg.ID, err)
		}
	}

	// Technique skills (static content)
	for _, name := range techniqueNames {
		if !closure.includesHostSkill(name) {
			continue
		}
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
		if err := writeDeterministicWithPlan(plan, p, content, refresh); err != nil {
			return err
		}
		if err := emitSkillSupportFiles(root, cfg, name, refresh, plan); err != nil {
			return fmt.Errorf("emit support files for technique skill %q (%s): %w", name, cfg.ID, err)
		}
	}

	// Namespace router skills keep the eager skill list small for profile-based
	// installs. They point agents back to command/host surfaces and never own
	// lifecycle transitions or evidence gates.
	for _, router := range closure.routerDefinitions() {
		content, err := renderSurfaceRouterSkill(cfg, router)
		if err != nil {
			return fmt.Errorf("render namespace router %q for %s: %w", router.ID, cfg.ID, err)
		}
		if err := writeDeterministicWithPlan(plan, filepath.Join(root, SkillPath(cfg, router.ID)), content, refresh); err != nil {
			return err
		}
	}

	// Command prompt surfaces (inline command prompts for prompt-backed adapters).
	// Skip when CommandsDir is empty (Codex uses command skills instead).
	if cfg.CommandsDir != "" {
		for _, id := range commandIDs() {
			content, err := renderCommandEntry(cfg, id)
			if err != nil {
				return fmt.Errorf("render command prompt %q for %s: %w", id, cfg.ID, err)
			}
			if err := writeDeterministicWithPlan(plan, commandEntryPath(root, cfg, id), content, refresh); err != nil {
				return err
			}
		}
	}

	// Command skill surfaces (Codex): one discoverable skill per command under
	// SkillsDir.
	if cfg.CommandSkillSurface {
		for _, id := range commandIDs() {
			if !closure.includesCommandSkill(id) {
				continue
			}
			content, err := renderCommandSkill(cfg, id)
			if err != nil {
				return fmt.Errorf("render command skill %q for %s: %w", id, cfg.ID, err)
			}
			if err := writeDeterministicWithPlan(plan, filepath.Join(root, SkillPath(cfg, id)), content, refresh); err != nil {
				return err
			}
		}
	}

	// Settings registration is keyed on whether the host owns a settings.json.
	//
	//   - Settings-capable hosts (claude, qwen): register each hook event as a
	//     bare inline `slipway hook <subcommand>` command directly in
	//     settings.json. No launcher script files are written, and on refresh any
	//     previously generated launcher files are pruned.
	//   - Pi registers generated skills/prompts in settings.json without hook
	//     semantics.
	//   - Settings-less hosts (cursor, opencode): keep emitting the advisory
	//     session-start launcher files (extensionless + .ps1 + .cmd), since they
	//     have no settings.json to register an inline command in.
	if strings.TrimSpace(cfg.SettingsPath) != "" {
		switch cfg.SettingsKind {
		case settingsKindPiRegistration:
			if err := mergePiRegistrationSettingsJSONWithPlan(root, cfg, refresh, plan); err != nil {
				return err
			}
		case settingsKindCodexHooks:
			if err := mergeCodexHooksTOMLWithPlan(root, cfg, refresh, plan); err != nil {
				return err
			}
		default:
			if refresh {
				if err := pruneHookLauncherFiles(root, cfg.SessionHook, plan); err != nil {
					return err
				}
				if err := pruneHookLauncherFiles(root, cfg.PostToolHook, plan); err != nil {
					return err
				}
			}
			if err := mergeHookSettingsJSONWithPlan(root, cfg, refresh, plan); err != nil {
				return err
			}
		}
	} else if strings.TrimSpace(cfg.SessionHook) != "" {
		for _, launcher := range hookLauncherOutputs(cfg.SessionHook, "session-start") {
			content, err := renderHookLauncher(cfg, launcher.template, root)
			if err != nil {
				return fmt.Errorf("render session hook launcher for %s: %w", cfg.ID, err)
			}
			p := filepath.Join(root, launcher.path)
			if err := writeDeterministicModeWithPlan(plan, p, content, refresh, launcher.mode); err != nil {
				return err
			}
		}
		if refresh {
			if err := cleanupLegacyHookLauncher(root, cfg.SessionHook, plan); err != nil {
				return err
			}
		}
	}

	// Workflow skill index (read by external agents; not consumed by the
	// Slipway kernel). Regenerated deterministically from the Go-owned
	// capability registry so every adapter sees direct host skill paths.
	exportedReg, err := exportedCapabilityRegistryForInstallClosure(reg, closure)
	if err != nil {
		return err
	}
	index := capability.BuildSkillIndexWithPaths(exportedReg, func(id string) string {
		return filepath.ToSlash(SkillPath(cfg, id))
	})
	indexPath := filepath.Join(root, SkillIndexPath(cfg))
	if err := writeDeterministicWithPlan(plan, indexPath, index, refresh); err != nil {
		return err
	}

	// Write sentinel last — a missing sentinel means the tree is invalid.
	if plan != nil {
		return plan.commit(sentinelPath, []byte("generated\n"))
	}
	return writeDeterministic(sentinelPath, "generated\n", true)
}

// purgeCommandPromptSurfaces removes all expected command prompt files for
// project-local adapters before rewrite. This ensures a failed refresh
// cannot leave previously trusted prompt surfaces in place.
func purgeCommandPromptSurfaces(root string, cfg ToolConfig, plan *toolRefreshPlan) error {
	if cfg.CommandsDir == "" {
		return nil
	}
	for _, id := range commandIDs() {
		if err := removePathIfExistsWithPlan(plan, commandEntryPath(root, cfg, id)); err != nil {
			return err
		}
	}
	return nil
}

func invalidateFailedRefreshTrustSurfaces(root string, cfg ToolConfig, sentinelPath string, plan *toolRefreshPlan) error {
	if err := plan.invalidateTrustedGeneratedFile(sentinelPath); err != nil {
		return err
	}
	if cfg.CommandsDir == "" {
		return nil
	}
	for _, id := range commandIDs() {
		if err := plan.invalidateTrustedGeneratedFile(commandEntryPath(root, cfg, id)); err != nil {
			return err
		}
	}
	return nil
}

func cleanupStaleGeneratedArtifacts(
	root string,
	cfg ToolConfig,
	hadGeneratedAdapter bool,
	plan *toolRefreshPlan,
	closure skillInstallClosure,
) error {
	if err := cleanupStaleSkillDirs(root, cfg, hadGeneratedAdapter, plan, closure); err != nil {
		return err
	}
	if err := cleanupStaleCommandEntries(root, cfg, hadGeneratedAdapter, plan); err != nil {
		return err
	}
	return nil
}

func cleanupStaleSkillDirs(
	root string,
	cfg ToolConfig,
	hadGeneratedAdapter bool,
	plan *toolRefreshPlan,
	closure skillInstallClosure,
) error {
	if !hadGeneratedAdapter {
		return nil
	}
	skillsRoot := filepath.Join(root, cfg.SkillsDir)
	if err := removePathIfExistsWithPlan(plan, filepath.Join(skillsRoot, "slipway")); err != nil {
		return err
	}

	expected := map[string]struct{}{}
	for _, meta := range closure.Skills {
		if meta.Kind == generatedSkillKindCommand && !cfg.CommandSkillSurface {
			continue
		}
		expected[meta.PublicName] = struct{}{}
	}
	if cfg.CommandSkillSurface {
		for _, meta := range closure.Skills {
			if meta.Kind == generatedSkillKindCommand {
				expected[meta.PublicName] = struct{}{}
			}
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
		if cfg.CommandSkillSurface && entry.IsDir() {
			cleaned, err := cleanupRetiredCommandSkillDir(plan, cfg, filepath.Join(skillsRoot, name))
			if err != nil {
				return err
			}
			if cleaned {
				continue
			}
		}
		if _, ok := managed[name]; ok {
			if err := removePathIfExistsWithPlan(plan, filepath.Join(skillsRoot, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

type retiredCommandSkillDir struct {
	generatedContent bool
	manifestTracked  bool
}

func cleanupRetiredCommandSkillDir(plan *toolRefreshPlan, cfg ToolConfig, dir string) (bool, error) {
	retired, ok, err := inspectRetiredCommandSkillDir(plan, cfg, dir)
	if err != nil || !ok {
		return ok, err
	}
	if retired.manifestTracked {
		return true, removePathIfExistsWithPlanPolicy(plan, dir, false)
	}
	if !retired.generatedContent {
		return true, nil
	}
	if err := removeVerifiedManifestlessRetiredCommandSkillDir(plan, dir); err != nil {
		return true, err
	}
	return true, nil
}

func inspectRetiredCommandSkillDir(plan *toolRefreshPlan, cfg ToolConfig, dir string) (retiredCommandSkillDir, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return retiredCommandSkillDir{}, false, nil
		}
		return retiredCommandSkillDir{}, false, err
	}
	skillPath := filepath.Join(dir, "SKILL.md")
	raw, err := os.ReadFile(skillPath) // #nosec G304 -- path is rooted under the adapter skills directory.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return retiredCommandSkillDir{}, false, nil
		}
		return retiredCommandSkillDir{}, false, err
	}

	content := normalizeTemplateLineEndings(string(raw))
	fm, _, err := splitSkillFrontmatter(content)
	if err != nil {
		return retiredCommandSkillDir{}, false, nil
	}
	var meta struct {
		Name      string `yaml:"name"`
		CommandID string `yaml:"command_id"`
		Surface   string `yaml:"surface"`
	}
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return retiredCommandSkillDir{}, false, nil
	}
	commandID := strings.TrimSpace(meta.CommandID)
	if commandID == "" || strings.TrimSpace(meta.Surface) != "skill" {
		return retiredCommandSkillDir{}, false, nil
	}
	if _, live := commandRegistryMap[commandID]; live {
		return retiredCommandSkillDir{}, false, nil
	}
	if filepath.Base(dir) != adapterSkillName(commandID) {
		return retiredCommandSkillDir{}, false, nil
	}
	if name := strings.TrimSpace(meta.Name); name != "" && name != adapterSkillName(commandID) {
		return retiredCommandSkillDir{}, false, nil
	}

	manifestTracked, err := manifestTracksPath(plan, skillPath)
	if err != nil {
		return retiredCommandSkillDir{}, false, err
	}
	generatedContent, err := commandSkillDirHasSingleGeneratedSkillFile(entries, content, cfg, commandID)
	if err != nil {
		return retiredCommandSkillDir{}, false, err
	}
	return retiredCommandSkillDir{
		generatedContent: generatedContent,
		manifestTracked:  manifestTracked,
	}, true, nil
}

func manifestTracksPath(plan *toolRefreshPlan, path string) (bool, error) {
	if plan == nil {
		return false, nil
	}
	rel, insideRoot, err := plan.relativePath(path)
	if err != nil || !insideRoot {
		return false, err
	}
	_, ok := plan.previousIndex[rel]
	return ok, nil
}

func commandSkillDirHasSingleGeneratedSkillFile(entries []os.DirEntry, content string, cfg ToolConfig, commandID string) (bool, error) {
	if len(entries) != 1 || entries[0].IsDir() || entries[0].Name() != "SKILL.md" {
		return false, nil
	}
	return isGeneratedCommandSkillContent(content, cfg, commandID)
}

func isGeneratedCommandSkillContent(content string, cfg ToolConfig, commandID string) (bool, error) {
	content = normalizeTemplateLineEndings(content)
	for _, sourceID := range commandIDs() {
		candidate, err := renderedCommandSkillAsRetiredCommand(cfg, sourceID, commandID)
		if err != nil {
			return false, err
		}
		if content == candidate {
			return true, nil
		}
	}
	if matchesLegacyGeneratedCommandSkillSignature(content, cfg, commandID) {
		return true, nil
	}
	return false, nil
}

const legacyGeneratedCommandSkillTrigger = "<legacy-command-trigger>"

type legacyGeneratedCommandSkillSignature struct {
	commandID              string
	description            string
	class                  string
	tier                   string
	includeInstallMetadata bool
	body                   string
}

// legacyGeneratedCommandSkillSignatures is a generated-content signature set for
// pre-manifest command-skill residue. Names alone are never deletion authority:
// callers only reach these signatures after parsing a command_id, confirming it
// is absent from the current command registry, and checking the full file body.
var legacyGeneratedCommandSkillSignatures = []legacyGeneratedCommandSkillSignature{
	{
		commandID:              "stats",
		description:            "Show repo-wide governance freshness and workflow statistics",
		class:                  string(CommandClassQuery),
		tier:                   "diagnostics",
		includeInstallMetadata: true,
		body: legacyGeneratedCommandSkillBody(
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
		),
	},
	{
		commandID:              "learn",
		description:            "Preview governance learning proposals from lifecycle evidence",
		class:                  string(CommandClassQuery),
		tier:                   "diagnostics",
		includeInstallMetadata: true,
		body: legacyGeneratedCommandSkillBody(
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
		),
	},
	{
		commandID:              "checkpoint",
		description:            "Set an active checkpoint to pause wave execution and request user input",
		class:                  string(CommandClassMutation),
		tier:                   "situational",
		includeInstallMetadata: true,
		body: legacyGeneratedCommandSkillBody(
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
		),
	},
	{
		commandID:   "pivot",
		description: "Reroute or rescope an active change",
		class:       string(CommandClassMutation),
		tier:        "situational",
		body: legacyGeneratedCommandSkillBody(
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
			"  `S2_EXECUTE`, `S3_REVIEW`, or `S4_VERIFY`; it returns the change to `S1_PLAN`",
			"  with discovery forced on. An invalid state is blocked (`pivot_state_invalid`).",
			"- `--rescope` is valid in `S2_EXECUTE`, `S3_REVIEW`, or `S4_VERIFY`; it returns",
			"  the change to `S0_INTAKE` (intake/clarify) and clears the intent",
			"  `## Approved Summary` so it must be re-confirmed. Before execution",
			"  (`S0_INTAKE`/`S1_PLAN`) and terminal states are blocked",
			"  (`rescope_state_invalid`).",
			"",
			"## Flags",
			"- `--reroute`: Re-evaluate routing/discovery and re-enter `S1_PLAN` (valid in S1_PLAN/S2_EXECUTE/S3_REVIEW/S4_VERIFY).",
			"- `--rescope`: Reopen intake — return to `S0_INTAKE` to amend scope, clearing the Approved Summary (valid in S2_EXECUTE/S3_REVIEW/S4_VERIFY).",
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
		),
	},
}

func legacyGeneratedCommandSkillBody(lines ...string) string {
	return strings.Join(lines, "\n")
}

func matchesLegacyGeneratedCommandSkillSignature(content string, cfg ToolConfig, commandID string) bool {
	content, ok := normalizeLegacyGeneratedCommandSkillTrigger(content, cfg, commandID)
	if !ok {
		return false
	}
	for _, signature := range legacyGeneratedCommandSkillSignatures {
		if signature.commandID != commandID {
			continue
		}
		if content == signature.content() {
			return true
		}
	}
	return false
}

func normalizeLegacyGeneratedCommandSkillTrigger(content string, cfg ToolConfig, commandID string) (string, bool) {
	triggerLine := "trigger: \"" + commandTrigger(cfg, commandID) + "\"\n"
	if strings.Count(content, triggerLine) != 1 {
		return "", false
	}
	normalized := "trigger: \"" + legacyGeneratedCommandSkillTrigger + "\"\n"
	return strings.Replace(content, triggerLine, normalized, 1), true
}

func (s legacyGeneratedCommandSkillSignature) content() string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: ")
	b.WriteString(adapterSkillName(s.commandID))
	b.WriteByte('\n')
	b.WriteString("description: ")
	b.WriteString(yamlDoubleQuoted(s.description))
	b.WriteByte('\n')
	if s.includeInstallMetadata {
		b.WriteString("install_profiles:\n")
		b.WriteString("  - full\n")
		b.WriteString("requires: []\n")
	}
	b.WriteString("command_id: \"")
	b.WriteString(s.commandID)
	b.WriteString("\"\n")
	b.WriteString("trigger: \"")
	b.WriteString(legacyGeneratedCommandSkillTrigger)
	b.WriteString("\"\n")
	b.WriteString("class: \"")
	b.WriteString(s.class)
	b.WriteString("\"\n")
	b.WriteString("tier: \"")
	b.WriteString(s.tier)
	b.WriteString("\"\n")
	b.WriteString("surface: \"skill\"\n")
	b.WriteString("---\n")
	b.WriteString(strings.TrimRight(s.body, "\n"))
	b.WriteString("\n\n")
	b.WriteString(commandSkillFooter(s.commandID))
	b.WriteByte('\n')
	return b.String()
}

func renderedCommandSkillAsRetiredCommand(cfg ToolConfig, sourceID, retiredID string) (string, error) {
	content, err := renderCommandSkill(cfg, sourceID)
	if err != nil {
		return "", err
	}
	return rewriteGeneratedCommandSkillIdentity(content, cfg, sourceID, retiredID)
}

func rewriteGeneratedCommandSkillIdentity(content string, cfg ToolConfig, fromID, toID string) (string, error) {
	content = normalizeTemplateLineEndings(content)
	replacements := []struct {
		old string
		new string
	}{
		{
			old: "name: " + adapterSkillName(fromID) + "\n",
			new: "name: " + adapterSkillName(toID) + "\n",
		},
		{
			old: "command_id: \"" + fromID + "\"\n",
			new: "command_id: \"" + toID + "\"\n",
		},
		{
			old: "trigger: \"" + commandTrigger(cfg, fromID) + "\"\n",
			new: "trigger: \"" + commandTrigger(cfg, toID) + "\"\n",
		},
		{
			old: commandSkillFooter(fromID),
			new: commandSkillFooter(toID),
		},
	}
	for _, replacement := range replacements {
		if count := strings.Count(content, replacement.old); count != 1 {
			return "", fmt.Errorf("generated command skill %q identity marker count for %q: got %d, want 1", fromID, replacement.old, count)
		}
		content = strings.Replace(content, replacement.old, replacement.new, 1)
	}
	return content, nil
}

func commandSkillFooter(commandID string) string {
	return fmt.Sprintf("Invoke the authoritative `slipway %s` CLI surface directly; do not reimplement Slipway lifecycle semantics.", commandID)
}

func removeVerifiedManifestlessRetiredCommandSkillDir(plan *toolRefreshPlan, dir string) error {
	if plan != nil {
		_, insideRoot, err := plan.relativePath(dir)
		if err != nil || !insideRoot {
			return err
		}
		plan.ops = append(plan.ops, fsutil.RemoveAllTransactionOp(dir))
		return nil
	}
	return removePathIfExists(dir)
}

func generatedSkillDirNameSet(cfg ToolConfig) map[string]struct{} {
	return allGeneratedSkillDirNameSet(cfg)
}

func cleanupStaleCommandEntries(root string, cfg ToolConfig, hadGeneratedAdapter bool, plan *toolRefreshPlan) error {
	if cfg.CommandsDir == "" {
		return nil
	}

	ext := commandFileExtension(cfg)
	expected := map[string]struct{}{}
	for _, id := range commandIDs() {
		expected[filepath.Base(commandEntryRelPath(cfg, id))] = struct{}{}
	}

	switch cfg.CommandStyle {
	case "flat":
		if err := cleanupPrefixedEntries(filepath.Join(root, cfg.CommandsDir), "slipway-", expected, plan); err != nil {
			return err
		}
		if hadGeneratedAdapter {
			return cleanupLegacyNestedCommandEntries(root, cfg, ext, plan)
		}
		return nil
	default:
		return cleanupUnexpectedEntries(filepath.Join(root, cfg.CommandsDir, "slipway"), expected, plan)
	}
}

func cleanupLegacyNestedCommandEntries(root string, cfg ToolConfig, ext string, plan *toolRefreshPlan) error {
	dir := filepath.Join(root, cfg.CommandsDir, "slipway")
	for _, id := range commandIDs() {
		// Flat adapters may have old nested command directories from pre-flat
		// generations. Without manifest proof, preserve them because nested
		// migration cleanup cannot distinguish generated files from user commands.
		if err := removePathIfExistsWithPlanPolicy(plan, filepath.Join(dir, id+ext), false); err != nil {
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
	if plan != nil {
		return nil
	}
	return os.Remove(dir)
}

func cleanupUnexpectedEntries(dir string, expected map[string]struct{}, plan *toolRefreshPlan) error {
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
		if err := removePathIfExistsWithPlan(plan, filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func cleanupPrefixedEntries(dir, prefix string, expected map[string]struct{}, plan *toolRefreshPlan) error {
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
		if err := removePathIfExistsWithPlan(plan, filepath.Join(dir, name)); err != nil {
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
	header += installMetadataFrontmatter(publicName)
	return "---\n" + header + fm + tail, nil
}

func installMetadataFrontmatter(publicName string) string {
	meta, ok := skillInstallMetadataByPublicName()[strings.TrimSpace(publicName)]
	if !ok {
		return ""
	}
	var b strings.Builder
	b.WriteString("install_profiles:")
	if len(meta.Profiles) == 0 {
		b.WriteString(" []\n")
	} else {
		b.WriteByte('\n')
		for _, profile := range meta.Profiles {
			b.WriteString("  - ")
			b.WriteString(string(profile))
			b.WriteByte('\n')
		}
	}
	if meta.AlwaysInstall {
		b.WriteString("always_install: true\n")
	}
	b.WriteString("requires:")
	if len(meta.Requires) == 0 {
		b.WriteString(" []\n")
		return b.String()
	}
	b.WriteByte('\n')
	for _, required := range meta.Requires {
		b.WriteString("  - ")
		b.WriteString(required)
		b.WriteByte('\n')
	}
	return b.String()
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
func emitSkillSupportFiles(root string, cfg ToolConfig, skillID string, refresh bool, plan *toolRefreshPlan) error {
	skillDirRel := filepath.Dir(SkillPath(cfg, skillID))
	dstBase := filepath.Join(root, skillDirRel)
	return emitSkillSupportFilesFromFSWithPlan(tmpl.TemplateFS(), skillID, dstBase, refresh, plan)
}

// emitSkillSupportFilesFromFSWithPlan is the testable core: it sources support
// files from an arbitrary fs.FS rooted like tmpl.TemplateFS() (so paths begin
// with "skills/<id>/...") and writes them under dstBase on the local
// filesystem.
func emitSkillSupportFilesFromFSWithPlan(srcFS fs.FS, skillID, dstBase string, refresh bool, plan *toolRefreshPlan) error {
	for _, sub := range optionalSkillSupportDirs {
		dstDir := filepath.Join(dstBase, sub)
		if refresh {
			if err := removePathIfExistsWithPlan(plan, dstDir); err != nil {
				return err
			}
		}
		if sub == "scripts" {
			continue
		}
		if sub == "references" {
			if err := emitSharedReferenceSupportFromFS(srcFS, skillID, dstDir, refresh, plan); err != nil {
				return fmt.Errorf("copy shared references for %q: %w", skillID, err)
			}
		}
		if err := copyTemplateSubtreeFromFS(srcFS, sourceSkillTemplatePath(skillID, sub), dstDir, refresh, plan); err != nil {
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

func emitSharedReferenceSupportFromFS(srcFS fs.FS, skillID, dstDir string, refresh bool, plan *toolRefreshPlan) error {
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
		if err := writeDeterministicWithPlan(plan, filepath.Join(dstDir, doc), string(content), refresh); err != nil {
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

func removePathIfExistsWithPlan(plan *toolRefreshPlan, name string) error {
	return removePathIfExistsWithPlanPolicy(plan, name, true)
}

func removePathIfExistsWithPlanPolicy(plan *toolRefreshPlan, name string, allowManifestlessBootstrap bool) error {
	if plan != nil {
		return plan.removeGeneratedPath(name, allowManifestlessBootstrap)
	}
	return removePathIfExists(name)
}

// copyTemplateSubtreeFromFS walks an embedded template directory and writes each
// file to dstDir preserving relative paths. Missing source directories are
// a no-op.
func copyTemplateSubtreeFromFS(tfs fs.FS, srcPrefix, dstDir string, refresh bool, plan *toolRefreshPlan) error {
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
		return writeDeterministicWithPlan(plan, filepath.Join(dstDir, filepath.FromSlash(rel)), string(content), refresh)
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

type surfaceRouterCommandData struct {
	ID          string
	Description string
	Trigger     string
	Tier        string
	Class       CommandClass
}

type surfaceRouterHostData struct {
	ID          string
	PublicName  string
	Path        string
	Description string
}

type surfaceRouterData struct {
	SkillID         string
	PublicName      string
	Title           string
	DescriptionYAML string
	ToolID          string
	Requires        []string
	Commands        []surfaceRouterCommandData
	HostSkills      []surfaceRouterHostData
	Notes           []string
}

func renderSurfaceRouterSkill(cfg ToolConfig, def namespaceRouterDefinition) (string, error) {
	meta, ok := skillInstallMetadataByPublicName()[adapterSkillName(def.ID)]
	if !ok {
		return "", fmt.Errorf("missing install metadata for namespace router %q", def.ID)
	}
	data := surfaceRouterData{
		SkillID:         def.ID,
		PublicName:      adapterSkillName(def.ID),
		Title:           def.Title,
		DescriptionYAML: yamlDoubleQuoted(def.Summary),
		ToolID:          cfg.ID,
		Requires:        append([]string(nil), meta.Requires...),
		Notes:           append([]string(nil), def.Notes...),
	}
	for _, id := range def.CommandIDs {
		meta, err := buildCommandRenderData(id)
		if err != nil {
			return "", err
		}
		data.Commands = append(data.Commands, surfaceRouterCommandData{
			ID:          meta.ID,
			Description: meta.Description,
			Trigger:     commandTrigger(cfg, meta.ID),
			Tier:        meta.Tier,
			Class:       meta.Class,
		})
	}
	for _, id := range def.HostSkillIDs {
		description := commandDescriptions[id]
		if description == "" {
			if sk, ok := capability.DefaultRegistry().Lookup(id); ok {
				description = sk.Summary
			}
		}
		data.HostSkills = append(data.HostSkills, surfaceRouterHostData{
			ID:          id,
			PublicName:  adapterSkillName(id),
			Path:        filepath.ToSlash(SkillPath(cfg, id)),
			Description: description,
		})
	}
	return tmpl.Render(sourceSkillTemplatePath("surface", "SKILL.md.tmpl"), data)
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
	codexHooksBlockStart       = "# BEGIN SLIPWAY MANAGED CODEX HOOKS"
	codexHooksBlockEnd         = "# END SLIPWAY MANAGED CODEX HOOKS"
)

// hookLaunch returns the command prefix and existence-probe binary that
// generated hook invocations use to launch Slipway from root. Inside the
// Slipway source module it returns ("go -C <root> run .", "go") so hooks track
// the worktree build; everywhere else it returns ("slipway", "slipway") so the
// inlined command resolves a release from PATH and the generated config stays
// machine-portable. The go-run prefix is used only when the module root is
// shell-safe (needs no quoting), so `go -C <root> run .` stays a bare,
// shell-operator-free token sequence valid across /bin/sh, cmd, and PowerShell.
//
// Both launcher-file hosts (cursor, opencode) and inline-config hosts (codex
// TOML, settings.json) adopt this prefix in-repo: a dogfooding checkout's
// generated config is per-machine, never shared, so tracking its own HEAD is
// strictly safer than a bare `slipway` that resolves a stale PATH release
// predating the current flag set. Launcher-file wrappers additionally probe
// `go` and force exit 0; the inline-config surfaces accept that a worktree which
// fails to compile makes `go run` exit non-zero — a rarer, self-evident edge for
// an in-repo developer than the version-skew break the go-run form removes.
func hookLaunch(root string) (prefix, probe string) {
	if dir := slipwaySourceModuleRoot(root); dir != "" && isShellSafePath(dir) {
		return "go -C " + dir + " run .", "go"
	}
	return "slipway", "slipway"
}

// applyHookLaunchPrefix rewrites the leading "slipway" token of an inline hook
// command to the launch prefix for root. It is a no-op for the release prefix,
// so non-repo generation keeps the bare, PATH-resolved command verbatim.
func applyHookLaunchPrefix(command, root string) string {
	prefix, _ := hookLaunch(root)
	if prefix == "slipway" {
		return command
	}
	if rest, ok := strings.CutPrefix(command, "slipway "); ok {
		return prefix + " " + rest
	}
	return command
}

// slipwaySourceModuleRoot returns the absolute path of root when root's go.mod
// declares the same module path this binary was built from, else "". Matching
// against the running binary's own module path (rather than a hardcoded
// "github.com/signalridge/slipway") makes in-repo detection fork-correct: a fork
// built and run from its own checkout reports its renamed module path, so its
// hooks dogfood the fork's HEAD instead of silently falling back to a release.
func slipwaySourceModuleRoot(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	self := selfModulePath()
	if self == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(abs, "go.mod")) // #nosec G304 -- reads the workspace's own go.mod to detect in-repo dogfooding.
	if err != nil {
		return ""
	}
	if modulePathFromGoMod(data) == self {
		return abs
	}
	return ""
}

// selfModulePath returns the Go module path this binary (or test binary) was
// built from, or "" when build information is unavailable.
func selfModulePath() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Path
	}
	return ""
}

// modulePathFromGoMod extracts the module path from go.mod contents, or "".
func modulePathFromGoMod(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "module")
		if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		rest = strings.TrimSpace(rest)
		if i := strings.Index(rest, "//"); i >= 0 {
			rest = strings.TrimSpace(rest[:i])
		}
		return strings.Trim(rest, "\"`")
	}
	return ""
}

// isShellSafePath reports whether the absolute path p can be embedded verbatim
// as a `go -C <p>` argument in a hook command string run by /bin/sh, cmd.exe, or
// PowerShell, without quoting. Callers pass filepath.Abs output, so p always
// starts with "/" or a drive letter — a leading '~' or '-' never occurs and only
// mid-token metacharacters matter. p is rejected if it contains whitespace, a
// control character, a character any of the three shells treats as active
// (quoting, expansion, redirection, command separation, globbing, or cmd's
// %VAR% expansion), or a backslash on a POSIX host (where '\' is the shell
// escape, not the path separator). On Windows '\' is the path separator and is
// allowed, so Windows absolute paths — including 8.3 short names like RUNNER~1 —
// stay launchable.
func isShellSafePath(p string) bool {
	if p == "" {
		return false
	}
	for _, r := range p {
		switch {
		case r <= ' ' || r == 0x7f:
			return false
		case r == '\\' && os.PathSeparator != '\\':
			return false
		case strings.ContainsRune("\"'`$&|;<>(){}[]*?!^%,#", r):
			return false
		}
	}
	return true
}

// stripSlipwayInvocation removes a recognized Slipway launcher head from an
// inline hook command and returns the remaining argument tail. It matches the
// bare/absolute binary forms ("slipway ...", "/abs/slipway ...") and the exact
// in-repo go-run form Slipway generates ("go -C <dir> run . ..."; see
// hookLaunch), so prune logic can identify a Slipway-owned command regardless of
// which launch prefix produced it. A bare `go run . ...` is deliberately not
// matched: Slipway never emits it, so a user-authored `go run . hook ...` hook
// is left untouched rather than misclassified as a stale Slipway entry.
func stripSlipwayInvocation(command string) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return "", false
	}
	if fields[0] == "go" {
		if len(fields) >= 5 && fields[1] == "-C" && fields[3] == "run" && fields[4] == "." {
			return strings.Join(fields[5:], " "), true
		}
		return "", false
	}
	if path.Base(strings.Trim(fields[0], `"'`)) == "slipway" {
		return strings.Join(fields[1:], " "), true
	}
	return "", false
}

func renderHookLauncher(cfg ToolConfig, templatePath, root string) (string, error) {
	prefix, probe := hookLaunch(root)
	data := map[string]string{
		"ToolID":       cfg.ID,
		"EntrySkill":   workflowEntryPublicName,
		"LaunchPrefix": prefix,
		"ProbeBin":     probe,
	}
	return tmpl.Render(templatePath, data)
}

func writeDeterministic(path, content string, refresh bool) error {
	return writeDeterministicWithPlan(nil, path, content, refresh)
}

func writeDeterministicWithPlan(plan *toolRefreshPlan, path, content string, refresh bool) error {
	return writeDeterministicModeWithPlan(plan, path, content, refresh, defaultFileModeForPath(path))
}

func writeDeterministicModeWithPlan(plan *toolRefreshPlan, path, content string, refresh bool, mode os.FileMode) error {
	if !refresh {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	if refresh && plan != nil {
		return plan.writeGeneratedFile(path, []byte(content), mode)
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

func mergePiRegistrationSettingsJSONWithPlan(root string, cfg ToolConfig, refresh bool, plan *toolRefreshPlan) error {
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

	settings["enableSkillCommands"] = true
	if err := mergeStringArraySetting(settings, "skills", "./skills"); err != nil {
		return fmt.Errorf("%s: %w", cfg.SettingsPath, err)
	}
	if err := mergeStringArraySetting(settings, "prompts", "./prompts"); err != nil {
		return fmt.Errorf("%s: %w", cfg.SettingsPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where searchable mode is intentional.
		return err
	}
	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	if refresh && plan != nil {
		return plan.writeUnmanagedFile(settingsPath, content, 0o644)
	}
	return os.WriteFile(settingsPath, content, 0o644) // #nosec G306 -- file is a user-facing project artifact where operator-readable mode is intentional.
}

func mergeCodexHooksTOMLWithPlan(root string, cfg ToolConfig, refresh bool, plan *toolRefreshPlan) error {
	settingsRoot := codexHookSettingsRoot(root)
	settingsPath := filepath.Join(settingsRoot, cfg.SettingsPath)
	if !refresh {
		if _, err := os.Stat(settingsPath); err == nil {
			return nil
		}
	}

	existing := ""
	if raw, err := os.ReadFile(settingsPath); err == nil { // #nosec G304 -- path is a project-local adapter config path.
		existing = string(raw)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	content := mergeManagedCodexHooksBlock(existing, renderCodexHooksBlock(cfg, root))
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project artifact location.
		return err
	}
	if refresh && plan != nil {
		return plan.writeUnmanagedFile(settingsPath, []byte(content), 0o644)
	}
	return os.WriteFile(settingsPath, []byte(content), 0o644) // #nosec G306 G703 -- project-local adapter config path with user-readable config permissions.
}

func codexHookSettingsRoot(root string) string {
	commonDir := state.GitCommonDir(root)
	if filepath.Base(commonDir) == ".git" {
		return filepath.Dir(commonDir)
	}
	return root
}

// renderCodexHooksBlock renders the managed Codex hooks TOML block. Inside the
// Slipway source tree the hook commands are rewritten to the in-repo go-run
// launch (see hookLaunch) so a dogfooding checkout always runs its own HEAD,
// not a stale PATH release. This is what keeps the hooks working under version
// skew: the bare `slipway hook ... --tool codex` form silently breaks the
// moment the PATH `slipway` predates the `--tool` flag (it exits non-zero with
// `unknown flag: --tool`, which Codex surfaces as a block), whereas go-run
// tracks the worktree and always understands the current flag set. `.codex/
// config.toml` is per-machine (gitignored), so the rewrite is rendered for the
// local OS and never shared across platforms. The residual edge — a worktree
// that fails to compile makes `go run` exit non-zero — is rarer and more
// self-evident for an in-repo developer than the version-skew break it
// replaces. Non-repo generation keeps the bare, PATH-resolved command verbatim.
func renderCodexHooksBlock(cfg ToolConfig, root string) string {
	sessionCommand := strings.TrimSpace(cfg.SessionHook)
	if sessionCommand == "" {
		sessionCommand = "slipway hook session-start --tool codex"
	}
	promptCommand := strings.TrimSpace(cfg.PostToolHook)
	if promptCommand == "" {
		promptCommand = "slipway hook context-pressure --tool codex"
	}
	sessionCommand = applyHookLaunchPrefix(sessionCommand, root)
	promptCommand = applyHookLaunchPrefix(promptCommand, root)
	return strings.TrimSpace(fmt.Sprintf(`%s
# Generated by Slipway. Hooks are inert until Codex trusts this repo and each hook; Slipway never edits global Codex trust.
[[hooks.SessionStart]]
hooks = [{ type = "command", command = %q }]

[[hooks.UserPromptSubmit]]
hooks = [{ type = "command", command = %q }]
%s
`, codexHooksBlockStart, sessionCommand, promptCommand, codexHooksBlockEnd)) + "\n"
}

func mergeManagedCodexHooksBlock(existing, block string) string {
	existing = strings.TrimRight(existing, "\n")
	start := strings.Index(existing, codexHooksBlockStart)
	end := strings.Index(existing, codexHooksBlockEnd)
	if start >= 0 && end >= start {
		end += len(codexHooksBlockEnd)
		merged := strings.TrimRight(existing[:start], "\n") + "\n\n" + strings.TrimSpace(block)
		if tail := strings.TrimLeft(existing[end:], "\n"); tail != "" {
			merged += "\n\n" + tail
		}
		return strings.TrimSpace(merged) + "\n"
	}
	if strings.TrimSpace(existing) == "" {
		return block
	}
	return existing + "\n\n" + block
}

func mergeStringArraySetting(settings map[string]any, key, value string) error {
	raw, ok := settings[key]
	if !ok || raw == nil {
		settings[key] = []any{value}
		return nil
	}

	items, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("%s field must be an array", key)
	}
	for _, item := range items {
		existing, ok := item.(string)
		if ok && existing == value {
			settings[key] = items
			return nil
		}
	}
	settings[key] = append(items, value)
	return nil
}

func mergeHookSettingsJSONWithPlan(root string, cfg ToolConfig, refresh bool, plan *toolRefreshPlan) error {
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

	// Inside the Slipway source tree these inline commands are rewritten to the
	// in-repo go-run launch (see hookLaunch / renderCodexHooksBlock) so a
	// dogfooding checkout runs its own HEAD instead of a stale PATH release that
	// may predate the current `--tool` flag. settings.json is per-machine, so the
	// rewrite is rendered for the local host. pruneStaleSlipwayHookCommands still
	// recognizes both the bare and the go-run forms across refreshes.
	sessionCmd := applyHookLaunchPrefix(sessionStartHookCommand, root)
	promptCmd := applyHookLaunchPrefix(contextPressureHookCommand, root)
	if strings.TrimSpace(cfg.SessionEvent) != "" && strings.TrimSpace(cfg.SessionHook) != "" {
		pruneStaleSlipwayHookCommands(hooks, cfg.SessionEvent, cfg.SessionHook, sessionCmd)
		mergeHookEventCommand(hooks, cfg.SessionEvent, sessionCmd)
	}
	if strings.TrimSpace(cfg.PostToolEvent) != "" && strings.TrimSpace(cfg.PostToolHook) != "" {
		pruneStaleSlipwayHookCommands(hooks, cfg.PostToolEvent, cfg.PostToolHook, promptCmd)
		mergeHookEventCommand(hooks, cfg.PostToolEvent, promptCmd)
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
	if refresh && plan != nil {
		return plan.writeUnmanagedFile(settingsPath, content, 0o644)
	}
	return os.WriteFile(settingsPath, content, 0o644) // #nosec G306 -- file is a user-facing project or governance artifact where operator-readable mode is intentional.
}

func legacyShellHookPath(basePath string) string {
	return filepath.ToSlash(strings.TrimSpace(basePath) + ".sh")
}

func cleanupLegacyHookLauncher(root, basePath string, plan *toolRefreshPlan) error {
	if strings.TrimSpace(basePath) == "" {
		return nil
	}
	return removePathIfExistsWithPlan(plan, filepath.Join(root, filepath.FromSlash(legacyShellHookPath(basePath))))
}

// hookLauncherFileSuffixes lists the extensions Slipway has ever emitted for a
// hook launcher beside the extensionless POSIX entry. Used to prune the
// now-retired launcher family for settings-capable hosts.
var hookLauncherFileSuffixes = []string{"", ".ps1", ".cmd", ".sh"}

// pruneHookLauncherFiles removes every Slipway-generated launcher file derived
// from basePath (the extensionless POSIX entry plus its .ps1/.cmd/.sh variants).
// It is the refresh-time orphan cleanup for settings-capable hosts that no
// longer emit launcher scripts.
func pruneHookLauncherFiles(root, basePath string, plan *toolRefreshPlan) error {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return nil
	}
	for _, suffix := range hookLauncherFileSuffixes {
		p := filepath.Join(root, filepath.FromSlash(basePath+suffix))
		if err := removePathIfExistsWithPlan(plan, p); err != nil {
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

// isStaleDirectHookCommand reports whether command is a Slipway-owned inline hook
// invocation for the same event as currentCommand but written with a different
// launcher or trailing flags (e.g. a bare `slipway hook session-start` left from a
// release install when the current command is the in-repo `go -C <root> run . hook
// session-start`, or a `--tool`/`|| exit 0` variant). Both are reduced to their
// post-launcher argument tail so the stale entry is pruned regardless of which
// launch prefix produced it, leaving the freshly merged currentCommand as the sole
// entry. Non-Slipway commands (no recognized launcher head) are never pruned.
func isStaleDirectHookCommand(command, currentCommand string) bool {
	command = strings.TrimSpace(command)
	currentCommand = strings.TrimSpace(currentCommand)
	if currentCommand == "" || command == currentCommand {
		return false
	}
	tail, ok := stripSlipwayInvocation(currentCommand)
	if !ok {
		return false
	}
	rest, ok := stripSlipwayInvocation(command)
	if !ok {
		return false
	}
	return rest == tail || strings.HasPrefix(rest, tail+" ")
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
	switch cfg.TriggerStyle {
	case "slash-colon", "at-colon":
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

// InvocationSummary describes, for humans, how generated command surfaces are
// invoked for this tool. `slipway init` prints it per selected tool so the
// invocation surface is explicit at setup time.
func (c ToolConfig) InvocationSummary() string {
	if c.CommandSkillSurface {
		if c.TriggerStyle != "" && c.TriggerStyle != "dollar-mention" {
			return fmt.Sprintf("invoke command skills as %s or via host skill picker", commandTrigger(c, "<command>"))
		}
		return "invoke skills: $slipway (entry), $slipway-<command> per command, or /skills"
	}
	switch c.TriggerStyle {
	case "slash-colon", "at-colon":
		return fmt.Sprintf("invoke commands as %s:<command>", c.TriggerPrefix)
	}
	return fmt.Sprintf("invoke commands as %s<command>", c.TriggerPrefix)
}
