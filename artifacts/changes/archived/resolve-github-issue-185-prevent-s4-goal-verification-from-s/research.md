# Research

Question: how should Slipway prevent S4 `goal-verification` and
`final-closeout` evidence from self-staling when recording evidence pointers
mutates the same `change.yaml` file that the skill digest includes?

## Alternatives Considered

### Architecture

- Affected modules:
  - `cmd/evidence.go:151` through `cmd/evidence.go:218` validates digest
    inputs, writes verification YAML, stamps `evidence-digests.yaml`, then writes
    `EvidenceRefs` into `change.yaml`.
  - `internal/engine/progression/evidence_digests.go:36` through
    `internal/engine/progression/evidence_digests.go:84` dispatches
    engine-owned digest inputs by governance skill.
  - `internal/engine/progression/evidence_digests.go:497` through
    `internal/engine/progression/evidence_digests.go:564` builds S4
    `goal-verification` inputs from execution-summary changed and target files.
  - `internal/engine/progression/authority.go:361` through
    `internal/engine/progression/authority.go:382` feeds S4 digest content paths
    from task `ChangedFiles` and `TargetFiles`.
  - `internal/engine/progression/evidence_digests.go:829` through
    `internal/engine/progression/evidence_digests.go:869` is the generic file
    and directory content hashing path.
- Dependency chain:
  - `slipway evidence skill` -> `CheckEvidenceDigestInputsForSkill` ->
    `certifiedSkillInputDigest`.
  - `slipway evidence skill` -> `StampEvidenceDigestForSkill` ->
    `state.SaveEvidenceDigests`.
  - `slipway evidence skill` then calls `change.RecordEvidenceRef` and
    `state.SaveChange`, mutating `change.yaml` after the digest has been
    stamped.
- Blast radius:
  - S4 digest inputs for `goal-verification` and `final-closeout`.
  - Existing review, plan-audit, wave, and generic digest paths should remain
    unchanged.
- Constraints:
  - Verification records and freshness state must remain engine-owned.
  - Meaningful `change.yaml` authority changes must still stale required skills.
  - Verification directory paths are already skipped by S4 content path reuse;
    the issue is the active `change.yaml` authority itself.

### Patterns

- Existing conventions:
  - Digest input construction is centralized in
    `certifiedSkillInputDigest`, with one branch per governance skill.
  - File inputs normally use raw content hashes, and special cases are kept close
    to the digest helper rather than in command code.
  - `change.yaml` is strict-decoded elsewhere with known fields before being
    treated as authority.
- Reusable abstractions:
  - `model.ComputeInputHash` provides canonical hashing over structured payloads.
  - `model.Change.Normalize` and `model.Change.Validate` preserve existing
    authority validation semantics.
- Convention deviations:
  - The current change authority needs a structured hash instead of raw bytes,
    but only for S4 goal/closeout content hashing and only with `EvidenceRefs`
    cleared.

### Risks

- Technical risks:
  - High: normalizing too much of `change.yaml` could hide real lifecycle or
    scope drift.
  - Medium: parsing `change.yaml` during digest creation could expose malformed
    authority earlier than raw hashing did; this is acceptable because malformed
    authority should fail closed.
  - Low: digest format changes for the current `change.yaml` input can stale
    already-stamped evidence once, but future stamps become stable across
    evidence-ref-only mutations.
- Guardrail domains:
  - None of the explicit sensitive domains apply. The change is governance
    engine behavior, not auth, credentials, PII, financial flow, schema
    migration, irreversible operations, or external API contracts.
- Reversibility:
  - The code change is source-only and can be reverted. No migration of existing
    evidence files is required.

### Test Strategy

- Existing coverage:
  - `internal/engine/progression/evidence_digests_test.go:809` through
    `internal/engine/progression/evidence_digests_test.go:831` proves
    `goal-verification` stales when a normal target file changes.
  - Existing S4 digest tests cover execution-summary metadata exclusion,
    deleted target files, assurance handling, and review digest stale reopen
    behavior.
- Infrastructure needs:
  - No new fixtures are needed beyond the existing `createReviewInputDigestFixture`
    and `digestPolicyExecutionSummary` helpers.
- Verification approach:
  - Add a regression where the execution summary target is
    `artifacts/changes/<slug>/change.yaml`.
  - Stamp `goal-verification` and `final-closeout`.
  - Mutate only `EvidenceRefs` and prove no stale blocker is emitted.
  - Mutate a meaningful authority field and prove the stale blocker still
    appears.
  - Run the focused test, the full progression package, then repo-wide checks.

### Options

- Option 1: Normalize the current change authority before S4 content hashing.
  - Design: detect `artifacts/changes/<slug>/change.yaml`, strict-decode it as
    `model.Change`, clear `EvidenceRefs`, then compute a structured input hash.
  - Tradeoffs: narrow root-cause fix; preserves stale checks for all non-pointer
    fields; changes digest semantics only for the current change authority in
    S4 goal/closeout hashing.
- Option 2: Change `slipway evidence skill` write order so evidence refs are
  saved before the digest is stamped.
  - Tradeoffs: may require rollback choreography across verification YAML,
    digest store, lifecycle event, and `change.yaml`; makes digest stability
    depend on command persistence sequencing rather than input semantics.
- Option 3: Add a supported restamp/recovery command for this Tier-0 case.
  - Tradeoffs: provides an escape hatch but leaves the default public recovery
    path looping and broadens the command surface beyond the reported root cause.
- Selected: Option 1.
  - Rationale: the issue is caused by engine-owned `EvidenceRefs` churn, not by
    operator-authored content changing. Normalizing only that field at the
    digest-input boundary keeps the command flow and fail-closed stale checks
    intact.

## Unknowns

- Resolved: which function owns skill digest inputs for `change.yaml`? ->
  `certifiedSkillInputDigest` routes `goal-verification` and `final-closeout`
  through `addGoalVerificationInputs` and `addContentPathInputs`.
- Resolved: should the fix normalize evidence-ref-only mutations or stamp after
  evidence pointer update? -> normalize the current `change.yaml` input with
  `EvidenceRefs` cleared; do not alter `evidence skill` persistence ordering.
- Remaining: None.

## Assumptions

- The reported Lattice loop is represented by an execution summary whose changed
  or target file list contains `artifacts/changes/<slug>/change.yaml` -
  Evidence: issue #185 body and `authority.go:361` through `authority.go:382`.
- `EvidenceRefs` is engine-owned runtime pointer state, not the substantive
  authority being certified by `goal-verification` - Evidence:
  `cmd/evidence.go:216` through `cmd/evidence.go:218` writes it after
  verification/digest stamping.
- Meaningful changes to other `change.yaml` fields should remain digest inputs -
  Evidence: the new regression mutates `Description` and still expects
  `required_skill_stale`.

## Canonical References

- `https://github.com/signalridge/slipway/issues/185`
- `cmd/evidence.go:151`
- `cmd/evidence.go:181`
- `cmd/evidence.go:193`
- `cmd/evidence.go:216`
- `internal/engine/progression/evidence_digests.go:36`
- `internal/engine/progression/evidence_digests.go:497`
- `internal/engine/progression/evidence_digests.go:539`
- `internal/engine/progression/evidence_digests.go:871`
- `internal/engine/progression/authority.go:361`
- `internal/engine/progression/evidence_digests_test.go:833`
