# Concerns

Re-authored for change
`resolve-github-issue-185-prevent-s4-goal-verification-from-s`
(GitHub issue #185).

- Load-bearing invariant: required skill evidence must stale when certified
  inputs materially change, but must not stale itself because the engine records
  where that same evidence lives.
- Self-stale risk: `cmd/evidence.go` stamps the digest before it writes the
  evidence pointer to `change.yaml`; raw-byte hashing of current `change.yaml`
  therefore makes a freshly-recorded S4 skill stale immediately.
- Over-normalization risk: clearing more than `EvidenceRefs` could hide real
  lifecycle, scope, preset, or artifact-state changes. The fix must clear only
  `EvidenceRefs`.
- Path scoping risk: a generic `change.yaml` filename elsewhere in the repo must
  still use raw content hashing. The special case applies only to
  `artifacts/changes/<current-slug>/change.yaml`.
- Corruption risk: strict-decoding the current change authority during hashing
  means malformed `change.yaml` fails the digest path. That is acceptable because
  malformed authority should block governed progression.
- Compatibility risk: old stored digests that used raw `change.yaml` bytes may
  stale once after this change. Restamping through the normal evidence path is
  acceptable; no migration or manual evidence editing is required.
