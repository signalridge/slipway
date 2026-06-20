# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `cmd/run.go` resolves effective auto mode and injects the
    `autoAcknowledgedResponse` sentinel for eligible checkpoints.
  - `cmd/next_context_build.go` carries `resumeResponse` into
    `resumeCheckpoint.UserResponsePayload` and marks active checkpoints for
    consumption.
  - `cmd/next.go` consumes active checkpoints and emits `checkpoint.resolved`
    lifecycle events; it also derives auto-softened confirmation requirements.
  - `cmd/learn.go` aggregates lifecycle `checkpoint.resolved` events into
    checkpoint resolution signals.
  - `internal/engine/progression/confirmation_boundaries.go` owns the current
    skill boundary helper used by `cmd/next.go`.
  - `internal/toolgen/toolgen.go`, `internal/tmpl/templates/_partials`, and
    `README.md` own the safety instruction surfaces that need regression pins.
- Dependency chains:
  - `run` -> `autoAckResumeResponse` -> `buildNextViewForCommand` ->
    `buildResumeCheckpoint` -> `consumeNextCheckpoint` -> lifecycle event log.
  - `next` / stage commands -> `deriveConfirmationRequirement` ->
    progression skill boundary helpers.
  - `learn` -> `state.ReadLifecycleEvents` -> aggregate signals from event
    fields and side effects.
- Blast radius:
  - Runtime behavior is limited to auto-mode checkpoint event metadata and
    skill handoff confirmation softening.
  - Manual `--resume-response` behavior must remain unchanged.
  - Evidence gates and guardrail hard stops must remain independent of auto mode.
- Constraints:
  - The lifecycle event log is append-only and already supports
    `ActorKind`, `Reason`, `Diagnostics`, and `SideEffects`, so no schema
    migration is needed.
  - The skill boundary helper already centralizes related logic and can expose a
    safe allowlist without changing `cmd/next.go`'s public JSON contract.

### Patterns
- Existing conventions:
  - Auto preset confirmation records a distinguishable lifecycle reason and
    side effect: `auto_preset_confirmed`.
  - Review companion handoff helpers use explicit allowlists rather than default
    permissive matching.
  - Tests in `cmd/auto_mode_test.go` use direct `nextView` fixtures for
    confirmation boundary decisions and command fixtures for real `run` paths.
  - Toolgen/template tests assert safety text with `assert.Contains` on rendered
    command entries and generated prompt content.
- Reusable abstractions:
  - Reuse lifecycle `SideEffects` for an `auto_checkpoint_acknowledged` marker.
  - Add a helper that detects auto checkpoint acknowledgments from the
    `resumeCheckpoint.UserResponsePayload`; this keeps event attribution close
    to the consumption path without widening public structs.
  - Replace `SkillRequiresManualAutoBoundary` with a pure-pacing allowlist
    helper and make manual-boundary semantics the negation of the allowlist.
- Convention deviations:
  - `ActorKind` should stay `cli` to preserve command attribution; the
    distinguishable audit surface can live in `Reason` and `SideEffects`.

### Risks
- Technical risks:
  - Medium: changing skill auto-softening polarity can make currently softened
    non-allowlisted skills hard-stop. This is intended fail-closed behavior, but
    tests must pin the expected allowlist.
  - Low: adding an event side effect could affect timeline consumers that compare
    exact event JSON. Existing event consumers should ignore unknown side effect
    kinds or benefit from the marker.
  - Low: README/run surface tests can become brittle if they assert paragraphs
    instead of stable redline phrases.
- Guardrail domains:
  - No domain such as auth, credentials, schema migration, or irreversible ops is
    modified. This is a governance-control hardening change.
- Reversibility:
  - Changes are local and reversible by removing helper changes and tests.
    Event schema remains compatible because it uses existing fields.

### Test Strategy
- Existing coverage:
  - `cmd/auto_mode_test.go` already covers auto flag resolution, guardrail
    hard stops, security-review hard stops, and the real auto checkpoint run
    path.
  - `internal/toolgen/toolgen_test.go` and `internal/tmpl/templates_test.go`
    cover parts of generated safety text, but not the run/README redlines now in
    scope.
- Infrastructure needs:
  - Use existing temp workspace and command fixtures from `cmd/auto_mode_test.go`.
  - Use existing rendered command and prompt helpers for toolgen/template checks.
  - No external services, mocks, or new test framework needed.
- Verification approach:
  - Add a command-path test that auto-acknowledged checkpoints emit an
    auto-specific lifecycle side effect/reason and that manual responses do not.
  - Add confirmation-boundary tests showing pure-pacing allowlisted skills soften
    under auto and unknown skills hard-stop.
  - Add C1 regression coverage for auto-off plus non-pacing blocker over a
    skill/review handoff.
  - Add README/run redline text assertions using stable phrases.

### Options
- Option 1: Minimal metadata marker only.
  - Add `auto_checkpoint_acknowledged` side effect to checkpoint resolution
    events and leave skill-boundary polarity unchanged.
  - Tradeoff: fixes M1 only and leaves the future fail-open skill concern.
- Option 2: Focused hardening of existing seams.
  - Add auto checkpoint audit markers, make `learn` distinguish manual from
    auto checkpoint resolutions, replace skill softening with an explicit
    pure-pacing allowlist, and add redline/blocker tests.
  - Tradeoff: slightly changes future default behavior for unlisted skills, but
    that is the intended fail-closed posture.
- Option 3: Broader auto-mode observability expansion.
  - Add top-level JSON effective-auto fields and session-start audit events in
    addition to Option 2.
  - Tradeoff: larger public API surface and broader compatibility concerns than
    needed for the confirmed findings.
- Selected: Option 2. It repairs M1, M2, M3, and C1 directly while preserving
  C2 and avoiding speculative JSON/API expansion.

## Unknowns
- Resolved: whether `artifacts/codebase` is relevant to this change -> it is
  populated but stale for this scope; it describes an adapter/toolgen change, so
  current-source citations are used instead.
- Resolved: whether C2 requires changes -> no; existing `security-review`
  hard-stop behavior is correct and remains out of scope.
- Remaining: None.

## Assumptions
- Existing lifecycle event fields are sufficient for audit distinction.
  Evidence: `cmd/next.go` already emits `SideEffects` for checkpoint clearing,
  and `advance_governed.go` uses side effects for auto preset confirmation.
- Manual checkpoint resolution should keep the current event shape except for
  unaffected existing fields. Evidence: manual and auto paths both flow through
  `consumeNextCheckpoint`; the fix can branch only when the payload equals the
  internal auto sentinel.
- Unknown or unclassified skills should hard-stop under auto until explicitly
  allowlisted as pure pacing. Evidence: other auto-mode boundaries fail closed
  for guardrail domains, stale checkpoints, decision/human_action checkpoints,
  and evidence gates.

## Canonical References
- `cmd/run.go:137-189`
- `cmd/next_context_build.go:320-329`
- `cmd/next.go:506-535`
- `cmd/next.go:690-735`
- `cmd/next.go:784-804`
- `cmd/learn.go:27-56`
- `cmd/learn.go:204-208`
- `cmd/learn.go:382-388`
- `internal/engine/progression/confirmation_boundaries.go:47-67`
- `internal/toolgen/toolgen.go:48-52`
- `internal/toolgen/toolgen.go:295-300`
- `README.md:111-131`
- `cmd/auto_mode_test.go:266-445`
- `cmd/auto_mode_test.go:635-657`
- `internal/toolgen/toolgen_test.go:1714-1768`
- `internal/tmpl/templates_test.go:1182-1237`
