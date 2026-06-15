# Assurance

## Scope Summary

This change removes Slipway-owned hook and generated skill helper behavior from
portable-script payloads and moves the behavior into compiled Slipway commands.
Generated hooks now use platform-native thin launchers for POSIX, PowerShell,
and `cmd.exe`. Generated skill instructions now route supported helper behavior
through `slipway tool ...`, and generated skill trees no longer ship
Slipway-owned helper scripts.

The scope also updates toolgen registration, worktree provisioning expectations,
generated template inventory, operator documentation, and governed verification
artifacts. The S3 review repair also hardens the new GitHub helper surface:
flag-only helpers reject positional arguments before token lookup, repository
validation rejects dot-only path segments, `fetch-review-requests` requires an
explicit `--org`, and `slipway tool` is registered as a public CLI-only command
surface in the command metadata and surface manifest.

The later user-driven rescope clarifies that "best helper backend" is not
"Go-only". GitHub helpers now support `--backend auto|gh|api`; `auto` prefers
authenticated `gh`, falls back to token-backed API only when `gh` is unavailable,
and fails closed when no authenticated backend exists.

The 2026-06-15 follow-up repair keeps the same scope and hardens review-found
edges: SARIF merge fails closed on any unparseable input, action pinning
preflights the whole batch before writing, CI helper documentation no longer
promises full failed-run log extraction, and helper error branches have
regression coverage. It also documents the refresh-only legacy hook migration
contract in the operator docs so existing installations know to run
`slipway init --refresh` to prune Slipway-owned retired hook launchers/settings
entries while preserving user hooks.

## Verification Verdict

Verification passed for the implemented scope. The final proof set includes:

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --check
go vet ./...
go test -count=1 ./...
```

The full suite completed successfully across all 25 test packages after the S3
review repairs and GitHub backend rescope. `gofmt -s -l` produced no output,
`go vet ./...` produced no output, `git diff --check` produced no output, and
the surface manifest check reported the generated manifest up to date.

## Evidence Index

- `verification/implementation-verification.md` records focused package tests,
  final full suite output, formatting checks, diff hygiene, the
  legacy-reference scan, the review-found `--repo` and SARIF version fixes, and
  the 2026-06-15 follow-up repair evidence for SARIF partial input failure,
  pin-actions batch atomicity, helper error branches, CI helper docs, and the
  documented `init --refresh` legacy hook cleanup contract.
- `verification/spec-compliance-review.yaml` records the post-repair
  spec-compliance review with `layer:R0=pass`, `scope_contract:pass`,
  `negative_path:pass`, and `decision_fidelity:pass`.
- `verification/code-quality-review.yaml` records the post-repair quality review
  with `layer:IR1=pass` and `toolchain_compat:pass`.
- `verification/wave-orchestration.yaml` records
  `dispatch_mode:wave=1:degraded_sequential` and
  `dispatch_mode:wave=2:degraded_sequential`.
- Runtime task evidence exists for `t-01` through `t-05` with
  `run_summary_version=1`.
- The active S4 `go run . validate --json` proof reports
  `scope_contract.status = pass` after the review-repair target files were
  added to `tasks.md`.

## Requirement Coverage

- REQ-001 is covered by compiled hook command tests and launcher contract tests.
- REQ-002 is covered by native launcher template tests, adapter contract tests,
  settings registration tests, and worktree provisioning tests for
  extensionless, PowerShell, and `cmd.exe` launchers.
- REQ-003 is covered by `cmd/tool_test.go`, including negative tests that
  require explicit `--repo` or `--org`, reject dot-only repository segments,
  reject unexpected positional arguments before token checks, reject unsupported
  SARIF versions, reject any unparseable SARIF input, verify pin-actions batch
  preflight, verify helper error branches, verify GitHub backend selection,
  assert direct `gh api` invocation arguments, and verify skill helper
  documentation contract tests that point at `slipway tool ...`. The
  refresh-only hook cleanup behavior is covered by
  `TestMergeHookSettingsPrunesLegacyShellHookAndPreservesUserHooks` and now
  documented in `docs/installation.md` and `docs/ai-tools.md`.
- REQ-004 is covered by support-file pruning tests, generated skill inventory
  golden updates, legacy hook command pruning tests, documentation updates,
  `docs/SURFACE-MANIFEST.json`, and a legacy-reference scan.

## Residual Risks and Exceptions

- Native Windows execution was verified structurally through generated `.ps1`
  and `.cmd` launcher tests and settings path selection logic, not by running
  the suite on a Windows host.
- `slipway tool find-polluter-go` intentionally invokes the Go toolchain because
  Go package discovery and test execution are the domain under investigation.
  That is an explicit domain dependency, not a generated script payload or
  automatic-hook dependency.
- GitHub helper tests use in-process HTTP servers and deterministic request
  assertions plus injected `gh` runners rather than live GitHub writes. The
  write-capable `reply-to-thread` helper remains dry-run by default and requires
  `--confirm` before posting.

## Rollback Readiness

Rollback is a normal git revert of the source changes in `cmd/`,
`internal/toolgen/`, `internal/tmpl/`, `internal/state/`, docs, and this
governed artifact bundle. Rollback verification should run:

```bash
go test -count=1 ./...
go run . validate --json
```

No data migration, external API state, or irreversible runtime operation is
part of this change.

## Archive Decision

Ready to archive after S4 goal-verification and final-closeout evidence are
fresh and the active `go run . validate --json` gate reports no blockers. The
change has no data migration, no external irreversible operation, no new module
dependency, and no intentional compatibility promise for the retired script
runtime surfaces. The archive should preserve the residual Windows-host caveat,
the explicit GitHub backend contract, and the explicit `find-polluter-go`
Go-toolchain exception.
