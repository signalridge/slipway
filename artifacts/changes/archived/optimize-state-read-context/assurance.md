# Assurance

## Scope Summary

This change optimizes Slipway state-read hot paths for `status`, `next`, and
`validate` without adding persistent cross-command authority. The delivered
scope is:

- an invocation-scoped `stateReadContext` for command-local reuse of loaded
  changes, resolved paths, verification records, and execution summaries;
- an explicit and bound-worktree fast path that uses local authority and
  git-local worktree bindings before falling back to global discovery for
  miss/archive/integrity diagnostics;
- status evidence pointer rendering from resolved-change verification records;
- bounded lifecycle event tail reading for status timeline display while
  preserving full lifecycle log reads for integrity surfaces;
- built-binary before/after performance evidence for the required command
  matrix.

No persistent workspace index, durable cache, or compatibility shim was added.

## Verification Verdict

S2 implementation evidence is passing for all four tasks and the current active
change reports execution evidence freshness as `fresh`. The implementation
entered `S3_REVIEW` after `wave-orchestration` evidence passed.

Selected S3 review evidence is now recorded and passing for:

- `spec-compliance-review` with `layer:R0=pass`, `scope_contract:pass`, and
  `negative_path:pass`;
- `code-quality-review` with `layer:IR1=pass`;
- `independent-review`;
- `security-review`.

Current active `validate --json --change optimize-state-read-context` reports
`G_plan=approved`, `G_scope=approved`, `scope_contract.status=pass`,
requirements/tasks/decision contracts valid, and the only remaining blockers are
the terminal `ship-verification` attestations that this closeout produces.

The terminal ship proof is produced at S3 after selected review convergence and
before `done`. The ship-verification notes and references point to the fresh
full-suite, lint, and stub-scan transcripts captured in the same closeout
window.

## Evidence Index

- `artifacts/changes/optimize-state-read-context/verification/state-read-baseline-before.md`
- `artifacts/changes/optimize-state-read-context/verification/state-read-baseline-after.md`
- `artifacts/changes/optimize-state-read-context/verification/wave-orchestration.yaml`
- `artifacts/changes/optimize-state-read-context/verification/spec-compliance-review.yaml`
- `artifacts/changes/optimize-state-read-context/verification/code-quality-review.yaml`
- `artifacts/changes/optimize-state-read-context/verification/independent-review.yaml`
- `artifacts/changes/optimize-state-read-context/verification/security-review.yaml`
- `artifacts/changes/optimize-state-read-context/verification/ship-suite.txt`
- `artifacts/changes/optimize-state-read-context/verification/ship-lint.txt`
- `artifacts/changes/optimize-state-read-context/verification/ship-stub-scan.txt`
- Runtime task evidence:
  - `t-01`: state fast path/read context tests.
  - `t-02`: `status`/`next`/`validate` command wiring tests.
  - `t-03`: lifecycle tail/status timeline tests.
  - `t-04`: after-baseline artifact plus `go test ./... -count=1`.
- Current active validation:
  - `go run . status --json --change optimize-state-read-context`
  - `go run . validate --json --change optimize-state-read-context`

## Requirement Coverage

- REQ-001: Covered by `state-read-baseline-before.md`,
  `state-read-baseline-after.md`, and `t-04` evidence. The after baseline used a
  built binary and recorded fixture counts and `real/user/sys` timings.
- REQ-002: Covered by `cmd/state_read_context.go`,
  `cmd/status.go`, `cmd/next.go`, `cmd/next_context_build.go`,
  `cmd/validate.go`, and `t-01`/`t-02` tests.
- REQ-003: Covered by `internal/state/store.go`, `cmd/common.go`, explicit
  `--change` command wiring, bound-worktree binding resolution, and after
  baseline timings for explicit root commands.
- REQ-004: Covered by `cmd/status_view_build.go` and
  `stateReadContext.verificationRecords`, with status evidence pointers built
  from resolved-change records.
- REQ-005: Covered by `internal/state/lifecycle_event.go` and
  `cmd/status_view_build.go`; the bounded tail reader is used for status
  timeline display, predecessor replay context is event-count bounded, malformed
  retained/context lines are tested, and malformed historical prefixes outside
  the bounded context are not decoded by the display path.
- REQ-006: Covered by existing and added regression tests for missing explicit
  slugs, archived fallback, bound worktree routing, missing authority, malformed
  lifecycle tail lines, and full lifecycle reads.

## Residual Risks and Exceptions

- Root unscoped `status --json` remains a global discovery path by design. The
  after baseline records it as essentially unchanged because this change is
  scoped to command-local reuse, explicit/bound authority fast paths, and
  status timeline tail reads, not to a persistent workspace index.
- The regenerated after-baseline fixture differs slightly from the removed
  before-baseline fixture: it contains 302 `change.yaml` files instead of 300
  because it includes both the target worktree authority and a root stale
  authority needed to exercise binding resolution without delete-recovery
  masking. The worktree count, verification count, lifecycle event count, and
  command matrix remain comparable.
- No security-sensitive data, credentials, network fetches, auth/authz logic, or
  external API contracts were changed.

## Rollback Readiness

Rollback is a normal Git revert of the implementation commit plus the governed
artifact changes. There are no data migrations, persistent caches, new runtime
indexes, or irreversible filesystem mutations. Reverting restores the previous
global discovery behavior and full lifecycle timeline read path. The rollback
verification command is `go test ./... -count=1`.

## Archive Decision

Archive is appropriate after ship-verification records the terminal pass
attestations. Active `validate --json --change optimize-state-read-context`
proof is captured at S3 before `done`; archived bundles should be treated as
frozen records after finalization, not as active validation targets.
