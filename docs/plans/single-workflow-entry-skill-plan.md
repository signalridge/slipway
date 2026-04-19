# Single Workflow Entry Skill Plan

Status: Draft
Date: 2026-04-19

## Goal

Add one exported standalone Slipway skill that serves as the unified AI-first
entrypoint for the framework:

- one skill only
- no per-command companion skill family
- no second progression kernel
- existing CLI commands remain the only execution surface

This skill is an orchestration layer for AI agents. It explains how to enter
Slipway correctly, how to continue an active change, when to read governed host
skills, and when to use catalog or technique skills.

It does not replace commands, global prompts, governance hosts, or catalog
skills. It gives the AI one stable exported entry skill that can drive the
existing Slipway lifecycle correctly.

## Why This Direction

Slipway currently has three different entry surfaces:

1. project-local command prompt surfaces under `CommandsDir`
2. Codex global prompts under `PromptsStyle == "global"`
3. exported skills under `SkillsDir`

That is workable, but the current exported skills are not a first-contact
framework guide. They are:

- governed host skills selected by `ResolveNextSkill`
- outbound catalog skills selected from the capability registry
- standalone technique skills

What is missing is one exported orchestration skill that teaches an AI:

- what Slipway is
- what a governed change is
- how lifecycle progression works
- how to enter with `slipway new --json`
- how to inspect and continue governed work with `status`, `next`, and `run`
- when to hand off to `using-slipway-catalog.md`
- when to read `next_skill.prompt_path` and follow `next_skill.agent_hint`

The smallest correct shape is therefore:

- keep commands as commands
- keep the catalog manifest as a manifest
- keep governed hosts selected only by the runtime
- add exactly one standalone workflow skill

## Hard Constraints

1. This plan adds exactly one new exported entry skill. It does not create a
   `slipway-new`, `slipway-run`, `slipway-status`, or similar command-skill
   family.
2. The new entry skill is a standalone exported skill, not a governance skill
   and not a catalog skill. It must not enter the capability registry or the
   governance progression kernel.
3. `ResolveNextSkill`, `commandRegistry`, `cmd/*`, and the existing CLI runtime
   remain the only authorities for lifecycle progression and command behavior.
4. The new skill must only orchestrate existing commands. It must never
   reimplement state transitions, gate logic, review ordering, host selection,
   or command semantics in prompt text as if it were authoritative.
5. Existing command surfaces and Codex global prompts remain shipped. This plan
   adds a new AI-first entry skill; it does not remove, merge, or rename any
   command surface.
6. `using-slipway-catalog.md` remains a standalone manifest, not a skill. The
   workflow skill may instruct agents to read it, but this plan must not wrap
   it in `SKILL.md` semantics.
7. The workflow skill must derive command descriptions, arguments, and
   prerequisites from the shared toolgen command-metadata helpers backed by
   `commandRegistry`, not from hand-maintained duplicate prose and not from raw
   duplicate reads that bypass helper defaults.
8. Existing adapter discovery remains `SkillsDir`-based. This plan adds a new
   exported entry skill via the existing generated skill tree; it must not rely
   on Codex global prompts, capability/surface registries, or governance
   registry loading to make the skill discoverable.
9. The workflow skill must not hardcode nested vs flattened generated path
   assumptions. Any rendered manifest path examples must come from toolgen
   helpers such as `CatalogManifestPath(cfg)`, and those rendered paths must be
   treated as workspace-root-relative paths, not as paths relative to the
   workflow skill directory.
10. The main `SKILL.md` must stay concise. Heavy command detail belongs in a
    generated reference file, not duplicated inline in the primary workflow
    prompt.
11. Any `tool:` frontmatter added by this plan is adapter-facing metadata only.
    This feature must not introduce a new Slipway runtime consumer that depends
    on `tool:` for routing, authority, or state progression.
12. No rule-based router is introduced. The skill may present decision rules in
    prose, but the AI remains responsible for judgment and the CLI remains
    responsible for truth.
13. No compatibility layer is introduced. This is an additive entry skill, not
    a dual-runtime migration design.
14. Do not introduce a generic standalone `.tmpl` framework unless more than
    this single workflow skill actually needs it. The initial implementation
    should special-case the workflow standalone path only.
15. Public exported skill naming remains `slipway-<id>`. This plan keeps
    internal authored id `workflow` and exported adapter-facing name
    `slipway-workflow`. Do not introduce a special bare `slipway` public name
    without a separate naming-contract change.

## Current State

### Confirmed command/runtime facts

- `internal/toolgen/toolgen.go:commandRegistry` is the single checked-in source
  of truth for 17 commands.
- 14 commands currently have `HasPromptSurface: true`.
- The command tiers are:
  - core: `new`, `next`, `run`, `status`, `done`
  - situational: `init`, `cancel`, `review`, `validate`, `checkpoint`,
    `preset`, `pivot`, `abort`, `repair`
  - diagnostics: `stats`, `health`, `codebase-map`
- The nearby source comment currently says `// Situational (8)`, but the actual
  registry entries are 9. Tier grouping must follow the registry entries, not
  the stale comment.
- Codex has `CommandsDir == ""` and uses `PromptsStyle == "global"`.
- Other adapters ship project-local command prompt surfaces.
- AI-first governed entry already supports `slipway new --json` with caller
  supplied intent classification (`guardrail_domain`, `needs_discovery`,
  `complexity`). When classification is omitted, the CLI applies conservative
  safe-degrade defaults.
- `slipway next --json` returns `next_skill.prompt_path`,
  `next_skill.agent_hint`, and `next_skill.agent_definition_path`, so governed
  host execution already has an explicit handoff contract.

### Confirmed skill-generation facts

- `internal/toolgen/toolgen.go:standaloneNames` already exists and is the hook
  for standalone exported skills.
- `standaloneNames` is currently empty.
- The standalone generation loop currently loads only static
  `skills/<id>/SKILL.md`.
- Every generated adapter already has a `SkillsDir`, including Codex
  (`.codex/skills`). `PromptsStyle == "global"` changes command prompt emission,
  not skill-tree generation.
- `renderTemplatedGovernanceSkill(cfg, id)` already exists and uses
  `tmpl.Render(...)` followed by `renderSourceManagedSkill(...)`.
- `renderSourceManagedSkill(...)` is the current source-managed path that
  validates `skill_id`, preserves authored frontmatter, and injects
  adapter-visible `name` and `description`.
- Adapter-visible exported skill names are canonically `slipway-<id>` via
  `adapterSkillName(id)`.
- Tool-aware frontmatter is not universal across all templated skills today.
  It is used where a skill genuinely needs adapter-specific context.

### Confirmed support-file facts

- `emitSkillSupportFiles(...)` copies static `references/` and `scripts/`
  payloads from the embedded template tree.
- `emitSkillSupportFiles(...)` does not template-render support files.
- refresh mode removes destination `references/` and `scripts/` directories
  before repopulating them.
- refresh cleanup already treats `standaloneNames` as first-class generated
  skill directories, so adding `workflow` there is sufficient for lifecycle
  cleanup and stale-dir pruning.
- `shouldSkipSupportArtifact(...)` currently skips Python cache artifacts only;
  it does not skip `.tmpl`.
- no current skill support subtree ships `.tmpl` files under `references/` or
  `scripts/`, so raw-template leakage has not surfaced yet.
- The generated skill-tree inventory classifies `SKILL.md` as `skill_md` and
  files under `/references/` as `reference`.
- `internal/toolgen/testdata/skill_tree_inventory.codex.golden` includes an
  `inventory_sha256` footer that changes when the generated skill tree changes.

### Confirmed manifest and architecture facts

- `using-slipway-catalog.md` is a one-way export manifest generated by
  `BuildCatalogManifest()`.
- The kernel does not read this file back.
- `CatalogManifestPath(cfg)` and `SkillPath(cfg, id)` both return
  workspace-root-relative paths under the active adapter tree. They are not
  relative-to-skill references.
- Governance progression remains owned by `ResolveNextSkill`.
- Catalog skills do not replace the governance kernel.
- `internal/engine/skill/registry_loader.go` is governance-overlay-only: it
  parses generated `slipway-*` skills only when their `skill_id` maps back to a
  known governance default, and ignores unknown standalone/catalog ids.
- `renderSourceManagedSkill(...)` preserves authored frontmatter fields other
  than `name` and `description`, so `tool:` survives generation; no current
  Slipway runtime component consumes `tool:` as a routing field.
- The missing layer is not more work skills. It is one AI-facing workflow skill
  that teaches agents how to use the existing command/runtime surfaces.

## Chosen Approach

Introduce one standalone exported skill with:

- internal authored id: `workflow`
- exported adapter-facing name: `slipway-workflow`

Generated authored source tree:

```text
internal/tmpl/templates/skills/workflow/
├── SKILL.md.tmpl
├── command-reference.md.tmpl
└── references/
    └── [future static reference payloads only]
```

Generated output tree per adapter:

```text
<SkillsDir>/slipway-workflow/
├── SKILL.md
└── references/
    └── command-reference.md
```

The main `SKILL.md` is the stable AI entry layer. It explains:

1. what Slipway is
2. what a governed change is
3. how lifecycle states and done-ready progression work
4. how intent classification works at `slipway new --json`
5. how skill classes differ (governance, catalog, technique, standalone)
6. how to decide between `status`, `new`, `next`, `run`, and `done`
7. when to read `using-slipway-catalog.md`
8. when to read `next_skill.prompt_path` and dispatch via
   `next_skill.agent_hint`

The reference file contains the detailed command reference for all 17 commands,
including:

- name
- description
- arguments
- prerequisites

The command-reference file is generated from shared toolgen command metadata
helpers backed by `commandRegistry` and its defaulting helpers, not copied from
handwritten prose. The main skill keeps only a concise summary table and links
to the reference file when detailed command contract data is needed.

The main `SKILL.md` is intentionally a thin routing layer. It teaches entry,
handoff, and authority boundaries, but it does not restate the full per-command
workflows or copy the existing tool-specific dispatch prose already owned by
the generated command surfaces and governed host skills.

## Implementation Steps

### Step 1: Add one standalone workflow skill export

Files:

- `internal/toolgen/toolgen.go`
- `internal/tmpl/templates/skills/workflow/`

Required updates:

- add `workflow` to `standaloneNames`
- create the authored workflow source tree:

```text
internal/tmpl/templates/skills/workflow/
├── SKILL.md.tmpl
├── command-reference.md.tmpl
└── references/
    └── [future static reference payloads only]
```

- export exactly one new standalone skill from this plan
- do not add any command-backed companion skills to `standaloneNames`

### Step 2: Add a workflow-specific standalone render path

Files:

- `internal/toolgen/toolgen.go`

Required updates:

- add `renderStandaloneWorkflowSkill(cfg ToolConfig) (string, error)`
- implement it with the same source-managed pattern already used by templated
  governance skills:
  - build a workflow data struct
  - call `tmpl.Render(sourceSkillTemplatePath("workflow", "SKILL.md.tmpl"), data)`
  - pass the rendered result through `renderSourceManagedSkill(...)`
- add a second explicit render path for the generated reference file:
  - `renderStandaloneWorkflowCommandReference(cfg ToolConfig) (string, error)`
  - call `tmpl.Render(sourceSkillTemplatePath("workflow", "command-reference.md.tmpl"), data)`
- special-case `workflow` inside the standalone generation loop
- keep all other standalone skills on the existing static path
- make the workflow branch order explicit:

```go
if name == "workflow" {
    skillContent := renderStandaloneWorkflowSkill(cfg)
    skillPath := filepath.Join(root, SkillPath(cfg, "workflow"))
    writeDeterministic(skillPath, skillContent, refresh)

    // Copy future static references/scripts first. In refresh mode this
    // recreates the support directories from scratch.
    emitSkillSupportFiles(root, cfg, "workflow", refresh)

    refContent := renderStandaloneWorkflowCommandReference(cfg)
    refPath := filepath.Join(
        root,
        filepath.Dir(SkillPath(cfg, "workflow")),
        "references",
        "command-reference.md",
    )
    writeDeterministic(refPath, refContent, refresh)
} else {
    // existing static standalone path
}
```
- keep the templated reference source outside the copy-managed
  `workflow/references/` subtree so raw `.tmpl` files can never leak into the
  generated skill tree

Non-goals for this step:

- do not add generic standalone `.tmpl` auto-detection
- do not create a general command-backed skill factory
- do not teach `emitSkillSupportFiles(...)` to template-render references
  globally
- do not introduce a bare `slipway` exported-name exception for this feature

Rationale:

- only one standalone skill currently needs templated rendering
- `renderSourceManagedSkill(...)` already carries the frontmatter and
  validation contract that generated skills must preserve
- `emitSkillSupportFiles(...)` is a static copier, so the generated command
  reference must be written explicitly rather than delegated to the generic
  support-file emitter
- keeping `command-reference.md.tmpl` outside the copy-managed support subtree
  solves the raw-template leakage problem without changing generic support-file
  copy behavior

### Step 3: Define the workflow template data contract

Files:

- `internal/toolgen/toolgen.go`

Required data types:

```go
type workflowSkillData struct {
    ToolID              string
    PublicName          string
    CatalogManifestPath string
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
```

Field semantics:

- `ToolID`: adapter id such as `claude`, `codex`, or `cursor`
- `PublicName`: exported adapter-facing name, always `slipway-workflow`
- `CatalogManifestPath`: generated by `CatalogManifestPath(cfg)`
- `LifecycleCommands`: `new`, `status`, `next`, `run`, `done`
- `SupportingCommands`: `review`, `validate`, `repair`, `init`, `cancel`,
  `checkpoint`, `preset`, `pivot`, `abort`
- `DiagnosticCommands`: `stats`, `health`, `codebase-map`

Population rules:

- source command metadata through a shared helper path backed by
  `commandRegistry`, `commandDescriptions`, `commandArguments`, and
  `commandPrerequisites`
- if a dedicated workflow-specific builder is added, it must wrap the existing
  helper path rather than duplicating raw registry reads
- preserve a deterministic display order using explicit ID lists per group
- validate that grouped commands still match the expected registry tier
  boundaries
- correct the stale `// Situational (8)` source comment to `// Situational (9)`
  while touching the grouping logic so the code no longer misleads future
  readers
- use `Arguments` and `Prerequisites` in the reference file
- use only `Name` and `Description` in the primary `SKILL.md`

### Step 4: Author the main workflow skill template

Files:

- `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl`

Required frontmatter:

- `skill_id: workflow`
- `name: slipway-workflow`
- trigger-oriented `description` that tells the AI when this entry skill should
  load, including concrete first-contact and lifecycle-navigation contexts
  rather than only a summary sentence
- `tool: "{{.ToolID}}"` because this workflow skill is intentionally
  tool-aware: Codex uses global prompts while other adapters may have
  project-local command prompt surfaces

Required content:

- framework introduction:
  - Slipway is a governance CLI for AI-assisted software delivery
  - a "change" is the governed work unit
  - lifecycle states and done-ready / done semantics
  - governed entry uses `slipway new --json` when the AI can provide intent
    classification directly
  - intent classification basics:
    `guardrail_domain`, `complexity`, `needs_discovery`
  - skill taxonomy:
    governance vs catalog vs technique vs standalone
- runtime authority boundaries:
    `ResolveNextSkill` selects governed host skills, `commandRegistry` owns
    command metadata, and the catalog manifest is outbound only
- lifecycle routing:
  - provide a concise route table only:
    when uncertain whether a change already exists -> `slipway status --json`
  - no active change and governed work should begin -> `slipway new --json`
  - active change and step-by-step governed work needed -> `slipway next --json`
  - governed host execution -> read `next_skill.prompt_path`
  - agent dispatch -> use `next_skill.agent_hint` and
    `next_skill.agent_definition_path` when present
  - continuous governed advance -> `slipway run`
  - completion only after done-ready -> `slipway done`
  - do not duplicate the full command workflow bodies already owned by the
    generated command surfaces
- detailed contract delegation:
  - when command-level semantics are needed, read the generated command surface
    for that command instead of re-explaining it inline
  - for Codex this means the existing global prompt files under
    `~/.codex/prompts/slipway-<command>.md`
  - for project-local adapters this means the generated command entry under the
    adapter `CommandsDir`
- work-skill handoff:
  - read the generated catalog manifest when catalog triage is needed
  - use technique skills as posture/procedure overlays, not as lifecycle
    authorities
- activation-scoping sections:
  - include `When to Use` with concrete first-contact entry cases
  - include `When NOT to Use` with named alternatives once a more specific
    governed host, catalog skill, technique skill, or command surface is
    clearly the right tool
- concise command summary table:
  - name + description only
  - full arguments/prerequisites moved to `references/command-reference.md`
- hard rules:
  - do not bypass gates
  - do not guess the next governed host without `slipway next --json`
  - do not treat the workflow skill as progression authority
  - do not invent command semantics not present in the CLI
  - do not duplicate long command reference prose or tool-specific command body
    prose in the main skill body

Content-shape target:

- keep the primary `SKILL.md` to framework overview + routing + concise command
  summary + hard rules
- move detailed command contracts into the generated reference file so the main
  skill remains readable and trigger-efficient
- keep adapter-specific wording narrow: use `tool:` only to explain surface
  differences such as Codex global command prompts vs project-local command
  entries, not as a runtime control field

### Step 5: Author the generated command reference template

Files:

- `internal/tmpl/templates/skills/workflow/command-reference.md.tmpl`

Required content:

- one generated reference document covering all 17 commands
- for each command include:
  - description
  - arguments
  - prerequisites
- group commands into:
  - lifecycle core
  - supporting commands
  - diagnostics
- clearly state that the reference is derived from `commandRegistry`
- clearly state that the rendered values come through the shared toolgen helper
  path so commands with helper-default prerequisites stay aligned with the
  command surfaces
- do not duplicate this level of detail back into the main `SKILL.md`

Rendering contract:

- output path must be:
  `<SkillsDir>/slipway-workflow/references/command-reference.md`
- this file is template-rendered, not static-copied
- the source template intentionally lives outside the copy-managed
  `workflow/references/` subtree
- any future static support payloads under `workflow/references/` or
  `workflow/scripts/` may still flow through `emitSkillSupportFiles(...)`, but
  `command-reference.md` itself must be rendered explicitly from the workflow
  data contract

### Step 6: Keep the workflow skill outside runtime authorities

Files:

- `internal/engine/capability/registry.go`
- `internal/engine/capability/surfaces.go`
- `internal/engine/progression/skill_resolution.go`
- `internal/engine/skill/registry_loader.go`

Required outcome:

- no new capability-registry entry for the workflow skill
- no new surface-policy alias for the workflow skill
- no changes to `ResolveNextSkill`
- no changes to governance host ordering
- no changes to command behavior, gate semantics, or active-change rules
- no change to `internal/engine/skill/registry_loader.go`; it remains
  governance-overlay-only and must not be widened to register standalone
  workflow skills
- adapter discovery remains existing `SkillsDir` discovery; do not add a second
  discovery path through Codex global prompts or runtime registries

This step is explicit because the main risk of this feature is accidental
runtime expansion into a second workflow engine.

### Step 7: Regenerate and cover the new surface with tests

Files likely affected:

- `internal/toolgen/toolgen_test.go`
- `internal/toolgen/support_files_test.go`
- `internal/toolgen/adapter_contract_test.go`
- `internal/toolgen/testdata/skill_tree_inventory.codex.golden`
- adapter-specific init/path tests that assert the generated skill tree

Required test coverage:

- the new standalone workflow skill is generated for every adapter that exports
  skills
- Codex emits `.codex/skills/slipway-workflow/SKILL.md` even though its command
  prompts remain global under `~/.codex/prompts/`
- no `slipway-workflow` global prompt is emitted; Codex global prompts remain
  command-only
- the rendered workflow skill carries the source-managed frontmatter contract
  (`name`, `skill_id`, `description`) plus the tool-aware `tool` field
- `tool:` preservation is verified as emitted adapter metadata only; no new
  runtime behavior depends on reading it
- the rendered workflow skill includes the framework introduction and lifecycle
  command summary
- the generated reference file exists at:
  `slipway-workflow/references/command-reference.md`
- the generated reference file contains arguments and prerequisites for
  representative commands such as `new`, `status`, `next`, `run`, `done`,
  `review`, and `repair`
- commands that rely on helper-default prerequisites still show those defaults
  in the generated workflow reference
- no per-command workflow skills are generated
- no raw `command-reference.md.tmpl` file is emitted into the generated skill
  tree
- `LoadGovernanceRegistry(...)` behavior is unchanged: the standalone workflow
  export is not treated as a governance registry entry
- the inventory golden reflects exactly one new standalone skill plus its
  generated reference file
- the inventory footer checksum (`inventory_sha256`) is updated accordingly
- `emitSkillSupportFiles(...)` remains a static copier; workflow templated
  reference generation is covered by explicit workflow render-path tests rather
  than by changing generic support-file behavior
- the workflow skill renders the catalog manifest location from
  `CatalogManifestPath(cfg)` as a workspace-root-relative path and does not
  emit relative-to-skill path guesses such as `../using-slipway-catalog.md`

Expected new inventory entries:

```text
slipway-workflow/SKILL.md	skill_md	non-exec
slipway-workflow/references/command-reference.md	reference	non-exec
```

## Scope Exclusions

- Do not add `slipway-new`, `slipway-run`, `slipway-status`, or any similar
  command-backed skill family.
- Do not add a command-to-skill generation framework for the whole command
  registry.
- Do not move existing command prompt surfaces into the skill tree.
- Do not remove commands or Codex global prompts.
- Do not turn `using-slipway-catalog.md` into a skill.
- Do not add a second lifecycle kernel, hidden route table, or prompt-owned
  state machine.
- Do not introduce rule-based production routing in place of AI judgment plus
  CLI truth.
- Do not add generic standalone `.tmpl` detection beyond the workflow
  special-case.
- Do not teach `emitSkillSupportFiles(...)` to render templates globally.

## Verification

1. `go test ./...` passes.
2. `go run . init --tools all --refresh` succeeds.
3. Exactly one new exported standalone workflow skill is emitted.
4. No `slipway-new`, `slipway-run`, `slipway-status`, or similar companion
   skills are emitted.
5. The workflow skill content includes the framework introduction:
   Slipway purpose, governed change concept, lifecycle states, intent
   classification, skill taxonomy, and runtime authority boundaries.
6. The rendered workflow skill frontmatter includes a concrete adapter-resolved
   `tool:` value, such as `tool: "codex"` or `tool: "claude"`.
7. The workflow skill is emitted under each adapter's existing `SkillsDir`; for
   Codex that means `.codex/skills/slipway-workflow/SKILL.md`, while global
   prompts remain command-only.
8. No `slipway-workflow` file is emitted into `~/.codex/prompts/`.
9. The workflow skill content includes lifecycle routing for `status`, `new`,
   `next`, `run`, and `done`.
10. The workflow skill content includes governed handoff rules for
   `next_skill.prompt_path`, `next_skill.agent_hint`, and
   `using-slipway-catalog.md`.
11. The main workflow skill contains only the concise command summary table,
   while the generated reference file contains detailed arguments and
   prerequisites for all 17 commands.
12. The workflow skill references the generated catalog manifest path through
    toolgen-rendered workspace-root-relative data rather than a hard-coded
    layout assumption or a relative-to-skill guess.
13. The generated command reference stays aligned with helper-default command
    metadata, including default prerequisites where the command surfaces would
    show them.
14. The generated inventory golden contains:
    `slipway-workflow/SKILL.md` and
    `slipway-workflow/references/command-reference.md`, and its
    `inventory_sha256` footer is refreshed.
15. No raw `command-reference.md.tmpl` or other workflow template source file
    appears in the generated skill tree.
16. The workflow template source remains outside the copy-managed
    `workflow/references/` subtree, and the workflow branch writes the rendered
    reference after `emitSkillSupportFiles(...)` completes.
17. No command semantics, governance ordering, capability/surface registries,
    or governance registry-loader behavior change as part of this feature.
