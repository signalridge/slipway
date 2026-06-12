# Decision

## Alternatives Considered

- Normalize current `change.yaml` before S4 goal/closeout content hashing.
  - Pros: fixes the root self-stale loop at the input semantics boundary,
    preserves the public `evidence skill` write order, and keeps meaningful
    authority fields covered.
  - Cons: introduces a special structured hash for the current change authority
    instead of raw bytes.
- Save `EvidenceRefs` before stamping the digest.
  - Pros: the digest would see the post-pointer `change.yaml`.
  - Cons: expands command transaction complexity across verification YAML,
    digest state, lifecycle events, and `change.yaml`; it solves this case by
    ordering rather than by defining which authority content matters.
- Add a public restamp command for this Tier-0 stale evidence case.
  - Pros: gives operators a recovery escape hatch.
  - Cons: leaves the default S4 recovery path capable of looping and broadens
    command surface beyond the reported root cause.

## Selected Approach

Use the first approach. `goal-verification` and `final-closeout` digest input
generation now detects the current
`artifacts/changes/<slug>/change.yaml`, strict-decodes it as `model.Change`,
clears `EvidenceRefs`, and computes a structured input hash. All other content
paths keep the existing raw file or directory hash behavior.

This selection matches the issue's expected behavior: evidence-ref-only
`change.yaml` mutations do not stale S4 evidence, but meaningful authority
changes still do.

## Interfaces and Data Flow

- Public CLI interfaces: none changed.
- Data flow before:
  - `slipway evidence skill` stamps a digest over raw `change.yaml` bytes.
  - The same command writes `EvidenceRefs` into `change.yaml`.
  - The next freshness check sees different raw bytes and reports stale.
- Data flow after:
  - S4 goal/closeout digest construction routes the current `change.yaml` path
    through `changeAuthorityInputHash`.
  - `changeAuthorityInputHash` validates the authority, clears `EvidenceRefs`,
    and hashes the structured change.
  - The next freshness check sees the same structured authority for
    evidence-ref-only mutations, but different hashes for non-pointer authority
    changes.

## Rollout and Rollback

- Rollout:
  - Source-only Go change in `internal/engine/progression/evidence_digests.go`.
  - Regression coverage in
    `internal/engine/progression/evidence_digests_test.go`.
- Rollback:
  - Revert the helper and regression test changes.
  - Re-run `go test -count=1 ./internal/engine/progression` to confirm the
    previous behavior is restored if rollback is required.

## Risk

- Over-normalization risk is controlled by clearing only `EvidenceRefs`.
- Path matching risk is controlled by requiring the exact current
  `artifacts/changes/<slug>/change.yaml` path.
- Malformed `change.yaml` now fails during digest input construction for this
  path; this is fail-closed and consistent with authority validation.
- Existing stale digest records that used raw `change.yaml` bytes may need normal
  evidence re-recording once; no migration is required.
