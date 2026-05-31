# Requirements
## Project Context
- Tech Stack: Go
- Conventions: repo-native (`go build ./...`, `go test ./...`)
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Git-track codebase maps by default
REQ-001: Slipway MUST NOT git-ignore `artifacts/codebase/` by default. The
managed `.gitignore` block produced by `internal/state/local_ignore.go` MUST
omit `/artifacts/codebase/` while continuing to ignore `evidence/`, `events/`,
`verification/`, and `/.worktrees/`. The repository's checked-in `.gitignore`
MUST also be migrated in this change so Slipway itself no longer contradicts
the new default. Existing repositories MUST auto-migrate when the managed block
is next rewritten.

#### Scenario: Existing repo migrates
GIVEN a repository whose `.gitignore` contains the legacy managed block with
`/artifacts/codebase/`
WHEN `slipway new`, `slipway codebase-map`, or `slipway init` rewrites the
managed block (the commands that call `EnsureLocalStateGitIgnore`; `next`/`run`/
`status`/`repair` do not)
THEN `/artifacts/codebase/` is removed while the evidence/events/verification/
worktree patterns remain, and `git check-ignore artifacts/codebase/ARCHITECTURE.md`
exits non-zero.

#### Scenario: Current repository is migrated
GIVEN this Slipway repository after the change lands
WHEN `git check-ignore artifacts/codebase/ARCHITECTURE.md` is run from the change
worktree
THEN the command exits non-zero because the checked-in managed `.gitignore` block
no longer ignores `artifacts/codebase/`.

### Requirement: Surface map freshness in the default handoff
REQ-002: `slipway next --json` and `slipway run --json` MUST include a
`codebase_map_status` field (and per-doc `codebase_map_doc_states`) in the
default `input_context`, reflecting `artifact.AssessCodebaseMapDocs`. The signal
MUST NOT require `--diagnostics` and MUST reuse the existing classification
rather than duplicating it. When no map exists, the field MUST report the valid
assessment value `codebase_map_status == "missing"` with `codebase_map_doc_states`
listing each expected doc as `missing` — `AssessCodebaseMapDocs` initializes
`Status: "missing"` and records each doc missing
(`internal/engine/artifact/codebase_map.go:628-660`). The `omitempty` JSON tag
(REQ-007) is an additive-contract guarantee for context that is genuinely
unavailable (e.g. a non-governed invocation where no change paths resolve); it
MUST NOT be used to drop the valid `missing` assessment state, because doing so
silently disables the default freshness signal in exactly the empty-map case
issue #27 is about.

#### Scenario: Fresh map reports status
GIVEN a governed change whose codebase map contains only scaffold documents
WHEN the caller runs `slipway next --json`
THEN `input_context.codebase_map_status` is `scaffold_only` without passing
`--diagnostics`.

#### Scenario: No map reports missing (not omitted)
GIVEN a governed change whose `artifacts/codebase/` directory has no docs
WHEN the caller runs `slipway next --json` (and `slipway run --json`)
THEN `input_context.codebase_map_status` is `"missing"` and
`codebase_map_doc_states` lists each expected doc as `missing` — the field is
present, NOT absent/empty.

#### Scenario: Handoff/run surface carries the field
GIVEN the same governed change
WHEN the caller runs `slipway run --json` (the compact handoff surface, whose
`input_context` is a separate `nextHandoffContext` projected field-by-field from
the full view)
THEN `input_context.codebase_map_status` is present and equal to the value on
the standard `next` view — i.e. the handoff projection copies the new field, not
only the standard `next` builder.

### Requirement: Consume-time advisory for non-populated maps
REQ-003: When the resolved next skill consumes the codebase map
(`research-orchestration` or `plan-audit`) and the map status is `scaffold_only`
or `baseline`, `next`/`run` MUST emit a non-blocking advisory in `warnings`. The
advisory MUST NOT block progression and MUST be absent when the status is
`populated`. The advisory MUST branch on the `codebase_map_status` field
(REQ-002), NOT on `progression.HasEmptyCodebaseMap`, because that predicate does
not match the advisory set (see the matrix below). The advisory MUST NOT
duplicate or contradict the existing empty-map technique hint.

`HasEmptyCodebaseMap` / existing-hint vs. new-advisory matrix (verified against
`internal/engine/progression/skill_resolution.go:79` and
`internal/engine/artifact/codebase_map.go:628-680`):

| status | `HasEmptyCodebaseMap` | existing "No durable docs" technique hint (S1_PLAN, any skill) | NEW `warnings` advisory (research/plan-audit) |
|---|---|---|---|
| `missing` | `true` | fires | no |
| `scaffold_only` (all docs scaffold) | `true` | fires | **yes** |
| `baseline` (incl. mixed scaffold+baseline; no populated, no missing) | `false` | no | **yes** |
| `partial` (any populated mixed in, or scaffold+missing) | depends | depends | no (surfaced via `codebase_map_doc_states` only) |
| `populated` | `false` | no | no |

`partial` is intentionally excluded from the whole-map advisory: per-doc
non-durability is already visible via `codebase_map_doc_states` (so REQ-004
template guidance MUST direct consumers to inspect per-doc states, not only the
whole-map status — otherwise `partial` carries signal but no guidance). For
`scaffold_only` the existing technique hint already fires, so the implementation
MUST keep a single coherent signal (technique hint + research/plan-audit
advisory), not two contradictory "no durable docs" messages. Both the technique
hint and the advisory MUST be derived from the **same** workspace-bound
assessment (REQ-009); the current `HasEmptyCodebaseMap(root, …)` probe MUST NOT
be retained as an independent second filesystem read, or the two signals can
diverge under a root `--change` invocation.

For `scaffold_only` the advisory IS emitted to `warnings` (top-level, host-prominent)
in addition to the technique hint (`next_skill.technique_hints`, skill-attached):
this double-surface is intentional — the channels serve different consumers — but
the advisory wording MUST add the consume-time framing ("research/plan-audit is
consuming a non-durable map") rather than restate the hint's "no durable docs"
text, so the two are complementary, not contradictory. The t-05 matrix pins this.

#### Scenario: Advisory on scaffold-only consumption
GIVEN the next skill is plan-audit and the codebase map status is `scaffold_only`
WHEN the caller runs `slipway next --json`
THEN `warnings` contains a codebase-map advisory and no new blocker is added.

#### Scenario: Advisory on baseline consumption (the case the predicate misses)
GIVEN the next skill is research-orchestration and the codebase map status is
`baseline` (so `HasEmptyCodebaseMap` is `false` and the existing technique hint
does NOT fire)
WHEN the caller runs `slipway next --json`
THEN `warnings` contains a codebase-map advisory and no new blocker is added.

#### Scenario: No advisory for populated or partial maps
GIVEN the codebase map status is `populated` (or `partial`)
WHEN the next skill is research-orchestration or plan-audit
THEN no codebase-map advisory is added to `warnings`.

### Requirement: Template guidance
REQ-004: The `research-orchestration` and `plan-audit` SKILL templates under
`internal/tmpl/templates/skills/` MUST instruct consumers to treat
`scaffold_only`/`baseline` maps as non-durable and to surface an advisory finding
rather than relying on them as durable context. The guidance MUST use the exact
JSON status value `scaffold_only` (underscore), not the hyphenated prose
spelling, because agents may compare the value literally. The guidance
MUST also direct
consumers to inspect the per-doc `codebase_map_doc_states` (not only the
whole-map `codebase_map_status`), so that a `partial` map — which by REQ-003
gets NO whole-map advisory — is still actionable via its individual
non-durable docs.

#### Scenario: Templates regenerate with guidance
GIVEN the updated templates
WHEN skill surfaces are regenerated
THEN the research-orchestration and plan-audit SKILL files describe the
non-durable handling of scaffold-only/baseline maps.

### Requirement: Documentation
REQ-005: `docs/commands.md`, `CLAUDE.md`, and the codebase-mapping SKILL template
MUST document the new `codebase_map_status` handoff field and the git-tracked
default for `artifacts/codebase`. Additionally, the now-contradictory "local-only
by default" narrative MUST be corrected at `README.md:198` and the
`artifacts/codebase/**` row in `docs/operator-guide.md:15`, since the change
makes maps git-tracked by default. (`internal/toolgen/toolgen_test.go:1001` only
asserts the README still mentions `` `artifacts/codebase/` ``; the string
survives, but the surrounding sentence must change.)

#### Scenario: Docs describe the new behavior
GIVEN the updated documentation
WHEN a reader consults the routed-commands and codebase-map sections
THEN the status field and the git-tracked default are described.

### Requirement: Test coverage
REQ-006: The change MUST add/update tests covering: (a) the status field on
**both** the standard `next` view and the compact `run`/handoff projection;
(a2) direct `run --json` coverage for the state-mutating command surface, not
only coverage inherited through the shared compact projection;
(b) the gitignore migration (managed block rewritten, evidence/events/
verification retained) — including **updating** the existing
`TestLocalStateGitIgnoreRulesHideProofDirsButNotGovernedRecords`
(`internal/state/local_ignore_test.go`) so `artifacts/codebase/ARCHITECTURE.md`
moves from the ignored set to the trackable set; (c) the advisory warning path
(present for non-populated consumed by research/plan-audit, absent for
populated, **and** for the `baseline`/mixed-scaffold+baseline case that
`HasEmptyCodebaseMap` misses — per the REQ-003 matrix); (d) the new template
guidance assertions in `internal/tmpl/templates_test.go` (REQ-004); (e) the
**workspace-consistency** case (REQ-009): a root-checkout `slipway next --change
<slug> --json` invocation against a bound worktree whose map is `baseline`/
`populated` MUST report that status AND emit no "no durable docs" hint — a test
that asserts the status and hint agree, exercising the path where the old
`HasEmptyCodebaseMap(root, …)` probe would read the wrong checkout; and (f) the
**missing-status default** case (REQ-002): with no map, `codebase_map_status`
is `"missing"` and `codebase_map_doc_states` is present (not absent/empty) on
both surfaces. Each of these tests MUST be authored as the RED step of its
corresponding production task (REQ-008), not retrofitted afterward.

#### Scenario: Suite verifies behavior
GIVEN the implemented change
WHEN `go test ./...` runs
THEN the new/updated tests for the status field on both surfaces, the gitignore
migration (including the flipped existing assertion), the advisory matrix
(scaffold_only, baseline, populated, partial), and the template guidance pass.

### Requirement: external_api_contracts guardrail compliance
REQ-007: Changes to the `next`/`run` JSON handoff MUST be additive and
backward-compatible (new `omitempty` fields only); no existing field is removed
or repurposed.

#### Scenario: Guardrail compliance
GIVEN the change touches the externally consumed handoff contract
WHEN the implementation is updated
THEN only additive optional fields are introduced and external_api_contracts
guardrail requirements remain satisfied.

### Requirement: RED-first execution discipline (TDD)
REQ-008: Because this change is `guardrail_domain: external_api_contracts`,
every production (`task_kind: code`) task that changes behavior — the gitignore
removal (t-02), the handoff status field (t-04), the consume-time advisory
(t-06), and the template guidance (t-08) — MUST be preceded by a RED test task
in a strictly earlier wave, and MUST capture RED→GREEN→REFACTOR evidence (a
failing test recorded *before* the production edit, the minimal change that
flips RED→GREEN, refactor only while green). A Go compile failure scoped to a
new test's reference to the not-yet-implemented field/behavior counts as RED.
Documentation-only work (t-09) is exempt; its README invariant is guarded by
`internal/toolgen/toolgen_test.go:1001`.

#### Scenario: RED precedes GREEN for each behavior change
GIVEN a code task that adds or changes behavior
WHEN its wave executes
THEN a corresponding failing (RED) test from an earlier wave is on record, and
the GREEN evidence shows the minimal production change that flipped it.

### Requirement: Workspace-consistent assessment (single source)
REQ-009: The `codebase_map_status`, the per-doc `codebase_map_doc_states`, the
consume-time advisory (REQ-003), and the existing empty-map technique hint MUST
all derive from a **single** `artifact.AssessCodebaseMapDocs(paths.WorkspaceRoot)`
assessment bound to the change's worktree. The implementation MUST NOT retain the
separate `progression.HasEmptyCodebaseMap(root, view.InputContext.CodebaseMapDocs)`
filesystem probe at `cmd/next_skill_view.go:230` as an independent signal source,
and the retired exported helper plus its self-only test MUST be removed so no
stale public-looking API remains.

Rationale (verified): `assembleSkillViewWithOptions` receives the **invocation**
`root` (`cmd/next_skill_view.go:69-70`), while `view.InputContext.CodebaseMapDocs`
holds display paths relative to `paths.WorkspaceRoot`
(`CodebaseMapDisplayDocs(paths.WorkspaceRoot, …)` at `cmd/next_context_build.go:41`
and `cmd/next_handoff.go:172`; `DisplayPath` returns a worktree-relative path,
`internal/state/paths.go:114`). `HasEmptyCodebaseMap` re-joins those relative
paths against the passed `root` (`internal/engine/progression/skill_resolution.go:83`).
Under `slipway next --change <slug>` run from the **root checkout** (where
`invocation_workspace_path` is `"."` but `bound_workspace_path` is
`.worktrees/<slug>`), the probe reads the **root** checkout's map while the new
status field reads the **worktree** map — producing internally contradictory
output (status `baseline`/`populated` while the hint says "no durable docs").
Deriving the hint from `codebase_map_status` eliminates the divergent read.

#### Scenario: Root `--change` invocation is workspace-consistent
GIVEN a bound worktree whose codebase map is `baseline` or `populated`, and a
root checkout whose `artifacts/codebase/` differs (e.g. empty)
WHEN the caller runs `slipway next --change <slug> --json` from the root checkout
THEN `input_context.codebase_map_status` reflects the **worktree** map, and the
"No durable codebase-map documents found" technique hint is NOT emitted (status
and hint agree).
