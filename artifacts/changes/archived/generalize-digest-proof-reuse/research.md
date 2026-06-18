# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `internal/engine/progression/authority.go`: ship authority wires
    `closeoutGoalVerificationReuseBlockers` into both verification blockers and
    `G_ship` unresolved reasons; the current validator is closeout-only and
    consumes final-closeout references.
  - `internal/engine/progression/evidence_digests.go`: goal-verification digest
    inputs already include suite-result, planning artifacts, task-plan scope,
    changed/target file set, and changed/target file content.
  - `internal/model/evidence_digests.go`: `SuiteResult` stores
    `run_summary_version`, `full_suite_digest`, and optional `sast_digests`;
    `SharedReviewerInputDigests` converts them into named digest inputs.
  - `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl` and
    `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl`: host-facing
    proof production/reuse guidance.
- Dependency chains:
  - S2 wave evidence -> `execution-summary.yaml` -> S3 selected review freshness.
  - Goal-verification -> `suite-result.yaml` plus
    `high_risk_check:<domain>.safety_baseline` when guardrail domains apply.
  - Final-closeout -> optional closeout reference tokens ->
    `closeoutGoalVerificationReuseBlockers` -> `G_ship`.
- Blast radius:
  - Narrow engine blast radius in `internal/engine/progression`.
  - Public evidence/reference vocabulary only if new reuse tokens are introduced.
  - Generated skill text must stay aligned with the engine contract.
- Constraints:
  - Engine remains the only verdict/freshness stamper.
  - Reuse must fail closed on stale run versions, changed content, missing
    suite-result proof, missing SAST digest, or digest-evaluation errors.
  - Do not weaken `high_risk_check:<domain>.safety_baseline`.

### Patterns
- Existing conventions:
  - Gate-specific validation returns `[]model.ReasonCode` and is appended to both
    verification blockers and ship/readiness blockers when the reason must be
    actionable at `G_ship`.
  - Verification proof freshness is represented as named digest inputs rather
    than timestamp-only checks.
  - Host skill records proof claims through `References`; the CLI owns timestamp
    and run-version stamping.
- Reusable abstractions:
  - Extract the closeout reuse run-version/digest validation shape into a helper
    that accepts source stage, consumer stage, run-version reference policy, and
    digest-freshness checks.
  - Keep `SuiteResult` as the suite/SAST proof carrier instead of creating a new
    proof artifact.
- Convention deviations:
  - A fully generic proof-reuse registry would be a new abstraction. It should be
    small and locally justified, not a broad scheduler redesign.

### Risks
- High: false reuse could skip required full-suite or SAST proof. Mitigation:
  preserve run-version equality, suite-result validation, changed-content digest
  inputs, and guardrail SAST digest checks.
- Medium: new public blocker vocabulary could bypass existing recovery contracts.
  Mitigation: reuse existing closeout blocker for the existing path; add new
  reason-code/recovery entries only when a new public failure class is required.
- Medium: execution-summary -> goal-verification reuse could undercut
  goal-verification's current producer role for `suite-result.yaml`. Mitigation:
  treat this as a guarded second consumer unless the first slice proves an
  explicit no-source/content-delta predicate.
- Low: codebase map was semantically stale for #258 because it was authored for
  #240. Mitigation: refreshed `ARCHITECTURE.md`, `TESTING.md`, and
  `CONCERNS.md` with proof-reuse-specific findings.
- Guardrail domains: none for the change itself, but the implementation must
  preserve guardrail-domain SAST reuse semantics for future governed changes.
- Reversibility: source changes are local to progression/model/template tests and
  can be rolled back by reverting the change; no data migration or irreversible
  operation is involved.

### Test Strategy
- Existing coverage:
  - `TestCloseoutGoalVerificationReuseBlockers` covers valid closeout reuse and
    invalidation by run-version mismatch, missing reuse run version, newer
    execution evidence, changed content, and stale execution-summary freshness.
  - `TestBuildShipAuthoritySurfacesCloseoutReuseBlocker` covers ship-gate
    surfacing.
  - `TestGoalVerificationDigestStalesWhenSharedSuiteInputsChange` covers
    suite-result digest invalidation.
- Infrastructure needs:
  - Reuse existing digest fixtures and `writeSuiteResultForDigestTest`.
  - Add SAST digest fixture coverage where guardrail-domain suite-result reuse is
    expected.
- Verification approach:
  - Focused engine tests for valid and invalid reuse.
  - Template/toolgen tests for generated skill guidance.
  - `go test ./internal/engine/wave ./internal/engine/progression -count=1`
    as the minimum focused command before completion claims.

### Options
- Approach A: Extract a small reusable proof-reuse validator from the existing
  closeout gate, keep the existing closeout reference tokens for the first
  consumer, make final-closeout reuse-first when validation passes, and add
  targeted tests. Tradeoff: highest safety and fastest ROI, but execution ->
  goal-verification reuse may remain limited until a follow-up slice. Dialectical
  concern: if the extraction is only a renamed closeout helper, it satisfies the
  local cleanup but not the deeper issue #258 root cause that proof reuse is a
  hand-built one-off.
- Approach B: Build a registry of reusable proof types and stage edges in one
  pass, including execution -> goal-verification suite reuse. Tradeoff: stronger
  long-term shape, but higher blast radius and more public vocabulary before the
  first consumer is proven. Dialectical concern: it may invent a framework before
  the second edge has enough empirical constraints, and it risks weakening
  goal-verification's current suite-result producer role.
- Approach D: Internal edge-spec validator with one enabled edge first. Extract a
  real reusable validator around an internal `proofReuseEdge`/`proofReuseCheck`
  shape that names the source skill, consumer skill, run-version source,
  execution-summary freshness requirement, digest checks, and blocker factory.
  Keep `closeoutGoalVerificationReuseBlockers` as a compatibility wrapper and
  keep the existing closeout public reference tokens for the first consumer.
  Rename closeout-only content/path helpers to proof-reuse-neutral helpers where
  they are genuinely shared. Add tests that prove the edge-spec validator is not
  hardcoded to closeout, but only enable the current closeout -> goal-verification
  edge in production. Record execution-summary -> goal-verification as a
  follow-up-capable edge only when a no-source/content-delta predicate and
  suite-result producer semantics are proven.
  Tradeoff: slightly more design than Approach A, far less blast radius than
  Approach B. It gives #258 a real generalized primitive without exposing a broad
  public registry or changing goal-verification semantics prematurely.
- Approach C: Template-only reuse-first posture for final-closeout, with no
  engine generalization. Tradeoff: lowest implementation cost, but it leaves the
  one-off root cause in place and does not solve #258's generalization ask.
- Selected: Approach D, confirmed by user on 2026-06-18T06:43:40Z. This keeps
  Approach A's safety and first-slice discipline while adopting only the useful
  part of Approach B: a real internal edge abstraction that can host later
  proof-reuse consumers without committing to a premature public registry or
  weakening goal-verification.

## Unknowns
- Resolved: Determine the smallest reusable proof-reuse API shape that avoids
  another closeout-specific special case while preserving existing blocker
  taxonomy -> use an internal edge-spec validator over the existing run-version,
  execution-summary freshness, suite-result, and digest-freshness checks; keep
  closeout's public blocker and reference tokens for the existing path.
- Resolved: Confirm whether execution-summary -> goal-verification reuse is safe
  in the first slice or should be deferred behind closeout reuse-first behavior
  -> first slice should not make goal-verification unconditionally reuse S2 proof,
  because goal-verification is currently the suite-result producer. Add only a
  test-proven, explicit no-source/content-delta predicate if included.
- Remaining: None.

## Assumptions
- Issue #258 remains the implementation target. Evidence: live issue triage and
  approved intake summary in `artifacts/changes/generalize-digest-proof-reuse/intent.md`.
- The current codebase map was stale for this scope and needed inline refresh.
  Evidence: `artifacts/codebase/ARCHITECTURE.md`, `TESTING.md`, and
  `CONCERNS.md` were authored for #240 and now include #258-specific sections.
- GSD-core is comparison context, not an implementation source for this change.
  Evidence: local `open-gsd/gsd-core` docs describe thin orchestrator and
  subagent context isolation, while #258 is a Slipway proof-reuse issue inside
  existing verification gates.

## Canonical References
- `internal/engine/progression/authority.go:252`
- `internal/engine/progression/authority.go:313`
- `internal/engine/progression/authority.go:394`
- `internal/engine/progression/authority.go:434`
- `internal/engine/progression/authority.go:941`
- `internal/engine/progression/evidence_digests.go:796`
- `internal/engine/progression/evidence_digests.go:851`
- `internal/model/evidence_digests.go:25`
- `internal/model/evidence_digests.go:125`
- `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl:33`
- `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl:45`
- `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl:74`
- `internal/engine/progression/authority_test.go:202`
- `internal/engine/progression/authority_test.go:312`
- `internal/engine/progression/evidence_digests_test.go:1004`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/docs/explanation/multi-agent-orchestration.md:21`
