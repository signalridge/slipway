# Decision

## Alternatives Considered
- Resolver-level fix: change `resolveExplicitChange` so a true missing explicit
  slug returns `change_not_found` instead of `no_active_change`. This keeps the
  explicit resolver contract shared and avoids validate-specific branching.
- Validate-only fix: make `validate` suppress diagnostics fallback when
  `--change` is present. This is smaller at the command boundary, but it leaves
  the shared explicit resolver with misleading semantics for other callers.
- Full route-model fix: implement the broader `InvocationRoute` model from
  opt.md section 1.1. This is the strategic direction, but it is too broad for
  the section 1.2 fail-closed bug.

## Selected Approach
Use the resolver-level fix. `resolveExplicitChange` is already the single path
for explicit `--change` active-governance targets, and `loadChangeBySlug`
already uses `change_not_found` for the same missing-slug concept. Updating the
explicit resolver keeps `validate` aligned with the shared command contract
without adding a validate-only exception.

## Interfaces and Data Flow
- CLI input: `validate --change <slug> --json`.
- Data flow before the change: `validate` calls `resolveActiveChangeRef`, which
  calls `resolveExplicitChange`; missing active and archived authority returns
  `no_active_change`; validate's diagnostics fallback converts that to a
  successful diagnostics view.
- Data flow after the change: the same path returns a `change_not_found`
  precondition error for true missing explicit slugs. Validate no longer enters
  the no-active diagnostics fallback because the error code is not
  `no_active_change`.
- No public flags, artifact schemas, runtime state layout, or persisted data
  formats change.

## Rollout and Rollback
Rollout is a normal source change with tests. Rollback is a straightforward git
revert of the resolver and tests. Verification command:

```text
go test ./cmd -run 'Validate.*(Missing|Archived|NoActive)|ResolveExplicitChange' -count=1
```

## Risk
- Regression risk: archived explicit slug handling could accidentally change;
  this is covered by a validate command test and existing explicit resolver
  tests.
- Regression risk: unscoped no-active validate could stop using diagnostics;
  this is covered by the zero-write no-active validate test.
- Compatibility risk: callers that depended on explicit missing slugs being
  softened to `no_active_change` will now receive the more precise
  `change_not_found` precondition error. This is intentional and matches the
  fail-closed requirement.
