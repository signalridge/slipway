# Intent

## Summary
Add a high-risk public lifecycle surface coverage gate for opt.md section 3.2, covering status/next/validate/done/evidence and state verification/worktree/runtime surfaces with actionable changed-surface diagnostics.
## Complexity Assessment
complex
Rationale: this touches CI coverage enforcement and public lifecycle command
surfaces, with failure-mode risk in both local developer flow and GitHub PR
checks.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Extend coverage gating for high-risk public lifecycle surfaces named by
  `opt.md` section 3.2: `status`, `next`, `validate`, `done`, and `evidence`
  command surfaces under `cmd`.
- Include high-risk `internal/state` verification, worktree, and runtime-state
  read/write paths in the gated public-surface coverage set.
- Include release/security helper or workflow-adjacent Go code when that code is
  part of the gated set or is changed by this implementation.
- Preserve the existing governance-kernel coverage baseline and add the new
  public-surface gate alongside it.
- Make coverage-gate failures name the missing or regressed package/file/surface
  so a PR author can add the targeted test instead of reading only a global
  percentage.

## Out of Scope
- No compatibility layer, transitional shim, or legacy fallback for old
  coverage-gate behavior beyond keeping the existing governance-kernel baseline.
- No broad rewrite of unrelated command implementations.
- No performance optimization work from later `opt.md` sections.
- No automatic inclusion of `internal/toolgen` or generated/reference sync code
  unless this change modifies that surface.

## Constraints
- Use repository-native Go tests and the existing `internal/coverage` package
  instead of introducing a separate coverage toolchain.
- The CI gate must fail closed when declared high-risk surfaces lose coverage
  data or regress below their committed floor.
- Keep diagnostics deterministic and actionable for local and CI runs.
- Respect the user directive that future implementation should not preserve
  compatibility layers.

## Acceptance Signals
- `go test ./internal/coverage -count=1` covers public-surface baseline and
  diagnostic behavior.
- `go test ./internal/coverage/cmd/covergate -count=1` covers the CLI behavior
  for the expanded gate.
- A full `go test ./... -count=1` run passes.
- `golangci-lint run ./...` passes before ship.
- The CI coverage workflow still enforces the governance-kernel baseline and
  also enforces the high-risk public lifecycle surface gate.

## Open Questions
None.

## Deferred Ideas
- Broader changed-line coverage for every package can be considered later; this
  change is scoped to the high-risk public surfaces named by `opt.md` 3.2.

## Approved Summary
Confirmed by user authorization on 2026-06-27. This change adds a second
coverage gate for high-risk public lifecycle surfaces while retaining the
existing governance-kernel gate. It covers the `cmd` lifecycle surfaces
`status`, `next`, `validate`, `done`, and `evidence`, plus `internal/state`
verification/worktree/runtime-state paths, with deterministic diagnostics that
identify the package/file/surface needing tests. It excludes compatibility
layers, unrelated command rewrites, and later performance work.
