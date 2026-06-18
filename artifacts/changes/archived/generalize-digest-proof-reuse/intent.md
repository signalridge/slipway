# Intent

## Summary
Generalize digest-keyed proof reuse so governed verification can reuse fresh
full-suite and SAST proof instead of rerunning unchanged proof across S4 stages.

## Complexity Assessment
complex
Rationale: this changes engine-owned proof-reuse validation and generated
governance skill posture. The change must preserve fail-closed freshness,
run-version, digest, and guardrail safety behavior while reducing redundant
verification work.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Issue #258: generalize the existing closeout -> goal-verification reuse
  validation into a reusable digest/run-version proof-reuse mechanism.
- Preserve the existing closeout reuse safety checks while making reuse-first
  behavior clearer for final-closeout when goal-verification proof is fresh.
- Evaluate execution-summary -> goal-verification full-suite reuse when review
  and content state have not changed after execution proof.
- Update generated skill templates and engine tests so agents cite validated
  proof reuse instead of rerunning full-suite/SAST proof by default when the
  engine can prove freshness.

## Out of Scope
- Host-native subagent dispatch overhead and #240/#114 execution isolation work.
- Removing S4 full-suite or SAST proof unconditionally.
- Copying GSD-core architecture wholesale; use it only as comparison context.
- Docs-only Diataxis work (#170), install transaction/manifest work (#167),
  install profile/router work (#168), and Go test-lint analyzers (#161).

## Constraints
- Fail closed on any stale run version, changed content, missing suite-result,
  missing guardrail SAST proof, or unverifiable digest.
- Keep exactly one canonical fresh proof path per safe cycle; do not weaken the
  ship gate or high-risk safety checks.
- Prefer the current worktree's Slipway behavior and bounded codebase-map refs
  over remembered workflow assumptions.

## Acceptance Signals
- Engine tests cover valid reuse plus invalidation by changed source/artifacts,
  stale run_version, missing suite-result, and guardrail SAST mismatch.
- Generated goal-verification/final-closeout skill text makes validated reuse the
  preferred path when conditions hold and rerun the fallback when they do not.
- `go test ./internal/engine/wave ./internal/engine/progression -count=1` passes.
- The selected implementation still blocks ship if reuse evidence is missing,
  stale, or inconsistent.

## Open Questions
- [x] Determine the smallest reusable proof-reuse API shape that avoids another
  closeout-specific special case while preserving existing blocker taxonomy.
  Research resolution: extract the existing closeout validation shape into a
  small reusable validator while keeping closeout's public blocker for the
  existing path.
- [x] Confirm whether execution-summary -> goal-verification reuse is safe in
  the first slice or should be deferred behind closeout reuse-first behavior.
  Research resolution: do not make goal-verification unconditionally reuse S2
  proof; include upstream reuse only behind an explicit, test-proven
  no-source/content-delta predicate.

## Deferred Ideas
- Broader proof reuse for coverage, mutation, and reviewer spot-reruns after the
  suite/SAST path is proven.
- Token-budget and install-surface work tracked by #168.

## Approved Summary
Confirmed by user on 2026-06-18T06:21:12Z with "继续帮我推进".

Implement the highest-ROI open issue, #258, by generalizing Slipway's
digest-keyed proof reuse beyond the current closeout -> goal-verification
special case. The change should preserve fail-closed run-version, digest,
suite-result, and guardrail SAST validation while reducing redundant full-suite
and SAST reruns across S4. The initial scope excludes subagent dispatch work,
unconditional S4 proof skipping, direct GSD-core architecture copying, docs
reorganization, install profile/router work, install transaction/manifest work,
and test-lint analyzer work. Success is proven by targeted engine/template tests
and `go test ./internal/engine/wave ./internal/engine/progression -count=1`,
with ship still blocked whenever reuse evidence is stale, missing, or
inconsistent.
