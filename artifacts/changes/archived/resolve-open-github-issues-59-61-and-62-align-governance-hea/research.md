# Research

## Research Findings

### Architecture
- Affected modules:
  - `cmd/health.go` owns `slipway health --governance` command routing, snapshot load/recompute, JSON output, text rendering, and doctor action synthesis. Evidence: `cmd/health.go:21`, `cmd/health.go:128`, `cmd/health.go:145`, `cmd/health.go:186`, `cmd/health.go:193`, `cmd/health.go:641`.
  - `internal/engine/governance/health.go` owns `GovernanceHealthCheck`, traceability health checks, snapshot preview/recompute, and active-control coherence. Evidence: `internal/engine/governance/health.go:17`, `internal/engine/governance/health.go:253`, `internal/engine/governance/health.go:397`, `internal/engine/governance/health.go:407`, `internal/engine/governance/health.go:451`.
  - `internal/engine/governance/traceability.go` already computes structured `TraceabilityGap` records with ID/type/issue/blocking but the health check message collapses them to a count. Evidence: `internal/engine/governance/traceability.go:48`, `internal/engine/governance/traceability.go:59`, `internal/engine/governance/traceability.go:122`, `internal/engine/governance/traceability.go:177`, `internal/engine/governance/traceability.go:258`, `internal/engine/governance/traceability.go:273`.
  - `cmd/next.go` owns `confirmation_requirement`; it currently maps any skill handoff to the same `hard_stop` constructor used for checkpoint-like boundaries. Evidence: `cmd/next.go:56`, `cmd/next.go:594`, `cmd/next.go:598`, `cmd/next.go:605`, `cmd/next.go:634`.
  - `cmd/next_context_build.go` owns active checkpoint projection and the `no_active_checkpoint` error when `--resume-response` is supplied without an actual checkpoint. Evidence: `cmd/next_context_build.go:300`, `cmd/next_context_build.go:310`, `cmd/next_context_build.go:333`.
  - `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl` emits the GNU-only placeholder scan. Evidence: `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl:56`, `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl:63`.
- Dependency chains:
  - `health` command -> `governance.LoadSnapshot` -> `governance.RecomputeGovernanceSnapshot` -> `governance.CollectGovernanceHealthWithSnapshot` -> `checkTraceabilityCoherence`.
  - `next/run` command -> `buildNextView` -> `deriveConfirmationRequirement`; checkpoint resume support is separate through `buildResumeCheckpoint`.
  - template content -> `internal/tmpl.Render`/toolgen tests -> generated `slipway-goal-verification` skill.
- Blast radius:
  - CLI JSON additions for governance health checks and confirmation requirements.
  - Text health rendering only if structured details are printed for human output.
  - One generated skill template and its tests.
- Constraints:
  - Preserve existing JSON fields and meanings; add optional fields rather than renaming current fields.
  - Keep recompute behavior from current `cmd/health_test.go` coverage; do not replace the snapshot cache design unless tests prove it stale.
  - Keep `--resume-response` scoped to real `active_checkpoint` state.

### Patterns
- Existing conventions:
  - Command JSON structs use optional `omitempty` fields for additive diagnostics, for example `healthView.Observations`, `healthView.Diagnostics`, and `nextView.Warnings`. Evidence: `cmd/health.go:21`, `cmd/next.go:16`.
  - Governance details commonly use existing model structs where possible; `TraceabilityGap` is already part of `TraceabilitySummary`. Evidence: `internal/model/governance_snapshot.go:16`, `internal/model/traceability.go:33`.
  - Tests exercise command JSON by unmarshalling into command view structs and asserting specific diagnostic fields. Evidence: `cmd/health_test.go:1032`, `cmd/progression_next_test.go:1939`.
  - Template tests render `goal-verification` directly and assert text contracts. Evidence: `internal/tmpl/templates_test.go:512`.
- Reusable abstractions:
  - Use `model.TraceabilityGap` as the structured details payload for `traceability_coherence`, avoiding a duplicate schema.
  - Extend `confirmationRequirement` with optional action metadata rather than inventing a separate checkpoint state.
  - Replace `grep -Pzo` with a Perl one-liner; Perl is available on the target macOS environment and already matches the reporter's successful workaround.
- Convention deviations:
  - Adding optional fields to `confirmation_requirement` and governance checks is an external JSON surface change, but it is additive and directly tied to `external_api_contracts` acceptance.

### Risks
- Technical risks:
  - Medium: exposing full traceability gap structs may make health JSON larger. Mitigation: only include details on non-OK traceability checks.
  - Medium: changing confirmation semantics could break tests or callers expecting only five fields. Mitigation: additive `next_action` / `resume_response_supported` fields; keep existing fields stable.
  - Low: replacing the placeholder scan with Perl changes copy, not runtime behavior. Mitigation: template tests assert absence of `grep -P`.
- Guardrail domains:
  - `external_api_contracts` is touched because machine-readable `health` and `next/run` JSON surfaces change.
- Reversibility:
  - All changes are local to command JSON structs, governance health formatting, tests, and templates. Rollback is a standard git revert with no data migration.

### Test Strategy
- Existing coverage:
  - Health recomputation tests already cover stale persisted snapshots and material artifact changes. Evidence: `cmd/health_test.go:1032`, `cmd/health_test.go:1133`, `cmd/health_test.go:1234`.
  - Confirmation requirement tests currently assert skill handoff hard stops but no actionable resume metadata. Evidence: `cmd/progression_next_test.go:1939`, `cmd/progression_next_test.go:1970`.
  - Resume-response rejection without checkpoint is already covered. Evidence: `cmd/progression_next_test.go:1588`.
  - Template rendering tests already cover goal-verification content. Evidence: `internal/tmpl/templates_test.go:512`.
- Infrastructure needs:
  - Add/extend command-level JSON tests in `cmd/health_test.go` and `cmd/progression_next_test.go`.
  - Add/extend template tests in `internal/tmpl/templates_test.go`.
- Verification approach:
  - `#59`: assert `traceability_coherence` JSON includes gap IDs/types/issues for a failing bundle; preserve existing recompute tests.
  - `#61`: assert skill handoff confirmation includes an explicit non-checkpoint next action and `resume_response_supported=false`; assert resume checkpoint remains `true`.
  - `#62`: assert rendered goal-verification template does not contain `grep -P` and does contain the portable Perl scan.

## Alternatives Considered

- Approach 1: Add optional structured details/action metadata and keep existing runtime behavior.
  - Tradeoffs: minimal blast radius, additive JSON compatibility, directly addresses operator ambiguity; does not redesign all command verdict semantics.
- Approach 2: Replace governance snapshot caching with always-live health evaluation and change skill handoffs from `hard_stop` to a new boundary.
  - Tradeoffs: stronger semantic cleanup, but higher risk to performance and external JSON consumers; likely overbroad for the three issues.
- Approach 3: Implement checkpoint creation for every skill handoff and allow `--resume-response` to advance through host skills.
  - Tradeoffs: resolves `#61` literally but conflates human/agent skill handoffs with execution checkpoints and would expand the state machine.
- Selected: Approach 1. It fixes the observed operator problems with the smallest external-contract change: health exposes the already-computed traceability gap identities, confirmation output tells callers whether `--resume-response` is supported and what action to take, and goal-verification uses a portable scan command.


## Unknowns
- Resolved: health snapshot owner -> `cmd/health.go` recomputes snapshots under the change lock and feeds `CollectGovernanceHealthWithSnapshot`; current tests already cover material artifact changes.
- Resolved: traceability gap source -> `EvaluateTraceability` already computes `TraceabilitySummary.Gaps`, so the missing behavior is surfacing, not detection.
- Resolved: confirmation output owner -> `deriveConfirmationRequirement` owns skill-handoff hard-stop metadata, while `buildResumeCheckpoint` owns real checkpoint resume support.
- Resolved: GNU-only scan source -> `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl`.
- Remaining: None for planning.


## Assumptions
- Adding optional JSON fields is acceptable because current consumers can ignore unknown fields and the issue explicitly asks for more actionable diagnostics. Evidence: current command view structs already use optional diagnostic fields (`cmd/health.go:21`, `cmd/next.go:16`).
- We should not replace the health cache wholesale because current tests already assert recomputation when material state changes (`cmd/health_test.go:1032`, `cmd/health_test.go:1234`).
- Perl is an acceptable portable fallback for macOS agents because the filed issue's successful workaround used Perl and the target environment is macOS.


## Canonical References
- `artifacts/changes/resolve-open-github-issues-59-61-and-62-align-governance-hea/intent.md` for the original request and intake context.
- `requirements.md` and `decision.md` in the same bundle once planning artifacts are refined.
- Existing code paths and tests related to the affected behavior in the repository.
- `cmd/health.go:128` for governance health command flow.
- `internal/engine/governance/health.go:17` for `GovernanceHealthCheck`.
- `internal/engine/governance/traceability.go:258` for existing gap storage on `TraceabilitySummary`.
- `cmd/next.go:56` and `cmd/next.go:594` for confirmation requirement shape and derivation.
- `cmd/next_context_build.go:300` for checkpoint resume projection.
- `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl:56` for placeholder scan instructions.
