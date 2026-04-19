# Plan: Simplify Requirements Validation Surface

## Problem

The current validation surface is harder to use than it needs to be.

### P0: `validate-requirements` duplicates the `validate` naming space while answering a different question

Today the CLI exposes two top-level commands:

```text
slipway validate              -> lifecycle readiness / gate / evidence check
slipway validate-requirements -> requirements.md artifact contract spot-check
```

The problem is not that `validate-requirements` checks the wrong thing. The
problem is that it looks like a sub-mode of `validate` even though it is a
different kind of operation:

- `validate` answers: can this change advance?
- `validate-requirements` answers: is `requirements.md` structurally valid?

That distinction is easy to miss from the command names alone. The result is
avoidable cognitive load in help text, generated prompt surfaces, docs, and
workflow guidance.

The requirements checker capability itself is still useful and should remain in
the system. What should change is the public surface shape.

### P1: `abort` vs `cancel` semantics are ambiguous

- `abort`: stop execution session, **do not archive**
- `cancel`: cancel change, **archive to terminal state**

The distinction is buried in descriptions. Users cannot reliably choose the
right one by name alone. This is out of scope for the current plan.

### P2: `sast` focus is registered on three commands identically

`review`, `validate`, and `repair` all expose `--focus sast` backed by the
same `sast-orchestration` skill. Users cannot tell which command is the right
entry point for SAST. This is also out of scope for the current plan.

## Scope

This plan covers P0 only. P1 and P2 are recorded for future work.

No compatibility shim, alias, or rename is included. This is a hard-cut public
surface cleanup.

This is an explicit breaking change. The repository currently treats
`validate-requirements` as a stable public product verb across Cobra help,
toolgen metadata, generated command entries, and docs. This plan intentionally
reverses that contract and therefore must update runtime, generated surfaces,
and stable docs in the same change.

---

## Current Architecture

### Public surface

```text
slipway validate              -> readiness / blockers / gate view
slipway validate-requirements -> requirements.md contract-only check
```

### Implementation shape

`validate-requirements` is currently implemented as a dedicated Cobra command in
`cmd/sync.go`. Its core logic is small and useful:

- resolve governed bundle path
- read `requirements.md`
- require at least one `Requirement` block
- require stable `REQ-*` IDs
- treat missing `requirements.md` as a read-only negative result
- treat unexpected `stat` / read failures as command errors, not soft summary states

The problem is where this logic lives and how it is exposed, not the logic
itself.

### Files in play

| File | Role |
|---|---|
| `cmd/validate.go` | `validate` command, `validateView`, `buildValidateViewForSlug` |
| `cmd/sync.go` | public `validate-requirements` command and current checker wrapper |
| `cmd/sync_test.go` | command-level tests for `validate-requirements` |
| `cmd/cli_e2e_test.go` | E2E tests invoking `validate-requirements` by name |
| `cmd/root.go` | command registration and root help grouping |
| `cmd/command_description_contract_test.go` | command description contracts |
| `cmd/template_flag_contract_test.go` | template/flag surface contracts |
| `internal/toolgen/toolgen.go` | command registry entry for `validate-requirements` |
| `internal/toolgen/toolgen_test.go` | generated prompt / README / refresh assertions |
| `internal/toolgen/adapter_contract_test.go` | adapter command inventory |
| `internal/tmpl/templates/_partials/command-validate-requirements-body.tmpl` | generated prompt partial |
| `.claude/commands/slipway/validate-requirements.md` | checked-in generated command surface |
| `README.md` | public command inventory |
| `docs/README.md` | stable-doc map |
| `docs/command-contract-matrix.md` | authoritative command inventory and tier map |
| `docs/adr-retire-sync-as-product-verb.md` | decision that currently preserves the command as a product verb |
| `docs/workflow-test-menu.md` | smoke-flow guidance that still treats the command as a public step |

## Target Architecture

### Public surface

```text
slipway validate -> readiness / blockers / gate view + requirements contract summary
```

`slipway validate-requirements` is removed as a public top-level command.

### Internal surface

The requirements contract checker remains as a reusable internal capability.
It becomes a shared helper rather than a standalone command surface.

### Output contract

`validateView` gains an optional nested summary:

```go
RequirementsContract *requirementsContractView `json:"requirements_contract,omitempty"`
```

Suggested shape:

```go
type requirementsContractView struct {
    Status  string `json:"status"` // valid|invalid|missing
    Source  string `json:"source,omitempty"`
    Message string `json:"message,omitempty"`
}
```

Design rules:

- `missing` is explicit. Do not omit the field to hide a missing
  `requirements.md`.
- `Source` is the resolved filesystem path actually checked at runtime,
  matching the path returned by `artifact.ResolveArtifactPath(...)`. Keep it as
  the concrete path used for evaluation so dedicated-worktree/L3 cases remain
  truthful.
- Populate `Source` whenever the governed bundle path was successfully
  resolved, including `status=missing`, so machine callers can see the exact
  path that was checked.
- This summary is informational in this plan. It does **not** change existing
  gate / blocker semantics.
- Do **not** add `status=unreadable`. On normal governed `validate` paths,
  unreadable / permission / unexpected read failures for `requirements.md`
  should continue to surface through the existing readiness blockers and
  diagnostics (`required_artifact_unreadable`,
  `plan_dimension_coverage_spec_unreadable`) rather than through this new
  summary field.
- The helper may still return a real error for unexpected `stat` / read /
  permission failures, but the merged `validate` surface must handle that
  consciously instead of silently becoming a stricter error surface than it is
  today.
- Normal governed `validate` views should populate the summary whenever the
  change can be resolved, the governed bundle path is known, and helper
  evaluation reaches a stable `valid|invalid|missing` result.
- Preset-confirmation-pending continues to return the current minimal
  `validate` view in this plan. Do **not** widen that early-return contract
  just to surface `requirements_contract`.
- If helper evaluation cannot reach a stable summary on the normal governed
  path, omit the field and preserve the existing `validate` blocker /
  diagnostics authority rather than promoting the sidecar into a new blocker or
  command error.
- Diagnostics mode with no active change continues to omit the field.

### Why this is better

- The useful checking capability remains available to the framework.
- Users see one `validate` surface instead of two similarly named top-level
  commands with different semantics.
- Machine consumers get a single readiness payload with a colocated artifact
  contract summary.
- We avoid folding artifact-lint semantics into gate/blocker semantics in an
  ad hoc way.

## Implementation Steps

### 1. Extract the requirements checker into a shared helper

Move the contract-check logic out of `cmd/sync.go` into
`internal/engine/artifact/requirements_contract.go`, next to the existing
requirements parsing helpers in `internal/engine/artifact/requirements.go`.

Requirements for the helper:

- pure read-only evaluation
- no Cobra dependency
- explicit summary result states: `valid`, `invalid`, `missing`
- reuse existing requirement-block parsing and stable-ID helpers
- return a real error for unexpected `stat` / read / permission failures rather
  than converting them into a soft summary status
- keep caller policy separate from helper policy: the helper reports evaluation
  failure, while `validate` remains responsible for preserving its current
  blocker/diagnostic behavior and for avoiding a new `status=unreadable`
  contract
- do not add a new command-specific lock; merged evaluation should inherit the
  existing read-only `validate` execution model
- explicitly acknowledge the lock change: retiring `validate-requirements`
  removes its standalone best-effort state lock and standardizes on `validate`'s
  current lock-free read-only behavior
- do not overclaim helper authority: existing readiness / traceability code may
  continue to own blocker semantics in this plan, even if they reuse the same
  parsing primitives

The helper should become the shared evaluator used by the merged `validate`
surface. Full cross-engine deduplication of every requirements-related check in
readiness and traceability is out of scope for this plan.

### 2. Add `requirements_contract` to `validateView`

In `cmd/validate.go`, add a nested `requirements_contract` summary field rather
than flattening two ad hoc booleans into the main payload.

This keeps the readiness surface readable and leaves room for future artifact
summaries without bloating the top-level JSON.

### 3. Populate the summary on the normal governed `validate` path

Update `buildValidateViewForSlug` so the requirements contract summary is
computed on the normal governed `validate` path after the existing
`loadExecutionContext(...)` / `buildWorkflowPresetView(...)` work and after the
preset-confirmation early-return guard, but before readiness assembly would
otherwise discard a known `valid|invalid|missing` contract result.

In particular:

- preset-confirmation-pending views should keep the current minimal contract and
  omit `requirements_contract`
- readiness failures should not silently suppress a known contract result unless
  the command already exits with a higher-level integrity error
- unreadable `requirements.md` must remain owned by the existing readiness
  surface on normal governed paths; the sidecar must not become a second,
  conflicting authority for those failures
- if helper evaluation fails because `requirements.md` cannot be read on the
  normal governed path, omit the sidecar and let existing readiness blockers /
  diagnostics remain authoritative instead of introducing a new `validate`
  error mode
- the implementation order should be explicit: load change -> load execution
  context -> build workflow preset view -> preset-pending early return ->
  resolve governed bundle path -> evaluate requirements contract -> continue
  with the existing `validate` view assembly

### 4. Remove the public `validate-requirements` command

Hard-cut the command:

- delete `makeValidateRequirementsCmd`
- delete the standalone `validateRequirementsView`
- remove registration from `cmd/root.go`
- remove the command from root help group output
- because `cmd/sync.go` currently only contains this command and its wrapper,
  delete `cmd/sync.go` once the helper has moved out
- because `cmd/sync_test.go` currently only contains command-level tests for
  this surface, delete `cmd/sync_test.go` after migrating the retained coverage

No alias and no replacement command such as `check-requirements` are added in
this plan.

### 5. Remove generated prompt and adapter surfaces for the retired command

Delete the command from:

- `internal/toolgen/toolgen.go`
- adapter contract inventories
- generated command prompt templates / partials
- checked-in generated command surfaces and refresh expectations
- Codex prompt refresh expectations in `internal/toolgen/toolgen_test.go`

Be explicit about the generated-surface cleanup scope:

- remove `.claude/commands/slipway/validate-requirements.md`
- remove the generated Codex prompt `slipway-validate-requirements.md`
- verify there is no separate skill-registry cleanup point beyond these
  generated command / prompt surfaces before widening scope into `.claude/skills`

This keeps CLI help, toolgen metadata, generated command entries, prompts, and
docs aligned.

### 6. Supersede the current ADR and update stable docs

The repository currently contains a stable decision that explicitly keeps
`validate-requirements` as the product verb. This plan reverses that decision.

Update the decision layer so runtime contract and docs agree:

- rewrite or supersede `docs/adr-retire-sync-as-product-verb.md`
- remove the command from `README.md`
- remove the command from `docs/README.md`
- remove it from `docs/command-contract-matrix.md`
- update `docs/workflow-test-menu.md` so requirements checking is no longer a
  standalone user-facing workflow step

### 7. Migrate tests to the new contract

Replace command-level coverage with two layers of tests.

Helper tests:

- live in `internal/engine/artifact/requirements_contract_test.go`
- valid requirements file
- missing file returns `status=missing` with `Source` populated
- malformed file with no Requirement blocks
- requirement blocks missing stable `REQ-*` IDs
- unreadable / permission-denied file returns an error instead of a soft status

`validate` contract tests:

- `validate --json --change <slug>` includes `requirements_contract` on normal
  governed changes
- preset-confirmation-pending view keeps the current minimal contract and omits
  `requirements_contract`
- diagnostics/no-active-change view omits the field
- missing requirements are reported as `status=missing` with `Source`
- invalid requirements are reported as `status=invalid`
- dedicated-worktree/L3 changes report the resolved worktree bundle path in
  `requirements_contract.source`
- unreadable requirements do **not** produce `status=unreadable`; current
  readiness blockers / diagnostics remain authoritative on normal governed
  paths

Hard-cut surface tests:

- `slipway validate-requirements` fails as unknown command
- root help no longer lists `validate-requirements`
- toolgen/adapter inventories no longer generate or expect prompt surfaces for
  the retired command
- refreshed generated outputs no longer contain
  `.claude/commands/slipway/validate-requirements.md` or the Codex prompt
  `slipway-validate-requirements.md`

### 8. Verify generated-surface cleanup and runtime behavior

Verification should cover both code and generated surfaces.

```bash
go build ./...
go test ./... -count=1
go run . --help
```

In addition, run a refresh check in a scratch workspace:

```bash
go run . init --tools codex --refresh
```

Runtime verification must use an explicit governed change fixture. Bare
`go run . validate --json` is insufficient as an acceptance command because it
can legitimately fall back to diagnostics when there is no active change or the
active context is ambiguous. Use either a known active governed change or
`--change <slug>`, and verify at least:

- valid requirements
- missing requirements
- invalid requirements
- retired `validate-requirements` unknown-command behavior

Acceptance checks:

- `validate-requirements` is absent from root help, README, docs, and generated
  prompt surfaces
- `validate --json --change <slug>` exposes `requirements_contract`
- missing / invalid requirements remain explicit in output
- missing status includes the concrete resolved `Source` path
- dedicated-worktree/L3 output still reports the truthful resolved source path
- unreadable requirements do not create a new `requirements_contract`
  unreadable state; existing readiness blockers / diagnostics remain the source
  of truth
- preset-confirmation-pending output remains the current minimal `validate`
  contract and does not gain `requirements_contract`
- no stale `validate-requirements` generated files survive refresh

## Risks

| Risk | Mitigation |
|---|---|
| Users or automation still call `validate-requirements` | Treat this as an explicit breaking change; update docs/ADR/help together and add an unknown-command regression test |
| Requirements summary is mistaken for a new readiness blocker | Keep blocker semantics unchanged in this plan and scope the new field as informational |
| Plan accidentally widens the preset-pending `validate` contract | Keep the current minimal preset-confirmation response and add a regression test that `requirements_contract` stays omitted there |
| Generated prompt / adapter surfaces drift after command removal | Verify `init --tools codex --refresh` and assert retired files are removed |
| Generated command entry / Codex prompt cleanup is mistaken for a separate skill-registry migration | Verify cleanup scope against live toolgen outputs first; only remove generated command / prompt surfaces that are actually produced from the retired command ID |
| Merged checker accidentally changes `validate` lock/error semantics | Keep the helper read-only, do not introduce a new lock, standardize on `validate`'s existing lock-free read path, and keep unreadable requirements owned by existing readiness blockers/diagnostics rather than a new summary state |

## Future Work

- **P1**: Clarify or consolidate `abort` / `cancel` archival semantics
- **P2**: Deduplicate `--focus sast` across `review`, `validate`, and `repair`

## Decision

Remove `validate-requirements` as a public top-level command.

Keep the requirements contract checker as an internal reusable capability, and
surface its result as `requirements_contract` inside `slipway validate`.

This produces a simpler public CLI without deleting useful checking logic or
blurring readiness semantics with artifact-contract semantics, while preserving
fail-closed error handling for unexpected authority read failures.
