# Decision

## Alternatives Considered

- Strip only HTML comments before hashing prose artifacts. This is safe but too
  narrow: scaffold-only headings and known default bodies can still churn a
  digest even though they are engine-owned structure.
- Add a prose artifact material-view digest. This strips comments, ignores empty
  scaffold sections, recognizes only narrow known defaults, and includes all
  unknown non-empty prose. This matches the GSD invariant without adding a GSD
  dependency.
- Build a full artifact AST and overwrite framework. This is broader than issue
  #155 needs and would change authoring semantics outside digest freshness.

Status: accepted.

## Selected Approach

Implement the prose artifact material-view digest in
`internal/engine/progression/evidence_digests.go` and keep stale recovery
unchanged. The digest layer is the right boundary because stale recovery already
operates on named digest drift; changing the recovery layer would only hide a
bad input classification.

The selected approach is Option 2 from `research.md`: suppress only
engine-owned scaffold/default noise, and treat unknown non-empty prose as
material.

## Interfaces and Data Flow

No public CLI or JSON interface changes. Internal data flow changes from:

`prose file bytes -> model.ComputeInputHash({"content": raw})`

to:

`prose file bytes -> material prose view -> model.ComputeInputHash(...)`.

`tasks.md` remains on `wave.TaskPlanStructuralHash`; `assurance.md` remains
excluded from plan-audit digest inputs.

## Rollout and Rollback

Rollout is covered by focused unit tests in
`internal/engine/progression/evidence_digests_test.go`, followed by
`go test -count=1 ./internal/engine/progression` and `go test -count=1 ./...`.

Rollback is a normal git revert of the helper and test additions. Existing
verification records remain governed by recomputed current digests on the next
validation run.

## Risk

Primary risk is a false negative that accepts stale evidence after a real prose
edit. The implementation mitigates this by including unknown non-empty prose in
the material view and recognizing only narrow defaults. Secondary risk is
over-normalizing whitespace; the implementation should avoid broad prose
rewrites beyond comments, empty scaffold sections, and known defaults.
