# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack:
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
Resolve GitHub issue #13: improve governed workflow diagnostics so source inspection is not required
## Complexity Assessment
critical
Rationale: issue #13 spans multiple public CLI JSON surfaces, lifecycle
diagnostics, governed evidence schemas, linked-worktree runtime paths, review
skill templates, and repair/status/health operator guidance. The change alters
externally consumed command contracts and must be planned, tested, and reviewed
as an external API contract change.

## Guardrail Domains
external_api_contracts

## In Scope
- Make `slipway next`, `slipway validate`, and `slipway run --diagnostics`
  expose the same actionable next skill in review states, especially when
  `spec-compliance-review` already passes and `code-quality-review` is missing.
- Replace opaque `evidence_input_hash` behavior with a thorough execution
  freshness contract that stores explicit structural input fields instead of
  relying on the legacy hash field. No compatibility layer is required for the
  old field.
- Update freshness diagnostics for `validate`, `next --diagnostics`, `status`,
  `health`, and `repair` so stale evidence reports identify stale
  artifact/evidence pairs, expected/current values, relevant timestamps, and
  the next safe remediation action. Diagnostics must also identify the first
  stale cause and downstream evidence chain when artifact drift cascades.
- Expose required review layer tokens exactly as the governance gate expects,
  and update skill templates so generated review evidence examples do not use
  misleading substitute tokens.
- Improve `run --resume` errors with current lifecycle state, resumable states,
  and the correct command path when the current state is not resumable.
- Surface linked-worktree authority paths: invocation workspace, bound
  worktree workspace, governed artifact bundle, git-common runtime evidence
  path, verification path, and change authority path where relevant.
- Make `repair --json` distinguish applied repairs from detected-but-unrepaired
  drift, including why unrepaired drift was not safe to fix and what command or
  artifact should be updated next.
- Clarify status artifact DAG `draft` / `ready:false` entries by identifying
  whether each entry is currently blocking a gate or only informational.
- Mark non-blocking health warnings, especially codebase-map warnings, as
  non-blocking for the active change when they do not affect current gates.
- Add focused regression tests and documentation/templates for the changed CLI
  contracts.
- Make common operator repair mistakes diagnosable from CLI output, including
  wrong structural freshness values and manual timestamp/hash alignment attempts.

## Out of Scope
- Rewriting the entire Slipway lifecycle state machine.
- Weak compatibility with the old `evidence_input_hash` field after the new
  explicit execution freshness contract is introduced.
- Expanding `slipway repair` into an unsafe broad auto-fixer for all artifact
  drift. Repair may fix only cases that are deterministic and safe.
- Bypassing governed review, domain review, or confirmation gates.
- Closing unrelated health or codebase-map product work beyond the
  non-blocking/blocking diagnostic distinction needed for issue #13.

## Constraints
- Preserve governed workflow safety semantics while making the CLI the
  authoritative explanation surface.
- Keep JSON changes structured and testable; avoid relying on human-only prose
  for machine-consumed diagnostics.
- Treat this as an external CLI contract change and verify affected command
  outputs directly.
- Existing local worktree state and generated governed artifacts are
  authoritative for this change.

## Acceptance Signals
- Each Suggested Acceptance Criteria in GitHub issue #13 maps to code,
  documentation/template changes, and regression test coverage or an explicit
  in-scope rationale.
- A review-state regression proves `next`, `validate`, and `run --diagnostics`
  agree on `code-quality-review` after passing `spec-compliance-review`
  evidence exists.
- Stale execution evidence diagnostics include explicit structural input fields
  and expected/current comparisons without requiring source inspection.
- Stale planning/execution diagnostics identify the first stale cause, the
  downstream evidence chain, and the correct regeneration order.
- Operator repair mistakes such as wrong structural freshness values or manual
  timestamp/hash edits are reported with expected/current values and safe
  remediation guidance.
- Review skill templates and CLI review context expose `layer:R0=pass`,
  `layer:IR1=pass`, and any domain-required layer tokens exactly as the gate
  consumes them.
- `run --resume` non-resumable-state errors include `current_state`, resumable
  states, and an action appropriate to the current lifecycle.
- Linked-worktree diagnostics include the authoritative git-common runtime
  evidence path.
- `repair --json` output separates applied repairs from unrepaired drift with
  reason and next action fields.
- `status --json` artifact DAG and `health --json` warnings distinguish
  blocking from non-blocking diagnostics.
- `go test ./...` and `go build ./...` pass.

## Open Questions
<!-- No user-blocking clarification questions remain. -->

## Deferred Ideas
- Implementation planning must determine the concrete execution-summary schema
  replacement and generated surface refresh needs.
- Rich interactive repair flows that ask the operator before each artifact
  rewrite.
- Broader health taxonomy redesign beyond issue #13's active-change blocking
  distinction.

## Approved Summary
Confirmed 2026-05-29T15:58:23Z.

Resolve issue #13 by making Slipway CLI diagnostics authoritative for governed
workflow blockers and recovery. The change will update command JSON surfaces,
execution freshness schema, review token templates, linked-worktree path
reporting, repair/status/health diagnostics, docs, and tests. The execution
freshness fix should be thorough: replace the opaque legacy
`evidence_input_hash` compatibility model with explicit structural input
fields and diagnostics rather than preserving a compatibility layer.
