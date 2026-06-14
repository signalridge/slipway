# Decision

## Alternatives Considered

### Option A: Structured Satisfied-By Attribution

Add an optional attribution field to `governance.RequiredAction`, populate it from the runtime evidence that satisfied the action, and map it through `governanceActionView`.

Tradeoffs:

- Pros: machine-readable, shared by status/validate/next, and keeps the policy decision in the engine layer.
- Cons: expands the required-action JSON shape and needs regression coverage for stale-evidence behavior.

### Option B: Description-Only Explanation

Append text such as "satisfied by spec-compliance-review" to the action description when `domain-review` is satisfied.

Tradeoffs:

- Pros: smaller code change.
- Cons: forces JSON consumers to parse prose, mixes remediation text with proof, and does not create a stable contract.

### Option C: Separate Domain Review Evidence

Require `domain-review.yaml` in addition to `spec-compliance-review.yaml`.

Tradeoffs:

- Pros: creates an obvious standalone evidence file.
- Cons: changes current policy semantics and adds workflow friction beyond issue #203, which asks for explanation if the existing mapping is intended.

## Selected Approach

Use Option A. `spec-compliance-review` remains the evidence source for `domain-review`, but the satisfied action will carry structured attribution that names the satisfying skill and reason. This fixes the black-box traceability gap without requiring a new evidence file.

## Interfaces and Data Flow

- Add an optional `SatisfiedBy` field to `internal/engine/governance.RequiredAction`.
- Add an attribution input to `RequiredActionsInput` for review controls.
- Extend runtime verification satisfaction to return attribution only when the evidence is passing, current, and otherwise acceptable.
- Map the engine attribution through `cmd/governance_surface.go` into `governanceActionView`.
- Expose the resulting field in command JSON as an additive field.

## Rollout and Rollback

Rollout:

- Implement the additive field and focused tests.
- Verify with `go test -count=1 ./internal/engine/governance ./cmd`.
- Continue governed review and closeout before final completion.

Rollback:

- Revert the additive fields, mapping, and tests. Existing `description` and `satisfied` fields remain unchanged, so rollback does not require data migration.

## Risk

- JSON compatibility risk is controlled by only adding optional fields.
- Review readiness risk is controlled by deriving attribution from the same readiness checks that already set `Satisfied`.
- Surface consistency risk is controlled by using the shared command surface mapper rather than patching individual commands.
