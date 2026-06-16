# Research

## Alternatives Considered

### Architecture
- Affected modules: `internal/engine/progression/evidence_digests.go`,
  `internal/engine/progression/stale_evidence_recovery.go`,
  `internal/stringutil/html.go`, and
  `internal/engine/progression/stale_evidence_recovery_test.go`.
- Dependency chains: `StaleEvidenceRecoveryAvailable` checks prior authority
  freshness through `skillDigestFreshnessBlockers`, which recomputes the
  skill-specific digest via `certifiedSkillInputDigest`. Intake and research
  both previously consumed `intent.md`, but research routing itself is driven
  by the Open Questions checklist parser in `stringutil.HasBlockingOpenQuestions`.
- Blast radius: limited to governance skill digest inputs for
  `intake-clarification`; research-orchestration, plan-audit, wave execution,
  review, and closeout inputs keep their existing behavior.
- Constraints: engine-owned verification/freshness state must remain
  fail-closed for substantive intent changes; no bypass or force-close path is
  allowed.

### Patterns
- Existing conventions: prose artifacts are hashed structurally through
  `computeProseFileInputHash`, which normalizes line endings, strips HTML
  comments, and hashes material markdown sections instead of raw mtime.
- Reusable abstractions: `proseArtifactMaterialSections` already yields
  section-level material, so an intake-specific section filter can reuse the
  same canonical hash pipeline.
- Convention deviations: no new external metadata field is needed; the change
  adds a narrow helper for intake evidence rather than changing the generic
  governed file hash.

### Risks
- Technical risks: medium if the filter is too broad because real intake scope
  changes could stop staling evidence; low with an Open Questions-only filter
  and paired negative regression coverage.
- Guardrail domains: governance lifecycle correctness and evidence freshness.
- Reversibility: high; the implementation is a small helper and one call-site
  change, with a targeted regression test.

### Test Strategy
- Existing coverage: `evidence_digests_test.go` already covers skill digest
  input boundaries, and `stale_evidence_recovery_test.go` covers S0 stale
  intake recovery.
- Infrastructure needs: no new test framework or clock manipulation; fixture
  writes an `intent.md`, stamps intake evidence through existing helpers, then
  queries `StaleEvidenceRecoveryAvailable`.
- Verification approach: first reproduce the issue by showing Open Questions
  resolution incorrectly produces an S0 stale recovery target; then assert the
  fixed path no longer returns a target while a substantive Summary edit still
  returns `required_skill_stale:intake-clarification:intent.md`.

### Options
- Option A: add an intake-specific `intent.md` digest input that reuses the
  existing prose hash and excludes only the `Open Questions` section. Tradeoff:
  keeps the change small and preserves fail-closed behavior for every other
  section, but leaves future research-owned fields to be handled explicitly if
  they appear.
- Option B: normalize only Open Questions checkbox markers before hashing.
  Tradeoff: preserves more of the Open Questions prose in intake digest, but it
  does not satisfy issue #238's resolution-note case.
- Option C: move Open Questions into a separate research-owned artifact or add
  lifecycle ownership metadata for individual intent sections. Tradeoff:
  cleaner ownership model long-term, but broadens the change into artifact
  schema/lifecycle redesign.
- Selected: Option A is recommended because it fixes issue #238 exactly at the
  skill input boundary, keeps research and plan-audit full-file semantics, and
  preserves fail-closed stale evidence recovery for substantive intake changes.

## Unknowns
- Resolved: Where is the narrowest authority boundary? -> The skill input
  mapping in `certifiedSkillInputDigest`; intake-clarification can use a
  narrower `intent.md` digest while downstream research and planning still
  consume full governed artifacts.
- Resolved: Which fixture reproduces issue #238? ->
  `StaleEvidenceRecoveryAvailable` with stamped intake-clarification evidence
  directly exercises the S0 reopen target and avoids brittle wall-clock
  assertions.
- Remaining: None. User confirmed Option A after reviewing the research
  alternatives.

## Assumptions
- The Open Questions section is research-routing state, not substantive intake
  scope. Evidence: `stringutil.HasBlockingOpenQuestions` gates only unchecked
  checklist entries under `## Open Questions`, while the intake confirmation
  checks are separate section-content checks.
- Substantive intake edits must remain stale-evidence triggers. Evidence: the
  regression test pairs the Open Questions resolution path with a Summary edit
  that still returns the S0 intake stale target.
- The existing codebase map is semantically stale for this issue because it was
  authored for a prior closeout-attestation change; current planning authority
  comes from direct code inspection and the issue-specific regression.

## Canonical References
- `internal/engine/progression/evidence_digests.go:36`
- `internal/engine/progression/evidence_digests.go:48`
- `internal/engine/progression/evidence_digests.go:52`
- `internal/engine/progression/evidence_digests.go:440`
- `internal/engine/progression/evidence_digests.go:462`
- `internal/engine/progression/stale_evidence_recovery.go:47`
- `internal/engine/progression/stale_evidence_recovery.go:79`
- `internal/stringutil/html.go:66`
- `internal/engine/progression/advance_intake.go:356`
- `internal/engine/progression/stale_evidence_recovery_test.go:132`
