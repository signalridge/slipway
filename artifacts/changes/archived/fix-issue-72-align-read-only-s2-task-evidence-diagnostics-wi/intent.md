# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
Fix issue #72: align read-only S2 task-evidence diagnostics with run diagnostics so present-but-stale or plan-mismatched task evidence is not reported as run_summary_missing
## Complexity Assessment
complex
The change touches Slipway lifecycle diagnostics for a governed S2 execution
state. The implementation is expected to be localized, but the surface is an
externally consumed CLI contract (`validate/status/next --json`) and must keep
read-only behavior non-mutating while matching execution-path diagnosis.

## Guardrail Domains
external_api_contracts

## In Scope
- Align read-only S2 task-evidence diagnostics on `validate --json`,
  `status --json`, and `next --json` with the diagnosis already available from
  `run --json --diagnostics`.
- Distinguish absent task evidence from task evidence that exists but is stale,
  plan-hash mismatched, or otherwise not acceptable for the current `tasks.md`.
- Keep the diagnostic actionable for issue #72's reproduced case: present
  task evidence should not collapse to `wave-orchestration:run_summary_missing`
  when the more specific task-evidence drift reason is available.
- Add focused Go regression coverage for the read-only surface behavior.

## Out of Scope
- Do not change the mutating `run` advancement semantics except where shared
  diagnostic helpers require a narrow consistency adjustment.
- Do not implement release-channel or installed-version skew remediation for
  `slipway 0.5.0`.
- Do not reopen issue #71 or broaden this change into unrelated governance
  readiness redesign.
- Do not alter Lattice Change 16 artifacts or completed governance state.

## Constraints
- Read-only commands must remain read-only and must not restamp or rewrite
  runtime evidence.
- JSON output contracts should remain backward compatible except for clearer
  blocker/detail diagnostics.
- The fix must preserve the existing external API contract guardrail posture
  for CLI consumers.

## Acceptance Signals
- A fixture with task evidence present but mismatched to current `tasks.md`
  causes read-only diagnostics to report task-evidence drift rather than only
  `wave-orchestration:run_summary_missing`.
- Existing no-task-evidence cases still report missing run summary/task evidence
  as appropriate.
- Targeted Go tests covering the affected read-only surfaces pass.
- `go test ./...` and `go build ./...` pass before closeout.

## Resolved Questions
- `SyncGovernedWaveExecution` in `internal/engine/progression/wave_sync.go`
  owns the more specific task-evidence diagnosis. The selected approach
  extracts a non-mutating preview from the same evaluator so
  `validate/status/next` can reuse the diagnosis without writing
  execution-summary or checklist state.

## Deferred Ideas
- Add release-version skew diagnostics for installed binaries versus current
  source once the current diagnostic inconsistency is fixed.

## Approved Summary
Approved 2026-06-05T01:44:46Z. Complete issue #72 by aligning read-only
S2 task-evidence diagnostics (`validate --json`, `status --json`, and
`next --json --diagnostics`) with the existing mutating execution-path
diagnosis. When runtime task evidence exists but is stale, plan-hash
mismatched, or otherwise unacceptable for the current `tasks.md`, read-only
surfaces must report the specific task-evidence drift/blocker instead of
collapsing to `wave-orchestration:run_summary_missing`. Keep absent-task
evidence cases on the existing missing-evidence path. Do not implement release
version skew remediation, do not change Lattice Change 16 artifacts, and do not
broaden into unrelated governance readiness redesign. Acceptance is focused Go
regressions plus full `go test ./...` and `go build ./...`.
