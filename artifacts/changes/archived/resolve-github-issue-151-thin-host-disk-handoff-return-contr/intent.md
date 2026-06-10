# Intent

## Summary
Resolve GitHub issue #151: thin-host disk-handoff return contract for heavy stages, with CLI-owned ingest/stamping and measurable host-context reduction
## Complexity Assessment
complex

Issue #151 affects governed host instructions, evidence ingestion semantics,
and fail-closed freshness guarantees across heavy lifecycle stages. The change
is not sensitive-domain work, but it touches AI-facing workflow contracts and
must keep engine-owned stamping authority intact.

## Guardrail Domains
None detected.

## In Scope
- Resolve the remaining issue #151 scope for heavy stages not already covered
  by issue #114: `research-orchestration`, `plan-audit`,
  `intake-clarification`, `spec-compliance-review`, and
  `code-quality-review`.
- Introduce or extend a disk-handoff contract where a subagent writes bulky
  artifacts under `artifacts/changes/<slug>/` and returns only a short
  confirmation to the thin host.
- Keep evidence freshness, run versions, timestamps, and pass/fail stamping
  owned by Slipway CLI ingestion/verification flows rather than by subagent
  self-attestation.
- Pass heavy context by paths or required-reading references instead of
  inlining large artifact bodies where the stage can operate through a
  disk-handoff.
- Use the local `ghq` checkout of `gsd` as an implementation reference during
  coding, without copying its workflow wholesale.
- Add focused regression coverage proving the contract wording and
  fail-closed ingest/stamping boundary.

## Out of Scope
- Reworking stages already covered by issue #114 unless needed for shared
  consistency.
- Copying GSD command or agent prompts directly into Slipway.
- Replacing the existing governed lifecycle, stage model, or freshness engine.
- Running `slipway done`; this goal stops at `done-ready`.

## Constraints
- Source-of-truth edits for generated skill surfaces belong under
  `internal/tmpl/templates/skills/`.
- Host-facing text must remain thin and path-oriented; detailed per-stage
  instructions should stay in adjacent references or generated command/skill
  surfaces.
- The plan does not need a separate GSD research artifact; the local GSD
  checkout is an implementation reference.
- Any evidence ingest/stamp path must fail closed when evidence is stale,
  forged, or missing CLI-owned freshness fields.

## Acceptance Signals
- At least one remaining heavy stage is refactored to an explicit
  disk-handoff thin-host contract, with tests pinning the contract.
- The relevant host context is measurably reduced or bounded by path references
  instead of pasted artifact bodies.
- Regression coverage proves subagent confirmation is only a claim and that
  CLI-owned ingestion/verification remains the evidence authority.
- Fresh `go test -count=1 ./...` passes.
- Fresh `go run . validate --json` and lifecycle checks advance the governed
  change to `done_ready`.

## Open Questions
None.

## Deferred Ideas
- Generalized multi-stage digest surfaces beyond the minimum needed to resolve
  issue #151.
- Broad host-token telemetry outside the changed stage contracts.

## Approved Summary
Confirmed by the user objective on 2026-06-10: use the governed Slipway flow to
resolve all problems in GitHub issue #151 through `done-ready`; when blocked,
make the best scope-preserving choice; reference the local `ghq` checkout of
`gsd` during implementation, while keeping the governed plan independent of a
separate GSD planning exercise.
