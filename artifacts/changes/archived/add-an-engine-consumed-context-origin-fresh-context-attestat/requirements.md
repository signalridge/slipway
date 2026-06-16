# Requirements

## Requirements

### Requirement: Always-on cross-stage chain-ordering gate (P1)

REQ-001: On the `standard` and `strict` presets the engine MUST consume the
previously engine-ignored final-closeout independence attestation by both (a)
REQUIRING the `closeout:reviewer_independence=pass` token to be present on the
final-closeout record (Pattern-A presence, analogue of
`closeout:assurance_complete=pass`) and (b) enforcing an always-on
chain-ordering invariant `closeout ≥ goal-verification ≥
max(spec-compliance-review, code-quality-review)` across the four
independence-critical verdicts, independent of the opt-in
`closeout:goal_verification_reuse=pass` token. Each facet MUST fail closed with a
DISTINCT new reason code (the ordering code SHALL NOT be folded into the existing
`closeout_goal_verification_reuse_invalid`) and MUST be advisory (non-blocking)
on `light`.

#### Scenario: In-order, attested chain passes on strict

GIVEN a strict change whose spec-compliance-review and code-quality-review
verdicts are stamped before goal-verification, which is stamped before
final-closeout, and final-closeout carries `closeout:reviewer_independence=pass`
WHEN the engine evaluates ship authority
THEN neither the presence facet nor the chain-ordering facet reports a blocker
and ship authority is not withheld on their account.

#### Scenario: Missing reviewer_independence presence fails closed on standard

GIVEN a standard change whose final-closeout record omits
`closeout:reviewer_independence=pass`
WHEN the engine evaluates ship authority
THEN a blocker carrying the new presence reason code is surfaced at error
severity with a remediation that names re-running final-closeout.

#### Scenario: Out-of-order chain fails closed on standard

GIVEN a standard change where goal-verification predates the latest review
verdict, or final-closeout predates goal-verification
WHEN the engine evaluates ship authority
THEN a blocker carrying the new distinct chain-ordering reason code is surfaced
at error severity in the ship gate, and it is NOT the
`closeout_goal_verification_reuse_invalid` code.

#### Scenario: Ordering is advisory on light

GIVEN a light change with the same out-of-order chain
WHEN the engine evaluates ship authority
THEN the chain-ordering finding is advisory only and does not block ship.

#### Scenario: Gate is independent of the reuse opt-in token

GIVEN a standard change that did NOT emit `closeout:goal_verification_reuse=pass`
WHEN the engine evaluates ship authority on an out-of-order chain
THEN the chain-ordering blocker still fires (the gate no longer early-returns
behind the reuse opt-in guard).

### Requirement: Distinct-context handles for the review pair (P2)

REQ-002: On `standard` and `strict` the engine MUST require, for the
spec-compliance-review / code-quality-review pair, two recorded review-context
handles expressed in a NEW pure `internal/model` reference grammar, consumed in
the review authority evaluation. The two handles MUST be present and MUST be
distinct from each other; a missing handle on either review, or two identical
handles across the pair, MUST fail closed at error severity. The gate MUST be
advisory on `light`. The recorded handle SHALL serve as the per-review
context identifier, subsuming any separate `context_origin` token (no second
token is introduced). The two reviews are unordered peers and the handle gate
MUST NOT impose an ordering between them.

#### Scenario: Two distinct review handles pass on strict

GIVEN a strict change where spec-compliance-review and code-quality-review each
recorded a distinct review-context handle
WHEN the engine evaluates review authority
THEN the distinct-handle gate reports no blocker.

#### Scenario: Identical handles across the pair fail closed on standard

GIVEN a standard change where spec-compliance-review and code-quality-review
recorded the same review-context handle value
WHEN the engine evaluates review authority
THEN a blocker with the new review-context reason code is surfaced at error
severity with a remediation that names how to re-record distinct review-context
evidence.

#### Scenario: Missing handle fails closed on strict

GIVEN a strict change where one of the two reviews recorded no review-context
handle
WHEN the engine evaluates review authority
THEN the gate fails closed at error severity for the review missing its handle.

#### Scenario: Handle gate is advisory on light

GIVEN a light change with identical or missing review-context handles
WHEN the engine evaluates review authority
THEN the finding is advisory only and does not block.

### Requirement: Relational test≠impl distinctness on engine-owned structure (P4 #5)

REQ-003: For tasks that share `target_files`, the engine MUST enforce that a
`task_kind=test` task is structurally distinct from and dispatched before its
dependent `task_kind=code` task, deriving the distinctness solely from
engine-owned `task_kind` + `target_files` structure. The gate MUST NOT key on
the host-supplied `session_id` (which is host-set and excluded on the empty
default and is therefore an invalid discriminator). The gate MUST fail closed on
`standard`/`strict` and be advisory on `light`.

#### Scenario: Test-before-code on shared files passes

GIVEN a change whose wave plan dispatches a `task_kind=test` task before the
`task_kind=code` task that shares the same `target_files`
WHEN the engine synchronizes governed wave execution
THEN no test/impl-distinctness blocker is raised.

#### Scenario: Code without a preceding distinct test fails closed on strict

GIVEN a strict change where a `task_kind=code` task shares `target_files` with no
distinct preceding `task_kind=test` task
WHEN the engine synchronizes governed wave execution
THEN a blocker with the new test/impl-distinctness reason code is surfaced at
error severity.

#### Scenario: session_id is never the discriminator

GIVEN a change where two tasks share `target_files` and carry empty (default) or
two distinct host-chosen `session_id` values
WHEN the engine evaluates the test/impl-distinctness gate
THEN the verdict is unchanged by the `session_id` values (the gate is decided
only by `task_kind` + `target_files`).

### Requirement: Attempt-based degraded_sequential acceptance (P4 #6)

REQ-004: The engine MUST reject a bare self-asserted `degraded_sequential`
dispatch token in `DispatchEvidenceBlockers`; it SHALL be accepted only when
paired with a genuine tool-unavailable justification carried by a NEW additive
`References` justification grammar (not a new struct field). The tightened gate
MUST fail closed on `standard`/`strict`, be advisory on `light`, and MUST fire on
every site that synchronizes governed wave execution, including the
`evidence skill` path (which now also synchronizes wave execution), not only the
advance/next path.

#### Scenario: Bare degraded_sequential is rejected on standard

GIVEN a standard change whose wave evidence asserts `degraded_sequential` with no
tool-unavailable justification reference
WHEN the engine evaluates dispatch evidence
THEN a blocker with the new degraded-justification reason code is surfaced at
error severity.

#### Scenario: Justified degraded_sequential is accepted

GIVEN a change whose wave evidence pairs `degraded_sequential` with the genuine
tool-unavailable justification reference token
WHEN the engine evaluates dispatch evidence
THEN no degraded-justification blocker is raised.

#### Scenario: Gate fires on the evidence-skill path

GIVEN a change recording skill evidence via `slipway evidence skill` with a bare
`degraded_sequential` claim
WHEN that command synchronizes governed wave execution
THEN the degraded-justification blocker fires on the evidence-skill path, not
only on advance/next.

### Requirement: Reason-code and recovery contract for every new blocker

REQ-005: Every new blocker introduced by REQ-001..REQ-004 MUST register a
distinct canonical reason code across the three-file reason-code contract
(`reason_code.go` definitions map, `reason_code_contract_test.go` snapshot list
and severity map, and `recovery.go` `blockerRemediations`), so that no code
silently downgrades to `unknown_reason_code`. Each reason code MUST carry an
actionable public remediation that names the owning skill or command a host runs
to recover, and the recovery completeness and `.Message`-prose lint tests MUST
remain green.

#### Scenario: New codes are registered and contract tests pass

GIVEN the new reason codes for the chain-ordering, review-context,
test/impl-distinctness, and degraded-justification blockers
WHEN the reason-code contract and recovery completeness tests run
THEN every new code appears in the snapshot with its declared severity and a
non-empty actionable remediation, and no code resolves to
`unknown_reason_code`.

#### Scenario: Remediation names the recovery path

GIVEN a blocker raised for any new gate
WHEN a host reads the blocker remediation from the public surface
THEN the remediation names the exact governed skill or command to re-enter, not
a private sequencing step.

### Requirement: Generated host surfaces emit and explain the new tokens

REQ-006: The generated host skills, thin-host content, toolgen surfaces, and docs
MUST emit and document the new attestation, review-context handle, and degraded
justification tokens for the affected skills, and the toolgen / thin-host /
template token contract tests MUST remain green. No host surface MUST instruct a
host to self-stamp a freshness or final verdict outside the engine.

#### Scenario: Templated skills carry the new tokens

GIVEN the generated skill templates for the review, verification, closeout, and
wave-orchestration surfaces
WHEN the template token contract tests run
THEN each new emitted token has a literal-token assertion and the suite is green.

#### Scenario: Docs explain the new contract

GIVEN the updated docs and thin-host content
WHEN a reader inspects them
THEN they describe what each new token attests, on which presets it is enforced,
and how to recover when a gate fails closed.

### Requirement: Fail-closed discipline, honest scoping, and dogfood (constraints + P3)

REQ-007: The change MUST NOT introduce any bypass, force-close, or
private/self-stamped attestation path; the engine MUST remain the sole inline
verdict-stamping authority and the new grammar MUST stay pure in
`internal/model` (no `internal/model` or `internal/state` import of
`cmd`/`tmpl`/`toolgen`). The design MUST honestly document the residual — that
genuine non-forgeable distinct-context discrimination (an engine-issued
per-stage nonce / lifecycle-event boundary, "Option B") is infeasible within
this change's constraints — so no gate is oversold as cryptographic proof. The
change MUST ship through its own strict governed flow with fresh dogfood
evidence, and `go test ./...`, `gofmt -s -l`, and golangci-lint MUST be clean.

#### Scenario: No bypass path is added

GIVEN the full diff of this change
WHEN it is reviewed against the fail-closed safety contract
THEN no new flag, env var, or token allows skipping, force-closing, or
self-stamping any new gate, and the engine remains the only Timestamp/RunVersion
stamper.

#### Scenario: Residual is documented, not oversold

GIVEN the shipped artifacts and docs
WHEN a reader inspects the claims made for the new gates
THEN the audit/structural tier of the handle gate and the infeasibility of the
nonce-based Option B are stated explicitly, with no claim of cryptographic
distinct-context proof.

#### Scenario: Change ships through its own strict flow

GIVEN this change running under the strict preset
WHEN it reaches ship readiness
THEN it satisfies its own new gates with fresh dogfood evidence and the full Go
test suite, `gofmt -s -l`, and golangci-lint are clean.
