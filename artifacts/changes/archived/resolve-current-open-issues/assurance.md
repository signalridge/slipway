# Assurance

## Scope Summary

This governed batch addresses the confirmed live open issue set:

- #263: public recovery for passing selected-review evidence with invalid
  review context-origin, plus the same recovery class observed in S1 stale
  research evidence recertification.
- #167: transactional generated adapter refresh with sha256 ownership manifests
  and managed/modified/unknown classification.
- #168: fail-closed install profiles and namespace routers that keep lifecycle,
  gate-owning, and sensitive-domain skills available.
- #170: Diataxis documentation with guided tutorials, Start Here, and real-world
  scenarios informed by GSD Core and Trellis references.
- #161: Go-native bad-test analyzer and delete-bad-tests policy.
- #169: live GitHub tracker correction so closed #258 is not listed as open.

## Verification Verdict

Implementation verification is passing for the local worktree:

- Full Go test suite passed with `go test ./... -count=1`.
- Focused integration checks passed for `cmd`, progression, toolgen, fsutil,
  and testlint.
- Surface manifest check passed.
- GolangCI-Lint configuration verification passed.
- The runnable test policy analyzer passed with
  `go run ./internal/testlint/cmd/testlint ./...`.
- Live GitHub issue recheck passed and #169 was updated.

Docs rendering via MkDocs was not executed because MkDocs is unavailable in the
current environment. This is an environment gap, not a code/test failure.

## Evidence Index

- `artifacts/changes/resolve-current-open-issues/verification/final-verification.md`
- `artifacts/changes/resolve-current-open-issues/verification/github-open-issue-recheck.md`
- Runtime task evidence for `t-01` through `t-07` under the Slipway task
  evidence ledger for run summary version 1.
- Planned wave executor handles:
  - `019eda69-b9e9-7e22-9542-d76c8ed5b75f` for `t-01`
  - `019eda69-bd60-70a1-9c71-eeb3e8a38b57` for `t-02`
  - `019eda69-c198-7382-8e28-be37968b5488` for `t-04`
  - `019eda69-c51b-7731-846a-4d202484af6a` for `t-05`
  - `019eda85-5c9d-7e40-9f7d-6cae09f95f05` for `t-03`

## Requirement Coverage

- REQ-001: Covered by `cmd/evidence.go`, `cmd/evidence_skill_test.go`,
  `internal/engine/progression/authority.go`, and
  `internal/engine/progression/authority_test.go`; verified by focused and full
  Go tests.
- REQ-002: Covered by `internal/toolgen` ownership/transaction changes,
  `internal/fsutil` transaction support, and init/bootstrap refresh contract
  tests; verified by focused toolgen/fsutil/cmd/bootstrap tests and full Go
  tests.
- REQ-003: Covered by `internal/toolgen/install_profiles.go`,
  `install_profiles_test.go`, and router template generation; verified by
  focused install-profile/toolgen tests and full Go tests.
- REQ-004: Covered by MkDocs nav/docs pages and surface manifest updates;
  verified by surface manifest checks and full Go tests. MkDocs rendering remains
  a follow-up environment check.
- REQ-005: Covered by `internal/testlint` analyzer/tests, the runnable
  `internal/testlint/cmd/testlint` command, CI wiring, and
  `docs/contributing.md`; verified by
  `go run ./internal/testlint/cmd/testlint ./...`,
  `go test ./internal/testlint -count=1`, and full Go tests.
- REQ-006: Covered by the GitHub recheck artifact and live #169 update.
- REQ-007: Covered by this assurance artifact, final verification evidence, and
  the pending S3 domain/security/independent review gates.

## Residual Risks and Exceptions

- MkDocs is not installed here, so `mkdocs build --strict` could not be run.
  The docs changes should still be rendered in an environment with MkDocs before
  publication.
- The current implementation is local and issue closure remains pending until
  review/merge confirms the changes.
- This guarded batch must continue through S3 domain, security, independent,
  goal-verification, and final-closeout gates before any done-ready claim.

## Rollback Readiness

Rollback is straightforward at the source level: revert the local batch changes
and restore the previous #169 issue body if the tracker update must be undone.
Generated adapter refresh now uses file transactions and ownership manifests, so
mid-refresh failures roll back generated file mutations. The external GitHub
tracker change is separately auditable in the issue timeline.

## Archive Decision

Not archive-ready yet. Implementation and local verification are complete enough
to request S3 review, but this change still requires fresh governed review
evidence and active `validate --json` / `next --diagnostics` readiness proof
after the review stage before `done` can be considered.
