# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `cmd/common.go`: active/explicit change resolution, bound-elsewhere errors,
    archived fallback, next-ready action helpers, execution freshness projection.
  - `cmd/status.go` and `cmd/status_view_build.go`: status-only route selection,
    status view shape, `next_ready_actions`, diagnostics/archived views.
  - `cmd/validate.go`: validate resolver fallback, readiness/freshness view,
    actionable next skill projection.
  - `cmd/next.go`, `cmd/run.go`, `cmd/done.go`, `cmd/evidence.go`: lifecycle
    command entry points that already use `resolveActiveChangeRef`.
  - `cmd/next_skill_view.go` and `internal/engine/capability`: host/technique
    hint generation, currently advisory rather than fail-closed capability state.
- Dependency chains:
  - `cmd/* command` -> route/active-change resolver -> `internal/state`
    worktree binding/archive loaders -> command-specific view/action logic.
  - `status` -> `resolveStatusRouteForRoot` -> `buildStatusViewFromChange` ->
    `projectNextReadyActionsWithPrimary` and `projectFreshnessForExecMode`.
  - `next/run` -> `resolveActiveChangeRef` -> `buildNextViewForCommand` ->
    `deriveConfirmationRequirement`.
  - `validate` -> `resolveActiveChangeRef` -> `buildValidateViewForSlug` ->
    `progression.EvaluateGovernanceReadiness`.
- Blast radius: public CLI JSON/prose for `status`, `next`, `validate`, `run`,
  `done`, and `evidence`; tests in `cmd`; additive model fields may touch
  generated skill/template assertions only if host capability output is added to
  command surfaces.
- Constraints:
  - Preserve archived-local #283 behavior.
  - Preserve zero-write/read-only behavior for status/validate/next probes.
  - Preserve `projectFreshnessForExecMode` as execution-evidence freshness; add
    readiness/governance freshness instead of redefining it in place.
  - Do not mix release/security hardening or performance work into this change.

### Patterns
- Existing conventions:
  - Commands resolve change identity before acquiring locks or loading detailed
    views.
  - `CLIError` precondition errors carry stable `error_code`, exit code 3, and
    remediation.
  - View-layer structs use additive JSON fields for richer machine contracts.
  - Tests prefer command-level black-box assertions through `commandForRoot` or
    `runRootCommandIn` when public CLI behavior matters.
- Reusable abstractions:
  - `resolveActiveChangeRef`, `wrapResolutionError`, archived worktree fallback,
    and `state.FindActiveChangeForWorktree` are the right raw ingredients for a
    shared invocation route.
  - `deriveConfirmationRequirement` already owns authoritative next action kind
    for `next`; `status` and `validate` should reuse or mirror that projection
    through a shared action-contract helper rather than inventing separate prose.
  - Existing capability registry bindings can seed host capability metadata, but
    availability/fallback state needs a new CLI-visible projection.
- Convention deviations:
  - `status` currently has a deliberately distinct diagnostics path; the shared
    route should not remove diagnostics mode, but it must prevent a normal
    governed detail view when the invocation workspace cannot execute lifecycle
    actions locally.

### Risks
- High: route unification can accidentally break archived-local reporting. Keep
  archived-local as an explicit route kind with tests.
- High: capability fail-closed can block legitimate auto-mode flows if it treats
  all skills as requiring delegation. Only skills/contexts that actually require
  external delegation or fresh independent context should report that
  capability requirement.
- Medium: adding freshness fields can confuse clients if names overlap. Use
  explicit names such as `execution_evidence_freshness`,
  `governance_evidence_freshness`, and `overall_readiness_freshness`, while
  leaving existing `evidence_freshness` as legacy execution freshness until
  later migration.
- Medium: `status` failing closed from root may be less convenient than a
  diagnostics view. A diagnostics view is acceptable if it does not expose local
  lifecycle actions and carries `cd <bound-worktree>` or `--change <slug>`.
- Low: broader scope increases test count and implementation size, but the
  failures are coupled through the same public action contract.
- Guardrail domains: none beyond lifecycle correctness; no credentials, PII,
  financial data, schema migration, or external API contract.
- Reversibility: additive fields and route/action helper changes are reversible.
  Error-code corrections for explicit missing slugs intentionally remove the
  old misleading behavior.

### Test Strategy
- Existing coverage:
  - Helper-level bound-worktree resolution exists in
    `cmd/active_change_resolution_test.go`.
  - Validate zero-write and archived explicit cases exist in
    `cmd/validate_readonly_test.go`.
  - S3 next/validate/run consistency exists in `cmd/progression_next_test.go`,
    but status action-kind consistency is missing.
  - Execution freshness behavior is pinned in `cmd/common_test.go`.
- New infrastructure needs:
  - A small test helper that creates an active change bound to another git
    worktree, then runs root-unscoped `status`, `next`, `validate`, `done`, and
    `evidence` as public commands.
  - JSON assertions for route kind, invocation workspace, bound path,
    local-executable flag, remediation, and absence of locally unexecutable
    `next/run/done` ready actions.
  - Freshness fixtures that combine execution-fresh/stale with skill/governance
    fresh/stale/blocked.
  - Host capability fixtures for available delegation, unavailable delegation
    with explicit fallback, and unavailable/no fallback.
- Verification approach:
  - First add failing tests for the current divergences.
  - Implement the shared route/action/freshness/capability projections.
  - Run focused route/action/freshness tests, then `go test ./cmd -count=1`,
    `go test ./... -count=1`, and `golangci-lint run ./...`.

### Options
- Option A: Patch `status` only.
  - Design: make root unscoped status call `resolveActiveChangeRef` and fail on
    `change_bound_to_other_worktree`.
  - Tradeoffs: fixes the loudest reproduction, but leaves explicit missing slug
    drift, freshness overclaim, action-contract drift, and #339 capability
    dead-end unsolved. This repeats the too-small scope problem.
- Option B: Shared route/actionability projection plus additive action,
  freshness, and capability fields.
  - Design: introduce an internal `InvocationRoute`/equivalent projection that
    all public lifecycle commands can consult. Add additive route fields to
    status/next/validate where useful; use the route to gate local actions;
    add readiness freshness and action-kind projections; add capability
    requirement/availability/fallback metadata and fail-closed blockers where
    unavailable.
  - Tradeoffs: medium-sized change, but it matches the coupled failure mode and
    gives tests one contract to pin.
- Option C: Move lifecycle routing into a larger engine-level state/action
  service.
  - Design: refactor command routing, readiness, and capability into a deeper
    engine package before touching views.
  - Tradeoffs: cleaner long term but too large for this change and risks mixing
    `opt.md` section 3 architecture work into the P0 lifecycle contract.
- Selected: Option B. It is the smallest approach that addresses `opt.md`
  section 1 as one coherent scope without dragging in release/security,
  architecture-boundary, or performance work.

## Unknowns

- Resolved: Is the current root-status bound-worktree behavior actually
  divergent? -> Yes. A live probe from repo root with the active change bound to
  `.worktrees/repair-public-lifecycle-contract` returned a normal governed
  `status --json` view with `next_ready_actions: ["next","cancel"]`, while
  root `next`, `validate`, `done`, and `evidence` all failed closed with
  `change_bound_to_other_worktree`, exit 3.
- Resolved: Is explicit missing `--change` already aligned? -> No. A live probe
  showed `status --change definitely-not-a-change --json` returns
  `change_not_found`, `validate --change definitely-not-a-change --json`
  returns diagnostics with exit 0, and `next --change definitely-not-a-change
  --json` returns `no_active_change`, exit 3.
- Resolved: Should this change cover `opt.md` section 2-4? -> No. Intake scope
  explicitly defers release/security, architecture/coverage, and performance
  hardening to later changes.
- Remaining: None.

## Assumptions

- Adding new JSON fields is acceptable when the old fields are ambiguous, as long
  as unsafe old values such as locally unexecutable ready actions are corrected.
  Evidence: existing view structs already use many additive JSON fields across
  status/validate/next.
- Existing `evidence_freshness` should remain execution-oriented for now, with
  new readiness freshness fields added beside it. Evidence:
  `cmd/common_test.go:560-580` asserts required-skill blockers do not make
  `projectFreshnessForExecMode` stale.
- Capability fail-closed should be tied to selected skills and host requirements,
  not all skill handoffs. Evidence: auto-mode tests intentionally allow
  non-guardrail review batches and skill handoffs to soften, while security
  review remains hard-stop.

## Canonical References

- `opt.md` section 1.1-1.5
- `cmd/common.go:339-390`
- `cmd/common.go:405-483`
- `cmd/common.go:909-934`
- `cmd/common.go:994-1034`
- `cmd/status.go:17-66`
- `cmd/status.go:299-378`
- `cmd/status.go:400-416`
- `cmd/status_view_build.go:17-180`
- `cmd/validate.go:17-52`
- `cmd/validate.go:140-184`
- `cmd/validate.go:262-318`
- `cmd/next.go:14-75`
- `cmd/next.go:703-883`
- `cmd/next_skill_view.go:894-925`
- `internal/engine/capability/resolver.go:48-70`
- `internal/engine/progression/confirmation_boundaries.go:139-152`
- `cmd/active_change_resolution_test.go:17-95`
- `cmd/active_change_resolution_test.go:98-175`
- `cmd/validate_readonly_test.go:52-96`
- `cmd/progression_next_test.go:1065-1183`
- `cmd/common_test.go:496-580`
- `cmd/auto_mode_test.go:173-205`
- `cmd/auto_mode_test.go:263-280`
