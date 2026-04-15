# Route Surface Refactor Plan

**Status.** Proposed. If adopted, this plan supersedes the raw
`--mode=<skill-id>` / `--view=<skill-id>` assumptions in the active
strengthening-wave plans:

- `2026-04-15-skills-wave2-plan*.md`
- `2026-04-15-skills-wave3-plan*.md`

This plan does **not** change Slipway's progression kernel. `ResolveNextSkill`
remains the only progression authority. The refactor is limited to the
user-facing route surface and the internal classification that feeds it.

Unless noted otherwise, every `*.md` doc / plan glob in this plan includes the
EN and zh-CN variants when both exist.

## 1. Problem

Current route semantics mix four different concerns in one mechanism:

- primary command routing
- optional advanced analyses
- read-only diagnostic views
- support attachments

That overloading creates three concrete problems in current code:

- The catalog exposes too many raw skill IDs on public CLI surfaces.
  There are 25 catalog skills total; 19 are currently admitted on at least one
  route/view surface, and 13 of those are `command-manual` entries.
- `command-manual` is not truly "manual". A skill with
  `BindingCommandManual` can still compete for `Supports` when its trigger
  clause matches, because support attachment selection currently considers both
  `BindingCommandAuto` and `BindingCommandManual`.
- `status` / `health` admit non-view skills through `ValidViewsForCommand`,
  which means checklist/procedure-oriented skills can masquerade as view
  selectors even though they are not read-only projections.

The result is a route layer that is conservative in the wrong place
(`13` manual selectors) and too leaky in the wrong place (publicly exposing
internal skill IDs).

## 2. Goals

- Hide raw catalog skill IDs from normal CLI usage.
- Keep the Go-owned catalog registry as the internal skill authority.
- Make each command surface legible:
  one default primary route, bounded suggestions, a small explicit focus set,
  and true read-only views only.
- Stop `command-manual` from behaving like a semi-automatic support channel.
- Reduce the public advanced selector set to a small number of user-comprehensible
  names rather than 10+ internal IDs.

## 3. Non-goals

- No second progression kernel.
- No new catalog skills.
- No broad rewrite of the trigger DSL.
- No change to host-embedded / technique-hint behavior. (Verified: the current
  registry does not use `mode:<cmd>:<id>` / `view:<cmd>:<id>` prefixed binding
  targets anywhere; all bindings use bare command names. The surface split
  below lands entirely in the new policy layer and does not touch binding
  target strings.)
- No selector compatibility layer for raw `skill-id` syntax once the public
  surface flips; this plan is a corrective hard cut, not a deprecate-then-wait
  migration.

## 4. Target Model

### 4.1 Two-layer authority

Keep two distinct authorities:

- **Skill registry** (`internal/engine/capability/registry*.go`)
  - owns internal skill identity, triggers, evidence contract, and
    host/support attachment behavior.
- **Surface policy registry** (new)
  - owns user-facing command exposure:
    `primary`, `suggested`, `explicit focus`, and `view`.
  - in this plan family, every policy record resolves directly to a catalog
    skill (`backing_id=<skill-id>`). If a later approved plan introduces a real
    command-owned diagnostics implementation, it must widen the schema in that
    later plan instead of pre-allocating a second backing kind here.

The skill registry stays internal and descriptive.
The surface policy registry becomes the sole authority for what the operator
can explicitly select.
This hard cut only requires one public `view` alias (`incident` ->
`incident-response`). No command-owned diagnostic view abstraction is shipped in
this refactor.

Route selection follows that split:

- default primary route/view resolution consults the surface policy registry,
  not `BindingCommandManual`
- explicit focus/view aliases resolve to their surface-policy backing record
  before hydrate lookup, diagnostics rendering, or help generation
- `BindingCommandAuto` remains internal metadata for command-scoped automatic
  candidates, but does not by itself grant public selector exposure
- `BindingCommandManual` becomes transition-only metadata once PR-1 lands:
  it no longer feeds `Supports`, it is never consulted for explicit focus
  resolution, and PR-3 removes the remaining entries and the type from
  command-surface validation once reclassification is complete
- surface policy records own hydrate behavior for explicit public selectors.
  In this plan family all public selectors are skill-backed and hydrate from
  their backing skill; no explicit alias may fall back to "try
  `HydrateReferenceKeysForSkill(view)` and hope it exists"

### 4.2 Exposure classes

Define four exposure classes:

- **Primary**
  - One default routed behavior per command surface.
  - Chosen automatically with no operator flag.
- **Suggested**
  - Surfaced as optional guidance based on objective signals or user text.
  - Never becomes the command's primary route by itself.
  - Not directly selected by raw skill ID.
- **Explicit focus**
  - Small opt-in set for heavy or specialized analyses that should never run
    silently.
  - Selected by user-facing aliases, not by raw skill IDs.
- **View**
  - Read-only diagnostic landing zones only.
  - Reserved for `status` / `health`.
  - Auto-selected views apply only when `status` / `health` are evaluating a
    concrete active/selected change. Diagnostics with no active change may
    legitimately return no selected view.

`change-scoped` is enforced at the command layer, not by a new resolver
`Signals` field. `status` / `health` only consult auto-view policy after they
have resolved a concrete active or explicitly selected change target; the
diagnostics-without-change path bypasses auto-view selection entirely.

### 4.3 Public CLI syntax

Refactor public CLI syntax to:

- `review --focus <name>`
- `validate --focus <name>`
- `repair --focus <name>`
- `status --view <name>`
- `health --view <name>`

Add:

- `review --list-focuses`
- `validate --list-focuses`
- `repair --list-focuses`
- `status --list-views`
- `health --list-views`

`--mode` is retired from public docs and help text. Raw `skill-id` values are
not shown as the canonical selector syntax anywhere in user-facing help.

`--focus` is registered only on `review` / `validate` / `repair`, `--view`
only on `status` / `health`, `--list-focuses` only on the focus-bearing
commands, and `--list-views` only on the view-bearing commands. Cross-command
misuse always fails at parse time as an ordinary unknown-flag error. This plan
does not preserve a second "usage error" path for discovery flags, and it does
not return empty lists on unsupported surfaces.

### 4.4 Output contract

Add a stable, user-facing suggestion channel to routed command outputs:

- JSON:
  - `suggested_capabilities[]` with:
    `name`, `summary`, `reason`, `kind` (`suggested` or `explicit_focus`)
- text:
  - `Suggested:` block listing user-facing names and one-line reasons

Constraints (lock these before PR-1):

- **Cap:** at most 3 entries, mirroring the resolver's existing `Supports`
  cap in `Resolve()`.
- **Order:** stable by (clause score desc, skill id asc) — same tiebreak the
  current `Resolve()` match ordering already uses.
- **Disjoint from `Supports` (deterministic):** `Supports` remains the
  host/technique attachment channel. If the current invocation matches a
  `BindingHostEmbedded` or `BindingTechniqueHint`, that skill must appear in
  `Supports` and must not also appear in `suggested_capabilities[]`. A
  command-scoped candidate that is not selected by a host-scoped binding may
  appear only in `suggested_capabilities[]`. The PR-1 census/migration task
  uses this rule as its acceptance oracle; there is no heuristic
  "when in doubt" fallback. After PR-1, `pickSupportAttachment()` may consult
  only `BindingHostEmbedded` and `BindingTechniqueHint`; neither
  `BindingCommandAuto` nor `BindingCommandManual` may populate `Supports`.
- **JSON schema:** published under the existing routed-command output
  contract docs; treated as stable once PR-2 lands.
- **JSON stability:** "stable once PR-2 lands" freezes field names, enum
  values, and presence/omission semantics. Explanatory prose such as
  `reason` and `summary` is not byte-stable contract text and may evolve.
  When prose is unavailable, `reason` / `summary` are omitted from JSON rather
  than emitted as empty strings; text renderers omit the corresponding line
  instead of printing a blank label.
- **Schema evolution:** PR-2 does not add a `schema_version` field. Instead,
  the contract becomes additive-only after launch: existing fields, enum
  values, and omission semantics stay stable; later plans may add new optional
  fields, but may not repurpose or weaken the existing ones silently.
- **Visibility budget:** this refactor keeps separate `Supports` and
  `suggested_capabilities[]` caps. A combined cross-channel cap is out of
  scope unless PR-2 golden output shows operator-facing noise that justifies a
  follow-up plan.

This solves discoverability without requiring the operator to know internal
skill IDs up front.

### 4.5 Hard-cut policy

This refactor intentionally preserves no selector compatibility layer. These
plans correct an invalid public surface rather than evolving a valid one.

- PR-2 is the cutover release: `--focus` / `--view` become the only supported
  public selectors on their respective command surfaces.
- Legacy raw `--mode=<skill-id>` and raw `--view=<id>` values become immediate
  hard errors with the existing `unknown_route_mode` /
  `unknown_route_view` usage-error family.
- There is no checked-in compatibility table, hidden alias map, stderr
  deprecation warning path, or telemetry hold-point between PR-2 and PR-3.
- External automation must update in the same correction window. If a later
  plan wants a compatibility shim, it must justify that from scratch instead of
  inheriting one from this plan.

### 4.6 Cross-plan ordering

Committed landing order:

- this plan's PR-1 / PR-2 / PR-3 first, in that order
- then `2026-04-15-skills-wave2-plan*.md` PR-1 / PR-2 / PR-3, followed by the
  Wave-2 closeout / metrics report review
- then `2026-04-15-skills-wave3-plan*.md` PR-1 / PR-2 / PR-3, followed by the
  Wave-3 closeout report review
- finally `2026-04-16-knowledge-only-refactor-plan*.md`

No alternative interleaving is permitted on `main`. This plan's PR-1
establishes the surface-policy registry as the sole public-surface authority.
Wave-2 and Wave-3 therefore consume the post-PR-3 surface model directly. No
temporary `surfaces[]` bridge allowlist, handoff table, compatibility alias,
or second surface authority is permitted between the plans.
The single cleanup PR for residual metadata and dead checked-in source is
`2026-04-16-knowledge-only-refactor-plan*.md`, which lands only after the
Wave-3 closeout review.

## 5. Reclassification

### 5.1 Primary routes

| Command surface | Primary route / view | Reason |
|----------------|----------------------|--------|
| `review` | `independent-review` | stable default review contract; already wins current auto route |
| `validate` | `spec-trace` | best single default for code-to-artifact verification |
| `repair` | `root-cause-tracing` | correct default posture before fixes |
| `status` / `health` | `incident-response` | only current command-view implementation; auto default is change-scoped |

Note: `incidentResponse()` remains the only real `BindingCommandView`
implementation for `status` / `health`, but auto-view routing applies only
when evaluating a concrete active/selected change. Diagnostics with no active
change intentionally keep an empty view; the `incident` alias in §5.4 is kept
for explicit intent and change-scoped default symmetry, not because it selects
a distinct fallback surface.

### 5.2 Suggested-only skills

These remain internally routed/suggested when signals justify them, but they
are **not** part of the public explicit selector set.

| Skill | Surfaces | Reason for suggestion-only |
|------|----------|----------------------------|
| `security-review` | `review` | high-signal complement to default review; should be automatic when security cues match |
| `threat-modeling` | `review`, `validate` | useful when trust-boundary cues exist, but too heavy for default primary |
| `gha-security-review` | `review`, `repair` | objective workflow-file signal exists; recommendation is better than raw selector leakage |
| `supply-chain-audit` | `review`, `repair`, `status` | objective manifest/lockfile signal exists; should not masquerade as a view. Concretely, `status --view=supply-chain-audit` is removed (§5.5); the skill only reaches `status` via the `suggested_capabilities[]` channel |
| `coverage-analysis` | `validate` | useful verification booster; not worth a first-class public selector |
| `performance-profiling` | `validate`, `status` | good recommendation path when perf cues exist; not a true view |
| `variant-analysis` | `review`, `repair` | valuable follow-on analysis, but secondary to primary review/repair posture |
| `ci-triage` | `repair`, `status` | objective CI-failure signal exists; better as auto-suggested recovery help |
| `review-comment-triage` | `repair` | best surfaced when PR-comment context exists; not broad enough for public selector |
| `git-recovery` | `repair`, `status` | blocker-driven safety posture; not a read-only view and not a public mode |

### 5.3 Explicit focuses

Keep the public explicit focus set intentionally small:

| Focus alias | Backing skill | Allowed commands | Reason |
|------------|---------------|------------------|--------|
| `sast` | `sast-orchestration` | `review`, `validate`, `repair` | expensive, tool-heavy, clearly opt-in |
| `calibration` | `multi-reviewer-calibration` | `review` | advanced review workflow; explicit human intent should be required |
| `property` | `property-testing` | `validate` | specialized test strategy; high-value but not default |
| `mutation` | `mutation-testing` | `validate` | expensive test-strength audit; must stay operator-driven |

Any later wave plan that wants to add a public focus alias must amend this
plan first. Wave-specific documents may not append focus aliases ad hoc.
Explicit focus aliases resolve through the surface policy registry directly to
their backing skill records; they do not depend on `BindingCommandManual`
lookup. This is why `multi-reviewer-calibration` can move from today's manual
binding into the `calibration` focus without preserving manual-binding
selection semantics at runtime.

### 5.4 Read-only views

Restrict public `--view` selectors to true diagnostics that already have
implemented behavior:

| View alias | Backing surface | Commands |
|-----------|------------------|----------|
| `incident` | `incident-response` | `status`, `health` |

Hydrate policy for the current public view:

- `incident` reuses `incident-response` hydrate references via its skill backing
- `review-queue` and `observability-query` are **not** part of PR-2. Current
  `cmd/` and `internal/engine/capability/` code only preserve those strings via
  override/help paths; they do not yet implement distinct diagnostics behavior
  or view-specific tests behind those IDs. This hard cut therefore deletes the
  override-only exposure instead of freezing string-only placeholders as
  first-class public views.

`sentry` is intentionally **not** part of PR-2. Current `cmd/` and
`internal/engine/capability/` code do not ship a `status` / `health`
diagnostic view implementation under that ID today, so this plan must not
pretend it already exists. If a `sentry` diagnostic view is added later, it
must enter through an amendment to this plan rather than by implication from
older view-only notes.

### 5.5 Absorbed / host-only corrections

| Current / planned surface | New disposition | Reason |
|---------------------------|-----------------|--------|
| `differential-review` | **remove from registry after absorption**; first merge its essential diff-scoped review obligations into `independent-review`, then defer checked-in template / mirror directory deletion to the later knowledge-only cleanup PR | today `differentialReview()` declares a pure `BindingCommandManual` review skill, so under §1's bug it always appears in `Supports` on any review invocation. No host-embedded or technique-hint binding exists, so there is no second consumer that would survive registry removal. Its distinct diff-only rules (`new` / `pre-existing` / `worsened`, diff-scoped blocker policy) must be preserved before deletion so behavior does not silently regress. Deferring the checked-in source-tree delete does not preserve runtime compatibility: toolgen generation and cleanup are registry-owned, so once the registry entry is gone the generated skill tree and manifest drop `differential-review` immediately. |
| `plan-authoring` future `--mode` assumption | host/support-only | belongs on planning hosts, not review/validate public selector surface |
| `tdd-proof` future `--mode` assumption | host/support-only | execution governance contract, not a public validate/review selector |
| `status --view=review-queue` / `observability-query` assumptions | remove from the PR-2 public surface; defer any replacement to a later amendment | current code only preserves override/help strings and does not implement distinct command-owned diagnostics behavior behind those IDs |
| `status --view=supply-chain-audit` / `ci-triage` / `git-recovery` / `performance-profiling` | remove | these are not true views |

Preferred absorption path for `differential-review`: keep
`independent-review` as the base review contract and conditionally layer the
diff-only obligations (`new` / `pre-existing` / `worsened` +
diff-scoped blocker policy) when diff-scoped inputs are present. Do **not**
recreate a second public diff route under a new name.

Diff-scoped activation contract for the absorbed path:

- The command layer, not free-form trigger text alone, decides whether
  `independent-review` is running in diff-scoped mode after absorption.
- Diff-only obligations apply only when review is executing against an explicit
  delta-scoped input set already represented in the command/workflow surface:
  today's changed/stale-unit review path (`review` default or explicit
  `--changed-only`) and any later explicit diff selector added by a new
  approved plan.
- `review --all` and any other full-review path must keep the pure
  `independent-review` contract with no inherited diff-only blocker rules.
- User-text cues such as "diff" or "pull request" may continue to influence
  routing and suggestions, but are insufficient on their own to activate the
  absorbed diff-only rules once `differential-review` is removed.

Binding cleanup policy:

- `BindingCommandManual` is not the long-term explicit-focus mechanism.
- PR-1 stops consulting it for `Supports`.
- PR-2 resolves public focuses/views via surface policy.
- PR-3 removes remaining `BindingCommandManual` entries and the type from
  command-surface validation once all affected skills have been reclassified.

## 6. PR-1 — Surface Policy Foundation

**Goal.** Introduce a dedicated surface policy layer that separates primary,
suggested, explicit, and view exposures from the internal skill registry.

### Code scope

- New: `internal/engine/capability/surfaces.go`
- New: `internal/engine/capability/surfaces_test.go`
- Update: `internal/engine/capability/resolver.go`
- Update: `internal/engine/capability/registry.go` comments to reflect the
  split authority

### Implementation

- Add surface policy records with:
  `command`, `class`, `public_name`, `backing_id`, `summary`
- Keep PR-1 skill-backed only. Do **not** pre-allocate
  `backing_kind=diagnostic_view`, `hydrate_source_id`, or any parallel schema
  surface for unimplemented command-owned diagnostics. Current code only
  preserves `review-queue` / `observability-query` through overrides/help text;
  they do not justify a future-proofing abstraction in this refactor.
- Export direct listing / lookup helpers from surface policy so later callers,
  including Wave-2 / Wave-3 follow-on work and the final knowledge-only
  cleanup PR, consume the same authority without wrappers or bridge tables.
- Make the surface policy registry authoritative for public route resolution:
  default primary route/view lookup and explicit focus/view alias lookup must
  resolve through surface records, not by consulting `BindingCommandManual`.
- Keep one primary record per command surface. For `status` / `health`, that
  primary view is change-scoped; diagnostics-without-change may still render
  with no auto-selected view.
- Add `Resolution.SuggestedCapabilities` (new) for bounded, deterministic
  suggestions — cap 3, sorted by (score desc, skill id asc), disjoint from
  `Supports` (§4.4).
- Stop using command-scoped bindings as implicit support input.
  `pickSupportAttachment()` must consult only host/technique bindings after
  the split. Support attachments should come from host/technique policy, not
  from explicit route metadata.
- Explicit alias resolution must use the surface policy record to determine the
  backing skill before hydrate lookup. There is no implicit fallback from a
  public alias to "treat the alias itself as a skill id".
- No new resolver `Signals` field is introduced for `change-scoped` views.
  Command-layer route resolution remains responsible for deciding whether a
  concrete active/selected change target exists before it requests an
  auto-selected view from surface policy.
- Factor the concrete-change gate for `status` / `health` into one shared
  helper before surface-policy lookup so the two diagnostic commands do not
  drift on "active or explicitly selected change" semantics.
- Census task before the resolver change: grep the repo for current
  consumers that rely on a command-scoped skill appearing in `Supports`,
  record the list in the PR description, and migrate each to either a
  host/technique binding or the new `SuggestedCapabilities` channel using the
  deterministic rule in §4.4. No silent behavior loss.

### Tests

- `TestPrimarySurfaceForCommand` — covers `review`, `validate`, `repair`
  (do not omit `repair`; it is the surface whose primary moves to
  `root-cause-tracing` and regressions are easy to miss).
- `TestChangeScopedPrimaryViewForCommand` — covers `status`, `health` when a
  concrete active/selected change is in scope.
- `TestAutoViewRequiresConcreteChangeTarget` — regression guard that the
  command layer does not request an auto-selected view for
  diagnostics-without-change.
- `TestDiagnosticsModeDoesNotAutoSelectViewWithoutChange` — regression guard
  for today's empty-view diagnostics behavior.
- `TestSuggestedCapabilitiesCappedAtThreeStableOrder`
- `TestSuggestedCapabilitiesDisjointFromSupports`
- `TestExplicitFocusRegistryPerCommand`
- `TestViewRegistryPerCommand`
- `TestSurfacePolicyBackingsResolveToRegisteredSkills`
- `TestCalibrationHostAttachmentSurvivesFocusMigration` — `code-quality-review`
  host still attaches `multi-reviewer-calibration` after the manual review
  binding is removed; the host-path support semantics (including today's
  no-hydrate behavior) stay unchanged.
- `TestCommandScopedBindingsDoNotAutoPopulateSupports` — regression guard for
  the `differential-review` / `security-review` / `threat-modeling` shaped bug
  described in §1.

### Acceptance

- Primary route is deterministic under the new contract for `review`,
  `validate`, `repair`.
- Change-scoped primary view is deterministic under the new contract for
  `status`, `health`, while diagnostics with no active change may still return
  an empty view.
- Suggested capabilities appear separately from primary route and supports,
  and never exceed the cap.
- No command-scoped binding can silently become a support attachment solely
  because its trigger matched.
- `multi-reviewer-calibration` keeps its `code-quality-review`
  host-embedded attachment behavior after the focus migration; only the public
  explicit selector changes.

## 7. PR-2 — CLI Cutover and Discovery

**Goal.** Replace raw `skill-id` route selection with a user-facing focus/view
 surface.

### Code scope

- Update: `cmd/route_flags.go`
- Update: `cmd/review.go`
- Update: `cmd/validate.go`
- Update: `cmd/repair.go`
- Update: `cmd/status.go`
- Update: `cmd/health.go`
- Update: `internal/engine/capability/routes.go`
- Update: `internal/toolgen/toolgen.go`
- Update: `internal/toolgen/testdata/*` goldens touched by command-registry
  selector/help output
- Update: `docs/command-contract-matrix.md`
- Update: command help / usage text

### Implementation

- Rename public selector flag:
  `--mode` -> `--focus` for `review` / `validate` / `repair`.
- Keep `--view` only for true read-only views.
- Add `--list-focuses` and `--list-views`, both with a `--format=json` mode
  (scripts must not have to parse the text variant).
- Render help, remediation text, routed-command output fields (`mode` /
  `view`), and text renderers from surface aliases + summaries, never raw
  skill IDs.
- Add `suggested_capabilities[]` to JSON output and `Suggested:` to text
  output (contract per §4.4).
- Delete ad hoc `routeOnlyViewOverrides` in the same PR. This hard cut keeps
  `incident` as the only supported public view alias; `review-queue` /
  `observability-query` do not get policy-backed replacements until a later
  approved plan introduces real command-owned diagnostics for them.
- Register discovery flags only on the command surfaces that actually own
  them. Wrong-surface `--list-focuses` / `--list-views` invocations fail at
  parse time as unknown flags, matching wrong-surface `--focus` / `--view`
  behavior; there is no special usage-error or empty-list fallback path.
- Cut `cmd/route_flags.go` over to direct surface-policy enumeration. Delete
  `ValidModesForCommand` / `ValidViewsForCommand` and move any remaining callers
  onto surface-policy APIs in the same PR; no thin wrapper survives the cutover.
- Hard-cut the selector surface in PR-2: legacy `--mode` and raw `--view`
  values reject immediately with `unknown_route_mode` /
  `unknown_route_view`; do not add a hidden alias map, compatibility fixture,
  deprecation warning, or telemetry pause gate.
- Explicit alias selection must resolve to its backing skill before hydrate
  lookup and diagnostics rendering. Alias cutover must not break the current
  explicit-view hydrate short-circuit path.
- Rewrite command help / usage strings that still advertise raw skill IDs or
  dead selectors. In particular, `review` help must stop mentioning
  `second-opinion`, and `status` / `health` help must stop mentioning
  `review-queue` / `observability-query`.
- Delete the stale `routeOnlyModeOverrides["review"] = {"second-opinion"}`
  entry in `cmd/route_flags.go`. `second-opinion` does not exist in any
  registry (only under `skills_ref/trailofbits/`), so the override is dead
  code today and must not survive the cutover.
- Rewrite `docs/command-contract-matrix.md` in the same PR so the surviving
  live doc set outside this plan family flips to `--focus` / `--view` and
  `suggested_capabilities[]` at the same moment as the CLI cutover. PR-3 may
  still reconcile later reclassification fallout, but PR-2 may not leave
  `main` in a state where runtime/help have cut over and the live contract doc
  still teaches the retired selector surface.

### Tests

- `cmd/route_flags_test.go`
  - accepts new focus aliases
  - accepts new view aliases
  - rejects legacy raw IDs and legacy raw view IDs immediately; there is no
    alias window and no warning path
  - `status --focus ...`, `review --view ...`, `status --list-focuses`, and
    `review --list-views` fail at parse time as unknown flags; they are not
    silently accepted, remapped, or turned into empty-list responses
  - `--list-focuses` / `--list-views` stable text **and** JSON output
  - help / remediation text no longer advertises `second-opinion` or raw
    `skill-id` selectors
- command golden tests for:
  - `review --focus sast`
  - `review --focus calibration`
  - `validate --focus property`
  - `validate --focus mutation`
  - `status --view incident`
- text / JSON output tests for `suggested_capabilities[]` (cap + order)
- text / JSON output tests showing routed `mode` / `view` fields emit public
  aliases, not raw backing IDs
- negative route tests proving `status --view review-queue`,
  `status --view observability-query`, and `health --view observability-query`
  now fail with `unknown_route_view`
- hydrate tests proving `status --view incident` still resolves through the
  explicit-view hydrate path after the cutover

### Acceptance

- Public help no longer instructs users to pass raw skill IDs.
- Routed command outputs and generated command catalog text no longer expose
  raw skill IDs as the canonical selector contract.
- `status` / `health` no longer accept non-view selectors.
- The only shipped public `--view` alias after the cutover is the implemented
  `incident` surface; `review-queue` / `observability-query` are not preserved
  as string-only placeholders.
- `docs/command-contract-matrix.md` flips in the same PR to the post-cutover
  selector contract and documents `suggested_capabilities[]`; there is no
  surviving live doc window that still teaches `--mode=<skill-id>` after the
  CLI surface has already hard-cut.
- All discovery paths work without prior knowledge of internal IDs.
- `rg -n "routeOnly(Mode|View)Overrides|ValidModesForCommand|ValidViewsForCommand" cmd internal/engine/capability`
  returns zero hits.

## 8. PR-3 — Reclassification and Plan-Family Reconciliation

**Goal.** Apply the new classification to current catalog surfaces and remove
conflicting future assumptions from active plan docs.

### Code scope

- Update: `internal/engine/capability/registry_b2.go`
- Update: `internal/engine/capability/registry_b3.go`
- Update: `internal/engine/capability/registry_b4.go`
- Update: `internal/engine/capability/registry_b5.go`
- Update: `internal/engine/capability/registry.go`
- Update: `internal/tmpl/templates/skills/independent-review/`
- Update: `.codex/skills/slipway/independent-review/` if the synced local
  skill mirror is still checked into the repo
- Update: `internal/toolgen/testdata/skill_tree_inventory.codex.golden`
- Update: generated catalog/export docs that currently list routed bindings
- Update:
  - `docs/plans/2026-04-15-skills-wave2-plan*.md`
  - `docs/plans/2026-04-15-skills-wave3-plan*.md`

### Implementation

- Reclassify the current route participants per §5.
- Remove pseudo-view assumptions from `status` / `health`.
- Absorb `differential-review`'s essential diff-scoped review semantics into
  `independent-review` before removing the registry entry. That absorption step
  is blocked until the preservation test listed below passes.
- Preserve `differential-review`'s verdict-shaped evidence contract during the
  absorption. Diff-scoped review must not silently degrade to an
  artifact-shaped output contract when its rules move under
  `independent-review`.
- Do **not** keep a registry stub, hidden selector, or compatibility alias for
  `differential-review` after the absorption. PR-3 is still the runtime hard
  cut: the registry entry disappears, toolgen output drops the skill, and
  refreshed generated workspaces clean the stale generated directory because
  catalog generation/cleanup is keyed off `DefaultRegistry().IDs()`.
- The checked-in template / mirror directories for `differential-review`
  become dead source only after PR-3 and are deleted later in the
  `2026-04-16-knowledge-only-refactor-plan*.md` cleanup PR. That later delete
  is repository hygiene, not a live compatibility phase.
- Reclassify `multi-reviewer-calibration`, `property-testing`,
  `mutation-testing`, and `sast-orchestration` as explicit-focus-backed
  surfaces whose runtime selection resolves through surface policy rather than
  `BindingCommandManual`.
- Remove all remaining `BindingCommandManual` entries and the type in this PR.
  By the end of PR-3, no validator, route enumerator, or public-surface helper
  may reference it.
- Rewrite future wave rows that currently say
  `Manual explicit via --mode=<skill-id>` to either:
  - `suggested`
  - `explicit focus <alias>`
  - `host/support-only`
  - `absorbed`
- Keep the explicit focus set bounded to the four aliases in §5.3 unless a
  later approved plan re-opens it.

### Tests

- extend resolver and command golden tests for the final classification
- add `TestIndependentReviewPreservesDiffOnlyRules` covering:
  - `new`
  - `pre-existing`
  - `worsened`
  - diff-scoped blocker policy
- add `TestIndependentReviewPreservesDifferentialReviewEvidenceVerdictContract`
  so the merge cannot silently weaken the absorbed diff-only review contract
  from `EvidenceVerdict` to `EvidenceArtifact`
- add `TestIndependentReviewWithoutDiffContextKeepsBaseReviewContract` so the
  absorbed path cannot leak diff-only obligations into `review --all` or other
  full-review execution paths
- add negative tests showing:
  - `differential-review` is no longer present in the registry and is not
    admitted on any surface
  - `supply-chain-audit` / `ci-triage` / `git-recovery` /
    `performance-profiling` are no longer valid `--view` values

### Acceptance

- Current code, template inventory, remaining live docs, and future wave plans
  describe the same surface model.
- No active plan file still instructs operators to use raw `--mode=<skill-id>`
  syntax as the preferred surface.
- `differential-review` disappears from the runtime registry, generated catalog
  manifest, and refreshed generated skill trees in PR-3 even if the checked-in
  dead source directories are deleted later.

## 9. Gates

Each PR in this plan runs:

- `go test ./internal/engine/capability/... ./cmd/... -count=1`
- command smoke checks for the touched selectors and outputs
- `git diff --check`

PR-2 and PR-3 additionally run:

- `go test ./internal/toolgen/... -count=1`
- `go test ./... -count=1`
- `go vet ./...`

PR-3 additionally runs docs residue checks on live paths:

- `rg -- "--mode=[a-z][a-z0-9-]*" internal/toolgen/`
  must return zero hits. Any surviving hit means a generated export surface
  still teaches the retired syntax.
- `rg -n -- "--mode=[a-z][a-z0-9-]*" docs/plans/2026-04-15-skills-wave*.md`
  may match only negative smoke / hard-error assertions that explicitly require
  `unknown_route_mode`. Any affirmative, tutorial, or preferred-surface use of
  raw `--mode=<skill-id>` syntax is a failure.
- `rg -n "suggested_capabilities" docs/command-contract-matrix.md` must return
  at least one hit. That file is the surviving live doc outside this plan
  family that carries the new output contract.
- `rg -n -- "--mode <skill-id>|--view <skill-id>" internal/toolgen/ docs/plans/2026-04-15-skills-wave*.md`
  must return zero hits.
- `rg -n "sentry" docs/plans/2026-04-15-skills-wave*.md` must return zero
  until a later approved plan adds a real `sentry` diagnostic view.

## 10. Out of Scope

- New standalone commands for every specialist skill.
- Reworking host names or the trigger DSL.
- Changing the progression state machine.
- Reintroducing a large public selector matrix after the explicit focus set has
  been reduced.
