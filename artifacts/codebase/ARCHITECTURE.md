# Architecture

Re-authored for change
`resolve-github-issue-185-prevent-s4-goal-verification-from-s`
(GitHub issue #185).

Question: where should S4 governance-skill digest logic ignore
evidence-ref-only `change.yaml` mutations without weakening stale detection for
real authority changes?

- Entry point:
  - `cmd/evidence.go:151` through `cmd/evidence.go:218` records passing skill
    evidence. It checks digest inputs, writes verification YAML, stamps
    `evidence-digests.yaml`, then records the skill pointer in
    `change.EvidenceRefs` and saves `change.yaml`.
- Digest authority:
  - `internal/engine/progression/evidence_digests.go:36` through
    `internal/engine/progression/evidence_digests.go:84` centralizes
    skill-specific input digest construction.
  - `internal/engine/progression/evidence_digests.go:539` through
    `internal/engine/progression/evidence_digests.go:564` is the
    `goal-verification` branch; `final-closeout` reuses it before adding
    assurance input.
  - `internal/engine/progression/evidence_digests.go:497` through
    `internal/engine/progression/evidence_digests.go:536` hashes task changed
    and target paths reused for S4 verification.
- Content path source:
  - `internal/engine/progression/authority.go:361` through
    `internal/engine/progression/authority.go:382` collects changed and target
    files from execution-summary tasks and only skips verification-directory
    paths. The current change `change.yaml` is therefore a valid S4 input.
- Fix boundary:
  - Keep command sequencing unchanged.
  - Special-case only the current `artifacts/changes/<slug>/change.yaml` input
    while building S4 goal/closeout content hashes.
  - Strict-decode the authority as `model.Change`, clear `EvidenceRefs`, and use
    a structured `model.ComputeInputHash` payload.
- Blast radius:
  - `internal/engine/progression/evidence_digests.go`
  - `internal/engine/progression/evidence_digests_test.go`
  - No public CLI syntax, generated command surface, model schema, or Lattice
    artifact changes are required.
