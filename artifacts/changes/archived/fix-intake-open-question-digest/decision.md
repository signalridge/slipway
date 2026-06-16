# Decision

## Alternatives Considered

- Option A: Add an `intake-clarification`-specific `intent.md` digest helper
  that reuses the existing prose artifact hash but excludes only the
  `Open Questions` section. This directly reflects ownership: intake owns
  clarified scope text; research owns Open Questions resolution state.
- Option B: Normalize only Open Questions checkbox markers before hashing.
  This would fix `- [ ]` to `- [x]` churn but not resolution notes, which issue
  #238 explicitly expects to be safe.
- Option C: Move Open Questions into a separate research-owned artifact or add
  section-level ownership metadata. This could be cleaner long-term, but it is a
  broader artifact-schema and lifecycle redesign than the issue requires.

## Selected Approach

Select Option A. `certifiedSkillInputDigest` routes only
`intake-clarification` through an intake-specific `intent.md` helper that
filters out `Open Questions`; `research-orchestration` and `plan-audit` keep the
existing full governed file input. This is the smallest change that satisfies
REQ-001 while preserving REQ-002 and REQ-003.

## Interfaces and Data Flow

No public CLI arguments, YAML fields, or verification record schemas change.
The internal data flow changes as follows:

`StaleEvidenceRecoveryAvailable` -> `skillDigestFreshnessBlockers` ->
`certifiedSkillInputDigest` -> skill-specific `intent.md` hash.

For `intake-clarification`, the hash input excludes the `Open Questions`
section before `model.ComputeInputHash`. For research and plan-audit, existing
full artifact hashing remains unchanged.

## Rollout and Rollback

Rollout is a normal code/test change in the Slipway CLI. Verification is:

- `go test ./internal/engine/progression -run TestStaleEvidenceRecoveryIgnoresIntakeOpenQuestionResolution -count=1`
- `go test ./internal/engine/progression -count=1`
- `go test ./...`

Rollback is a revert of the helper and test changes. The rollback verification
command is `go test ./internal/engine/progression -count=1`; however, reverting
would intentionally reintroduce issue #238.

## Risk

- If the filtered section is broader than `Open Questions`, real intake scope
  changes could stop staling evidence. The implementation limits filtering to
  an exact heading match and tests a Summary edit negative path.
- If downstream skills accidentally share the intake-specific helper, research
  or plan-audit could miss material planning changes. A digest boundary test
  covers this by asserting downstream digests still change on Open Questions
  edits.
- Existing active changes with old full-file intake digests may need one public
  lifecycle recovery/re-stamp after this change lands, because historical digest
  values were computed with the older input definition.
