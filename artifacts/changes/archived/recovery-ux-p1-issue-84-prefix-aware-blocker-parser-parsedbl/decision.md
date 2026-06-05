# Decision

## Project Context
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Change: Recovery UX P1 (#84), part of #81; depends on merged P0 (#83/#87).

## Alternatives Considered
- **A — Enrich `ReasonCode` with Remediation/Command fields.** Max reuse, but `ReasonCode` is persisted (`yaml:` tags) so it leaks presentation into verification/gate evidence and forces broad golden-test churn. Rejected.
- **B — Replace every view's `Blockers []ReasonCode` with a rendered blocker type.** Per-blocker remediation inline, but `nextView.Blockers` is used internally across `assembleSkillView`, so the type change ripples invasively. Rejected.
- **C — New `internal/model` vocabulary + a top-level `recovery` projection at the view boundary.** Selected (see below).

## Selected Approach
**Approach C.** Add a presentation-only recovery vocabulary in `internal/model`, projected onto the read-only views without mutating the persisted `ReasonCode` or any blocker producer.

New in `internal/model` (one new file, e.g. `recovery.go`):
- `ParsedBlocker{Code, Subject, Detail, Raw}` + `ParseBlocker(ReasonCode) ParsedBlocker` (and a spec-string variant). The **single** decomposition point: it reuses the existing Code/Detail split and further splits Detail into Subject (2nd `:`-segment) + Detail (remainder). `cmd`'s ad-hoc `blockerSkillName` (`next_skill_view.go:474`) is refactored to call it.
- `blockerRemediation{Remediation, CommandTemplate, RecoveryClass}` and a `blockerRemediations map[string]blockerRemediation` table keyed by Code (parallel to `canonicalReasonDefinitions`). Because every prefix family's Code is its first segment, Code-keying covers all families; `CommandTemplate` is filled from the parsed Subject/Detail.
- `RecoverySummary{PrimaryCommand, PrimaryAction, RecoveryClass, Steps []RecoveryStep}` and `RecoveryStep{Code, Subject, Detail, Severity, Message, Remediation, Command, RecoveryClass}`. `BuildRecovery(blockers []ReasonCode) *RecoverySummary` parses each blocker, looks up its remediation, builds one Step per actionable blocker, and picks the primary by a **static stage-priority constant** (a fixed ordered list of recovery classes — NOT a derived dependency graph). Returns `nil` when no actionable recovery exists (so the field stays absent via `omitempty`).
- Add the missing canonical message for `tasks_plan_changed_since_task_evidence` (and any peer recovery-relevant prefix tokens that currently humanize-fallthrough) to `canonicalReasonDefinitions`.

Surface wiring (additive, `omitempty`):
- `nextView` (cmd/next.go), `nextHandoffView` (cmd/next_handoff.go), and `validateView` (cmd/validate.go) each gain `Recovery *model.RecoverySummary json:"recovery,omitempty"`, populated from their existing `[]ReasonCode` blockers via `model.BuildRecovery`. The compact handoff (`buildNextHandoffView`) carries the same `recovery` (preserving the primary command — the surface a host hits first).
- `CLIError` (cmd/errors.go) gains `Recovery *model.RecoverySummary json:"recovery,omitempty"`, built from its `Reasons` in `newCLIErrorWithReasons`, so views and the error surface consume the **same** builder.

## Interfaces and Data Flow
- Producers (`internal/engine/progression/*`, gates) → `[]model.ReasonCode` (unchanged) → `model.NormalizeReasonCodes` → view structs.
- At each serialized view boundary: `view.Recovery = model.BuildRecovery(view.Blockers)`. `BuildRecovery` → `ParseBlocker` (per blocker) → `blockerRemediations[code]` lookup → `RecoveryStep` list → primary selection by `recoveryClassPriority`.
- JSON shape (additive): each affected surface gains
  `"recovery": { "primary_command": "...", "primary_action": "...", "recovery_class": "...", "steps": [ { "code","subject","detail","severity","message","remediation","command","recovery_class" } ] }`,
  present only when there is an actionable blocker. Existing `blockers` arrays are byte-identical otherwise.

## Rollout and Rollback
- Rollout: pure additive surface + new model file; no migration, no state change, no producer change. Ships behind no flag (additive JSON is safe).
- Rollback: delete the new model file and the `Recovery` fields/wiring; persisted state and evidence are untouched, so rollback is clean and immediate.

## Risk
- Low: additive JSON drift (mitigated by `omitempty` + docs) and golden-test churn (only blocked/stale fixtures gain the field).
- Medium: the primary-command rule must not grow into the P2 dependency planner — held by implementing it as a static class-priority list with its own unit test and a documented P1/P2 boundary.
- No guardrail domains; fully reversible.
