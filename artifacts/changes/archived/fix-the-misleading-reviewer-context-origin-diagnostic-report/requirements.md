# Requirements

## Requirements

### Requirement: Reviewer handle resolves despite multiple fix handles
REQ-001: The system MUST resolve the unique `context_origin:stage=review=<handle>`
reference on a reviewer record even when that same record also carries multiple
distinct `context_origin:stage=fix=<handle>` references. The presence of more than
one `stage=fix` handle MUST NOT cause the whole-record context-origin parse to fail
closed and mask the review handle.

#### Scenario: review handle reads through coexisting multi-fix handles
GIVEN a passing reviewer verification record whose references include
`context_origin:stage=review=ctx-review` plus
`context_origin:stage=fix=ctx-fix-1` and `context_origin:stage=fix=ctx-fix-2`
WHEN `ReviewContextOriginHandleFromVerification` parses that record
THEN it returns ok=true with the resolved handle `ctx-review`.

#### Scenario: record with only multiple fix handles has no review handle
GIVEN a passing reviewer record whose only context-origin references are
`context_origin:stage=fix=ctx-fix-1` and `context_origin:stage=fix=ctx-fix-2`
WHEN `ReviewContextOriginHandleFromVerification` parses that record
THEN it returns ok=false because no `stage=review` handle is present, without
failing closed on the fix multiplicity.

### Requirement: Single-valued stages remain fail-closed
REQ-002: The system MUST keep the single-valued context-origin stages —
`review`, `plan_origin`, and `audit_origin` — fail-closed: two references naming
the same single-valued stage with two different handles MUST be rejected as
ambiguous evidence rather than letting one reference win. This change MUST NOT
introduce any attestation bypass, force-close, restamp, or private-attestation
path for any stage.

#### Scenario: conflicting review handles still fail closed
GIVEN a reviewer record carrying
`context_origin:stage=review=ctx-a` and `context_origin:stage=review=ctx-b`
WHEN the context-origin handles are parsed
THEN the parse fails closed (ok=false) and no review handle is resolved.

#### Scenario: conflicting plan/audit-origin handles still fail closed
GIVEN a record carrying two different `plan_origin:<handle>` references (or two
different `audit_origin:<handle>` references)
WHEN `PlanOriginHandleFromVerification` (respectively
`AuditOriginHandleFromVerification`) parses it
THEN the parse fails closed (ok=false), unchanged from current behavior.

### Requirement: Fix stage exposes a complete handle set
REQ-003: The system MUST expose every distinct recorded `context_origin:stage=fix`
handle on a record as a deduplicated set with set semantics, so the multi-valued
`fix` stage contributes its full handle set to the cross-stage independence
lattice. Repeated identical fix handles MUST collapse to one set member.

#### Scenario: all distinct fix handles are extracted
GIVEN a record carrying `context_origin:stage=fix=ctx-fix-1`,
`context_origin:stage=fix=ctx-fix-2`, and a repeated
`context_origin:stage=fix=ctx-fix-1`
WHEN the fix handle set is extracted from that record
THEN the result is the deduplicated set {ctx-fix-1, ctx-fix-2}.

#### Scenario: a record with no fix handles yields an empty set
GIVEN a record whose references carry no `context_origin:stage=fix` token
WHEN the fix handle set is extracted
THEN the result is an empty set (not a parse failure).

### Requirement: Authority layer stops the false reviewer-missing blocker for the multi-fix shape
REQ-004: The cross-stage context-participants builder MUST NOT emit the
`context_origin_handle_invalid` "recorded no context-origin handle for selected
reviewer" blocker for a passing reviewer record that carries a valid review handle
alongside multiple fix handles. It MUST collect every recorded `stage=fix` handle
across the selected reviewers into the `fix` participant `HandleSet`.

#### Scenario: multi-fix reviewer evidence is not false-flagged and fix set is complete
GIVEN a selected reviewer with a passing record carrying one valid review handle
and two distinct fix handles
WHEN `crossStageContextParticipants` builds the lattice participants
THEN no `context_origin_handle_invalid` reviewer-missing blocker is returned for
that reviewer, and the `fix` participant `HandleSet` contains both fix handles.

#### Scenario: end-to-end validate no longer reports the misleading diagnostic
GIVEN reviewer evidence with multiple `stage=fix` handles plus one `stage=review`
handle for a selected reviewer
WHEN `slipway validate` evaluates the change
THEN it does not report that the selected reviewer recorded no context-origin
handle and does not instruct the user to re-run the reviewer in a fresh subagent.

### Requirement: Fix command instructions match the accepted evidence shape
REQ-005: The generated `slipway fix` command surface MUST make explicit that a
reviewer's evidence may accumulate multiple `context_origin:stage=fix` handles —
one per fresh-context repair subagent / batch — and the template contract test
MUST pin that wording.

#### Scenario: fix command body documents multiple fix handles
GIVEN the rendered `command-fix-body` surface
WHEN its content is inspected
THEN it states that a reviewer may record more than one
`context_origin:stage=fix=<handle>` (one per repair subagent / batch), and the
existing reexecution-mode contract assertions still hold.

### Requirement: Full verification from the current worktree
REQ-006: The change MUST keep the full Go test suite (`go test ./...`) and the
project lint gate (golangci-lint, including gofmt simplify) green from the current
worktree, with new regression coverage for the multi-fix behavior added rather
than relaxing the existing single-valued fail-closed tests.

#### Scenario: suite and lint pass with new coverage
GIVEN the implemented change in the current worktree
WHEN `go test ./...` and the lint gate are run
THEN both pass, and the previously existing same-stage fail-closed test cases
remain present and green.
