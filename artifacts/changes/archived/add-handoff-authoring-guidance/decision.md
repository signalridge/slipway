# Decision

## Alternatives Considered
- Targeted authoring-contract adoption: add handoff authoring guidance to
  existing Slipway workflow/session surfaces, fold skill-writing quality checks
  into the existing shared checklist, add supersession guidance to the existing
  decision template, and pin the contracts with tests. This preserves the CLI as
  lifecycle authority and keeps the change small.
- Standalone handoff helper command or user-invoked skill: expose an explicit
  surface that writes or templates `.git/slipway/runtime/handoff.md`. This could
  standardize file creation later, but it is premature for this change because
  the confirmed gap is an authoring contract, not missing lifecycle machinery.
- First-class handoff lifecycle mechanism: add a command, gate, or evidence
  model for `.git/slipway/runtime/handoff.md`. This would enforce more, but it
  would make an advisory narrative compete with Slipway runtime state.
- Full productivity-skill import: add complete `handoff`,
  `writing-great-skills`, and `teach`-style skills. This would add vocabulary,
  but it would create duplicate workflow concepts and permanent context load.

## Selected Approach
Select targeted authoring-contract adoption. The implementation will borrow the
`handoff` writing contract as a Slipway-specific session handoff discipline,
borrow only the predictable-skill checklist pieces of `writing-great-skills`,
and borrow only the supersession discipline from `teach`.

This matches the confirmed user choice, keeps `handoff.md` advisory, and avoids
new lifecycle machinery. It also keeps a standalone helper command as a future
option if agents still write poor handoffs after guidance exists. Prose cleanup
is allowed only where the touched surface has repetitive, contradictory, or
behaviorally inert wording.

Amend the selected approach with the minimal lifecycle unblock found while
advancing this change: stale S0 intake evidence is still fail-closed before
current planning evidence exists, but a fresh passing plan-audit digest
supersedes historical intake drift at S1/audit. This keeps the CLI-owned
freshness model forward-only without treating old intake evidence as replayable
in the wrong state.

## Interfaces and Data Flow
- Template source: authored guidance changes live in
  `internal/tmpl/templates/...`, not generated `.codex/skills` or
  `.claude/skills` copies.
- Handoff runtime path: `.git/slipway/runtime/handoff.md` remains a runtime
  context file surfaced by hook output as presence/path only.
- Handoff authoring contract: the workflow skill owns the full writing
  discipline; run/context-pressure surfaces may point at it but should not
  duplicate a second contract.
- Runtime authority: `slipway status --json`, `slipway next --json`, and
  stage commands remain the only lifecycle routing and gate authority.
- Test flow: template and toolgen tests render or load authored templates and
  assert positive guidance, cross-adapter propagation, and negative
  non-authority wording.
- Skill-quality guidance: keep the new checklist material as a compact section
  scoped to generated Slipway skill-template editing. If it stops being compact,
  split it into a named shared reference and update shared-reference generation
  tests instead of hiding must-have guidance behind a weak pointer.
- Supersession guidance: `decision.md` should tell authors to mark replaced
  decisions as superseded by a concrete replacement decision, section, or date.
  Do not import the `teach` workspace layout or LR-style numbering.
- Stale-evidence repair: `internal/engine/progression/evidence_repair.go`
  selects actionable stale evidence repair targets. It should not route
  historical S0 intake drift once `plan-audit` is present, passing, and
  digest-fresh for current planning inputs. The existing evidence command
  remains the owner of state/substep admissibility.

## Rollout and Rollback
Rollout is a normal source change to templates, hook wording if needed, and
tests. Verification commands:
- `go test ./internal/tmpl/...`
- `go test ./internal/toolgen/...`
- `go test ./cmd/...` if hook wording changes
- `go test ./internal/engine/progression/...` for the stale-intake lifecycle
  unblock repair
- `go test ./...`
- Optional generated-surface smoke: run `slipway init --tools all` in a
  throwaway directory and inspect for the new guidance plus absence of bypass
  wording.

Rollback is a normal git revert of the changed template, hook, and test files.
No data migration or runtime state conversion is required because
`handoff.md` remains advisory.

## Risk
- Agents may over-trust `handoff.md` if the wording is loose. Mitigation: state
  that it is advisory and require `slipway status --json` / `slipway next
  --json` for next action.
- Skill-quality prose could become a broad rewrite license. Mitigation: state
  that contract tokens and useful project knowledge are preserved, weak context
  pointers are fixed before moving must-have material, and cleanup is limited to
  touched surfaces.
- A new shared reference could fail to propagate across adapters. Mitigation:
  use existing referenced surfaces where possible and cover rendered content in
  template tests.
- Supersession guidance could imply a new artifact type. Mitigation: keep it in
  `decision.md` as a status/update discipline only and name the replacement
  decision directly rather than introducing learning-record identifiers.
- A standalone handoff command could look attractive and expand scope.
  Mitigation: defer it unless later evidence shows agents still cannot write
  useful handoffs with guidance alone; if added later, it must be a helper file
  writer only, not lifecycle authority.
- The stale-intake unblock could hide real S0 scope drift if it is too broad.
  Mitigation: limit it to S1/audit or later when `plan-audit` is passing and
  digest-fresh, and keep the existing S1/research stale-intake blocker tests.
