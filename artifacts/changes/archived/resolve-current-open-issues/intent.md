# Intent

## Summary
Resolve the current live open Slipway issue set in one governed batch after
confirming each issue is still real on current main and has developer-experience,
safety, or maintainability impact.
## Complexity Assessment
critical
Rationale: the confirmed live set spans user-facing evidence recovery,
transactional adapter generation, generated skill surface installation policy,
documentation architecture, and test-quality linting. The change is coordinated
but bounded to the current open issue set.

## Guardrail Domains
irreversible_operations

## In Scope
- #263: make invalid `context_origin` selected-review evidence recoverable from
  public CLI surfaces without weakening fail-closed reviewer independence.
- #167: route toolgen adapter refresh through the existing transaction primitive
  and add generated-file ownership checksums so cleanup preserves unknown or
  modified user files.
- #168: add Slipway-native generated-skill install profile / dependency closure
  support and namespace-router surfaces while proving lifecycle-critical and
  sensitive-domain skills cannot be pruned.
- #170: reorganize docs into Diataxis and add guided tutorials for a first
  governed change and onboarding an existing codebase.
- #161: add Go-native test-quality policy and analyzer coverage for bad tests,
  with explicit exceptions for generated-surface/golden/contract tests.
- #169: update or resolve the tracker so it reflects the live open set after the
  above work, not stale closed issue #258.

## Out of Scope
- Already-closed GitHub issues, including #258.
- Unrelated worktrees, branches, archived changes, or release automation.
- Force/skip paths that bypass governed evidence, review, sensitive-domain, or
  irreversible-operation guardrails.
- Direct copies of GSD-core internals; borrow only mechanisms that fit Slipway's
  Go implementation and fail-closed lifecycle.

## Constraints
- Preserve current worktree authority: all edits happen in the bound worktree for
  `resolve-current-open-issues`.
- Prefer existing Slipway primitives (`fsutil.ApplyFileTransaction`, generated
  skill templates, command surfaces, reason-code taxonomy) over parallel
  implementations.
- Unknown or user-modified generated-adapter files must not be destructively
  removed without ownership evidence.
- Documentation examples must stay aligned with current CLI behavior and must not
  teach bypasses.

## Acceptance Signals
- Current GitHub open issue set is rechecked before closeout and no fixed issue
  remains open without an update or closure action.
- Targeted unit/contract tests cover evidence replacement, transactional toolgen
  refresh, ownership manifest behavior, install profile closure guardrails, docs
  navigation/manifest updates, and analyzer policy.
- `go test ./...` passes, plus focused toolgen/surface-manifest checks.
- `go run . validate --json` and `go run . next --json --diagnostics` show the
  governed change is ready for the next lifecycle step through final readiness.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
None.

## Deferred Ideas
<!-- Identified but postponed ideas -->
- Future GSD-core borrowable ideas that are not in the current live open issue
  set.
- Release publication, PR opening, or issue closure automation unless explicitly
  requested after governed readiness.

## Approved Summary
Confirmed by user on 2026-06-18 after a second live recheck. Resolve the six
current open issues (#263, #170, #169, #168, #167, #161) only when they are
confirmed to be real developer-experience or optimization concerns. The batch
fixes recoverability for invalid selected-review context-origin evidence,
transactional and ownership-safe adapter generation, fail-closed install
profiles/namespace routers, Diataxis docs with guided tutorials, Go-native
bad-test policy/analyzers, and the stale GSD-core tracker. Already-closed issues
such as #258 and unrelated local work are excluded. Completion is evidenced by
targeted tests, full Go test pass, generated surface/docs checks, and fresh
Slipway readiness output.
