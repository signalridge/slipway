# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `cmd/run.go:216-280` owns the `slipway run` governed loop and its stop predicate.
  - `cmd/stage.go:64-90` threads config-level `execution.auto` into stage commands, and `cmd/stage.go:130-157` mirrors the run-loop stop behavior for `intake`, `plan`, and `implement`.
  - `cmd/next.go:794-835` derives the machine-readable confirmation boundary, including command-required, hard-stop, and evidence-continuation outcomes.
  - `cmd/next.go:1111-1128` defines auto standing authorization for pure-pacing handoffs without weakening evidence requirements.
  - `internal/engine/progression/confirmation_boundaries.go:130-175` owns the pure-pacing allowlist and fail-closed manual-boundary rule.
  - `cmd/evidence.go:120-235` owns CLI-stamped skill evidence recording; `cmd/evidence.go:1541-1565` enforces fresh S3 review context-origin evidence.
  - `cmd/done.go:294-350` owns final archive and rechecks done-ready ship authority before marking a change done.
- Dependency chain:
  - `run/stage` -> `buildNextViewForCommand` -> `advanceIfReadyAuto` / `tryAdvance` -> progression engine -> `deriveConfirmationRequirement` -> handoff JSON.
  - `next` is preview-only; it can report auto-continuable boundaries but must not mutate state.
  - `evidence skill` remains the only authoritative path for skill verdict timestamps, run versions, and digest stamps.
- Blast radius:
  - Narrow: run/stage stop policy, confirmation boundary interpretation, handoff/readme/docs/tests.
  - Not in scope: embedded subagent execution, evidence synthesis, policy control selection, or done archival.
- Constraints:
  - Auto may only continue when the current view proves no fresh confirmation is required and no non-pacing blocker is present.
  - The run loop must still stop at `done_ready`, `DONE`, unsupported/noop, stale evidence, unknown freshness, missing evidence, security-review, guardrail/sensitive domains, and explicit command finalization.

### Patterns
- Existing conventions:
  - `run` and stage commands already collect `AutoTransitions`, so any additional auto continuation should reuse this trace rather than inventing a second transition history (`cmd/run.go:239-260`, `cmd/stage.go:133-155`).
  - `deriveConfirmationRequirement` is the canonical classifier for whether a boundary is hard stop, command-required, or evidence continuation (`cmd/next.go:794-835`).
  - Pure-pacing eligibility is centralized in progression helpers, not duplicated in command code (`internal/engine/progression/confirmation_boundaries.go:146-174`).
  - Evidence is recorded through `slipway evidence skill`, not by hand-editing YAML (`cmd/evidence.go:120-235`).
- Reusable abstraction:
  - Add a small helper around `confirmationRequirement` / view state to decide whether the loop may continue under auto.
  - Keep `shouldStopRunLoop` safe for manual mode, but allow `auto=true` to continue through command-required or pure-pacing evidence-continuation boundaries that are already authorized.
- Convention deviations:
  - None required. The change can stay in existing command/progression tests and docs.

### Risks
- Technical risks:
  - High: accidentally auto-crossing a hard gate if the continuation predicate is too broad.
  - Medium: infinite or noisy loop if command-required/noop conditions are misclassified.
  - Medium: JSON contract drift if `next` and `run` disagree about confirmation requirements.
  - Low: docs/templates drifting from command behavior.
- Guardrail domains:
  - None directly touched by this change, but guardrail fail-closed behavior is a required invariant.
- Reversibility:
  - Reversible with code/test/doc rollback. No schema migration or durable state format change is required.

### Test Strategy
- Existing coverage:
  - Auto confirmation and hard-stop contracts live in `cmd/auto_mode_test.go`.
  - Run transition trace coverage exists in `cmd/progression_next_test.go`.
  - Progression auto-confirm and evidence-gate safety live in `internal/engine/progression/advance_governed_test.go`.
- Infrastructure needs:
  - Add focused fixtures for auto-loop continuation over `run_slipway_run_to_advance` and pure-pacing evidence-continuation boundaries.
  - Add negative fixtures for no-auto, hard-stop, next-skill missing evidence, security-review, guardrail, stale/unknown blockers, and done-ready.
- Verification approach:
  - Focused `go test ./cmd` tests covering run/stage loop behavior and handoff contract.
  - Focused `go test ./internal/engine/progression` tests preserving allowlist/manual-boundary semantics.
  - Package-level tests for touched packages before final review.

### Options
- Option 1: Keep current behavior and update docs only.
  - Tradeoff: safest but fails the user-visible goal; auto remains too weak.
- Option 2: Add full embedded autopilot inside Slipway.
  - Tradeoff: would execute skills/subagents and record evidence from inside the CLI, but conflicts with current host-delegation and evidence-authority design.
- Option 3: Implement bounded auto-to-next-real-gate.
  - Tradeoff: improves usefulness by continuing over already-authorized routine boundaries, while preserving hard stops and leaving execution/evidence to host skills.
- Selected: Option 3.
  - Rationale: it matches the approved direction, uses existing `confirmation_requirement` semantics, preserves fail-closed boundaries, and keeps Slipway as lifecycle authority rather than an executor.

## Unknowns
- Resolved: Whether `command-auto` catalog bindings imply automatic skill execution -> no; they attach hints/capabilities and do not replace `ResolveNextSkill` or execute a skill.
- Resolved: Whether `run --auto` should finalize `done` -> no; `cmd/done.go:294-350` rechecks ship authority and records `operator_finalized_done_ready`, so final archive remains explicit.
- Remaining: None.

## Assumptions
- `confirmation_requirement.prior_authorization_sufficient=true` is the safest source for detecting auto-continuable pure-pacing boundaries. Evidence: `cmd/next.go:1111-1128`.
- Existing non-pacing blocker classification remains authoritative for hard-stop safety. Evidence: `cmd/next.go:809-828`.
- The public command contract should remain additive: update docs/templates without removing existing fields. Evidence: existing handoff is built from `cmd/next_handoff.go` and status/run tests already rely on additive JSON fields.

## Canonical References
- `cmd/run.go:216-280`
- `cmd/stage.go:64-90`
- `cmd/stage.go:130-157`
- `cmd/next.go:794-835`
- `cmd/next.go:1111-1128`
- `internal/engine/progression/confirmation_boundaries.go:130-175`
- `cmd/evidence.go:120-235`
- `cmd/evidence.go:1541-1565`
- `cmd/done.go:294-350`
- `artifacts/changes/enhance-execution-auto-mode/intent.md`
