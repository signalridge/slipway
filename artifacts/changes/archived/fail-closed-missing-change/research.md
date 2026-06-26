# Research

## Alternatives Considered

### Architecture
- Affected modules: `cmd/common.go` for active/explicit change resolution and
  `cmd/validate.go` for validate's read-only command boundary.
- Dependency chains: `validate` calls `resolveActiveChangeRef`; explicit
  `--change` dispatches to `resolveExplicitChange`; that in turn loads active
  bundle state through `internal/state` and returns CLI errors consumed by the
  command wrapper.
- Blast radius: low. The desired behavior is constrained to explicit missing
  slugs and should not alter active worktree resolution, archived slug handling,
  or broader status/next/done semantics.
- Constraints: explicit archived slug must continue to return
  `archived_change_not_validatable`; malformed or orphaned active authority
  must remain fail-closed to state-integrity/recovery errors; unscoped no-active
  validate semantics must stay documented by tests.

### Patterns
- Existing conventions: command boundaries use typed `CLIError` values with
  stable `error_code`, `exit_code`, remediation, and optional `slug` details.
  Existing `loadChangeBySlug` already maps `os.ErrNotExist` to
  `change_not_found`.
- Reusable abstractions: `newPreconditionError` and existing explicit resolver
  tests in `cmd/resolve_explicit_change_authority_test.go` are the right
  pattern for fail-closed CLI errors.
- Convention deviations: none needed. The fix should prefer changing the
  explicit resolver's missing-slug error over adding validate-specific
  string handling.

### Risks
- Technical risks: low. The main regression risk is accidentally changing
  unscoped `validate --json` diagnostics or archived slug handling.
- Guardrail domains: none. No auth, credentials, external API, schema,
  irreversible operation, or financial/PII surface is touched.
- Reversibility: straightforward source rollback in `cmd/common.go` and test
  files.

### Test Strategy
- Existing coverage: `cmd/validate_readonly_test.go` already covers no-active
  zero-write behavior, explicit archived slug zero-write behavior, malformed
  verification recovery, and orphan active bundle zero-write behavior.
  `cmd/resolve_explicit_change_authority_test.go` covers archived fallback,
  missing-authority fail-closed behavior, and malformed authority.
- Infrastructure needs: no new helpers are required. Existing command tests can
  use `commandForRoot`, `makeValidateCmd`, `createGovernedRequest`,
  `state.ArchiveChange`, and `asCLIError`.
- Verification approach: add failing tests first for explicit missing
  `validate --change`, explicit archived `validate --change`, and unscoped
  no-active validate behavior; then update the resolver behavior and run
  targeted `cmd` tests.

### Options
- Option A: Change `resolveExplicitChange` so a true missing explicit slug
  returns `change_not_found` instead of `no_active_change`. Tradeoff: this
  aligns all callers of explicit resolution, not only validate; this is the
  selected behavior because explicit slugs are user-specified identities and
  should not be softened into ambient no-active diagnostics.
- Option B: Keep `resolveExplicitChange` as-is and make
  `shouldFallbackValidateDiagnostics` detect whether `--change` was supplied.
  Tradeoff: narrower code diff, but validate would diverge from the shared
  resolver contract and other callers could keep receiving the misleading
  `no_active_change` code.
- Option C: Introduce a larger route model now and migrate validate onto it.
  Tradeoff: aligns with opt.md section 1.1, but it is larger than the section
  1.2 bug and would delay the fail-closed fix.
- Selected: Option A, with tests in the validate command boundary and explicit
  resolver area.

## Unknowns
None.

## Assumptions
- The current product bug is reproduced on main: `go run . validate --change
  definitely-not-a-change --json` exits 0 with generic diagnostics rather than
  a typed missing-change error. Evidence: live command run before change
  creation.
- The relevant public contract is a typed precondition error, not a diagnostics
  view, for explicit missing identities. Evidence: `loadChangeBySlug` already
  uses `change_not_found` for missing explicit slug loads in `cmd/common.go`.
- The existing codebase map is semantically stale for most of this change
  because it was authored for handoff work, but its active-change-resolution
  note correctly identifies `resolveActiveChangeRef` as the command-entry seam.
  Evidence: `artifacts/codebase/ARCHITECTURE.md`.

## Canonical References
- `cmd/common.go`
- `cmd/validate.go`
- `cmd/validate_readonly_test.go`
- `cmd/resolve_explicit_change_authority_test.go`
- `cmd/active_change_resolution_test.go`
- `artifacts/changes/fail-closed-missing-change/intent.md`
- `artifacts/codebase/ARCHITECTURE.md`
