# Structure

Re-authored for change
`resolve-github-issue-185-prevent-s4-goal-verification-from-s`
(GitHub issue #185).

- `cmd/`
  - `evidence.go`: public `slipway evidence skill` command. It remains
    unchanged; it demonstrates why digest stamping happens before
    `EvidenceRefs` are written.
- `internal/engine/progression/`
  - `evidence_digests.go`: owned implementation surface for #185. S4
    goal/closeout digest helpers live here.
  - `evidence_digests_test.go`: focused regression location for the self-stale
    and meaningful-change cases.
  - `authority.go`: source of the changed/target paths reused by
    goal-verification and final-closeout.
- `internal/model/`
  - `change.go`: `Change.EvidenceRefs` is the engine-owned runtime pointer map
    stored in `change.yaml`.
  - `evidence.go`: `ComputeInputHash` provides canonical structured hashing.
- `artifacts/changes/resolve-github-issue-185-prevent-s4-goal-verification-from-s/`
  - Governed artifact bundle for #185.
