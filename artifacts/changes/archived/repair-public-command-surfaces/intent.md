# Intent

## Summary
repair public command surfaces
## Complexity Assessment
complex
Rationale: this changes public CLI/documentation/manifest contracts across
command registration, generated surface metadata, localized reference pages,
and user-facing help/documentation. The work is low runtime-risk but contract
coupled.

## Guardrail Domains
external_api_contracts

## In Scope
- Bring the public `config` command into the command inventory, surface
  manifest, and command documentation with explicit CLI/adapter semantics.
- Repair stale command-token tables for `run --json` and `handoff show --json`
  across canonical and localized command documentation.
- Reconcile the split command reference pages so the shorter reference does not
  drift from the detailed command reference or manifest contract.
- Update AI adapter documentation and diagram text to match the current adapter
  inventory.
- Document `handoff write --section` usage in README-level command examples.
- Remove or repair the unsupported `review --artifact` public flag surface.
- Clean up low-risk misleading internal names/tests only where doing so reduces
  public-surface confusion without broad refactoring.
- Add or update regression checks so manifest/docs/toolgen drift is detected by
  the existing verification commands.

## Out of Scope
- Do not change unrelated active governed worktrees, including
  `fix-evidence-and-recovery-ux`.
- Do not modify the unrelated untracked `.gemini/` directory in the root
  checkout.
- Do not add new CLI features beyond the public-surface repairs needed for the
  reported issues.
- Do not preserve retired or unsupported public behavior solely for backward
  compatibility if the chosen repair removes it intentionally.

## Constraints
- Use the governed worktree
  `.worktrees/repair-public-command-surfaces` as the implementation authority.
- Prefer registry/toolgen-owned surfaces over hand-maintained duplicate docs
  wherever the existing codebase supports that path.
- Keep edits scoped to command registry, docs, manifest generation/tests, and
  directly affected command code.
- Preserve existing repository conventions and use repo-native Go checks.

## Acceptance Signals
- `slipway --help` public command inventory, generated manifest rows, and
  command documentation agree on the `config` command semantics.
- Stable JSON token tables use `slipway run --json` and include
  `slipway handoff show --json` consistently in English, Japanese, and Chinese
  command docs.
- Adapter docs and diagram text agree with the current AI tool reference
  inventory.
- README documents `handoff write --section` without implying unsupported
  behavior.
- `review --artifact` no longer exposes a dead public flag, or the chosen
  behavior is fully implemented and documented.
- Regression tests/checks fail on the repaired manifest/docs drift class.
- Fresh verification includes targeted Go tests, manifest check, docs/token
  checks as applicable, `git diff --check`, and governed review gates.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
- [x] User confirmed the scope and acceptance summary in chat on
  2026-06-25T16:09:23Z.

## Deferred Ideas
- Broader documentation-site restructuring unrelated to the reported command
  and adapter drift.
- Feature work for unsupported review artifact ingestion beyond this repair.

## Approved Summary
Confirmed by the user on 2026-06-25T16:09:23Z. This change repairs the reported
public command-surface drift in one governed pass: `config` becomes accurately
represented in registry/manifest/docs, command JSON tokens and handoff entries
are made consistent, command reference divergence is constrained, adapter
documentation is brought current, README handoff examples include sectioned
writes, and the unsupported `review --artifact` flag is removed or made
truthful. The change excludes unrelated governed worktrees, root-only untracked
agent files, and broad feature expansion. Completion requires fresh manifest,
targeted Go, docs/token, diff hygiene, and governed review evidence.
